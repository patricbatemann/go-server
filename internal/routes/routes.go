package routes

import (
	"github.com/gin-gonic/gin"

	"go-server/internal/auth"
	"go-server/internal/controllers"
)

func SetupRouter(chatController *controllers.ChatController, conversationController *controllers.ConversationController) *gin.Engine {
	r := gin.Default()

	// Public Routes
	r.GET("/auth/:provider/login", auth.LoginHandler)
	r.GET("/auth/:provider/callback", auth.CallbackHandler)

	// Protected Routes
	api := r.Group("/")
	api.Use(auth.AuthMiddleware())
	{
		api.GET("/auth/me", auth.MeHandler)
		api.GET("/auth/logout", auth.LogoutHandler)
		
		api.POST("/upload", chatController.Upload)
		api.POST("/chat", chatController.Chat)

		// Conversation Routes
		api.GET("/conversations", conversationController.GetConversations)
		api.GET("/conversations/:id", conversationController.GetConversation)
		api.PUT("/conversations/:id", conversationController.UpdateConversation)
		api.DELETE("/conversations/:id", conversationController.DeleteConversation)
	}

	return r
}
