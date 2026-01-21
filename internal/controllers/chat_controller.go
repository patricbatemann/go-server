package controllers

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go-server/internal/models"
	"go-server/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"go-server/internal/database"
)

type ChatController struct {
	geminiService *services.GeminiService
}

func NewChatController(gs *services.GeminiService) *ChatController {
	return &ChatController{geminiService: gs}
}

func (c *ChatController) Upload(ctx *gin.Context) {
	file, err := ctx.FormFile("file")
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "No file uploaded"})
		return
	}

	tempDir := os.TempDir()
	tempFilePath := filepath.Join(tempDir, file.Filename)
	
	if err := ctx.SaveUploadedFile(file, tempFilePath); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file"})
		return
	}
	defer os.Remove(tempFilePath) 

	uploadResult, err := c.geminiService.UploadFile(ctx.Request.Context(), tempFilePath)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to upload to Gemini: %v", err)})
		return
	}

	ctx.JSON(http.StatusOK, models.UploadResponse{
		Name:  uploadResult.Name,
		URI:   uploadResult.URI,
		State: string(uploadResult.State),
	})
}

func (c *ChatController) Chat(ctx *gin.Context) {
	var req models.ChatRequest
	// We bind JSON but also check logic later
	// Note: ShouldBindJSON might consume body, verify if we can stream response 
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userIDVal, exists := ctx.Get("user_id")
	if !exists {
		// Even for authorized routes, good to be safe
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userID := userIDVal.(int64)
	dbClient := database.GetClient()

	var conversationID uuid.UUID
	var err error

	// Handle Conversation Logic
	if req.ConversationID == "" {
		// improvements: Generate a title from prompt? For now: "New Chat"
		conversationID = uuid.New()
		title, err := c.geminiService.GenerateTitle(ctx.Request.Context(), req.Prompt)
		if err != nil {
			fmt.Printf("Failed to generate title: %v\n", err)
			// Continue without saving history? Or fail? Fail is safer for data integrity.
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate title"})
			return
		}
		newConv := models.Conversation{
			ID:        conversationID,
			UserID:    userID,
			Title:     title,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		_, _, err = dbClient.From("conversations").Insert(newConv, false, "representation", "", "exact").Execute()
		if err != nil {
			fmt.Printf("Failed to create conversation: %v\n", err)
			// Continue without saving history? Or fail? Fail is safer for data integrity.
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create conversation history"})
			return
		}
	} else {
		conversationID, err = uuid.Parse(req.ConversationID)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid conversation_id format"})
			return
		}
		// Verify ownership? Implicitly handled if we strictly associate messages, but querying it first is good practice.
		// For speed, we might skip checking existence if foreign key constrains it, but let's trust FK.
	}

	// Increment Chat Count Logic
	var user models.User
	_, err = dbClient.From("users").Select("chat_count", "exact", false).Eq("id", fmt.Sprintf("%d", userID)).Single().ExecuteTo(&user)
	if err == nil {
		updates := map[string]interface{}{
			"chat_count": user.ChatCount + 1,
		}
		dbClient.From("users").Update(updates, "", "representation").Eq("id", fmt.Sprintf("%d", userID)).Execute()
	}

	// Prepare Streaming Response
	ctx.Writer.Header().Set("Content-Type", "text/plain")
	ctx.Writer.Header().Set("Transfer-Encoding", "chunked")
	// Send new conversation ID in header so client knows it?
	ctx.Writer.Header().Set("X-Conversation-ID", conversationID.String())
	ctx.Writer.Flush()

	var generatedImages []string
	imageCallback := func(imgB64 string) {
		generatedImages = append(generatedImages, imgB64)
	}

	// Buffer full response for saving
	var fullBotResponse strings.Builder

	stream, err := c.geminiService.ChatStream(ctx.Request.Context(), req.Prompt, req.FileURI, imageCallback)
	if err != nil {
		fmt.Fprintf(ctx.Writer, "Error starting chat: %v", err)
		return
	}

	for resp, err := range stream {
		if err != nil {
			fmt.Fprintf(ctx.Writer, "\n[ERROR] Stream error: %v", err)
			return
		}

		if resp == nil || len(resp.Candidates) == 0 {
			continue
		}

		for _, part := range resp.Candidates[0].Content.Parts {
			if part.Text != "" {
				fmt.Fprint(ctx.Writer, part.Text)
				fullBotResponse.WriteString(part.Text)
				ctx.Writer.Flush()
			}
		}
	}

	// Images
	for _, imgB64 := range generatedImages {
		imgMarkdown := fmt.Sprintf("\n\n![Generated Image](data:image/png;base64,%s)", imgB64)
		fmt.Fprint(ctx.Writer, imgMarkdown)
		fullBotResponse.WriteString(imgMarkdown)
		ctx.Writer.Flush()
	}

	// Save Message to DB (Asynchronously preferably, but sync is fine for now)
	go func() {
		msg := models.Message{
			ID:             uuid.New(),
			ConversationID: conversationID,
			UserPrompt:     req.Prompt,
			BotResponse:    fullBotResponse.String(),
			Timestamp:      time.Now(),
		}
		_, _, err := dbClient.From("messages").Insert(msg, false, "representation", "", "exact").Execute()
		if err != nil {
			fmt.Printf("Failed to save message to DB: %v\n", err)
		}
		
		// Update Conversation UpdatedAt
		dbClient.From("conversations").Update(map[string]interface{}{"updated_at": time.Now()}, "", "representation").Eq("id", conversationID.String()).Execute()
	}()
}
