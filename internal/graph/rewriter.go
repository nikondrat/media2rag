package graph

import (
	"context"
	"strconv"
	"strings"

	"media2rag/internal/llm"
	"media2rag/internal/model"
)

// QueryRewriter preprocesses user queries into structured graph queries
type QueryRewriter struct {
	client llm.LLMClient
	model  string
}

// NewQueryRewriter creates a new rewriter
func NewQueryRewriter(client llm.LLMClient, model string) *QueryRewriter {
	return &QueryRewriter{
		client: client,
		model:  model,
	}
}

const queryRewritePrompt = `Rewrite the user query into a structured graph search query.

Patterns: root_cause, counterfactual, prerequisites, commonality, global, drift
Modes: local, global, drift
Relations: causes, enables, prevents, requires, solves, blocks, competes_with, serves, leverages, leads_to, correlates, supports, contradicts, part_of

Return in this exact format:
entities: <comma-separated entity names to search for>
pattern: <detected pattern>
relations: <comma-separated relevant relation types>
mode: <local|global|drift>
depth: <2 or 3>

Example:
entities: продажи, конверсия, воронка
pattern: root_cause
relations: causes, leads_to
mode: local
depth: 2`

// Rewrite converts a natural language query to a structured query
func (r *QueryRewriter) Rewrite(ctx context.Context, query string) (*model.GraphQuery, error) {
	pattern := detectPattern(query)
	mode := autoSelectMode(pattern)
	depth := estimateDepth(query)

	resp, err := r.client.Chat(ctx, model.ChatRequest{
		Model: r.model,
		Messages: []model.Message{
			{Role: "system", Content: "Rewrite queries into structured graph search format. Use the exact format specified."},
			{Role: "user", Content: queryRewritePrompt + "\n\nQuery: " + query},
		},
	})
	if err != nil {
		// Fallback to keyword-based detection
		return &model.GraphQuery{
			Entities: extractKeywords(query),
			Pattern:  pattern,
			Mode:     mode,
			Depth:    depth,
		}, nil
	}

	gq := parseQueryRewriteResponse(resp.Message.Content)

	// Fill in defaults from heuristic detection if LLM didn't provide
	if gq.Pattern == "" {
		gq.Pattern = pattern
	}
	if gq.Mode == "" || gq.Mode == "auto" {
		gq.Mode = autoSelectMode(gq.Pattern)
	}
	if gq.Depth <= 0 {
		gq.Depth = estimateDepth(query)
	}
	if len(gq.Entities) == 0 {
		gq.Entities = extractKeywords(query)
	}

	return &gq, nil
}

func parseQueryRewriteResponse(response string) model.GraphQuery {
	var gq model.GraphQuery

	lines := strings.Split(response, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)

		switch {
		case strings.HasPrefix(lower, "entities:"):
			val := strings.TrimPrefix(trimmed, "entities:")
			val = strings.TrimSpace(val)
			for _, e := range strings.Split(val, ",") {
				e = strings.TrimSpace(e)
				if e != "" {
					gq.Entities = append(gq.Entities, e)
				}
			}
		case strings.HasPrefix(lower, "pattern:"):
			gq.Pattern = strings.TrimSpace(strings.TrimPrefix(trimmed, "pattern:"))
		case strings.HasPrefix(lower, "relations:"):
			val := strings.TrimPrefix(trimmed, "relations:")
			val = strings.TrimSpace(val)
			for _, r := range strings.Split(val, ",") {
				r = strings.TrimSpace(r)
				if r != "" {
					gq.Relations = append(gq.Relations, r)
				}
			}
		case strings.HasPrefix(lower, "mode:"):
			gq.Mode = strings.TrimSpace(strings.TrimPrefix(trimmed, "mode:"))
		case strings.HasPrefix(lower, "depth:"):
			val := strings.TrimSpace(strings.TrimPrefix(trimmed, "depth:"))
			gq.Depth, _ = strconv.Atoi(val)
		}
	}

	return gq
}

// detectPattern detects the query pattern from keywords
func detectPattern(query string) string {
	lower := strings.ToLower(query)

	if strings.Contains(lower, "почему") || strings.Contains(lower, "причина") ||
		strings.Contains(lower, "из-за") || strings.Contains(lower, "cause") ||
		strings.Contains(lower, "why") {
		return model.PatternRootCause
	}

	if strings.Contains(lower, "что если") || strings.Contains(lower, "убрать") ||
		strings.Contains(lower, "без ") || strings.Contains(lower, "если не") ||
		strings.Contains(lower, "what if") || strings.Contains(lower, "without") {
		return model.PatternCounterfactual
	}

	if strings.Contains(lower, "как достичь") || strings.Contains(lower, "что нужно") ||
		strings.Contains(lower, "как сделать") || strings.Contains(lower, "как получить") ||
		strings.Contains(lower, "how to") || strings.Contains(lower, "achieve") {
		return model.PatternPrerequisites
	}

	if strings.Contains(lower, "что общего") || strings.Contains(lower, "связь между") ||
		strings.Contains(lower, "как связаны") || strings.Contains(lower, "common") ||
		strings.Contains(lower, "connection between") {
		return model.PatternCommonality
	}

	if strings.Contains(lower, "какие топ") || strings.Contains(lower, "топ-") ||
		strings.Contains(lower, "обзор") || strings.Contains(lower, "что есть") ||
		strings.Contains(lower, "overview") || strings.Contains(lower, "top ") {
		return model.PatternGlobal
	}

	if strings.Contains(lower, "как решить") || strings.Contains(lower, "что делать") ||
		strings.Contains(lower, "как исправить") || strings.Contains(lower, "how to solve") ||
		strings.Contains(lower, "what to do") {
		return model.PatternDRIFT
	}

	return model.PatternRootCause
}

func autoSelectMode(pattern string) string {
	switch pattern {
	case model.PatternRootCause, model.PatternCounterfactual:
		return model.ModeLocal
	case model.PatternGlobal:
		return model.ModeGlobal
	case model.PatternDRIFT:
		return model.ModeDRIFT
	case model.PatternPrerequisites, model.PatternCommonality:
		return model.ModeLocal
	default:
		return model.ModeLocal
	}
}

func estimateDepth(query string) int {
	words := strings.Fields(query)
	if len(words) <= 4 {
		return 2
	}
	return 3
}

func extractKeywords(query string) []string {
	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "is": true, "are": true,
		"was": true, "were": true, "be": true, "been": true, "being": true,
		"have": true, "has": true, "had": true, "do": true, "does": true,
		"did": true, "will": true, "would": true, "could": true, "should": true,
		"to": true, "of": true, "in": true, "for": true, "on": true,
		"with": true, "at": true, "by": true, "from": true, "as": true,
		"and": true, "but": true, "or": true, "not": true, "so": true,
		"что": true, "как": true, "почему": true, "где": true, "когда": true,
		"кто": true, "какой": true, "в": true, "на": true, "с": true,
		"у": true, "о": true, "из": true, "по": true, "для": true,
		"это": true, "не": true, "или": true, "но": true, "а": true,
		"ли": true, "бы": true, "же": true, "все": true, "всё": true,
	}

	var keywords []string
	for _, word := range strings.Fields(query) {
		word = strings.ToLower(strings.Trim(word, ".,!?;:()[]{}\"'"))
		if word == "" || stopWords[word] {
			continue
		}
		keywords = append(keywords, word)
	}

	return keywords
}
