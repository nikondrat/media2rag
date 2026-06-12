package pipeline

import (
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
	Index     int     `yaml:"index"`
	Done      bool    `yaml:"done"`
	Failed    bool    `yaml:"failed,omitempty"`
	Error     string  `yaml:"error,omitempty"`
	Cost      float64 `yaml:"cost,omitempty"`
	TokensIn  int     `yaml:"tokens_in,omitempty"`
	TokensOut int     `yaml:"tokens_out,omitempty"`
	Model     string  `yaml:"model,omitempty"`
	LatencyMs int64   `yaml:"latency_ms,omitempty"`
}

type StageCost struct {
	Calls     int     `yaml:"calls"`
	Cost      float64 `yaml:"cost"`
	TokensIn  int     `yaml:"tokens_in"`
	TokensOut int     `yaml:"tokens_out"`
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
	TotalCost       float64             `yaml:"total_cost,omitempty"`
	TotalTokensIn   int                 `yaml:"total_tokens_in,omitempty"`
	TotalTokensOut  int                 `yaml:"total_tokens_out,omitempty"`
	StageBreakdown  map[string]StageCost `yaml:"stage_breakdown,omitempty"`
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

func (s *PipelineStatus) ChunkDone(index int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if index >= 0 && index < len(s.Chunks) {
		existing := s.Chunks[index]
		s.Chunks[index] = ChunkStatus{
			Index:     index,
			Done:      true,
			Cost:      existing.Cost,
			TokensIn:  existing.TokensIn,
			TokensOut: existing.TokensOut,
			Model:     existing.Model,
			LatencyMs: existing.LatencyMs,
		}
	}
	s.UpdatedAt = time.Now()
	s.Save()
}

func (s *PipelineStatus) ChunkFailed(index int, errMsg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if index >= 0 && index < len(s.Chunks) {
		existing := s.Chunks[index]
		s.Chunks[index] = ChunkStatus{
			Index:     index,
			Failed:    true,
			Error:     errMsg,
			Cost:      existing.Cost,
			TokensIn:  existing.TokensIn,
			TokensOut: existing.TokensOut,
			Model:     existing.Model,
			LatencyMs: existing.LatencyMs,
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