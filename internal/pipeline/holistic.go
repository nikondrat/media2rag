package pipeline

import (
	"context"
	"strings"

	"media2rag/internal/model"
)

func (p *Pipeline) holisticAnalysis(ctx context.Context, results []ChunkResult) (string, []string, error) {
	summaries := collectSummaries(results)
	if len(summaries) == 0 {
		return "", nil, nil
	}

	combined := strings.Join(summaries, "\n\n")
	raw, err := p.retryLLMCall(ctx, "holistic", func(callCtx context.Context) (string, error) {
		resp, err := p.llmClient.Chat(callCtx, model.ChatRequest{
			Messages: []model.Message{
				{Role: "system", Content: holisticPrompt},
				{Role: "user", Content: combined},
			},
		})
		if err != nil {
			return "", err
		}
		return resp.Message.Content, nil
	})
	if err != nil {
		return "", nil, err
	}

	return parseHolisticResult(raw)
}

func collectSummaries(results []ChunkResult) []string {
	var summaries []string
	for _, r := range results {
		if r.Summary != "" {
			summaries = append(summaries, r.Summary)
		}
	}
	return summaries
}

func parseHolisticResult(raw string) (string, []string, error) {
	coreThesis := ""
	var domains []string

	lines := strings.Split(raw, "\n")
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
