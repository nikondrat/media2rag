package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"media2rag/internal/events"
	"media2rag/internal/model"
)

const cleaningPrompt = `You are a text cleaning assistant. Clean the following text by:
1. Removing timestamps (e.g. "0:00", "12:34", "[Music]", "[Applause]")
2. Removing advertisements and promotional content
3. Removing duplicate lines or paragraphs
4. Removing OCR artifacts and garbled text
5. Preserving all meaningful content including technical terms and code

Return only the cleaned text with no additional commentary.`

const maxCompressLen = 28000

func (p *Pipeline) compress(ctx context.Context, text string, emitter events.EventEmitter) (string, error) {
	if p.checkpointDir != "" {
		cp := filepath.Join(p.checkpointDir, "compressed.md")
		if data, err := os.ReadFile(cp); err == nil {
			emitter.Emit(model.Event{Type: EventCheckpointRestore, Data: map[string]string{"stage": "compressor"}})
			return string(data), nil
		}
	}

	var cleaned string
	var err error
	if len(text) <= maxCompressLen {
		cleaned, err = p.cleanSingle(ctx, text)
	} else {
		cleaned, err = p.cleanLarge(ctx, text, emitter)
	}
	if err != nil {
		return "", err
	}

	if p.checkpointDir != "" {
		os.MkdirAll(p.checkpointDir, 0755)
		os.WriteFile(filepath.Join(p.checkpointDir, "compressed.md"), []byte(cleaned), 0644)
	}

	return cleaned, nil
}

func (p *Pipeline) cleanLarge(ctx context.Context, text string, emitter events.EventEmitter) (string, error) {
	parts := splitParagraphParts(text, maxCompressLen)
	var cleanedParts []string
	for i, part := range parts {
		emitter.Emit(model.Event{Type: EventCleaningPart, Data: map[string]int{"part": i + 1, "total": len(parts)}})
		result, err := p.cleanSingle(ctx, part)
		if err != nil {
			return "", err
		}
		cleanedParts = append(cleanedParts, result)
	}
	return strings.Join(cleanedParts, "\n\n"), nil
}

func (p *Pipeline) cleanSingle(ctx context.Context, text string) (string, error) {
	callCtx, cancel := p.timeoutCtx(ctx)
	defer cancel()
	start := time.Now()
	resp, err := p.llmClient.Chat(callCtx, model.ChatRequest{
		Messages: []model.Message{
			{Role: "system", Content: cleaningPrompt},
			{Role: "user", Content: text},
		},
	})
	latency := int(time.Since(start).Milliseconds())
	if err != nil {
		if p.tracer != nil && p.runID != "" {
			p.tracer.SaveLLMCall(p.runID, p.model, "compress", len(text)/4, 0, latency, 0, cleaningPrompt, "", "error", err.Error())
		}
		return "", err
	}
	if p.tracer != nil && p.runID != "" {
		p.tracer.SaveLLMCall(p.runID, p.model, "compress", len(text)/4, len(resp.Message.Content)/4, latency, 0, cleaningPrompt, resp.Message.Content, "success", "")
	}
	return resp.Message.Content, nil
}

func (p *Pipeline) timeoutCtx(ctx context.Context) (context.Context, context.CancelFunc) {
	if p.config.LLMTimeout > 0 {
		return context.WithTimeout(ctx, p.config.LLMTimeout)
	}
	return ctx, func() {}
}

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
