package pipeline

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

type Stage string

const (
	StageExtracted    Stage = "extracted"
	StageCleaned      Stage = "cleaned"
	StageSplit        Stage = "split"
	StageProcessing   Stage = "processing"
	StageHolistic     Stage = "holistic"
	StageCausal       Stage = "causal"
	StageContextEnrich Stage = "context_enrichment"
	StageDone        Stage = "done"
	StageFailed      Stage = "failed"
)

type ChunkStatus struct {
	Index       int       `yaml:"index"`
	Done        bool      `yaml:"done"`
	Failed      bool      `yaml:"failed,omitempty"`
	Error       string    `yaml:"error,omitempty"`
	StartedAt   time.Time `yaml:"started_at,omitempty"`
	CompletedAt time.Time `yaml:"completed_at,omitempty"`
	DurationMs  int64     `yaml:"duration_ms,omitempty"`
	Cost        float64   `yaml:"cost,omitempty"`
	TokensIn    int       `yaml:"tokens_in,omitempty"`
	TokensOut   int       `yaml:"tokens_out,omitempty"`
	Model       string    `yaml:"model,omitempty"`
	LatencyMs   int64     `yaml:"latency_ms,omitempty"`
	Stage       string    `yaml:"stage,omitempty"`
}

type StageCost struct {
	Calls     int     `yaml:"calls"`
	Cost      float64 `yaml:"cost"`
	TokensIn  int     `yaml:"tokens_in"`
	TokensOut int     `yaml:"tokens_out"`
}

type StageTiming struct {
	StartedAt   time.Time `yaml:"started_at"`
	CompletedAt time.Time `yaml:"completed_at,omitempty"`
	DurationMs  int64     `yaml:"duration_ms,omitempty"`
}

type ModelUsage struct {
	Model     string  `yaml:"model"`
	Calls     int     `yaml:"calls"`
	TokensIn  int     `yaml:"tokens_in"`
	TokensOut int     `yaml:"tokens_out"`
	Cost      float64 `yaml:"cost"`
}

type PipelineStatus struct {
	mu              sync.Mutex
	filePath        string
	Source          string              `yaml:"source"`
	Stage           Stage               `yaml:"stage"`
	ChunksTotal     int                 `yaml:"chunks_total"`
	Chunks          []ChunkStatus       `yaml:"chunks,omitempty"`
	FailedAt        string              `yaml:"failed_at,omitempty"`
	StartedAt       time.Time           `yaml:"started_at"`
	UpdatedAt       time.Time           `yaml:"updated_at"`
	CompletedAt     time.Time           `yaml:"completed_at,omitempty"`
	Duration        int64               `yaml:"duration_seconds,omitempty"`
	DurationHuman   string              `yaml:"duration_human,omitempty"`
	TotalCost       float64             `yaml:"total_cost,omitempty"`
	TotalTokensIn   int                 `yaml:"total_tokens_in,omitempty"`
	TotalTokensOut  int                 `yaml:"total_tokens_out,omitempty"`
	TotalCharsIn    int                 `yaml:"total_chars_in,omitempty"`
	TotalCharsOut   int                 `yaml:"total_chars_out,omitempty"`
	ChunksDone      int                 `yaml:"chunks_done,omitempty"`
	ChunksFailed    int                 `yaml:"chunks_failed,omitempty"`
	SuccessRate     float64             `yaml:"success_rate,omitempty"`
	AvgLatencyMs    int64               `yaml:"avg_latency_ms,omitempty"`
	AvgTokensIn     int                 `yaml:"avg_tokens_in,omitempty"`
	AvgTokensOut    int                 `yaml:"avg_tokens_out,omitempty"`
	AvgCost         float64             `yaml:"avg_cost,omitempty"`
	AvgLatencyHuman string              `yaml:"avg_latency_human,omitempty"`
	StageBreakdown  map[string]StageCost `yaml:"stage_breakdown,omitempty"`
	StageTiming     map[string]StageTiming `yaml:"stage_timing,omitempty"`
	ModelsUsed      []ModelUsage        `yaml:"models_used,omitempty"`
}

