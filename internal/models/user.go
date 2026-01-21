package models

import (
	"time"
)

type User struct {
	ID           int64     `json:"id"`
	Email        string    `json:"email"`
	Name         string    `json:"name"`
	Provider     string    `json:"provider"` // google or microsoft
	ProviderID   string    `json:"provider_id"`
	LoginsCount  int       `json:"logins_count"`
	ChatCount    int       `json:"chat_count"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
