package rag

import (
	"context"
	"fmt"
	"strings"

	"media2rag/internal/llm"
	"media2rag/internal/model"
)

var questionPrefixes = []string{"как", "что", "почему", "зачем", "где", "когда", "кто", "какой", "сколько", "есть ли"}
var commandPrefixes = []string{"напиши", "объясни", "расскажи", "перечисли", "сравни", "опиши", "покажи", "найди"}

type Rewriter struct {
	client llm.LLMClient
}

func NewRewriter(client llm.LLMClient) *Rewriter {
	return &Rewriter{client: client}
}

func (rw *Rewriter) DetectFormat(query string) QueryFormat {
	trimmed := strings.TrimSpace(query)
	if strings.HasSuffix(trimmed, "?") {
		return FormatQuestion
	}

	lower := strings.ToLower(trimmed)
	for _, p := range questionPrefixes {
		if strings.HasPrefix(lower, p) {
			return FormatQuestion
		}
	}
	for _, p := range commandPrefixes {
		if strings.HasPrefix(lower, p) {
			return FormatCommand
		}
	}
	if len(strings.Fields(trimmed)) < 3 {
		return FormatFragment
	}
	return FormatStatement
}

func (rw *Rewriter) Rewrite(ctx context.Context, query string, format QueryFormat) (string, error) {
	var instruction string
	switch format {
	case FormatQuestion:
		instruction = "Rephrase the following question as clear, concise search terms (8-12 words). Focus on key concepts only."
	case FormatCommand:
		instruction = "Convert the following command into focused search terms (8-12 words) that would retrieve the relevant information."
	case FormatStatement:
		instruction = "Extract the key search terms from the following statement (8-12 words). Focus on the main topic and concepts."
	case FormatFragment:
		instruction = "Complete the following query fragment into clear search terms (8-12 words). Assume the user wants information about this topic."
	}

	resp, err := rw.client.Chat(ctx, model.ChatRequest{
		Messages: []model.Message{
			{Role: "system", Content: instruction},
			{Role: "user", Content: query},
		},
	})
	if err != nil {
		return query, fmt.Errorf("rewrite: %w", err)
	}

	rewritten := strings.TrimSpace(resp.Message.Content)
	if rewritten == "" {
		return query, nil
	}
	return rewritten, nil
}

func (rw *Rewriter) Expand(ctx context.Context, query string) ([]string, error) {
	resp, err := rw.client.Chat(ctx, model.ChatRequest{
		Messages: []model.Message{
			{Role: "system", Content: `Generate 3 alternative search queries for the given query.
Each query should explore a different aspect or use different terminology.
Return one query per line, no numbering or prefixes.`},
			{Role: "user", Content: query},
		},
	})
	if err != nil {
		return []string{query}, fmt.Errorf("expand: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(resp.Message.Content), "\n")
	queries := make([]string, 0, 3)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			queries = append(queries, line)
		}
	}
	if len(queries) == 0 {
		return []string{query}, nil
	}
	return queries, nil
}
