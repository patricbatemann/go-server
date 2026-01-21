package controllers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/supabase-community/postgrest-go"

	"go-server/internal/database"
	"go-server/internal/models"
)

type ConversationController struct{}

func NewConversationController() *ConversationController {
	return &ConversationController{}
}

func (c *ConversationController) GetConversations(ctx *gin.Context) {
	userIDVal, exists := ctx.Get("user_id")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userID := userIDVal.(int64)

	dbClient := database.GetClient()
	var conversations []models.Conversation

	// Fetch conversations for the user, ordered by updated_at desc
	_, err := dbClient.From("conversations").Select("*", "exact", false).Eq("user_id", fmt.Sprintf("%d", userID)).Order("updated_at", &postgrest.OrderOpts{Ascending: false}).ExecuteTo(&conversations)
	
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch conversations"})
		return
	}

	ctx.JSON(http.StatusOK, conversations)
}

func (c *ConversationController) GetConversation(ctx *gin.Context) {
	idStr := ctx.Param("id")
	conversationID, err := uuid.Parse(idStr)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid conversation ID"})
		return
	}

	userIDVal, exists := ctx.Get("user_id")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userID := userIDVal.(int64)

	dbClient := database.GetClient()
	var conversation models.Conversation

	// Fetch conversation to verify ownership
	_, err = dbClient.From("conversations").Select("*", "exact", false).Eq("id", conversationID.String()).Eq("user_id", fmt.Sprintf("%d", userID)).Single().ExecuteTo(&conversation)
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Conversation not found"})
		return
	}

	// Fetch messages for the conversation
	var messages []models.Message
	_, err = dbClient.From("messages").Select("*", "exact", false).Eq("conversation_id", conversationID.String()).Order("timestamp", &postgrest.OrderOpts{Ascending: true}).ExecuteTo(&messages)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch messages"})
		return
	}

	conversation.Messages = messages
	ctx.JSON(http.StatusOK, conversation)
}

func (c *ConversationController) UpdateConversation(ctx *gin.Context) {
	idStr := ctx.Param("id")
	conversationID, err := uuid.Parse(idStr)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid conversation ID"})
		return
	}

	userIDVal, exists := ctx.Get("user_id")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userID := userIDVal.(int64)

	var updateData struct {
		Title string `json:"title" binding:"required"`
	}
	if err := ctx.ShouldBindJSON(&updateData); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	dbClient := database.GetClient()
	
	// Verify ownership and update
	// Note: Supabase Update doesn't easily support "verify ownership via where" in one go if RLS isn't set up perfectly or using service role.
	// But assuming we want to be safe, we Eq user_id.
	
	updates := map[string]interface{}{
		"title":      updateData.Title,
		"updated_at": time.Now(),
	}

	_, _, err = dbClient.From("conversations").Update(updates, "", "representation").Eq("id", conversationID.String()).Eq("user_id", fmt.Sprintf("%d", userID)).Execute()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update conversation"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "Conversation updated"})
}

func (c *ConversationController) DeleteConversation(ctx *gin.Context) {
	idStr := ctx.Param("id")
	conversationID, err := uuid.Parse(idStr)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid conversation ID"})
		return
	}

	userIDVal, exists := ctx.Get("user_id")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userID := userIDVal.(int64)

	dbClient := database.GetClient()

	// Delete conversation (Messages cascade deleted by DB foreign key)
	_, _, err = dbClient.From("conversations").Delete("", "representation").Eq("id", conversationID.String()).Eq("user_id", fmt.Sprintf("%d", userID)).Execute()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete conversation"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "Conversation deleted"})
}
