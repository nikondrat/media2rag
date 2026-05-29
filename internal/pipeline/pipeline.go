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
	runID         string
	tracer        Tracer
	model         string
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

	modelName := ""
	if client != nil {
		if o, ok := client.(interface{ Model() string }); ok {
			modelName = o.Model()
		}
	}

	return &Pipeline{config: cfg, llmClient: client, model: modelName}
}

func (p *Pipeline) SetCheckpointDir(dir string) {
	p.checkpointDir = dir
}

func (p *Pipeline) SetTracer(t Tracer) {
	p.tracer = t
}

func (p *Pipeline) SetRunID(id string) {
	p.runID = id
}

func (p *Pipeline) Run(ctx context.Context, ec model.ExtractedContent, emitter events.EventEmitter) (*model.RAGDocument, error) {
	p.source = ec.Source
	p.docType = ec.DocType
	p.author = ec.Author
	p.language = ec.Language

	emitter.Emit(model.Event{Type: EventPipelineStart, Data: map[string]int{"text_length": len(ec.Content)}})

	stageScore := 0.95
	errStr := ""
	totalTokens := 0

	emitter.Emit(model.Event{Type: EventCompressionStart})
	startCompress := time.Now()
	cleaned, err := p.compress(ctx, ec.Content, emitter)
	compressMs := int(time.Since(startCompress).Milliseconds())
	if err != nil {
		return nil, fmt.Errorf("compressor: %w", err)
	}
	emitter.Emit(model.Event{Type: EventCompressionDone, Data: map[string]int{"cleaned_length": len(cleaned)}})
	totalTokens += len(cleaned) / 4

	if p.tracer != nil && p.runID != "" {
		p.tracer.SaveStage(TraceEntry{
			RunID: p.runID, StageName: "compress", Seq: 1,
			Prompt: cleaningPrompt, Response: cleaned,
			TokensIn: len(ec.Content) / 4, TokensOut: len(cleaned) / 4,
			LatencyMs: compressMs, Score: stageScore, Model: p.model,
		})
		p.tracer.BroadcastEvent("stage_complete", map[string]interface{}{
			"run_id": p.runID, "stage": "compress", "score": stageScore, "latency_ms": compressMs,
		})
	}

	emitter.Emit(model.Event{Type: EventSplitting})
	startSplit := time.Now()
	chunks := p.splitText(cleaned)
	splitMs := int(time.Since(startSplit).Milliseconds())
	emitter.Emit(model.Event{Type: EventSplitDone, Data: map[string]int{"chunks": len(chunks)}})

	if p.tracer != nil && p.runID != "" {
		p.tracer.SaveStage(TraceEntry{
			RunID: p.runID, StageName: "split", Seq: 2,
			TokensIn: len(cleaned) / 4, TokensOut: 0,
			LatencyMs: splitMs, Score: 1.0, Model: "",
		})
		p.tracer.BroadcastEvent("stage_complete", map[string]interface{}{
			"run_id": p.runID, "stage": "split", "score": 1.0, "latency_ms": splitMs,
		})
	}

	emitter.Emit(model.Event{Type: EventProcessingStart, Data: map[string]int{"total": len(chunks)}})
	startProcess := time.Now()
	results, err := p.processChunks(ctx, chunks, emitter)
	processMs := int(time.Since(startProcess).Milliseconds())
	if err != nil {
		errStr = err.Error()
		if p.tracer != nil && p.runID != "" {
			p.tracer.SaveStage(TraceEntry{
				RunID: p.runID, StageName: "process", Seq: 3,
				Prompt: chunkPrompt, Response: errStr,
				TokensIn: len(cleaned) / 4, TokensOut: 0,
				LatencyMs: processMs, Score: 0, Model: p.model, Error: errStr,
			})
		}
		return nil, fmt.Errorf("processor: %w", err)
	}
	emitter.Emit(model.Event{Type: EventProcessingDone})

	processResp := fmt.Sprintf("processed %d chunks, %d results", len(chunks), len(results))
	if p.tracer != nil && p.runID != "" {
		p.tracer.SaveStage(TraceEntry{
			RunID: p.runID, StageName: "process", Seq: 3,
			Prompt: chunkPrompt, Response: processResp,
			TokensIn: len(cleaned) / 4, TokensOut: totalTokens,
			LatencyMs: processMs, Score: stageScore, Model: p.model,
		})
		p.tracer.BroadcastEvent("stage_complete", map[string]interface{}{
			"run_id": p.runID, "stage": "process", "score": stageScore, "latency_ms": processMs,
		})
	}

	var holistic struct {
		coreThesis string
		domains    []string
	}
	if p.config.HolisticAnalysis {
		emitter.Emit(model.Event{Type: EventHolisticAnalysis})
		startHolistic := time.Now()
		holistic.coreThesis, holistic.domains, err = p.holisticAnalysis(ctx, cleaned)
		holisticMs := int(time.Since(startHolistic).Milliseconds())
		if err != nil {
			if p.tracer != nil && p.runID != "" {
				p.tracer.SaveLLMCall(p.runID, p.model, "holistic", len(cleaned)/4, 0, holisticMs, 0, holisticPrompt, "", "error", err.Error())
			}
			return nil, fmt.Errorf("holistic: %w", err)
		}
		emitter.Emit(model.Event{Type: EventHolisticDone})

		if p.tracer != nil && p.runID != "" {
			holisticResp := fmt.Sprintf("core_thesis: %s\ndomains: %s", holistic.coreThesis, strings.Join(holistic.domains, ", "))
			p.tracer.SaveLLMCall(p.runID, p.model, "holistic", len(cleaned)/4, len(holisticResp)/4, holisticMs, 0, holisticPrompt, holisticResp, "success", "")
			p.tracer.SaveStage(TraceEntry{
				RunID: p.runID, StageName: "holistic", Seq: 4,
				Prompt: holisticPrompt, Response: holisticResp,
				TokensIn: len(cleaned) / 4, TokensOut: 0,
				LatencyMs: holisticMs, Score: 1.0, Model: p.model,
			})
			p.tracer.BroadcastEvent("stage_complete", map[string]interface{}{
				"run_id": p.runID, "stage": "holistic", "score": 1.0, "latency_ms": holisticMs,
			})
		}
	}

	emitter.Emit(model.Event{Type: EventAssembling})
	startAssemble := time.Now()
	doc := assemble(results, cleaned, assembleOpts{
		source:     p.source,
		docType:    p.docType,
		author:     p.author,
		language:   p.language,
		coreThesis: holistic.coreThesis,
		domains:    holistic.domains,
	})
	assembleMs := int(time.Since(startAssemble).Milliseconds())

	if p.tracer != nil && p.runID != "" {
		p.tracer.SaveStage(TraceEntry{
			RunID: p.runID, StageName: "assemble", Seq: 5,
			TokensIn: totalTokens, TokensOut: len(doc.Markdown) / 4,
			LatencyMs: assembleMs, Score: 1.0, Model: "",
		})
		p.tracer.BroadcastEvent("stage_complete", map[string]interface{}{
			"run_id": p.runID, "stage": "assemble", "score": 1.0, "latency_ms": assembleMs,
		})
	}

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
