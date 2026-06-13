package pipeline

import (
	"context"
	"fmt"
	"strings"

	"media2rag/internal/model"
)

var sectionHeaders = []string{"causal_chains:", "preconditions:", "counterfactuals:"}

func (p *Pipeline) causalExtraction(ctx context.Context, results []ChunkResult) ([]model.CausalLink, []string, []string, error) {
	summaries := collectCausalSummaries(results)
	if len(summaries) == 0 {
		return nil, nil, nil, nil
	}

	combined := strings.Join(summaries, "\n")
	raw, err := p.retryLLMCall(ctx, "causal", func(callCtx context.Context) (string, error) {
		resp, err := p.llmClient.Chat(callCtx, model.ChatRequest{
			Messages: []model.Message{
				{Role: "system", Content: causalExtractPrompt},
				{Role: "user", Content: combined},
			},
		})
		if err != nil {
			return "", err
		}
		return resp.Message.Content, nil
	})
	if err != nil {
		return nil, nil, nil, err
	}

	return parseCausalResult(raw)
}

func collectCausalSummaries(results []ChunkResult) []string {
	var summaries []string
	for _, r := range results {
		if r.Summary != "" {
			summaries = append(summaries, fmt.Sprintf("[chunk %d] %s", r.Index+1, r.Summary))
		}
		if len(r.KeyPoints) > 0 {
			summaries = append(summaries, "  Key points: "+strings.Join(r.KeyPoints, "; "))
		}
	}
	return summaries
}

func parseCausalResult(raw string) ([]model.CausalLink, []string, []string, error) {
	var chains []model.CausalLink
	var preconditions []string
	var counterfactuals []string

	lines := strings.Split(raw, "\n")
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
			currentChain = parseChainField(currentChain, lower, trimmed)
			if strings.HasPrefix(lower, "- ") {
				chains = append(chains, *currentChain)
				currentChain = nil
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

func parseChainField(chain *model.CausalLink, lower, trimmed string) *model.CausalLink {
	switch {
	case strings.HasPrefix(lower, "mechanism:"):
		chain.Mechanism = strings.TrimSpace(trimmed[10:])
	case strings.HasPrefix(lower, "effect:"):
		chain.Effect = strings.TrimSpace(trimmed[7:])
	case strings.HasPrefix(lower, "relation:"):
		chain.Relation = strings.TrimSpace(trimmed[9:])
	}
	return chain
}

func inSection(lines []string, currentLine string, sectionName string) bool {
	found := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)
		if lower == sectionName+":" {
			found = true
		}
		if trimmed == currentLine {
			return found
		}
		for _, h := range sectionHeaders {
			if strings.HasPrefix(lower, h) && lower != sectionName+":" {
				found = false
				break
			}
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
