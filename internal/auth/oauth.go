package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"golang.org/x/oauth2/microsoft"

	"go-server/internal/database"
	"go-server/internal/models"
)

var (
	GoogleOAuthConfig    *oauth2.Config
	MicrosoftOAuthConfig *oauth2.Config
)

func InitOAuth() {
	GoogleOAuthConfig = &oauth2.Config{
		ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		RedirectURL:  os.Getenv("GOOGLE_REDIRECT_URL"),
		Scopes:       []string{"https://www.googleapis.com/auth/userinfo.email", "https://www.googleapis.com/auth/userinfo.profile"},
		Endpoint:     google.Endpoint,
	}

	MicrosoftOAuthConfig = &oauth2.Config{
		ClientID:     os.Getenv("MICROSOFT_CLIENT_ID"),
		ClientSecret: os.Getenv("MICROSOFT_CLIENT_SECRET"),
		RedirectURL:  os.Getenv("MICROSOFT_REDIRECT_URL"),
		Scopes:       []string{"User.Read"},
		Endpoint:     microsoft.AzureADEndpoint("common"),
	}
}

func LoginHandler(c *gin.Context) {
	provider := c.Param("provider")
	platform := c.Query("platform")
	var config *oauth2.Config

	switch provider {
	case "google":
		config = GoogleOAuthConfig
	case "microsoft":
		config = MicrosoftOAuthConfig
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid provider"})
		return
	}

	url := config.AuthCodeURL("state", oauth2.AccessTypeOffline, oauth2.SetAuthURLParam("platform", platform))
	c.Redirect(http.StatusTemporaryRedirect, url)
}

func CallbackHandler(c *gin.Context) {
	provider := c.Param("provider")
	code := c.Query("code")
	platform := c.Query("platform")

	var config *oauth2.Config
	switch provider {
	case "google":
		config = GoogleOAuthConfig
	case "microsoft":
		config = MicrosoftOAuthConfig
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid provider"})
		return
	}

	token, err := config.Exchange(context.Background(), code)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to exchange token"})
		return
	}

	client := config.Client(context.Background(), token)
	var userInfo map[string]interface{}

	switch provider {
		case "google":
			resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user info"})
				return
			}
			defer resp.Body.Close()
			if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decode user info"})
				return
			}
		case "microsoft":
			resp, err := client.Get("https://graph.microsoft.com/v1.0/me")
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user info"})
				return
			}
			defer resp.Body.Close()
			if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decode user info"})
				return
			}
	}

	// Extract user details
	var email, name, providerID string
	if provider == "google" {
		email = userInfo["email"].(string)
		name = userInfo["name"].(string)
		providerID = userInfo["id"].(string)
	} else {
		// Microsoft Graph API might return different fields
		if val, ok := userInfo["mail"]; ok {
			email = val.(string)
		} else if val, ok := userInfo["userPrincipalName"]; ok {
			email = val.(string)
		}
		if val, ok := userInfo["displayName"]; ok {
			name = val.(string)
		}
		providerID = userInfo["id"].(string)
	}

	// Save or Update User
	dbClient := database.GetClient()
	var user models.User
	
	// Check if user exists
	// Supabase return 406 Not Acceptable if no rows found with Single(), but here we use MaybeSingle() if available or just check list
	// Actually, just try to select by email
	res, _, err := dbClient.From("users").Select("*", "exact", false).Eq("email", email).Single().Execute()

	if err != nil {
		// Verify if it's a "not found" error. Supabase-go might return error or empty result.
		// If using Single(), it errors if row not found.
		// We assume error means not found for now or handle specifically if we can distinguish.
		// For simplicity: if error, assume creating new user.
		
		// Create new user
		user = models.User{
			Email:       email,
			Name:        name,
			Provider:    provider,
			ProviderID:  providerID,
			LoginsCount: 1,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}
		
		// Insert
		_, _, err := dbClient.From("users").Insert(user, false, "", "", "representation").Execute()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
			return
		}

		// Re-fetch to get ID
		dbClient.From("users").Select("*", "exact", false).Eq("email", email).Single().ExecuteTo(&user)

	} else {
		// Decode existing user
		// The `res` contains the JSON body. We can unmarshal it or use ExecuteTo above.
		// Let's re-fetch to be safe or just use ExecuteTo in the first place if we could.
		
		// For update, we want to update LoginsCount and Name
		// But first let's get the user ID
		json.Unmarshal(res, &user)

		updates := map[string]interface{}{
			"logins_count": user.LoginsCount + 1,
			"name":         name,
			"updated_at":   time.Now(),
		}
		
		_, _, err := dbClient.From("users").Update(updates, "", "representation").Eq("id", fmt.Sprintf("%d", user.ID)).Execute()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user"})
			return
		}
		
		// Update local object for token generation
		user.LoginsCount++
		user.Name = name
	}

	// Generate JWT
	tokenString, err := GenerateToken(user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	// Set HTTP-only Cookie
	// MaxAge: 24 hours (24 * 60 * 60)
	// c.SetCookie("jwt_token", tokenString, 24*60*60, "/", "", false, true)

	if platform == "mobile" {
		frontendURL := os.Getenv("FRONTEND_MOBILE_URL")
		// Redirect with the token in the URL
		target := fmt.Sprintf("%s/auth-success?token=%s", frontendURL, tokenString)
		c.Redirect(http.StatusTemporaryRedirect, target)
		return
	}

	frontendURL := os.Getenv("FRONTEND_URL") // http://34.180.49.123

	// Redirect with the token in the URL
	target := fmt.Sprintf("%s/auth-success?token=%s", frontendURL, tokenString)
	c.Redirect(http.StatusTemporaryRedirect, target)

	// c.Redirect(http.StatusTemporaryRedirect, os.Getenv("FRONTEND_URL") + "/auth")
}

func MeHandler(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if (!exists) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	dbClient := database.GetClient()
	var user models.User
	// UserID from context is int64 now (changed in jwt.go) or uint? 
	// In jwt.go it is int64. In Middleware it is set. 
	// We need to make sure we cast correctly.

	// But wait, the middleware sets it from claims.UserID which is int64.
	// So userID variable below needs to be int64.
	
	// Check how it is retrieved in Middleware first:
	// c.Set("user_id", claims.UserID)
	
	// Here in MeHandler:
	if id, ok := userID.(int64); ok {
		_, err := dbClient.From("users").Select("*", "exact", false).Eq("id", fmt.Sprintf("%d", id)).Single().ExecuteTo(&user)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}
	} else {
		// Fallback if type assertion fails (e.g. float64 from json unmarshal sometimes?)
		// But here it comes strictly from JWT struct
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID type"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"user": user})
}

func LogoutHandler(c *gin.Context) {
	// Clear the cookie by setting max age to -1
	c.SetCookie("jwt_token", "", -1, "/", "", false, true)
	c.JSON(http.StatusOK, gin.H{"message": "Logout successful"})
}

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString, err := c.Cookie("jwt_token")
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization cookie missing"})
			c.Abort()
			return
		}

		claims, err := ValidateToken(tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			c.Abort()
			return
		}

		// Set info in context
		c.Set("user_id", claims.UserID)
		c.Set("user_email", claims.Email)
		c.Set("user_name", claims.Name)
		
		c.Next()
	}
}