func LoadStatus(dir string) *PipelineStatus {
	path := filepath.Join(dir, "status.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var s PipelineStatus
	if err := yaml.Unmarshal(data, &s); err != nil {
		return nil
	}
	s.filePath = path
	return &s
}

func NewStatus(dir, source string) *PipelineStatus {
	now := time.Now()
	s := &PipelineStatus{
		filePath:  filepath.Join(dir, "status.yaml"),
		Source:    source,
		Stage:     StageExtracted,
		StartedAt: now,
		UpdatedAt: now,
	}
	_ = os.MkdirAll(dir, 0755)
	s.Save()
	return s
}

func (s *PipelineStatus) SetStage(stage Stage) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Stage = stage
	s.UpdatedAt = time.Now()
	s.Save()
}

func (s *PipelineStatus) SetChunks(total int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ChunksTotal = total
	oldChunks := s.Chunks
	s.Chunks = make([]ChunkStatus, total)
	for i := range s.Chunks {
		if i < len(oldChunks) {
			s.Chunks[i] = oldChunks[i]
		} else {
			s.Chunks[i] = ChunkStatus{Index: i}
		}
	}
	s.UpdatedAt = time.Now()
	s.Save()
}

func (s *PipelineStatus) ChunkStarted(index int, stage string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if index >= 0 && index < len(s.Chunks) {
		s.Chunks[index].StartedAt = time.Now()
		s.Chunks[index].Stage = stage
	}
	s.UpdatedAt = time.Now()
	s.Save()
}

func (s *PipelineStatus) ChunkDone(index int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if index >= 0 && index < len(s.Chunks) {
		now := time.Now()
		existing := s.Chunks[index]
		durationMs := int64(0)
		if !existing.StartedAt.IsZero() {
			durationMs = now.Sub(existing.StartedAt).Milliseconds()
		}
		s.Chunks[index] = ChunkStatus{
			Index:       index,
			Done:        true,
			StartedAt:   existing.StartedAt,
			CompletedAt: now,
			DurationMs:  durationMs,
			Cost:        existing.Cost,
			TokensIn:    existing.TokensIn,
			TokensOut:   existing.TokensOut,
			Model:       existing.Model,
			LatencyMs:   existing.LatencyMs,
			Stage:       existing.Stage,
		}
	}
	s.UpdatedAt = time.Now()
	s.Save()
}

func (s *PipelineStatus) ChunkFailed(index int, errMsg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if index >= 0 && index < len(s.Chunks) {
		now := time.Now()
		existing := s.Chunks[index]
		durationMs := int64(0)
		if !existing.StartedAt.IsZero() {
			durationMs = now.Sub(existing.StartedAt).Milliseconds()
		}
		s.Chunks[index] = ChunkStatus{
			Index:       index,
			Failed:      true,
			Error:       errMsg,
			StartedAt:   existing.StartedAt,
			CompletedAt: now,
			DurationMs:  durationMs,
			Cost:        existing.Cost,
			TokensIn:    existing.TokensIn,
			TokensOut:   existing.TokensOut,
			Model:       existing.Model,
			LatencyMs:   existing.LatencyMs,
			Stage:       existing.Stage,
		}
	}
	s.UpdatedAt = time.Now()
	s.Save()
}

func (s *PipelineStatus) SetFailed(msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Stage = StageFailed
	s.FailedAt = msg
	s.UpdatedAt = time.Now()
	s.Save()
}

func (s *PipelineStatus) SetCompleted() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.CompletedAt = time.Now()
	s.Stage = StageDone
	s.UpdatedAt = s.CompletedAt
	s.CalculateStats()
	s.Save()
}

