package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"media2rag/internal/events"
	"media2rag/internal/model"
)

func (p *Pipeline) Run(ctx context.Context, ec model.ExtractedContent, emitter events.EventEmitter) (*model.RAGDocument, error) {
	p.source = ec.Source
	p.docType = ec.DocType
	p.author = ec.Author
	p.language = ec.Language

	if p.outputDir != "" {
		p.initOutputDirs()
		p.status = LoadStatus(p.outputDir)
		if p.status == nil {
			p.status = NewStatus(p.outputDir, ec.Source)
		}
		if p.status.Stage == StageFailed {
			p.status.Stage = StageExtracted
			p.status.Save()
		}
		if p.recorder != nil {
			p.wrapWithTelemetry()
		}
	}

	emitter.Emit(model.Event{Type: EventPipelineStart, Data: map[string]int{"text_length": len(ec.Content)}})

	cleaned := ec.Content

	// === EXTRACT ===
	p.trackStage("extract", func() {
		p.saveIntermediate("raw.md", ec.Content)
	})

	// === CLEAN ===
	if p.status != nil && stagePast(p.status.Stage, StageCleaned) {
		if data, err := os.ReadFile(filepath.Join(p.outputDir, "intermediate", "cleaned.md")); err == nil {
			cleaned = string(data)
			emitter.Emit(model.Event{Type: EventCheckpointRestore, Data: map[string]string{"stage": "cleaned"}})
		}
	} else {
		p.trackStage("clean", func() {
			if ec.DocType == "transcript" {
				var err error
				cleaned, err = p.preClean(ctx, ec.Content, emitter)
				if err != nil {
					p.setFailed(err)
					return
				}
				p.saveIntermediate("cleaned.md", cleaned)
				emitter.Emit(model.Event{Type: EventPreCleanDone, Data: map[string]int{"text_length": len(cleaned)}})
			} else {
				p.saveIntermediate("cleaned.md", cleaned)
				emitter.Emit(model.Event{Type: EventPreCleanDone, Data: map[string]int{"text_length": len(cleaned)}})
			}
		})
	}

	if p.status != nil && p.status.Stage == StageFailed {
		return nil, fmt.Errorf("pipeline failed during clean stage")
	}

	// === SPLIT ===
	var rawChunks []string
	if p.status != nil && stagePast(p.status.Stage, StageSplit) {
		chunksDir := filepath.Join(p.outputDir, "chunks")
		if entries, err := os.ReadDir(chunksDir); err == nil && len(entries) > 0 {
			rawChunks = make([]string, len(entries))
			for _, entry := range entries {
				if entry.IsDir() {
					continue
				}
				var idx int
				if _, err := fmt.Sscanf(entry.Name(), "chunk_%d.md", &idx); err == nil && idx >= 1 && idx <= len(rawChunks) {
					data, err := os.ReadFile(filepath.Join(chunksDir, entry.Name()))
					if err == nil {
						rawChunks[idx-1] = string(data)
					}
				}
			}
			emitter.Emit(model.Event{Type: EventCheckpointRestore, Data: map[string]string{"stage": "split"}})
		}
	} else {
		emitter.Emit(model.Event{Type: EventSplitting})
		p.trackStage("split", func() {
			var err error
			rawChunks, err = p.splitText(cleaned)
			if err != nil {
				p.setFailed(err)
				return
			}
			p.saveChunks(rawChunks)
			emitter.Emit(model.Event{Type: EventSplitDone, Data: map[string]int{"chunks": len(rawChunks)}})
		})
	}

	if p.status != nil && p.status.Stage == StageFailed {
		return nil, fmt.Errorf("pipeline failed during split stage")
	}

	// === PROCESS ===
	var results []ChunkResult
	if p.status != nil && stagePast(p.status.Stage, StageProcessing) {
		results = p.loadResultsFromCheckpoint()
		if len(results) > 0 {
			emitter.Emit(model.Event{Type: EventCheckpointRestore, Data: map[string]string{"stage": "processing"}})
		}
	}

	if len(results) == 0 {
		if p.status != nil {
			p.status.SetChunks(len(rawChunks))
			p.status.SetStage(StageProcessing)
		}
		var err error
		results, err = p.processChunks(ctx, rawChunks, emitter)
		if err != nil {
			p.setFailed(err)
			return nil, fmt.Errorf("processor: %w", err)
		}
		emitter.Emit(model.Event{Type: EventProcessingDone})
	}

	// === HOLISTIC ===
	holistic := p.runHolisticAnalysis(ctx, results, emitter)

	// === CAUSAL ===
	causalChains, preconditions, counterfactuals := p.runCausalExtraction(ctx, results, emitter)

	// === CONTEXT ENRICHMENT ===
	emitter.Emit(model.Event{Type: EventContextEnrich})
	if err := p.contextualEnrich(ctx, results, holistic.coreThesis, emitter); err != nil {
		emitter.Emit(model.Event{Type: "context_enrichment_error", Data: map[string]string{"error": err.Error()}})
	}
	emitter.Emit(model.Event{Type: EventContextEnrichDone})

	// === VISION ===
	var imageDescs []ImageDescription
	if len(ec.Images) > 0 {
		visionProc := NewVisionProcessor(p.llmClient, p.config.MaxConcurrency)
		imageDescs, _ = visionProc.Process(ctx, ec.Images, emitter)
	}

	// === ASSEMBLE ===
	emitter.Emit(model.Event{Type: EventAssembling})
	doc := assemble(results, assembleOpts{
		source:          p.source,
		docType:         p.docType,
		author:          p.author,
		language:        p.language,
		domains:         holistic.domains,
		coreThesis:      holistic.coreThesis,
		causalChains:    causalChains,
		preconditions:   preconditions,
		counterfactuals: counterfactuals,
		images:          imageDescs,
	})
	doc.CleanedText = cleaned
	doc.Metadata.CoreThesis = holistic.coreThesis

	if p.outputDir != "" {
		_ = os.WriteFile(filepath.Join(p.outputDir, "output", "final.md"), []byte(doc.Markdown), 0644)
		p.status.SetCompleted()
	}

	emitter.Emit(model.Event{Type: EventPipelineCompleted, Data: map[string]interface{}{
		"title":      doc.Metadata.Title,
		"word_count": doc.Metadata.WordCount,
		"topics":     doc.Metadata.Topics,
	}})

	return doc, nil
}

