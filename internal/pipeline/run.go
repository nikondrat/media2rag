package pipeline

import (
	"context"
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
		if p.recorder != nil {
			p.wrapWithTelemetry()
		}
	}

	emitter.Emit(model.Event{Type: EventPipelineStart, Data: map[string]int{"text_length": len(ec.Content)}})
	p.saveIntermediate("raw.md", ec.Content)

	cleaned := ec.Content
	if ec.DocType == "transcript" {
		var err error
		cleaned, err = p.preClean(ctx, ec.Content, emitter)
		if err != nil {
			p.setFailed(err)
			return nil, fmt.Errorf("pre-clean: %w", err)
		}
		p.saveIntermediate("cleaned.md", cleaned)
		emitter.Emit(model.Event{Type: EventPreCleanDone, Data: map[string]int{"text_length": len(cleaned)}})
	} else {
		p.saveIntermediate("cleaned.md", cleaned)
		emitter.Emit(model.Event{Type: EventPreCleanDone, Data: map[string]int{"text_length": len(cleaned)}})
	}

	emitter.Emit(model.Event{Type: EventSplitting})
	rawChunks, err := p.splitText(cleaned)
	if err != nil {
		p.setFailed(err)
		return nil, fmt.Errorf("splitter: %w", err)
	}
	p.saveChunks(rawChunks)
	emitter.Emit(model.Event{Type: EventSplitDone, Data: map[string]int{"chunks": len(rawChunks)}})

	if p.status != nil {
		p.status.SetChunks(len(rawChunks))
		p.status.SetStage(StageProcessing)
	}

	results, err := p.processChunks(ctx, rawChunks, emitter)
	if err != nil {
		p.setFailed(err)
		return nil, fmt.Errorf("processor: %w", err)
	}
	emitter.Emit(model.Event{Type: EventProcessingDone})

	holistic := p.runHolisticAnalysis(ctx, results, emitter)
	causalChains, preconditions, counterfactuals := p.runCausalExtraction(ctx, results, emitter)

	emitter.Emit(model.Event{Type: EventContextEnrich})
	if err := p.contextualEnrich(ctx, results, holistic.coreThesis, emitter); err != nil {
		emitter.Emit(model.Event{Type: "context_enrichment_error", Data: map[string]string{"error": err.Error()}})
	}
	emitter.Emit(model.Event{Type: EventContextEnrichDone})

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

type holisticResult struct {
	coreThesis string
	domains    []string
}

func (p *Pipeline) runHolisticAnalysis(ctx context.Context, results []ChunkResult, emitter events.EventEmitter) holisticResult {
	if !p.config.HolisticAnalysis {
		return holisticResult{}
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
