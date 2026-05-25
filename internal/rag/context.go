package rag

import (
	"context"
	"fmt"
	"strings"

	"media2rag/internal/llm"
	"media2rag/internal/model"
	"media2rag/internal/store"
)

const systemPrompt = `You are an AI assistant with access to retrieved documents.
Answer the user's question based on the provided context.

Rules:
- Base your answer ONLY on the provided context
- If the context does not contain enough information, say so
- Cite sources using [N] notation where N is the source number
- Always include a "Sources:" section at the end listing all cited sources
- Be concise and accurate`

type ContextBuilder struct {
	client llm.LLMClient
}

func NewContextBuilder(client llm.LLMClient) *ContextBuilder {
	return &ContextBuilder{client: client}
}

func (cb *ContextBuilder) BuildAndQuery(ctx context.Context, query string, results []store.SearchResult) (*model.ChatResponse, error) {
	var contextParts []string
	var sources []string

	for i, r := range results {
		content := r.Payload["content"]
		title := r.Payload["document_id"]
		docType := r.Payload["chunk_type"]

		contextParts = append(contextParts, fmt.Sprintf("Source [%d]:\n> %s", i+1, content))
		sources = append(sources, fmt.Sprintf("[%d]: %s (%s)", i+1, title, docType))
	}

	contextStr := strings.Join(contextParts, "\n\n")
	sourcesStr := strings.Join(sources, "\n")

	fullPrompt := fmt.Sprintf(`Context:
%s

Sources:
%s

Query: %s

Answer with citations using [N] notation.`, contextStr, sourcesStr, query)

	resp, err := cb.client.Chat(ctx, model.ChatRequest{
		Messages: []model.Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: fullPrompt},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("context query: %w", err)
	}

	return resp, nil
}

func (cb *ContextBuilder) BuildSystemPrompt() string {
	return systemPrompt
}

func BuildSourcesBlock(results []store.SearchResult) string {
	var sb strings.Builder
	sb.WriteString("\n\nSources:\n")
	for i, r := range results {
		title := r.Payload["document_id"]
		docType := r.Payload["chunk_type"]
		sb.WriteString(fmt.Sprintf("[%d]: %s (%s)\n", i+1, title, docType))
	}
	return sb.String()
}
