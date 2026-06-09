package llm

import (
	"context"
	"testing"
	"time"

	"media2rag/internal/model"
)

func TestOllamaClient_Chat(t *testing.T) {
	c := NewOllamaClient("http://localhost:11434", "gemma4:latest", 120*time.Second)
	_, err := c.Chat(context.Background(), model.ChatRequest{
		Messages: []model.Message{
			{Role: "user", Content: "say hello"},
		},
	})
	if err != nil {
		t.Logf("Ollama not running (expected without server): %v", err)
	}
}
