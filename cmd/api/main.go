package main

import (
	"context"
	"log"
	"os"

	"github.com/joho/godotenv"

	"go-server/internal/auth"
	"go-server/internal/controllers"
	"go-server/internal/database"
	"go-server/internal/routes"
	"go-server/internal/services"
)

func main() {
	// Load environment variables from .env file if it exists
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, relying on environment variables")
	}

	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		log.Fatal("GEMINI_API_KEY environment variable is required")
	}

	ctx := context.Background()
	geminiService, err := services.NewGeminiService(ctx, apiKey)
	if err != nil {
		log.Fatalf("Failed to initialize Gemini service: %v", err)
	}

	chatController := controllers.NewChatController(geminiService)
	conversationController := controllers.NewConversationController()

	// Initialize Database
	database.InitDB()

	// Initialize OAuth
	auth.InitOAuth()

	r := routes.SetupRouter(chatController, conversationController)

	port := os.Getenv("PORT")
	if port == "" {
		port = "5000"
	}

	log.Printf("Starting server on port %s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
