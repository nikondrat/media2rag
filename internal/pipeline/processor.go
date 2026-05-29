package pipeline

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"media2rag/internal/events"
	"media2rag/internal/model"
)

const chunkPrompt = `Analyze the following text and extract:
- title: A concise title (max 10 words)
- topics: Comma-separated list of key topics (max 8)
- summary: A 2-3 sentence summary

Return in this exact format:
title: <title>
topics: <topic1, topic2, ...>
summary: <summary>`

const claimsPrompt = `Extract all factual claims from the text. For each claim, output:
claim: <statement> | confidence: <0.0-1.0> | source: text

Return one claim per line in this format:
claim: <statement> | confidence: <0.0-1.0> | source: text`

const mentalModelsPrompt = `Identify mental models, frameworks, or thinking patterns used in the text.
Return one per line in this format:
mental_model: <model name>`

const keyTermsPrompt = `Extract key technical terms and their definitions from the text.
Return one per line in this format:
key_term: <term> | definition: <definition>`

const coreThesisPrompt = `Identify the single most important thesis or central argument of this text.
Return in this format:
core_thesis: <thesis statement>`

const takeawaysPrompt = `Extract actionable takeaways or key lessons from this text.
Return one per line in this format:
takeaway: <takeaway text>`

func (p *Pipeline) processChunks(ctx context.Context, chunks []string, emitter events.EventEmitter) ([]ChunkResult, error) {
	done := make(map[int]bool)
	var results []ChunkResult

	if p.checkpointDir != "" {
		existing := loadResults(p.checkpointDir)
		if existing != nil {
			results = existing
			for _, r := range results {
				if r.Index >= 0 {
					done[r.Index] = true
				}
			}
		}
	}

	if results == nil {
		results = make([]ChunkResult, len(chunks))
	}

	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, p.config.MaxConcurrency)
	errCh := make(chan error, len(chunks))

	for i, chunk := range chunks {
		if done[i] {
			continue
		}

		wg.Add(1)
		go func(idx int, text string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			emitter.Emit(model.Event{Type: EventProcessingChunk, Data: map[string]int{"chunk": idx + 1, "total": len(chunks)}})

			r, err := p.processSingle(ctx, text)
			if err != nil {
				errCh <- err
				return
			}
			r.Index = idx
			r.Content = text

			mu.Lock()
			results[idx] = r
			if p.checkpointDir != "" {
				saveResults(p.checkpointDir, results)
			}
			mu.Unlock()

			emitter.Emit(model.Event{Type: EventProcessingChunkDone, Data: map[string]int{"chunk": idx + 1}})
		}(i, chunk)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			return nil, err
		}
	}

	return results, nil
}

type promptTask struct {
	id      string
	prompt  string
	enabled bool
}

func (p *Pipeline) processSingle(ctx context.Context, text string) (ChunkResult, error) {
	sem := make(chan struct{}, p.config.MaxConcurrency)
	var mu sync.Mutex
	var wg sync.WaitGroup
	result := ChunkResult{}
	errCh := make(chan error, 6)

	tasks := []promptTask{
		{"v1", chunkPrompt, true},
		{"claims", claimsPrompt, p.config.ExtractClaims},
		{"mental_models", mentalModelsPrompt, p.config.ExtractMentalModels},
		{"key_terms", keyTermsPrompt, p.config.ExtractKeyTerms},
		{"core_thesis", coreThesisPrompt, p.config.ExtractCoreThesis},
		{"takeaways", takeawaysPrompt, p.config.ExtractTakeaways},
	}

	for _, t := range tasks {
		if !t.enabled {
			continue
		}
		wg.Add(1)
		go func(task promptTask) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			callCtx, cancel := p.timeoutCtx(ctx)
			defer cancel()
			start := time.Now()
			resp, err := p.llmClient.Chat(callCtx, model.ChatRequest{
				Messages: []model.Message{
					{Role: "system", Content: task.prompt},
					{Role: "user", Content: text},
				},
			})
			latency := int(time.Since(start).Milliseconds())
			if err != nil {
				if p.tracer != nil && p.runID != "" {
					p.tracer.SaveLLMCall(p.runID, p.model, "process_"+task.id, len(text)/4, 0, latency, 0, task.prompt, "", "error", err.Error())
				}
				errCh <- err
				return
			}
			if p.tracer != nil && p.runID != "" {
				p.tracer.SaveLLMCall(p.runID, p.model, "process_"+task.id, len(text)/4, len(resp.Message.Content)/4, latency, 0, task.prompt, resp.Message.Content, "success", "")
			}

			parsed := parsePromptResult(task.id, resp.Message.Content)
			mu.Lock()
			mergePromptResult(&result, task.id, parsed)
			mu.Unlock()
		}(t)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			return ChunkResult{}, err
		}
	}

	return result, nil
}

