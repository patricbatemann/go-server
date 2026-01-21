package models

type UploadResponse struct {
	Name string `json:"name"`
	URI  string `json:"uri"`
	State string `json:"state"`
}

type ChatRequest struct {
	Prompt         string `json:"prompt" binding:"required"`
	FileURI        string `json:"file_uri,omitempty"`
	ConversationID string `json:"conversation_id,omitempty"` // UUID string
}
