package pipeline

import "time"

type TraceEntry struct {
	RunID      string
	StageName  string
	Seq        int
	Prompt     string
	Response   string
	TokensIn   int
	TokensOut  int
	LatencyMs  int
	Score      float64
	Model      string
	Error      string
}

type Tracer interface {
	SaveStage(entry TraceEntry)
	SaveLLMCall(runID, model, operation string, promptTokens, completionTokens, latencyMs int, cost float64, prompt, response, status, errMsg string)
	SaveRunComplete(runID string, score float64, totalTokens, totalLatencyMs int, totalCost float64, err string)
	SaveRunStart(runID, source, sourceType string)
	BroadcastEvent(typ string, data map[string]interface{})
}

type noopTracer struct{}

func (*noopTracer) SaveStage(TraceEntry)                              {}
func (*noopTracer) SaveLLMCall(string, string, string, int, int, int, float64, string, string, string, string) {}
func (*noopTracer) SaveRunComplete(string, float64, int, int, float64, string) {}
func (*noopTracer) SaveRunStart(string, string, string)                   {}
func (*noopTracer) BroadcastEvent(string, map[string]interface{})         {}

type StageTiming struct {
	Start    time.Time
	Duration time.Duration
	Name     string
	Seq      int
}
