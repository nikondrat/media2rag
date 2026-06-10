package model

import "time"

type LLMTelemetry struct {
	Source          string    `json:"source"`
	Stage           string    `json:"stage"`
	ChunkIndex      int       `json:"chunk_index"`
	RetryAttempt    int       `json:"retry_attempt"`
	Model           string    `json:"model"`
	PromptTokens    int       `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	PromptChars     int       `json:"prompt_chars"`
	CompletionChars int       `json:"completion_chars"`
	Cost            float64   `json:"cost"`
	LatencyMs       int64     `json:"latency_ms"`
	Success         bool      `json:"success"`
	Error           string    `json:"error,omitempty"`
	Timestamp       time.Time `json:"timestamp"`
}

type TelemetryRecorder interface {
	Record(t LLMTelemetry)
}

type TeeRecorder struct {
	recorders []TelemetryRecorder
}

func NewTeeRecorder(recorders ...TelemetryRecorder) *TeeRecorder {
	return &TeeRecorder{recorders: recorders}
}

func (t *TeeRecorder) Record(telemetry LLMTelemetry) {
	for _, r := range t.recorders {
		r.Record(telemetry)
	}
}
