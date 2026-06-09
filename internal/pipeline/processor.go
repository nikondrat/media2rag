package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"media2rag/internal/events"
	"media2rag/internal/model"
)

const chunkPrompt = `Analyze the following text and extract structured information. Preserve the original language of the text.

1. type: Classify the chunk into exactly one knowledge ontology type from:
   - idea: A concept, notion, or thought
   - framework: A structured approach or methodology
   - principle: A fundamental truth or rule
   - example: A concrete instance or illustration
   - case_study: A detailed examination of a real-world scenario
   - tool: A practical instrument, technique, or resource
   - warning: A cautionary note or risk
   - action_step: A specific actionable instruction
   - quote: A notable direct quotation
   - question: An open question or point of inquiry
   - personal_note: A subjective reflection or note

2. topic: The single primary topic (3-8 words)

3. summary: A 2-3 sentence summary of the key content in the original language

4. key_points: Comma-separated list of the most important points (max 6)

5. source_quote: A notable direct quote from the text, if any (otherwise leave empty)

6. my_takeaway: The single most important personal takeaway or lesson in the original language

7. confidence: Your confidence in this extraction as a float between 0.0 and 1.0

8. applicability: How this content can be applied in practice (1-2 sentences) in the original language

Return in this exact format:
type: <type>
topic: <topic>
summary: <summary>
key_points: <point1, point2, ...>
source_quote: <quote>
my_takeaway: <takeaway>
confidence: <0.0-1.0>
applicability: <applicability>`

func (p *Pipeline) processChunks(ctx context.Context, chunks []string, emitter events.EventEmitter) ([]ChunkResult, error) {
	results := make([]ChunkResult, len(chunks))

	for i := range results {
		results[i].Index = i
	}

	for i := range chunks {
		if p.status != nil && p.status.IsChunkDone(i) {
			resultPath := filepath.Join(p.outputDir, "results", fmt.Sprintf("result_%03d.json", i+1))
			if data, err := os.ReadFile(resultPath); err == nil {
				var r ChunkResult
				if err := json.Unmarshal(data, &r); err == nil {
					results[i] = r
					emitter.Emit(model.Event{Type: EventCheckpointRestore, Data: map[string]string{"stage": fmt.Sprintf("chunk %d", i+1)}})
					continue
				}
			}
		}

		if data, err := loadResultFromCheckpoint(p.checkpointDir, i); err == nil && data.Index == i {
			results[i] = data
			p.writeResultJSON(data)
			if p.status != nil {
				p.status.ChunkDone(i)
			}
			emitter.Emit(model.Event{Type: EventCheckpointRestore, Data: map[string]string{"stage": fmt.Sprintf("chunk %d", i+1)}})
			continue
		}
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	errCh := make(chan error, len(chunks))

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	numWorkers := p.config.MaxConcurrency
	if numWorkers <= 0 {
		numWorkers = 3
	}

	type job struct {
		index int
		text  string
	}
	jobs := make(chan job, len(chunks))

	total := len(chunks)
	emitter.Emit(model.Event{Type: EventProcessingStart, Data: map[string]int{"total": total}})

	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				if ctx.Err() != nil {
					return
				}

				emitter.Emit(model.Event{Type: EventProcessingChunk, Data: map[string]int{"chunk": j.index + 1, "total": total}})

				r, err := p.processSingle(ctx, j.text)
				if err != nil {
					errCh <- fmt.Errorf("chunk %d: %w", j.index+1, err)
					if p.status != nil {
						p.status.ChunkFailed(j.index, err.Error())
					}
					continue
				}
				r.Index = j.index
				r.Content = j.text

				mu.Lock()
				results[j.index] = r
				p.writeResultJSON(r)
				mu.Unlock()

				if p.status != nil {
					p.status.ChunkDone(j.index)
				}

				emitter.Emit(model.Event{Type: EventProcessingChunkDone, Data: map[string]int{"chunk": j.index + 1}})
			}
		}()
	}

	for i, chunk := range chunks {
		if results[i].Summary != "" || results[i].Type != "" {
			continue
		}
		jobs <- job{index: i, text: chunk}
	}
	close(jobs)

	wg.Wait()
	close(errCh)

	var errs []error
	for err := range errCh {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("processing failed (%d errors): %v", len(errs), errs[0])
	}

	return results, nil
}