func (p *Pipeline) trackStage(name string, fn func()) {
	stage := Stage(name)
	if p.status != nil && stagePast(p.status.Stage, stage) {
		return
	}
	if p.status != nil {
		p.status.StageStarted(name)
	}
	fn()
	if p.status != nil {
		p.status.StageCompleted(name)
	}
}

func (p *Pipeline) loadResultsFromCheckpoint() []ChunkResult {
	if p.outputDir == "" {
		return nil
	}

	resultsDir := filepath.Join(p.outputDir, "results")
	entries, err := os.ReadDir(resultsDir)
	if err != nil || len(entries) == 0 {
		return nil
	}

	var results []ChunkResult
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(resultsDir, entry.Name()))
		if err != nil {
			continue
		}
		var r ChunkResult
		if err := json.Unmarshal(data, &r); err != nil {
			continue
		}
		results = append(results, r)
	}

	if len(results) == 0 {
		return nil
	}

	return results
}

func (p *Pipeline) RunString(ctx context.Context, rawText string, emitter events.EventEmitter) (*model.RAGDocument, error) {
	return p.Run(ctx, model.ExtractedContent{Content: rawText}, emitter)
}

type holisticResult struct {
	coreThesis string
	domains    []string
}

func (p *Pipeline) runHolisticAnalysis(ctx context.Context, results []ChunkResult, emitter events.EventEmitter) holisticResult {
	if !p.config.HolisticAnalysis {
		return holisticResult{}
	}

	if p.status != nil && stagePast(p.status.Stage, StageHolistic) {
		if data, err := os.ReadFile(filepath.Join(p.outputDir, "intermediate", "holistic.md")); err == nil {
			lines := strings.SplitN(string(data), "\n\nDomains: ", 2)
			coreThesis := lines[0]
			var domains []string
			if len(lines) > 1 {
				domains = strings.Split(lines[1], ", ")
			}
			emitter.Emit(model.Event{Type: EventCheckpointRestore, Data: map[string]string{"stage": "holistic"}})
			return holisticResult{coreThesis: coreThesis, domains: domains}
		}
	}

	if p.status != nil {
		p.status.SetStage(StageHolistic)
	}
	emitter.Emit(model.Event{Type: EventHolisticAnalysis})

	coreThesis, domains, err := p.holisticAnalysis(ctx, results)
	if err != nil {
		p.setFailed(err)
		emitter.Emit(model.Event{Type: EventHolisticDone})
		return holisticResult{}
	}

	p.saveIntermediate("holistic.md", coreThesis+"\n\nDomains: "+strings.Join(domains, ", "))
	emitter.Emit(model.Event{Type: EventHolisticDone})

	return holisticResult{coreThesis: coreThesis, domains: domains}
}

