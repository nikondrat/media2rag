package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"media2rag/internal/events"
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
)

const cleaningPrompt = `You are a text cleaning assistant. Clean the following text by:
1. Removing timestamps (e.g. "0:00", "12:34", "[Music]", "[Applause]")
2. Removing advertisements and promotional content
3. Removing duplicate lines or paragraphs
4. Removing OCR artifacts and garbled text
5. Preserving all meaningful content including technical terms and code

IMPORTANT: Preserve the original language of the text. Do NOT translate.
Return only the cleaned text with no additional commentary.`

const holisticPrompt = `Analyze the following document summaries and extract:
- core_thesis: The single central thesis or main argument of the entire document (in the original language of the document, 1-2 sentences)
- domains: Comma-separated list of knowledge domains relevant to the content (e.g. business, technology, marketing, psychology, management, entrepreneurship, personal-development, leadership)

Return in this exact format (using the original language of the document):
core_thesis: <thesis statement>
domains: <domain1, domain2, ...>`

const contextEnrichPrompt = `You are given a chunk of text extracted from a larger document, along with a brief summary of the entire document. Your task is to write a short context (1-2 sentences) that situates this chunk within the broader document, so that someone reading only this chunk can understand what document it comes from and what the chunk is about in context.

IMPORTANT:
- Write in the same language as the chunk
- Keep it concise: 1-2 sentences maximum
- Include the document topic/main theme
- Do NOT repeat the chunk content — only add surrounding context
- Do NOT add any headers, labels, or formatting — just the context sentence(s)

Example input:
Document summary: A lecture about scaling a construction business to 1M profit, covering sales funnels, lead generation, and team building.
Chunk: "The goal is to get 15-20 measurements per week. At the meeting, you propose a contract and a prepayment."

Example output:
This is from a lecture on scaling a construction business to 1 million rubles profit, discussing the key stage of the sales funnel — getting measurements and signing contracts with prepayment.`

const causalExtractPrompt = `Analyze the following document (chunk summaries in order) and extract causal relationships between concepts. Focus on:

1. causal_chains: Direct cause-effect relationships found in the content
   Format: cause -> mechanism -> effect
   Relation types: causes, enables, prevents, requires, correlates

2. preconditions: Conditions that make events/processes possible ("If X, then Y becomes possible")

3. counterfactuals: What would change if a key factor were removed ("Without X, Y would not happen")

Rules:
- Extract only relationships explicitly stated or strongly implied in the text
- Do NOT invent causal relationships — only extract from content
- Write in the same language as the source material
- Be specific: name the actual concepts, not generic terms

Return in this exact format:
causal_chains:
- cause: <cause>
  mechanism: <how it works>
  effect: <effect>
  relation: <causes|enables|prevents|requires|correlates>

- cause: <cause>
  mechanism: <how it works>
  effect: <effect>
  relation: <causes|enables|prevents|requires|correlates>

preconditions:
- <precondition 1>
- <precondition 2>

counterfactuals:
- <counterfactual 1>
- <counterfactual 2>`

type ChunkResult struct {
	Index         int              `json:"index"`
	Title         string           `json:"title"`
	Type          string           `json:"type"`
	Topic         string           `json:"topic"`
	Topics        []string         `json:"topics"`
	Summary       string           `json:"summary"`
	Content       string           `json:"content"`
	Context       string           `json:"context,omitempty"`
	KeyPoints     []string         `json:"key_points,omitempty"`
	SourceQuote   string           `json:"source_quote,omitempty"`
	MyTakeaway    string           `json:"my_takeaway,omitempty"`
	Confidence    float64          `json:"confidence,omitempty"`
	Applicability string           `json:"applicability,omitempty"`
	Steps         []string         `json:"steps,omitempty"`
	Domains       []string         `json:"domains,omitempty"`
	CausalChains  []model.CausalLink `json:"causal_chains,omitempty"`
}

type PipelineConfig struct {
	ChunkSize        int           `json:"chunk_size"`
	ChunkOverlap     int           `json:"chunk_overlap"`
	MaxConcurrency   int           `json:"max_concurrency"`
	LLMTimeout       time.Duration `json:"llm_timeout"`
	HolisticAnalysis bool          `json:"holistic_analysis"`
}

