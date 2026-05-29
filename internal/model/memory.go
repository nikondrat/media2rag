package model

type MemoryEntry struct {
	ID        string `json:"id"`
	UserID    string `json:"user_id"`
	Content   string `json:"content"`
	Category  string `json:"category"`
	CreatedAt int64  `json:"created_at"`
}
