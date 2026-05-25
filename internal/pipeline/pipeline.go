package pipeline

import (
	"context"
	"fmt"
	"strings"
	"time"

	"media2rag/internal/events"
	"media2rag/internal/llm"
	"media2rag/internal/model"
)

const (
	EventPipelineStart        = "pipeline_start"
	EventCompressionStart     = "compression_start"
	EventCleaningPart         = "cleaning_part"
	EventCompressionDone      = "compression_done"
	EventSplitting            = "splitting"
	EventSplitDone            = "split_done"
	EventProcessingStart      = "processing_start"
	EventProcessingChunk      = "processing_chunk"
	EventProcessingChunkDone  = "processing_chunk_done"
	EventProcessingDone       = "processing_done"
	EventHolisticAnalysis     = "holistic_analysis"
	EventHolisticDone         = "holistic_done"
	EventAssembling           = "assembling"
	EventPipelineCompleted    = "pipeline_completed"
	EventCheckpointRestore    = "checkpoint_restore"
)

const holisticPrompt = `Analyze the following document and extract:
- core_thesis: The single central thesis or main argument of the entire document
- domains: Comma-separated list of knowledge domains (e.g. technology, business, law, science, medicine)

Return in this exact format:
core_thesis: <thesis statement>
domains: <domain1, domain2, ...>`

type ChunkResult struct {
	Index        int            `json:"index"`
	Title        string         `json:"title"`
	Topics       []string       `json:"topics"`
	Summary      string         `json:"summary"`
	Content      string         `json:"content"`
	Claims       []model.Claim  `json:"claims,omitempty"`
	MentalModels []string       `json:"mental_models,omitempty"`
	KeyTerms     []model.KeyTerm `json:"key_terms,omitempty"`
	CoreThesis   string         `json:"core_thesis,omitempty"`
	Takeaways    []string       `json:"takeaways,omitempty"`
	Domains      []string       `json:"domains,omitempty"`
}

type PipelineConfig struct {
	ChunkSize           int           `json:"chunk_size"`
	ChunkOverlap        int           `json:"chunk_overlap"`
	MaxConcurrency      int           `json:"max_concurrency"`
	LLMTimeout          time.Duration `json:"llm_timeout"`
	ExtractClaims       bool          `json:"extract_claims"`
	ExtractMentalModels bool          `json:"extract_mental_models"`
	ExtractKeyTerms     bool          `json:"extract_key_terms"`
	ExtractCoreThesis   bool          `json:"extract_core_thesis"`
	ExtractTakeaways    bool          `json:"extract_takeaways"`
	HolisticAnalysis    bool          `json:"holistic_analysis"`
}

func DefaultConfig() PipelineConfig {
	return PipelineConfig{
		ChunkSize:           2000,
		ChunkOverlap:        200,
		MaxConcurrency:      3,
		LLMTimeout:          120 * time.Second,
		ExtractClaims:       true,
		ExtractMentalModels: true,
		ExtractKeyTerms:     true,
		ExtractCoreThesis:   true,
		ExtractTakeaways:    true,
		HolisticAnalysis:    true,
	}
}

type Pipeline struct {
	config        PipelineConfig
	llmClient     llm.LLMClient
	checkpointDir string
	source        string
	docType       string
	author        string
	language      string
}

func New(cfg PipelineConfig, client llm.LLMClient) *Pipeline {
	if cfg.ChunkSize <= 0 {
		cfg.ChunkSize = 2000
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
	return &Pipeline{config: cfg, llmClient: client}
}

func (p *Pipeline) SetCheckpointDir(dir string) {
	p.checkpointDir = dir
}

func (p *Pipeline) Run(ctx context.Context, ec model.ExtractedContent, emitter events.EventEmitter) (*model.RAGDocument, error) {
	p.source = ec.Source
	p.docType = ec.DocType
	p.author = ec.Author
	p.language = ec.Language

	emitter.Emit(model.Event{Type: EventPipelineStart, Data: map[string]int{"text_length": len(ec.Content)}})

	emitter.Emit(model.Event{Type: EventCompressionStart})
	cleaned, err := p.compress(ctx, ec.Content, emitter)
	if err != nil {
		return nil, fmt.Errorf("compressor: %w", err)
	}
	emitter.Emit(model.Event{Type: EventCompressionDone, Data: map[string]int{"cleaned_length": len(cleaned)}})

	emitter.Emit(model.Event{Type: EventSplitting})
	chunks := p.splitText(cleaned)
	emitter.Emit(model.Event{Type: EventSplitDone, Data: map[string]int{"chunks": len(chunks)}})

	emitter.Emit(model.Event{Type: EventProcessingStart, Data: map[string]int{"total": len(chunks)}})
	results, err := p.processChunks(ctx, chunks, emitter)
	if err != nil {
		return nil, fmt.Errorf("processor: %w", err)
	}
	emitter.Emit(model.Event{Type: EventProcessingDone})

	var holistic struct {
		coreThesis string
		domains    []string
	}
	if p.config.HolisticAnalysis {
		emitter.Emit(model.Event{Type: EventHolisticAnalysis})
		holistic.coreThesis, holistic.domains, err = p.holisticAnalysis(ctx, cleaned)
		if err != nil {
			return nil, fmt.Errorf("holistic: %w", err)
		}
		emitter.Emit(model.Event{Type: EventHolisticDone})
	}

	emitter.Emit(model.Event{Type: EventAssembling})
	doc := assemble(results, cleaned, assembleOpts{
		source:     p.source,
		docType:    p.docType,
		author:     p.author,
		language:   p.language,
		coreThesis: holistic.coreThesis,
		domains:    holistic.domains,
	})

	emitter.Emit(model.Event{Type: EventPipelineCompleted, Data: map[string]interface{}{
		"title":      doc.Metadata.Title,
		"word_count": doc.Metadata.WordCount,
		"topics":     doc.Metadata.Topics,
	}})

	return doc, nil
}

func (p *Pipeline) RunString(ctx context.Context, rawText string, emitter events.EventEmitter) (*model.RAGDocument, error) {
	return p.Run(ctx, model.ExtractedContent{Content: rawText}, emitter)
}

func (p *Pipeline) holisticAnalysis(ctx context.Context, cleaned string) (string, []string, error) {
	callCtx, cancel := p.timeoutCtx(ctx)
	defer cancel()
	resp, err := p.llmClient.Chat(callCtx, model.ChatRequest{
		Messages: []model.Message{
			{Role: "system", Content: holisticPrompt},
			{Role: "user", Content: cleaned},
		},
	})
	if err != nil {
		return "", nil, err
	}

	coreThesis := ""
	var domains []string
	lines := strings.Split(resp.Message.Content, "\n")
	for _, line := range lines {
		lower := strings.ToLower(strings.TrimSpace(line))
		if strings.HasPrefix(lower, "core_thesis:") {
			coreThesis = strings.TrimSpace(line[12:])
		} else if strings.HasPrefix(lower, "domains:") {
			raw := strings.TrimSpace(line[8:])
			for _, d := range strings.Split(raw, ",") {
				d = strings.TrimSpace(d)
				if d != "" {
					domains = append(domains, d)
				}
			}
		}
	}
	return coreThesis, domains, nil
}