func (p *Pipeline) runCausalExtraction(ctx context.Context, results []ChunkResult, emitter events.EventEmitter) ([]model.CausalLink, []string, []string) {
	if !p.config.HolisticAnalysis {
		return nil, nil, nil
	}

	if p.status != nil && stagePast(p.status.Stage, StageCausal) {
		if data, err := os.ReadFile(filepath.Join(p.outputDir, "intermediate", "causal.md")); err == nil {
			emitter.Emit(model.Event{Type: EventCheckpointRestore, Data: map[string]string{"stage": "causal"}})
			causalChains, preconditions, counterfactuals := parseCausalOutput(string(data))
			return causalChains, preconditions, counterfactuals
		}
	}

	if p.status != nil {
		p.status.SetStage(StageCausal)
	}
	emitter.Emit(model.Event{Type: EventCausalExtraction})

	causalChains, preconditions, counterfactuals, err := p.causalExtraction(ctx, results)
	if err != nil {
		emitter.Emit(model.Event{Type: "causal_extraction_error", Data: map[string]string{"error": err.Error()}})
	}

	if len(causalChains) > 0 {
		causalData := fmt.Sprintf("## Causal Chains\n\n%s\n\n## Preconditions\n\n%s\n\n## Counterfactuals\n\n%s",
			formatCausalChains(causalChains),
			strings.Join(preconditions, "\n"),
			strings.Join(counterfactuals, "\n"))
		p.saveIntermediate("causal.md", causalData)
	}

	emitter.Emit(model.Event{Type: EventCausalDone})
	return causalChains, preconditions, counterfactuals
}

func parseCausalOutput(data string) ([]model.CausalLink, []string, []string) {
	var causalChains []model.CausalLink
	var preconditions []string
	var counterfactuals []string

	sections := strings.Split(data, "## ")
	for _, section := range sections {
		if strings.HasPrefix(section, "Causal Chains") {
			lines := strings.Split(strings.TrimPrefix(section, "Causal Chains\n\n"), "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line != "" {
					causalChains = append(causalChains, model.CausalLink{Cause: line, Effect: line})
				}
			}
		} else if strings.HasPrefix(section, "Preconditions") {
			lines := strings.Split(strings.TrimPrefix(section, "Preconditions\n\n"), "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line != "" {
					preconditions = append(preconditions, line)
				}
			}
		} else if strings.HasPrefix(section, "Counterfactuals") {
			lines := strings.Split(strings.TrimPrefix(section, "Counterfactuals\n\n"), "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line != "" {
					counterfactuals = append(counterfactuals, line)
				}
			}
		}
	}

	return causalChains, preconditions, counterfactuals
}
