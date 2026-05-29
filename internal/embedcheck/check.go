package embedcheck

import (
	"context"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"media2rag/internal/dashboard"
	"media2rag/internal/llm"
	"media2rag/internal/model"
)

type Config struct {
	SampleSize         int
	SimilarityThreshold float64
	RelevanceThreshold  float64
}

func DefaultConfig() Config {
	return Config{
		SampleSize:         5,
		SimilarityThreshold: 0.6,
		RelevanceThreshold:  0.7,
	}
}

type SimilarityResult struct {
	QueryText string
	ChunkText string
	Score     float64
}

type Runner struct {
	client llm.LLMClient
	model  string
	tracer *dashboard.Tracer
	cfg    Config
}

func NewRunner(client llm.LLMClient, model string, tracer *dashboard.Tracer, cfg Config) *Runner {
	return &Runner{
		client: client,
		model:  model,
		tracer: tracer,
		cfg:    cfg,
	}
}

func (r *Runner) Check(ctx context.Context, runID string, queryText string, similarities []SimilarityResult) []dashboard.EmbeddingCheck {
	if len(similarities) == 0 {
		return nil
	}

	sampleSize := r.cfg.SampleSize
	if sampleSize <= 0 {
		sampleSize = 5
	}
	if sampleSize > len(similarities) {
		sampleSize = len(similarities)
	}
	sample := similarities[:sampleSize]

	var wg sync.WaitGroup
	var mu sync.Mutex
	var checks []dashboard.EmbeddingCheck

	for _, sim := range sample {
		wg.Add(1)
		go func(s SimilarityResult) {
			defer wg.Done()

			check := r.checkOne(ctx, runID, queryText, s)
			mu.Lock()
			checks = append(checks, check)
			mu.Unlock()
		}(sim)
	}

	wg.Wait()
	return checks
}

func (r *Runner) checkOne(ctx context.Context, runID, queryText string, sim SimilarityResult) dashboard.EmbeddingCheck {
	start := time.Now()

	prompt := fmt.Sprintf(`Evaluate whether the following chunk of text is relevant to the given query.

Query: %s

Chunk: %s

Rate relevance on a scale of 0.0 to 1.0 (0 = completely irrelevant, 1 = perfectly relevant).
Return only a single number.`, queryText, sim.ChunkText)

	relevanceScore := 0.5

	resp, err := r.client.Chat(ctx, model.ChatRequest{
		Messages: []model.Message{
			{Role: "user", Content: prompt},
		},
	})
	latencyMs := int(time.Since(start).Milliseconds())

	if err == nil {
		parsed := parseFloatScore(resp.Message.Content)
		if parsed >= 0 && parsed <= 1 {
			relevanceScore = parsed
		}
	}

	passed := relevanceScore >= r.cfg.RelevanceThreshold && sim.Score >= r.cfg.SimilarityThreshold

	check := dashboard.EmbeddingCheck{
		RunID:           runID,
		QueryText:       queryText,
		ChunkText:       truncateText(sim.ChunkText, 500),
		SimilarityScore: math.Round(sim.Score*100) / 100,
		RelevanceScore:  math.Round(relevanceScore*100) / 100,
		Passed:          passed,
		LatencyMs:       latencyMs,
		CreatedAt:       time.Now(),
	}

	if r.tracer != nil {
		r.tracer.SaveEmbeddingCheck(&check)
	}

	return check
}

func parseFloatScore(text string) float64 {
	text = strings.TrimSpace(text)

	var v float64
	if _, err := fmt.Sscanf(text, "%f", &v); err == nil {
		return v
	}

	return 0.5
}

func truncateText(text string, maxLen int) string {
	runes := []rune(text)
	if len(runes) > maxLen {
		return string(runes[:maxLen]) + "..."
	}
	return text
}
