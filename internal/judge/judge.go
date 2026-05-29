package judge

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"media2rag/internal/dashboard"
	"media2rag/internal/llm"
	"media2rag/internal/model"
)

type Type string

const (
	TypeQuality     Type = "quality"
	TypeRelevance   Type = "relevance"
	TypeFaithfulness Type = "faithfulness"
	TypeHelpfulness Type = "helpfulness"
)

var AllTypes = []Type{TypeQuality, TypeRelevance, TypeFaithfulness, TypeHelpfulness}

var Weights = map[Type]float64{
	TypeQuality:     0.4,
	TypeRelevance:   0.3,
	TypeFaithfulness: 0.2,
	TypeHelpfulness: 0.1,
}

type Runner struct {
	client  llm.LLMClient
	model   string
	tracer  *dashboard.Tracer
	mu      sync.Mutex
}

type Config struct {
	Model    string
	Enabled  bool
}

func NewRunner(client llm.LLMClient, model string, tracer *dashboard.Tracer) *Runner {
	return &Runner{
		client: client,
		model:  model,
		tracer: tracer,
	}
}

func (r *Runner) SetModel(model string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.model = model
}

func promptForType(t Type) string {
	switch t {
	case TypeQuality:
		return `Evaluate the quality of the following answer. Consider correctness, completeness, and clarity.

Rate on a scale of 1-10.

Return your evaluation using the TypedBlock format:

> score: type=quality
<score as a number between 0.0 and 1.0>
<

> reasoning
<brief explanation of the score>
<`
	case TypeRelevance:
		return `Evaluate the relevance of the following answer. Does it directly address the user's question?

Rate on a scale of 1-10.

Return your evaluation using the TypedBlock format:

> score: type=relevance
<score as a number between 0.0 and 1.0>
<

> reasoning
<brief explanation of the score>
<`
	case TypeFaithfulness:
		return `Evaluate the faithfulness of the following answer. Does it contain information not present in the provided context? Is it hallucinated?

Rate on a scale of 1-10. Higher score = more faithful (less hallucination).

Return your evaluation using the TypedBlock format:

> score: type=faithfulness
<score as a number between 0.0 and 1.0>
<

> reasoning
<brief explanation of the score>
<`
	case TypeHelpfulness:
		return `Evaluate the helpfulness of the following answer. How useful is it for the user?

Rate on a scale of 1-10.

Return your evaluation using the TypedBlock format:

> score: type=helpfulness
<score as a number between 0.0 and 1.0>
<

> reasoning
<brief explanation of the score>
<`
	default:
		return ""
	}
}

func (r *Runner) Evaluate(ctx context.Context, t Type, query, answer, contextText string) (*dashboard.JudgeEvaluation, error) {
	start := time.Now()

	prompt := promptForType(t)
	userMsg := fmt.Sprintf("Query: %s\n\nAnswer: %s\n\nContext: %s", query, answer, contextText)

	blocks, err := r.client.ChatAndParse(ctx, model.ChatRequest{
		Messages: []model.Message{
			{Role: "system", Content: prompt},
			{Role: "user", Content: userMsg},
		},
	})
	latencyMs := int(time.Since(start).Milliseconds())

	if err != nil {
		return nil, fmt.Errorf("judge %s: %w", t, err)
	}

	score := 0.5
	reasoning := ""

	for _, block := range blocks {
		if block.Type == "score" {
			score = parseScore(block.Content)
		}
		if block.Type == "reasoning" {
			reasoning = block.Content
		}
	}

	passed := score >= 0.7

	var tokens int
	for _, b := range blocks {
		tokens += len(strings.Fields(b.Content))
	}

	eval := &dashboard.JudgeEvaluation{
		RunID:     "",
		JudgeModel: r.model,
		JudgeType: string(t),
		Prompt:    userMsg,
		Response:  formatBlocks(blocks),
		Score:     score,
		Reasoning: reasoning,
		Passed:    passed,
		LatencyMs: latencyMs,
		TokensUsed: tokens,
		CreatedAt: time.Now(),
	}

	return eval, nil
}

func (r *Runner) EvaluateAll(ctx context.Context, runID, query, answer, contextText string) ([]dashboard.JudgeEvaluation, float64) {
	var evals []dashboard.JudgeEvaluation
	var results []struct {
		eval dashboard.JudgeEvaluation
		err  error
	}

	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, t := range AllTypes {
		wg.Add(1)
		go func(jt Type) {
			defer wg.Done()
			eval, err := r.Evaluate(ctx, jt, query, answer, contextText)
			mu.Lock()
			if err == nil && eval != nil {
				eval.RunID = runID
				results = append(results, struct {
					eval dashboard.JudgeEvaluation
					err  error
				}{eval: *eval})
			}
			mu.Unlock()
		}(t)
	}
	wg.Wait()

	totalWeight := 0.0
	weightedScore := 0.0

	for _, res := range results {
		jt := Type(res.eval.JudgeType)
		weight := Weights[jt]
		weightedScore += res.eval.Score * weight
		totalWeight += weight

		if res.eval.RunID == "" {
			res.eval.RunID = runID
		}
		if res.eval.CreatedAt.IsZero() {
			res.eval.CreatedAt = time.Now()
		}
		evals = append(evals, res.eval)

		res.eval.RunID = runID

		if r.tracer != nil {
			r.tracer.SaveJudgeEvaluation(&res.eval)
		}
	}

	if totalWeight == 0 {
		return evals, 0.5
	}

	return evals, math.Round(weightedScore/totalWeight*100) / 100
}

func parseScore(text string) float64 {
	text = strings.TrimSpace(text)

	re := regexp.MustCompile(`(\d+)\s*/\s*10`)
	if m := re.FindStringSubmatch(text); len(m) >= 2 {
		if v, err := strconv.ParseFloat(m[1], 64); err == nil {
			return v / 10.0
		}
	}

	re2 := regexp.MustCompile(`(\d+(?:\.\d+)?)`)
	if m := re2.FindStringSubmatch(text); len(m) >= 2 {
		if v, err := strconv.ParseFloat(m[1], 64); err == nil {
			if v > 1.0 {
				return v / 10.0
			}
			return v
		}
	}

	return 0.5
}

func formatBlocks(blocks []model.TypedBlock) string {
	var sb strings.Builder
	for _, b := range blocks {
		sb.WriteString(fmt.Sprintf("> %s", b.Type))
		if len(b.Params) > 0 {
			sb.WriteString(": ")
			first := true
			for k, v := range b.Params {
				if !first {
					sb.WriteString(", ")
				}
				sb.WriteString(fmt.Sprintf("%s=%s", k, v))
				first = false
			}
		}
		sb.WriteString("\n")
		sb.WriteString(b.Content)
		sb.WriteString("\n<\n")
	}
	return sb.String()
}
