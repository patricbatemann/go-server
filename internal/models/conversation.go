package models

import (
	"time"

	"github.com/google/uuid"
)

type Conversation struct {
	ID        uuid.UUID `json:"id" db:"id"`
	UserID    int64     `json:"user_id" db:"user_id"`
	Title     string    `json:"title" db:"title"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
	Messages  []Message `json:"messages,omitempty" db:"-"`
}

type Message struct {
	ID             uuid.UUID `json:"id" db:"id"`
	ConversationID uuid.UUID `json:"conversation_id" db:"conversation_id"`
	UserPrompt     string    `json:"user_prompt" db:"user_prompt"`
	BotResponse    string    `json:"bot_response" db:"bot_response"`
	Timestamp      time.Time `json:"timestamp" db:"timestamp"`
}
