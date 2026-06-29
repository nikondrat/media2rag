package pipeline

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"media2rag/internal/llm"
	"media2rag/internal/model"
)

const (
	EventPipelineStart       = "pipeline_start"
	EventPreClean            = "pre_clean"
	EventPreCleanDone        = "pre_clean_done"
	EventCleaningPart        = "cleaning_part"
	EventSplitting           = "splitting"
	EventSplitDone           = "split_done"
	EventProcessingStart     = "processing_start"
	EventProcessingChunk     = "processing_chunk"
	EventProcessingChunkDone = "processing_chunk_done"
	EventProcessingDone      = "processing_done"
	EventHolisticAnalysis    = "holistic_analysis"
	EventHolisticDone        = "holistic_done"
	EventCausalExtraction    = "causal_extraction"
	EventCausalDone          = "causal_done"
	EventContextEnrich       = "context_enrichment"
	EventContextEnrichDone   = "context_enrichment_done"
	EventAssembling          = "assembling"
	EventPipelineCompleted   = "pipeline_completed"
	EventCheckpointRestore   = "checkpoint_restore"
	EventProcessingRetry     = "processing_retry"
)

type ChunkResult struct {
	Index         int                `json:"index"`
	Title         string             `json:"title"`
	Type          string             `json:"type"`
	Topic         string             `json:"topic"`
	Topics        []string           `json:"topics"`
	Summary       string             `json:"summary"`
	Content       string             `json:"content"`
	Context       string             `json:"context,omitempty"`
	KeyPoints     []string           `json:"key_points,omitempty"`
	SourceQuote   string             `json:"source_quote,omitempty"`
	MyTakeaway    string             `json:"my_takeaway,omitempty"`
	Confidence    float64            `json:"confidence,omitempty"`
	Applicability string             `json:"applicability,omitempty"`
	Steps         []string           `json:"steps,omitempty"`
	Domains       []string           `json:"domains,omitempty"`
	CausalChains  []model.CausalLink `json:"causal_chains,omitempty"`
}

type PipelineConfig struct {
	ChunkSize          int           `json:"chunk_size"`
	ChunkOverlap       int           `json:"chunk_overlap"`
	MaxConcurrency     int           `json:"max_concurrency"`
	MaxFileConcurrency int           `json:"max_file_concurrency"`
	LLMTimeout         time.Duration `json:"llm_timeout"`
	MaxTokens          int           `json:"max_tokens"`
	FrequencyPenalty   float64       `json:"frequency_penalty"`
	PresencePenalty    float64       `json:"presence_penalty"`
	HolisticAnalysis   bool          `json:"holistic_analysis"`
}

func DefaultConfig() PipelineConfig {
	return PipelineConfig{
		ChunkSize:        1500,
		ChunkOverlap:     200,
		MaxConcurrency:   3,
		LLMTimeout:       120 * time.Second,
		MaxTokens:        8192,
		FrequencyPenalty: 0.3,
		PresencePenalty:  0.3,
		HolisticAnalysis: true,
	}
}

type Pipeline struct {
	config        PipelineConfig
	llmClient     llm.LLMClient
	checkpointDir string
	outputDir     string
	status        *PipelineStatus
	source        string
	docType       string
	author        string
	language      string
	model         string
	recorder      model.TelemetryRecorder
}

func New(cfg PipelineConfig, client llm.LLMClient) *Pipeline {
	if cfg.ChunkSize <= 0 {
		cfg.ChunkSize = 1500
	}
	if cfg.ChunkOverlap <= 0 {
		cfg.ChunkOverlap = 200
	}
	if cfg.MaxConcurrency <= 0 {
		cfg.MaxConcurrency = 3
	}
	if cfg.LLMTimeout <= 0 {
		cfg.LLMTimeout = 120 * time.Second
	}

	modelName := ""
	if client != nil {
		if o, ok := client.(interface{ Model() string }); ok {
			modelName = o.Model()
		}
	}

	return &Pipeline{
		config:    cfg,
		llmClient: client,
		model:     modelName,
	}
}

func (p *Pipeline) SetCheckpointDir(dir string) {
	p.checkpointDir = dir
}

func (p *Pipeline) SetOutputDir(dir string) {
	p.outputDir = dir
}

func (p *Pipeline) SetRecorder(r model.TelemetryRecorder) {
	p.recorder = r
}

func (p *Pipeline) initOutputDirs() {
	_ = os.MkdirAll(p.outputDir, 0755)
	_ = os.MkdirAll(filepath.Join(p.outputDir, "intermediate"), 0755)
	_ = os.MkdirAll(filepath.Join(p.outputDir, "chunks"), 0755)
	_ = os.MkdirAll(filepath.Join(p.outputDir, "results"), 0755)
	_ = os.MkdirAll(filepath.Join(p.outputDir, "output"), 0755)
}

func (p *Pipeline) wrapWithTelemetry() {
	statusRec := NewStatusRecorder(p.status)
	teeRec := model.NewTeeRecorder(p.recorder, statusRec)
	p.llmClient = llm.NewInstrumentedClient(p.llmClient, nil, teeRec)
}

func (p *Pipeline) saveIntermediate(name, content string) {
	if p.outputDir != "" {
		_ = os.WriteFile(filepath.Join(p.outputDir, "intermediate", name), []byte(content), 0644)
	}
}

func (p *Pipeline) saveChunks(chunks []string) {
	if p.outputDir != "" {
		for i, chunk := range chunks {
			name := fmt.Sprintf("chunk_%03d.md", i+1)
			_ = os.WriteFile(filepath.Join(p.outputDir, "chunks", name), []byte(chunk), 0644)
		}
	}
}

func (p *Pipeline) setFailed(err error) {
	if p.status != nil {
		p.status.SetFailed(err.Error())
	}
}