func (s *PipelineStatus) StageStarted(stage string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.StageTiming == nil {
		s.StageTiming = make(map[string]StageTiming)
	}
	s.StageTiming[stage] = StageTiming{
		StartedAt: time.Now(),
	}
	s.UpdatedAt = time.Now()
	s.Save()
}

func (s *PipelineStatus) StageCompleted(stage string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.StageTiming == nil {
		s.StageTiming = make(map[string]StageTiming)
	}
	now := time.Now()
	st := s.StageTiming[stage]
	st.CompletedAt = now
	if !st.StartedAt.IsZero() {
		st.DurationMs = now.Sub(st.StartedAt).Milliseconds()
	}
	s.StageTiming[stage] = st
	s.UpdatedAt = now
	s.Save()
}

func (s *PipelineStatus) CalculateStats() {
	if s.CompletedAt.IsZero() || s.StartedAt.IsZero() {
		return
	}

	dur := s.CompletedAt.Sub(s.StartedAt)
	s.Duration = int64(dur.Seconds())
	s.DurationHuman = FormatDuration(dur)

	var totalLatency int64
	var totalTokensIn, totalTokensOut, totalCharsIn, totalCharsOut int
	var totalCost float64
	modelMap := make(map[string]*ModelUsage)

	for _, ch := range s.Chunks {
		if ch.Done {
			s.ChunksDone++
			totalLatency += ch.DurationMs
			totalTokensIn += ch.TokensIn
			totalTokensOut += ch.TokensOut
			totalCost += ch.Cost

			if ch.Model != "" {
				if mu, ok := modelMap[ch.Model]; ok {
					mu.Calls++
					mu.TokensIn += ch.TokensIn
					mu.TokensOut += ch.TokensOut
					mu.Cost += ch.Cost
				} else {
					modelMap[ch.Model] = &ModelUsage{
						Model:     ch.Model,
						Calls:     1,
						TokensIn:  ch.TokensIn,
						TokensOut: ch.TokensOut,
						Cost:      ch.Cost,
					}
				}
			}
		} else if ch.Failed {
			s.ChunksFailed++
		}
	}

	if s.ChunksDone > 0 {
		s.AvgLatencyMs = totalLatency / int64(s.ChunksDone)
		s.AvgTokensIn = totalTokensIn / s.ChunksDone
		s.AvgTokensOut = totalTokensOut / s.ChunksDone
		s.AvgCost = totalCost / float64(s.ChunksDone)
		s.AvgLatencyHuman = FormatDuration(time.Duration(s.AvgLatencyMs) * time.Millisecond)
	}

	if s.ChunksTotal > 0 {
		s.SuccessRate = float64(s.ChunksDone) / float64(s.ChunksTotal)
	}

	s.TotalCharsIn = totalCharsIn
	s.TotalCharsOut = totalCharsOut

	s.ModelsUsed = make([]ModelUsage, 0, len(modelMap))
	for _, mu := range modelMap {
		s.ModelsUsed = append(s.ModelsUsed, *mu)
	}
}

func FormatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm %ds", days, hours, minutes, seconds)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm %ds", hours, minutes, seconds)
	}
	if minutes > 0 {
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}
	return fmt.Sprintf("%ds", seconds)
}

func (s *PipelineStatus) CompletedChunks() []int {
	s.mu.Lock()
	defer s.mu.Unlock()
	var done []int
	for _, c := range s.Chunks {
		if c.Done {
			done = append(done, c.Index)
		}
	}
	return done
}

func (s *PipelineStatus) IsChunkDone(index int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if index < 0 || index >= len(s.Chunks) {
		return false
	}
	return s.Chunks[index].Done
}

func (s *PipelineStatus) Save() {
	if s.filePath == "" {
		return
	}
	data, err := yaml.Marshal(s)
	if err != nil {
		return
	}
	_ = os.WriteFile(s.filePath, data, 0644)
}