func parsePromptResult(id, response string) ChunkResult {
	var r ChunkResult
	lines := strings.Split(response, "\n")
	for _, line := range lines {
		lower := strings.ToLower(strings.TrimSpace(line))
		switch id {
		case "v1":
			if strings.HasPrefix(lower, "title:") {
				r.Title = strings.TrimSpace(line[6:])
			} else if strings.HasPrefix(lower, "topics:") {
				raw := strings.TrimSpace(line[7:])
				for _, t := range strings.Split(raw, ",") {
					t = strings.TrimSpace(t)
					if t != "" {
						r.Topics = append(r.Topics, t)
					}
				}
			} else if strings.HasPrefix(lower, "summary:") {
				r.Summary = strings.TrimSpace(line[8:])
			}
		case "claims":
			if strings.HasPrefix(lower, "claim:") {
				rest := strings.TrimSpace(line[6:])
				c := parseClaimLine(rest)
				r.Claims = append(r.Claims, c)
			}
		case "mental_models":
			if strings.HasPrefix(lower, "mental_model:") {
				m := strings.TrimSpace(line[13:])
				if m != "" {
					r.MentalModels = append(r.MentalModels, m)
				}
			}
		case "key_terms":
			if strings.HasPrefix(lower, "key_term:") {
				rest := strings.TrimSpace(line[9:])
				kt := parseKeyTermLine(rest)
				r.KeyTerms = append(r.KeyTerms, kt)
			}
		case "core_thesis":
			if strings.HasPrefix(lower, "core_thesis:") {
				r.CoreThesis = strings.TrimSpace(line[12:])
			}
		case "takeaways":
			if strings.HasPrefix(lower, "takeaway:") {
				t := strings.TrimSpace(line[10:])
				if t != "" {
					r.Takeaways = append(r.Takeaways, t)
				}
			}
		}
	}
	return r
}

func parseClaimLine(s string) model.Claim {
	c := model.Claim{Confidence: 1.0}
	parts := strings.Split(s, "|")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		lower := strings.ToLower(p)
		if strings.HasPrefix(lower, "confidence:") {
			val := strings.TrimSpace(p[11:])
			if f, err := strconv.ParseFloat(val, 64); err == nil {
				c.Confidence = f
			}
		} else if strings.HasPrefix(lower, "source:") {
			c.Source = strings.TrimSpace(p[7:])
		} else if strings.HasPrefix(lower, "claim:") {
			c.Statement = strings.TrimSpace(p[6:])
		} else if c.Statement == "" {
			c.Statement = p
		}
	}
	return c
}

func parseKeyTermLine(s string) model.KeyTerm {
	kt := model.KeyTerm{}
	parts := strings.Split(s, "|")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		lower := strings.ToLower(p)
		if strings.HasPrefix(lower, "definition:") {
			kt.Definition = strings.TrimSpace(p[11:])
		} else if strings.HasPrefix(lower, "key_term:") {
			kt.Term = strings.TrimSpace(p[9:])
		} else if kt.Term == "" {
			kt.Term = p
		}
	}
	return kt
}

func mergePromptResult(dst *ChunkResult, id string, src ChunkResult) {
	switch id {
	case "v1":
		if dst.Title == "" {
			dst.Title = src.Title
		}
		dst.Topics = append(dst.Topics, src.Topics...)
		if dst.Summary == "" {
			dst.Summary = src.Summary
		}
	case "claims":
		dst.Claims = append(dst.Claims, src.Claims...)
	case "mental_models":
		dst.MentalModels = append(dst.MentalModels, src.MentalModels...)
	case "key_terms":
		dst.KeyTerms = append(dst.KeyTerms, src.KeyTerms...)
	case "core_thesis":
		if dst.CoreThesis == "" {
			dst.CoreThesis = src.CoreThesis
		}
	case "takeaways":
		dst.Takeaways = append(dst.Takeaways, src.Takeaways...)
	}
}

func loadResults(dir string) []ChunkResult {
	data, err := os.ReadFile(filepath.Join(dir, "results.json"))
	if err != nil {
		return nil
	}
	var results []ChunkResult
	if err := json.Unmarshal(data, &results); err != nil {
		return nil
	}
	return results
}

func saveResults(dir string, results []ChunkResult) {
	os.MkdirAll(dir, 0755)
	data, _ := json.MarshalIndent(results, "", "  ")
	os.WriteFile(filepath.Join(dir, "results.json"), data, 0644)
}