func DefaultConfig() PipelineConfig {
	return PipelineConfig{
		ChunkSize:        1500,
		ChunkOverlap:     200,
		MaxConcurrency:   3,
		LLMTimeout:       120 * time.Second,
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

func (p *Pipeline) Run(ctx context.Context, ec model.ExtractedContent, emitter events.EventEmitter) (*model.RAGDocument, error) {
	p.source = ec.Source
	p.docType = ec.DocType
	p.author = ec.Author
	p.language = ec.Language

	if p.outputDir != "" {
		_ = os.MkdirAll(p.outputDir, 0755)
		_ = os.MkdirAll(filepath.Join(p.outputDir, "intermediate"), 0755)
		_ = os.MkdirAll(filepath.Join(p.outputDir, "chunks"), 0755)
		_ = os.MkdirAll(filepath.Join(p.outputDir, "results"), 0755)
		_ = os.MkdirAll(filepath.Join(p.outputDir, "output"), 0755)
		p.status = NewStatus(p.outputDir, ec.Source)
	}

	emitter.Emit(model.Event{Type: EventPipelineStart, Data: map[string]int{"text_length": len(ec.Content)}})

	if p.outputDir != "" {
		rawPath := filepath.Join(p.outputDir, "intermediate", "raw.md")
		_ = os.WriteFile(rawPath, []byte(ec.Content), 0644)
	}

	cleaned, err := p.preClean(ctx, ec.Content, emitter)
	if err != nil {
		if p.status != nil {
			p.status.SetFailed(err.Error())
		}
		return nil, fmt.Errorf("pre-clean: %w", err)
	}

	if p.outputDir != "" {
		_ = os.WriteFile(filepath.Join(p.outputDir, "intermediate", "cleaned.md"), []byte(cleaned), 0644)
	}
	emitter.Emit(model.Event{Type: EventPreCleanDone, Data: map[string]int{"text_length": len(cleaned)}})

	emitter.Emit(model.Event{Type: EventSplitting})
	rawChunks, err := p.splitText(cleaned)
	if err != nil {
		if p.status != nil {
			p.status.SetFailed(err.Error())
		}
		return nil, fmt.Errorf("splitter: %w", err)
	}
	if p.outputDir != "" {
		for i, chunk := range rawChunks {
			name := fmt.Sprintf("chunk_%03d.md", i+1)
			_ = os.WriteFile(filepath.Join(p.outputDir, "chunks", name), []byte(chunk), 0644)
		}
	}
	emitter.Emit(model.Event{Type: EventSplitDone, Data: map[string]int{"chunks": len(rawChunks)}})

	if p.status != nil {
		p.status.SetChunks(len(rawChunks))
		p.status.SetStage(StageProcessing)
	}

	emitter.Emit(model.Event{Type: EventProcessingStart, Data: map[string]int{"total": len(rawChunks)}})
	results, err := p.processChunks(ctx, rawChunks, emitter)
	if err != nil {
		if p.status != nil {
			p.status.SetFailed(err.Error())
		}
		return nil, fmt.Errorf("processor: %w", err)
	}
	emitter.Emit(model.Event{Type: EventProcessingDone})

	var holistic struct {
		coreThesis string
		domains    []string
	}
	if p.config.HolisticAnalysis {
		if p.status != nil {
			p.status.SetStage(StageHolistic)
		}
		emitter.Emit(model.Event{Type: EventHolisticAnalysis})
		holistic.coreThesis, holistic.domains, err = p.holisticAnalysis(ctx, results)
		if err != nil {
			if p.status != nil {
				p.status.SetFailed(err.Error())
			}
			return nil, fmt.Errorf("holistic: %w", err)
		}
		if p.outputDir != "" {
			_ = os.WriteFile(filepath.Join(p.outputDir, "intermediate", "holistic.md"), []byte(holistic.coreThesis+"\n\nDomains: "+strings.Join(holistic.domains, ", ")), 0644)
		}
		emitter.Emit(model.Event{Type: EventHolisticDone})
	}

	var causalChains []model.CausalLink
	var preconditions []string
	var counterfactuals []string
	if p.config.HolisticAnalysis {
		if p.status != nil {
			p.status.SetStage(StageCausal)
		}
		emitter.Emit(model.Event{Type: EventCausalExtraction})
		causalChains, preconditions, counterfactuals, err = p.causalExtraction(ctx, results)
		if err != nil {
			emitter.Emit(model.Event{Type: "causal_extraction_error", Data: map[string]string{"error": err.Error()}})
		}
		if p.outputDir != "" && len(causalChains) > 0 {
			causalData := fmt.Sprintf("## Causal Chains\n\n%s\n\n## Preconditions\n\n%s\n\n## Counterfactuals\n\n%s",
				formatCausalChains(causalChains),
				strings.Join(preconditions, "\n"),
				strings.Join(counterfactuals, "\n"))
			_ = os.WriteFile(filepath.Join(p.outputDir, "intermediate", "causal.md"), []byte(causalData), 0644)
		}
		emitter.Emit(model.Event{Type: EventCausalDone})
	}

	emitter.Emit(model.Event{Type: EventContextEnrich})
	if err := p.contextualEnrich(ctx, results, holistic.coreThesis, emitter); err != nil {
		emitter.Emit(model.Event{Type: "context_enrichment_error", Data: map[string]string{"error": err.Error()}})
	}
	emitter.Emit(model.Event{Type: EventContextEnrichDone})

	emitter.Emit(model.Event{Type: EventAssembling})
	doc := assemble(results, assembleOpts{
		source:        p.source,
		docType:       p.docType,
		author:        p.author,
		language:      p.language,
		domains:       holistic.domains,
		coreThesis:    holistic.coreThesis,
		causalChains:  causalChains,
		preconditions: preconditions,
		counterfactuals: counterfactuals,
	})
	doc.CleanedText = cleaned
	doc.Metadata.CoreThesis = holistic.coreThesis

	if p.outputDir != "" {
		_ = os.WriteFile(filepath.Join(p.outputDir, "output", "final.md"), []byte(doc.Markdown), 0644)
		p.status.SetStage(StageDone)
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

func (p *Pipeline) contextualEnrich(ctx context.Context, results []ChunkResult, docContext string, emitter events.EventEmitter) error {
	docSummary := docContext
	if docSummary == "" {
		var topics []string
		for _, r := range results {
			if r.Topic != "" {
				topics = append(topics, r.Topic)
			}
		}
		if len(topics) > 0 {
			unique := map[string]bool{}
			var deduped []string
			for _, t := range topics {
				if !unique[t] {
					unique[t] = true
					deduped = append(deduped, t)
				}
			}
			docSummary = "Document covering: " + strings.Join(deduped[:min(5, len(deduped))], ", ")
		}
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	type job struct {
		index int
	}
	jobs := make(chan job, len(results))
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error

	numWorkers := p.config.MaxConcurrency
	if numWorkers <= 0 {
		numWorkers = 3
	}

	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				if ctx.Err() != nil {
					return
				}

				r := &results[j.index]
				if r.Content == "" || r.Summary == "" {
					continue
				}

				chunkContext, err := p.enrichSingle(ctx, docSummary, r)
				if err != nil {
					mu.Lock()
					if firstErr == nil {
						firstErr = err
						cancel()
					}
					mu.Unlock()
					return
				}

				mu.Lock()
				r.Context = chunkContext
				p.writeResultJSON(*r)
				mu.Unlock()

				emitter.Emit(model.Event{Type: EventContextEnrichDone, Data: map[string]int{"chunk": j.index + 1}})
			}
		}()
	}

	for i := range results {
		jobs <- job{index: i}
	}
	close(jobs)
	wg.Wait()

	return firstErr
}

func (p *Pipeline) enrichSingle(ctx context.Context, docSummary string, r *ChunkResult) (string, error) {
	callCtx, cancel := p.timeoutCtx(ctx)
	defer cancel()

	content := r.Content
	if len(content) > 1000 {
		content = content[:1000]
	}

	userMsg := fmt.Sprintf("Document summary: %s\n\nChunk content:\n%s", docSummary, content)
	resp, err := p.llmClient.Chat(callCtx, model.ChatRequest{
		Messages: []model.Message{
			{Role: "system", Content: contextEnrichPrompt},
			{Role: "user", Content: userMsg},
		},
	})
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(resp.Message.Content), nil
}

func (p *Pipeline) preClean(ctx context.Context, text string, emitter events.EventEmitter) (string, error) {
	if p.status != nil && p.status.Stage != "" && p.status.Stage != StageExtracted {
		if data, err := os.ReadFile(filepath.Join(p.outputDir, "intermediate", "cleaned.md")); err == nil {
			emitter.Emit(model.Event{Type: EventCheckpointRestore, Data: map[string]string{"stage": "cleaned"}})
			return string(data), nil
		}
	}

	if p.checkpointDir != "" {
		if data, err := loadCheckpointFile(p.checkpointDir, "cleaned.md"); err == nil {
			emitter.Emit(model.Event{Type: EventCheckpointRestore, Data: map[string]string{"stage": "cleaned"}})
			return string(data), nil
		}
	}

	emitter.Emit(model.Event{Type: EventPreClean})

	var cleaned string
	var err error

	if len(text) <= maxCompressLen {
		cleaned, err = p.cleanSinglePart(ctx, text)
		if err != nil {
			return "", err
		}
	} else {
		parts := splitParagraphParts(text, maxCompressLen)
		var cleanedParts []string
		for i, part := range parts {
			emitter.Emit(model.Event{Type: EventCleaningPart, Data: map[string]int{"part": i + 1, "total": len(parts)}})
			result, err := p.cleanSinglePart(ctx, part)
			if err != nil {
				return "", fmt.Errorf("clean part %d/%d: %w", i+1, len(parts), err)
			}
			cleanedParts = append(cleanedParts, result)
		}
		cleaned = strings.Join(cleanedParts, "\n\n")
	}

	if p.checkpointDir != "" {
		_ = saveCheckpointFile(p.checkpointDir, "cleaned.md", []byte(cleaned))
	}

	if p.status != nil {
		p.status.SetStage(StageCleaned)
	}

	return cleaned, nil
}

func (p *Pipeline) cleanSinglePart(ctx context.Context, text string) (string, error) {
	callCtx, cancel := p.timeoutCtx(ctx)
	defer cancel()
	resp, err := p.llmClient.Chat(callCtx, model.ChatRequest{
		Messages: []model.Message{
			{Role: "system", Content: cleaningPrompt},
			{Role: "user", Content: text},
		},
	})
	if err != nil {
		return "", err
	}
	return resp.Message.Content, nil
}

func (p *Pipeline) holisticAnalysis(ctx context.Context, results []ChunkResult) (string, []string, error) {
	var summaries []string
	for _, r := range results {
		if r.Summary != "" {
			summaries = append(summaries, r.Summary)
		}
	}
	if len(summaries) == 0 {
		return "", nil, nil
	}

	combined := strings.Join(summaries, "\n\n")
	callCtx, cancel := p.timeoutCtx(ctx)
	defer cancel()
	resp, err := p.llmClient.Chat(callCtx, model.ChatRequest{
		Messages: []model.Message{
			{Role: "system", Content: holisticPrompt},
			{Role: "user", Content: combined},
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

func (p *Pipeline) causalExtraction(ctx context.Context, results []ChunkResult) ([]model.CausalLink, []string, []string, error) {
	var summaries []string
	for _, r := range results {
		if r.Summary != "" {
			summaries = append(summaries, fmt.Sprintf("[chunk %d] %s", r.Index+1, r.Summary))
		}
		if len(r.KeyPoints) > 0 {
			summaries = append(summaries, "  Key points: "+strings.Join(r.KeyPoints, "; "))
		}
	}
	if len(summaries) == 0 {
		return nil, nil, nil, nil
	}

	combined := strings.Join(summaries, "\n")
	callCtx, cancel := p.timeoutCtx(ctx)
	defer cancel()
	resp, err := p.llmClient.Chat(callCtx, model.ChatRequest{
		Messages: []model.Message{
			{Role: "system", Content: causalExtractPrompt},
			{Role: "user", Content: combined},
		},
	})
	if err != nil {
		return nil, nil, nil, err
	}

	var chains []model.CausalLink
	var preconditions []string
	var counterfactuals []string

	lines := strings.Split(resp.Message.Content, "\n")
	var currentChain *model.CausalLink

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)

		if strings.HasPrefix(lower, "- cause:") {
			if currentChain != nil {
				chains = append(chains, *currentChain)
			}
			currentChain = &model.CausalLink{Cause: strings.TrimSpace(trimmed[8:])}
		} else if currentChain != nil {
			if strings.HasPrefix(lower, "mechanism:") {
				currentChain.Mechanism = strings.TrimSpace(trimmed[10:])
			} else if strings.HasPrefix(lower, "effect:") {
				currentChain.Effect = strings.TrimSpace(trimmed[7:])
			} else if strings.HasPrefix(lower, "relation:") {
				currentChain.Relation = strings.TrimSpace(trimmed[9:])
			} else if strings.HasPrefix(lower, "- ") {
				if currentChain != nil {
					chains = append(chains, *currentChain)
					currentChain = nil
				}
			}
		} else if strings.HasPrefix(lower, "- ") && !strings.HasPrefix(lower, "- cause:") {
			content := strings.TrimSpace(trimmed[2:])
			if inSection(lines, line, "preconditions") {
				preconditions = append(preconditions, content)
			} else if inSection(lines, line, "counterfactuals") {
				counterfactuals = append(counterfactuals, content)
			}
		}
	}
	if currentChain != nil {
		chains = append(chains, *currentChain)
	}

	return chains, preconditions, counterfactuals, nil
}

func inSection(lines []string, currentLine string, sectionName string) bool {
	found := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.ToLower(trimmed) == sectionName+":" {
			found = true
		}
		if trimmed == currentLine {
			return found
		}
		if strings.HasPrefix(strings.ToLower(trimmed), "causal_chains:") && sectionName != "causal_chains" {
			found = false
		}
	}
	return false
}

func formatCausalChains(chains []model.CausalLink) string {
	var b strings.Builder
	for _, c := range chains {
		if c.Mechanism != "" {
			b.WriteString(fmt.Sprintf("- %s → %s → %s (%s)\n", c.Cause, c.Mechanism, c.Effect, c.Relation))
		} else {
			b.WriteString(fmt.Sprintf("- %s → %s (%s)\n", c.Cause, c.Effect, c.Relation))
		}
	}
	return b.String()
}

func (p *Pipeline) timeoutCtx(ctx context.Context) (context.Context, context.CancelFunc) {
	if p.config.LLMTimeout > 0 {
		return context.WithTimeout(ctx, p.config.LLMTimeout)
	}
	return ctx, func() {}
}

func (p *Pipeline) writeResultJSON(result ChunkResult) {
	if p.outputDir == "" {
		return
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return
	}
	name := fmt.Sprintf("result_%03d.json", result.Index+1)
	_ = os.WriteFile(filepath.Join(p.outputDir, "results", name), data, 0644)
}

const maxCompressLen = 28000

func splitParagraphParts(text string, maxLen int) []string {
	if len(text) <= maxLen {
		return []string{text}
	}

	var parts []string
	for len(text) > 0 {
		if len(text) <= maxLen {
			parts = append(parts, text)
			break
		}

		cut := text[:maxLen]
		lastPara := strings.LastIndex(cut, "\n\n")
		if lastPara > maxLen/2 {
			parts = append(parts, text[:lastPara])
			text = text[lastPara:]
			continue
		}

		lastNewline := strings.LastIndex(cut, "\n")
		if lastNewline > maxLen/2 {
			parts = append(parts, text[:lastNewline])
			text = text[lastNewline:]
			continue
		}

		parts = append(parts, cut)
		text = text[maxLen:]
	}
	return parts
}

func loadCheckpointFile(dir, name string) ([]byte, error) {
	return os.ReadFile(filepath.Join(dir, name))
}

func saveCheckpointFile(dir, name string, data []byte) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create checkpoint dir: %w", err)
	}
	return os.WriteFile(filepath.Join(dir, name), data, 0644)
}