func (p *Pipeline) processSingle(ctx context.Context, text string) (ChunkResult, error) {
	result := ChunkResult{}

	callCtx, cancel := p.timeoutCtx(ctx)
	defer cancel()
	start := time.Now()
	resp, err := p.llmClient.Chat(callCtx, model.ChatRequest{
		Messages: []model.Message{
			{Role: "system", Content: chunkPrompt},
			{Role: "user", Content: text},
		},
	})
	_ = start

	if err != nil {
		return result, err
	}

	parsed := parsePromptResult(resp.Message.Content)
	mergePromptResult(&result, parsed)

	return result, nil
}

func parsePromptResult(response string) ChunkResult {
	var r ChunkResult
	lines := strings.Split(response, "\n")
	for _, line := range lines {
		lower := strings.ToLower(strings.TrimSpace(line))
		if strings.HasPrefix(lower, "type:") {
			r.Type = strings.TrimSpace(line[5:])
		} else if strings.HasPrefix(lower, "title:") {
			r.Title = strings.TrimSpace(line[6:])
		} else if strings.HasPrefix(lower, "topic:") {
			r.Topic = strings.TrimSpace(line[6:])
			r.Topics = parseCommaList(line[6:])
		} else if strings.HasPrefix(lower, "summary:") {
			r.Summary = strings.TrimSpace(line[8:])
		} else if strings.HasPrefix(lower, "key_points:") {
			r.KeyPoints = parseCommaList(line[11:])
		} else if strings.HasPrefix(lower, "source_quote:") {
			r.SourceQuote = strings.TrimSpace(line[13:])
		} else if strings.HasPrefix(lower, "my_takeaway:") {
			r.MyTakeaway = strings.TrimSpace(line[12:])
		} else if strings.HasPrefix(lower, "confidence:") {
			val := strings.TrimSpace(line[11:])
			if f, err := strconv.ParseFloat(val, 64); err == nil {
				r.Confidence = f
			}
		} else if strings.HasPrefix(lower, "applicability:") {
			r.Applicability = strings.TrimSpace(line[15:])
		}
	}
	return r
}

func parseCommaList(raw string) []string {
	var items []string
	for _, item := range strings.Split(raw, ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			items = append(items, item)
		}
	}
	return items
}

func mergePromptResult(dst *ChunkResult, src ChunkResult) {
	if dst.Title == "" {
		dst.Title = src.Title
	}
	if dst.Type == "" {
		dst.Type = src.Type
	}
	if dst.Topic == "" {
		dst.Topic = src.Topic
	}
	dst.Topics = append(dst.Topics, src.Topics...)
	if dst.Summary == "" {
		dst.Summary = src.Summary
	}
	dst.KeyPoints = append(dst.KeyPoints, src.KeyPoints...)
	if dst.SourceQuote == "" {
		dst.SourceQuote = src.SourceQuote
	}
	if dst.MyTakeaway == "" {
		dst.MyTakeaway = src.MyTakeaway
	}
	if dst.Confidence == 0 {
		dst.Confidence = src.Confidence
	}
	if dst.Applicability == "" {
		dst.Applicability = src.Applicability
	}
	dst.Steps = append(dst.Steps, src.Steps...)
}

func loadResultFromCheckpoint(dir string, index int) (ChunkResult, error) {
	if dir == "" {
		return ChunkResult{}, fmt.Errorf("no checkpoint dir")
	}
	all := loadResults(dir)
	for _, r := range all {
		if r.Index == index && r.Summary != "" {
			return r, nil
		}
	}
	return ChunkResult{}, fmt.Errorf("not found")
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

func saveResults(dir string, results []ChunkResult) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create checkpoint dir: %w", err)
	}
	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal results: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "results.json"), data, 0644); err != nil {
		return fmt.Errorf("write results: %w", err)
	}
	return nil
}