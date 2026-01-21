package auth

import (
	"fmt"
	"os"
	"time"

	"go-server/internal/models"

	"github.com/golang-jwt/jwt/v5"
)

type CustomClaims struct {
	UserID   int64  `json:"user_id"`
	Email    string `json:"email"`
	Name     string `json:"name"`
	Provider string `json:"provider"`
	jwt.RegisteredClaims
}

func GenerateToken(user models.User) (string, error) {
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		return "", fmt.Errorf("JWT_SECRET environment variable is not set")
	}

	claims := CustomClaims{
		UserID:   user.ID,
		Email:    user.Email,
		Name:     user.Name,
		Provider: user.Provider,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "go-server-auth",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(jwtSecret))
}

func ValidateToken(tokenString string) (*CustomClaims, error) {
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET environment variable is not set")
	}

	token, err := jwt.ParseWithClaims(tokenString, &CustomClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(jwtSecret), nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*CustomClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token")
}
