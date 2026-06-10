package pipeline

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"media2rag/internal/model"
)

type JSONLRecorder struct {
	mu   sync.Mutex
	file *os.File
}

func NewJSONLRecorder(outputDir string) (*JSONLRecorder, error) {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, err
	}
	path := filepath.Join(outputDir, "telemetry.jsonl")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	return &JSONLRecorder{file: f}, nil
}

func (r *JSONLRecorder) Record(t model.LLMTelemetry) {
	r.mu.Lock()
	defer r.mu.Unlock()

	data, err := json.Marshal(t)
	if err != nil {
		return
	}
	data = append(data, '\n')
	_, _ = r.file.Write(data)
}

func (r *JSONLRecorder) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.file != nil {
		return r.file.Close()
	}
	return nil
}

type StatusRecorder struct {
	status *PipelineStatus
}

func NewStatusRecorder(status *PipelineStatus) *StatusRecorder {
	return &StatusRecorder{status: status}
}

func (r *StatusRecorder) Record(t model.LLMTelemetry) {
	r.status.mu.Lock()
	defer r.status.mu.Unlock()

	if t.ChunkIndex >= 0 && t.ChunkIndex < len(r.status.Chunks) {
		ch := r.status.Chunks[t.ChunkIndex]
		if t.Success && t.Cost > 0 {
			ch.Cost = t.Cost
			ch.TokensIn = t.PromptTokens
			ch.TokensOut = t.CompletionTokens
			ch.Model = t.Model
			ch.LatencyMs = t.LatencyMs
		}
		r.status.Chunks[t.ChunkIndex] = ch
	}

	r.status.TotalCost += t.Cost
	r.status.TotalTokensIn += t.PromptTokens
	r.status.TotalTokensOut += t.CompletionTokens

	if t.Stage != "" {
		if r.status.StageBreakdown == nil {
			r.status.StageBreakdown = make(map[string]StageCost)
		}
		s := r.status.StageBreakdown[t.Stage]
		s.Calls++
		s.Cost += t.Cost
		s.TokensIn += t.PromptTokens
		s.TokensOut += t.CompletionTokens
		r.status.StageBreakdown[t.Stage] = s
	}

	r.status.UpdatedAt = time.Now()
	r.status.Save()
}
