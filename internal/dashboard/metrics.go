package dashboard

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

type OverviewMetrics struct {
	OverallScore      float64               `json:"overallScore"`
	PipelinePassRate  float64               `json:"pipelinePassRate"`
	PipelineFailRate  float64               `json:"pipelineFailRate"`
	DocumentsProcessed int                  `json:"documentsProcessed"`
	Radar             map[string]float64    `json:"radar,omitempty"`
	Timeline          []TimelinePoint       `json:"timeline,omitempty"`
	TopFailures       []FailureEntry        `json:"topFailures,omitempty"`
	LatestPipeline    *LatestPipelineEntry  `json:"latestPipeline,omitempty"`
}

type TimelinePoint struct {
	Date     string  `json:"date"`
	AvgScore float64 `json:"avgScore"`
}

type FailureEntry struct {
	ID     string  `json:"id"`
	Source string  `json:"source"`
	Score  float64 `json:"score"`
}

type LatestPipelineEntry struct {
	ID          string             `json:"id"`
	Source      string             `json:"source"`
	Score       float64            `json:"score"`
	StageScores map[string]float64 `json:"stageScores"`
}

type MetricsData struct {
	ScoreDistribution map[string]int      `json:"scoreDistribution"`
	ModelComparison   []ModelMetric       `json:"modelComparison"`
	ScoreByType       []TypeMetric        `json:"scoreByType"`
	Usage             UsageMetrics        `json:"usage"`
}

type ModelMetric struct {
	Model    string  `json:"model"`
	AvgScore float64 `json:"avgScore"`
}

type TypeMetric struct {
	Type     string  `json:"type"`
	AvgScore float64 `json:"avgScore"`
}

type UsageMetrics struct {
	TodayTokens  int     `json:"todayTokens"`
	WeekTokens   int     `json:"weekTokens"`
	MonthTokens  int     `json:"monthTokens"`
	TodayCost    float64 `json:"todayCost"`
	WeekCost     float64 `json:"weekCost"`
	MonthCost    float64 `json:"monthCost"`
}

type TimelineMetrics struct {
	Timeline []TimelinePoint `json:"timeline"`
}

type EmbeddingMetrics struct {
	AvgSimilarity float64 `json:"avgSimilarity"`
	AvgRelevance  float64 `json:"avgRelevance"`
	PassRate      float64 `json:"passRate"`
	TotalChecks   int     `json:"totalChecks"`
}

type FeedbackSummary struct {
	Total     int                    `json:"total"`
	Likes     int                    `json:"likes"`
	Dislikes  int                    `json:"dislikes"`
	AvgRating float64                `json:"avgRating"`
	Categories map[string]int        `json:"categories"`
	Entries   []Feedback             `json:"entries"`
}

type RegressionSignal struct {
	Name         string  `json:"name"`
	Current      float64 `json:"current"`
	Baseline     float64 `json:"baseline"`
	Delta        float64 `json:"delta"`
	DeltaPercent float64 `json:"deltaPercent"`
	Status       string  `json:"status"`
	Direction    string  `json:"direction"`
}

type RegressionResult struct {
	Period         string             `json:"period"`
	Baseline       string             `json:"baseline"`
	Signals        []RegressionSignal `json:"signals"`
	TopRegressed   []RegressedRun     `json:"topRegressed"`
}

type RegressedRun struct {
	ID             string  `json:"id"`
	Source         string  `json:"source"`
	CurrentScore   float64 `json:"currentScore"`
	BaselineAvg    float64 `json:"baselineAvg"`
	ScoreDrop      float64 `json:"scoreDrop"`
}

func (s *Store) GetOverview(days int, docCount int) (*OverviewMetrics, error) {
	cutoff := time.Now().Add(-time.Duration(days) * 24 * time.Hour).Format(time.RFC3339)

	var avgScore sql.NullFloat64
	var totalRuns, failedRuns int

	s.db.QueryRow(`SELECT COUNT(*), COALESCE(AVG(score),0), COALESCE(SUM(CASE WHEN status='failed' THEN 1 ELSE 0 END),0) FROM pipeline_runs WHERE created_at >= ?`, cutoff).Scan(&totalRuns, &avgScore, &failedRuns)

	passRate := 100.0
	if totalRuns > 0 {
		passRate = float64(totalRuns-failedRuns) / float64(totalRuns) * 100
	}

	radar := make(map[string]float64)
	rows, err := s.db.Query(`SELECT judge_type, AVG(score) FROM judge_evaluations WHERE created_at >= ? AND score > 0 GROUP BY judge_type`, cutoff)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var jt string
			var avg float64
			if rows.Scan(&jt, &avg) == nil {
				radar[jt] = avg
			}
		}
	}

	timeline := make([]TimelinePoint, 0)
	tRows, err := s.db.Query(`SELECT DATE(created_at) as d, AVG(COALESCE(score,0)) FROM pipeline_runs WHERE created_at >= ? AND status='completed' GROUP BY d ORDER BY d`, cutoff)
	if err == nil {
		defer tRows.Close()
		for tRows.Next() {
			var date string
			var avg float64
			if tRows.Scan(&date, &avg) == nil {
				timeline = append(timeline, TimelinePoint{Date: date, AvgScore: avg})
			}
		}
	}

	failures := make([]FailureEntry, 0)
	fRows, err := s.db.Query(`SELECT id, source, COALESCE(score,0) FROM pipeline_runs WHERE created_at >= ? AND status='completed' AND score IS NOT NULL AND score < 0.5 ORDER BY score ASC LIMIT 10`, cutoff)
	if err == nil {
		defer fRows.Close()
		for fRows.Next() {
			var f FailureEntry
			if fRows.Scan(&f.ID, &f.Source, &f.Score) == nil {
				failures = append(failures, f)
			}
		}
	}

	var latest *LatestPipelineEntry
	lRow := s.db.QueryRow(`SELECT id, source, COALESCE(score,0) FROM pipeline_runs WHERE status='completed' ORDER BY created_at DESC LIMIT 1`)
	var l LatestPipelineEntry
	if err := lRow.Scan(&l.ID, &l.Source, &l.Score); err == nil {
		l.StageScores = make(map[string]float64)
		sRows, err := s.db.Query(`SELECT name, COALESCE(score,0) FROM pipeline_stages WHERE run_id = ? ORDER BY seq`, l.ID)
		if err == nil {
			defer sRows.Close()
			for sRows.Next() {
				var name string
				var sc float64
				if sRows.Scan(&name, &sc) == nil {
					l.StageScores[name] = sc
				}
			}
		}
		latest = &l
	}

	return &OverviewMetrics{
		OverallScore:       avgScore.Float64,
		PipelinePassRate:   passRate,
		PipelineFailRate:   100 - passRate,
		DocumentsProcessed: docCount,
		Radar:              radar,
		Timeline:           timeline,
		TopFailures:        failures,
		LatestPipeline:     latest,
	}, nil
}

func (s *Store) GetTimeline(days int) ([]TimelinePoint, error) {
	cutoff := time.Now().Add(-time.Duration(days) * 24 * time.Hour).Format(time.RFC3339)
	points := make([]TimelinePoint, 0)
	rows, err := s.db.Query(`SELECT DATE(created_at) as d, AVG(COALESCE(score,0)) FROM pipeline_runs WHERE created_at >= ? AND status='completed' GROUP BY d ORDER BY d`, cutoff)
	if err != nil {
		return nil, fmt.Errorf("timeline: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var p TimelinePoint
		if err := rows.Scan(&p.Date, &p.AvgScore); err != nil {
			return nil, err
		}
		points = append(points, p)
	}
	return points, nil
}

func (s *Store) GetPipelines(limit int, status string) ([]PipelineRun, error) {
	query := `SELECT id, source, source_type, status, score, total_tokens, total_latency_ms, total_cost, error, pipeline_version, created_at, completed_at FROM pipeline_runs`
	var args []interface{}

	if status != "" {
		query += ` WHERE status = ?`
		args = append(args, status)
	}
	query += ` ORDER BY created_at DESC`
	if limit > 0 {
		query += fmt.Sprintf(` LIMIT %d`, limit)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list pipelines: %w", err)
	}
	defer rows.Close()

	var runs []PipelineRun
	for rows.Next() {
		var r PipelineRun
		var createdAt, completedAt sql.NullString
		if err := rows.Scan(&r.ID, &r.Source, &r.SourceType, &r.Status, &r.Score, &r.TotalTokens,
			&r.TotalLatencyMs, &r.TotalCost, &r.Error, &r.PipelineVersion, &createdAt, &completedAt); err != nil {
			return nil, err
		}
		if createdAt.Valid {
			r.CreatedAt, _ = time.Parse(time.RFC3339, createdAt.String)
		}
		if completedAt.Valid {
			t, _ := time.Parse(time.RFC3339, completedAt.String)
			r.CompletedAt = &t
		}
		runs = append(runs, r)
	}
	return runs, nil
}

func (s *Store) GetPipeline(id string) (*PipelineRun, error) {
	var r PipelineRun
	var createdAt, completedAt sql.NullString
	err := s.db.QueryRow(`SELECT id, source, source_type, status, score, total_tokens, total_latency_ms, total_cost, error, pipeline_version, config_snapshot, created_at, completed_at FROM pipeline_runs WHERE id = ?`, id).
		Scan(&r.ID, &r.Source, &r.SourceType, &r.Status, &r.Score, &r.TotalTokens,
			&r.TotalLatencyMs, &r.TotalCost, &r.Error, &r.PipelineVersion, &r.ConfigSnapshot,
			&createdAt, &completedAt)
	if err != nil {
		return nil, fmt.Errorf("get pipeline: %w", err)
	}
	if createdAt.Valid {
		r.CreatedAt, _ = time.Parse(time.RFC3339, createdAt.String)
	}
	if completedAt.Valid {
		t, _ := time.Parse(time.RFC3339, completedAt.String)
		r.CompletedAt = &t
	}
	return &r, nil
}

func (s *Store) GetPipelineStages(runID string) ([]PipelineStage, error) {
	rows, err := s.db.Query(`SELECT id, run_id, name, seq, status, score, latency_ms, tokens_in, tokens_out, prompt, response, model, error, created_at FROM pipeline_stages WHERE run_id = ? ORDER BY seq`, runID)
	if err != nil {
		return nil, fmt.Errorf("get stages: %w", err)
	}
	defer rows.Close()

	var stages []PipelineStage
	for rows.Next() {
		var st PipelineStage
		var createdAt string
		if err := rows.Scan(&st.ID, &st.RunID, &st.Name, &st.Seq, &st.Status, &st.Score,
			&st.LatencyMs, &st.TokensIn, &st.TokensOut, &st.Prompt, &st.Response, &st.Model,
			&st.Error, &createdAt); err != nil {
			return nil, err
		}
		st.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		stages = append(stages, st)
	}
	return stages, nil
}

func (s *Store) GetLLMCalls(runID string) ([]LLMCall, error) {
	rows, err := s.db.Query(`SELECT id, run_id, model, operation, prompt_tokens, completion_tokens, latency_ms, cost, prompt, response, status, error_message, created_at FROM llm_calls WHERE run_id = ? ORDER BY created_at`, runID)
	if err != nil {
		return nil, fmt.Errorf("get llm calls: %w", err)
	}
	defer rows.Close()

	var calls []LLMCall
	for rows.Next() {
		var c LLMCall
		var createdAt string
		if err := rows.Scan(&c.ID, &c.RunID, &c.Model, &c.Operation, &c.PromptTokens,
			&c.CompletionTokens, &c.LatencyMs, &c.Cost, &c.Prompt, &c.Response,
			&c.Status, &c.ErrorMessage, &createdAt); err != nil {
			return nil, err
		}
		c.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		calls = append(calls, c)
	}
	return calls, nil
}

func (s *Store) GetJudgeEvaluations(runID string) ([]JudgeEvaluation, error) {
	rows, err := s.db.Query(`SELECT id, run_id, judge_model, judge_type, prompt, response, score, reasoning, passed, latency_ms, tokens_used, cost, created_at FROM judge_evaluations WHERE run_id = ? ORDER BY judge_type`, runID)
	if err != nil {
		return nil, fmt.Errorf("get judge evals: %w", err)
	}
	defer rows.Close()

	var evals []JudgeEvaluation
	for rows.Next() {
		var e JudgeEvaluation
		var createdAt string
		if err := rows.Scan(&e.ID, &e.RunID, &e.JudgeModel, &e.JudgeType, &e.Prompt,
			&e.Response, &e.Score, &e.Reasoning, &e.Passed, &e.LatencyMs,
			&e.TokensUsed, &e.Cost, &createdAt); err != nil {
			return nil, err
		}
		e.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		evals = append(evals, e)
	}
	return evals, nil
}

func (s *Store) GetEmbeddingChecks(runID string) ([]EmbeddingCheck, error) {
	rows, err := s.db.Query(`SELECT id, run_id, query_text, chunk_text, similarity_score, relevance_score, passed, latency_ms, created_at FROM embedding_checks WHERE run_id = ? ORDER BY id`, runID)
	if err != nil {
		return nil, fmt.Errorf("get embedding checks: %w", err)
	}
	defer rows.Close()

	var checks []EmbeddingCheck
	for rows.Next() {
		var c EmbeddingCheck
		var createdAt string
		if err := rows.Scan(&c.ID, &c.RunID, &c.QueryText, &c.ChunkText, &c.SimilarityScore,
			&c.RelevanceScore, &c.Passed, &c.LatencyMs, &createdAt); err != nil {
			return nil, err
		}
		c.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		checks = append(checks, c)
	}
	return checks, nil
}

func (s *Store) GetFeedback(runID string) ([]Feedback, error) {
	rows, err := s.db.Query(`SELECT id, run_id, rating, like_dislike, category, comment, created_at FROM feedback WHERE run_id = ? ORDER BY created_at`, runID)
	if err != nil {
		return nil, fmt.Errorf("get feedback: %w", err)
	}
	defer rows.Close()

	var fbs []Feedback
	for rows.Next() {
		var fb Feedback
		var createdAt string
		if err := rows.Scan(&fb.ID, &fb.RunID, &fb.Rating, &fb.LikeDislike, &fb.Category,
			&fb.Comment, &createdAt); err != nil {
			return nil, err
		}
		fb.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		fbs = append(fbs, fb)
	}
	return fbs, nil
}

func (s *Store) GetMetrics(period string) (*MetricsData, error) {
	now := time.Now()
	var since time.Time
	switch period {
	case "7d":
		since = now.Add(-7 * 24 * time.Hour)
	case "30d":
		since = now.Add(-30 * 24 * time.Hour)
	default:
		since = now.Add(-24 * time.Hour)
	}
	cutoff := since.Format(time.RFC3339)

	scoreDist := make(map[string]int)
	distBuckets := []string{"0.0-0.2", "0.2-0.4", "0.4-0.6", "0.6-0.8", "0.8-1.0"}
	for _, b := range distBuckets {
		scoreDist[b] = 0
	}
	sRows, err := s.db.Query(`SELECT CAST(CAST(COALESCE(score,0) * 5 AS INTEGER) AS REAL) / 5.0 as bucket FROM pipeline_runs WHERE created_at >= ? AND status='completed'`, cutoff)
	if err == nil {
		defer sRows.Close()
		for sRows.Next() {
			var bucket float64
			if sRows.Scan(&bucket) == nil {
				key := fmt.Sprintf("%.1f-%.1f", bucket, bucket+0.2)
				scoreDist[key]++
			}
		}
	}

	modelComp := make([]ModelMetric, 0)
	mRows, err := s.db.Query(`SELECT model, AVG(COALESCE(case when status='success' then 1.0 else 0.0 end,0)) FROM llm_calls WHERE created_at >= ? GROUP BY model ORDER BY AVG(case when status='success' then 1.0 else 0.0 end) DESC`, cutoff)
	if err == nil {
		defer mRows.Close()
		for mRows.Next() {
			var m ModelMetric
			if mRows.Scan(&m.Model, &m.AvgScore) == nil {
				modelComp = append(modelComp, m)
			}
		}
	}

	scoreByType := make([]TypeMetric, 0)
	tRows, err := s.db.Query(`SELECT judge_type, AVG(score) FROM judge_evaluations WHERE created_at >= ? AND score > 0 GROUP BY judge_type`, cutoff)
	if err == nil {
		defer tRows.Close()
		for tRows.Next() {
			var t TypeMetric
			if tRows.Scan(&t.Type, &t.AvgScore) == nil {
				scoreByType = append(scoreByType, t)
			}
		}
	}

	dayStart := now.Format("2006-01-02")
	weekStart := now.Add(-7 * 24 * time.Hour).Format(time.RFC3339)
	monthStart := now.Add(-30 * 24 * time.Hour).Format(time.RFC3339)

	var usage UsageMetrics
	s.db.QueryRow(`SELECT COALESCE(SUM(prompt_tokens+completion_tokens),0), COALESCE(SUM(cost),0) FROM llm_calls WHERE created_at >= ?`, dayStart).Scan(&usage.TodayTokens, &usage.TodayCost)
	s.db.QueryRow(`SELECT COALESCE(SUM(prompt_tokens+completion_tokens),0), COALESCE(SUM(cost),0) FROM llm_calls WHERE created_at >= ?`, weekStart).Scan(&usage.WeekTokens, &usage.WeekCost)
	s.db.QueryRow(`SELECT COALESCE(SUM(prompt_tokens+completion_tokens),0), COALESCE(SUM(cost),0) FROM llm_calls WHERE created_at >= ?`, monthStart).Scan(&usage.MonthTokens, &usage.MonthCost)

	return &MetricsData{
		ScoreDistribution: scoreDist,
		ModelComparison:   modelComp,
		ScoreByType:       scoreByType,
		Usage:             usage,
	}, nil
}

func (s *Store) GetLogs(model, operation, status string) ([]LLMCall, error) {
	query := `SELECT id, run_id, model, operation, prompt_tokens, completion_tokens, latency_ms, cost, prompt, response, status, error_message, created_at FROM llm_calls WHERE 1=1`
	var args []interface{}

	if model != "" {
		query += ` AND model = ?`
		args = append(args, model)
	}
	if operation != "" {
		query += ` AND operation = ?`
		args = append(args, operation)
	}
	if status != "" {
		query += ` AND status = ?`
		args = append(args, status)
	}
	query += ` ORDER BY created_at DESC LIMIT 200`

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list logs: %w", err)
	}
	defer rows.Close()

	var calls []LLMCall
	for rows.Next() {
		var c LLMCall
		var createdAt string
		if err := rows.Scan(&c.ID, &c.RunID, &c.Model, &c.Operation, &c.PromptTokens,
			&c.CompletionTokens, &c.LatencyMs, &c.Cost, &c.Prompt, &c.Response,
			&c.Status, &c.ErrorMessage, &createdAt); err != nil {
			return nil, err
		}
		c.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		calls = append(calls, c)
	}
	return calls, nil
}

func (s *Store) GetLog(id string) (*LLMCall, error) {
	var c LLMCall
	var createdAt string
	err := s.db.QueryRow(`SELECT id, run_id, model, operation, prompt_tokens, completion_tokens, latency_ms, cost, prompt, response, status, error_message, created_at FROM llm_calls WHERE id = ?`, id).
		Scan(&c.ID, &c.RunID, &c.Model, &c.Operation, &c.PromptTokens, &c.CompletionTokens,
			&c.LatencyMs, &c.Cost, &c.Prompt, &c.Response, &c.Status, &c.ErrorMessage, &createdAt)
	if err != nil {
		return nil, fmt.Errorf("get log: %w", err)
	}
	c.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	return &c, nil
}

func (s *Store) GetEmbeddingMetrics(period string) (*EmbeddingMetrics, error) {
	cutoff := periodCutoff(period)
	rows, err := s.db.Query(`SELECT COUNT(*), COALESCE(AVG(similarity_score),0), COALESCE(AVG(relevance_score),0), COALESCE(SUM(CASE WHEN passed=1 THEN 1 ELSE 0 END),0) FROM embedding_checks WHERE created_at >= ?`, cutoff)
	if err != nil {
		return nil, fmt.Errorf("embedding metrics: %w", err)
	}
	defer rows.Close()

	var m EmbeddingMetrics
	var total, passed int
	if rows.Next() {
		if err := rows.Scan(&total, &m.AvgSimilarity, &m.AvgRelevance, &passed); err != nil {
			return nil, err
		}
	}
	m.TotalChecks = total
	if total > 0 {
		m.PassRate = float64(passed) / float64(total) * 100
	}
	return &m, nil
}

func (s *Store) GetFeedbackSummary(period string) (*FeedbackSummary, error) {
	cutoff := periodCutoff(period)
	rows, err := s.db.Query(`SELECT COUNT(*), COALESCE(SUM(CASE WHEN like_dislike='like' THEN 1 ELSE 0 END),0), COALESCE(SUM(CASE WHEN like_dislike='dislike' THEN 1 ELSE 0 END),0), COALESCE(AVG(rating),0) FROM feedback WHERE created_at >= ?`, cutoff)
	if err != nil {
		return nil, fmt.Errorf("feedback summary: %w", err)
	}
	defer rows.Close()

	var summary FeedbackSummary
	summary.Categories = make(map[string]int)
	if rows.Next() {
		if err := rows.Scan(&summary.Total, &summary.Likes, &summary.Dislikes, &summary.AvgRating); err != nil {
			return nil, err
		}
	}

	catRows, err := s.db.Query(`SELECT category, COUNT(*) FROM feedback WHERE created_at >= ? AND category != '' GROUP BY category`, cutoff)
	if err == nil {
		defer catRows.Close()
		for catRows.Next() {
			var cat string
			var count int
			if catRows.Scan(&cat, &count) == nil {
				summary.Categories[cat] = count
			}
		}
	}

	entries := make([]Feedback, 0)
	eRows, err := s.db.Query(`SELECT id, run_id, rating, like_dislike, category, comment, created_at FROM feedback WHERE created_at >= ? ORDER BY created_at DESC LIMIT 100`, cutoff)
	if err == nil {
		defer eRows.Close()
		for eRows.Next() {
			var fb Feedback
			var createdAt string
			if eRows.Scan(&fb.ID, &fb.RunID, &fb.Rating, &fb.LikeDislike, &fb.Category, &fb.Comment, &createdAt) == nil {
				fb.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
				entries = append(entries, fb)
			}
		}
	}
	summary.Entries = entries

	return &summary, nil
}

func (s *Store) GetRegressions(period, baseline string, baselineDuration string) (*RegressionResult, error) {
	currentStart := periodStart(period)
	baselineStartTime := computeBaselineStart(baseline)
	baselineEndTime := computeBaselineEnd(baseline, baselineDuration)

	result := &RegressionResult{
		Period:       period,
		Baseline:     baseline,
		Signals:      make([]RegressionSignal, 0),
		TopRegressed: make([]RegressedRun, 0),
	}

	signals := []struct {
		Name      string
		CurrentFn func() (float64, error)
		BaselineFn func() (float64, error)
		WorseDirection string
	}{
		{"avg_score",
			func() (float64, error) { return s.avgQuery(`SELECT COALESCE(AVG(score),0) FROM pipeline_runs WHERE status='completed' AND created_at >= ?`, currentStart) },
			func() (float64, error) { return s.avgQuery(`SELECT COALESCE(AVG(score),0) FROM pipeline_runs WHERE status='completed' AND created_at >= ? AND created_at < ?`, baselineStartTime, baselineEndTime) },
			"down"},
		{"pass_rate",
			func() (float64, error) { return s.rateQuery(currentStart, "") },
			func() (float64, error) { return s.rateQuery(baselineStartTime, baselineEndTime) },
			"down"},
		{"avg_latency",
			func() (float64, error) { return s.avgQuery(`SELECT COALESCE(AVG(total_latency_ms),0) FROM pipeline_runs WHERE status='completed' AND created_at >= ?`, currentStart) },
			func() (float64, error) { return s.avgQuery(`SELECT COALESCE(AVG(total_latency_ms),0) FROM pipeline_runs WHERE status='completed' AND created_at >= ? AND created_at < ?`, baselineStartTime, baselineEndTime) },
			"up"},
		{"avg_tokens",
			func() (float64, error) { return s.avgQuery(`SELECT COALESCE(AVG(total_tokens),0) FROM pipeline_runs WHERE status='completed' AND created_at >= ?`, currentStart) },
			func() (float64, error) { return s.avgQuery(`SELECT COALESCE(AVG(total_tokens),0) FROM pipeline_runs WHERE status='completed' AND created_at >= ? AND created_at < ?`, baselineStartTime, baselineEndTime) },
			"up"},
		{"error_rate",
			func() (float64, error) { return s.errorRateQuery(currentStart, "") },
			func() (float64, error) { return s.errorRateQuery(baselineStartTime, baselineEndTime) },
			"up"},
		{"embedding_quality",
			func() (float64, error) { return s.avgQuery(`SELECT COALESCE(AVG(relevance_score),0) FROM embedding_checks WHERE created_at >= ?`, currentStart) },
			func() (float64, error) { return s.avgQuery(`SELECT COALESCE(AVG(relevance_score),0) FROM embedding_checks WHERE created_at >= ? AND created_at < ?`, baselineStartTime, baselineEndTime) },
			"down"},
		{"negative_feedback_ratio",
			func() (float64, error) { return s.negativeFeedbackRate(currentStart, "") },
			func() (float64, error) { return s.negativeFeedbackRate(baselineStartTime, baselineEndTime) },
			"up"},
	}

	for _, sig := range signals {
		current, errC := sig.CurrentFn()
		baselineVal, errB := sig.BaselineFn()
		if errC != nil || errB != nil {
			continue
		}

		delta := current - baselineVal
		deltaPct := 0.0
		if baselineVal != 0 {
			deltaPct = delta / baselineVal * 100
		}

		status := "ok"
		switch sig.Name {
		case "avg_score":
			if delta < -0.1 {
				status = "major"
			} else if delta < -0.05 {
				status = "minor"
			}
		case "pass_rate":
			if delta < -10 {
				status = "major"
			} else if delta < -5 {
				status = "minor"
			}
		case "avg_latency":
			if current > baselineVal*1.5 {
				status = "minor"
			}
		case "error_rate":
			if delta > 2 {
				status = "major"
			} else if delta > 1 {
				status = "minor"
			}
		case "embedding_quality":
			if delta < -0.15 {
				status = "major"
			} else if delta < -0.1 {
				status = "minor"
			}
		case "negative_feedback_ratio":
			if delta > 0.1 {
				status = "major"
			} else if delta > 0.05 {
				status = "minor"
			}
		}

		direction := "unchanged"
		if sig.WorseDirection == "down" {
			if delta < 0 {
				direction = "down"
			} else if delta > 0 {
				direction = "up"
			}
		} else {
			if delta > 0 {
				direction = "up"
			} else if delta < 0 {
				direction = "down"
			}
		}

		result.Signals = append(result.Signals, RegressionSignal{
			Name:         sig.Name,
			Current:      current,
			Baseline:     baselineVal,
			Delta:        delta,
			DeltaPercent: deltaPct,
			Status:       status,
			Direction:    direction,
		})
	}

	regRows, err := s.db.Query(`SELECT id, source, COALESCE(score,0) FROM pipeline_runs WHERE status='completed' AND created_at >= ? ORDER BY score ASC LIMIT 10`, currentStart)
	if err == nil {
		defer regRows.Close()
		for regRows.Next() {
			var rr RegressedRun
			if regRows.Scan(&rr.ID, &rr.Source, &rr.CurrentScore) == nil {
				s.db.QueryRow(`SELECT COALESCE(AVG(score),0) FROM pipeline_runs WHERE status='completed' AND created_at >= ? AND created_at < ?`, baselineStartTime, baselineEndTime).Scan(&rr.BaselineAvg)
				rr.ScoreDrop = rr.BaselineAvg - rr.CurrentScore
				result.TopRegressed = append(result.TopRegressed, rr)
			}
		}
	}

	return result, nil
}

func (s *Store) avgQuery(query string, args ...interface{}) (float64, error) {
	var val sql.NullFloat64
	err := s.db.QueryRow(query, args...).Scan(&val)
	if err != nil {
		return 0, err
	}
	return val.Float64, nil
}

func (s *Store) rateQuery(since string, until string) (float64, error) {
	var total, failed int
	if until == "" {
		s.db.QueryRow(`SELECT COUNT(*), SUM(CASE WHEN status='failed' THEN 1 ELSE 0 END) FROM pipeline_runs WHERE created_at >= ?`, since).Scan(&total, &failed)
	} else {
		s.db.QueryRow(`SELECT COUNT(*), SUM(CASE WHEN status='failed' THEN 1 ELSE 0 END) FROM pipeline_runs WHERE created_at >= ? AND created_at < ?`, since, until).Scan(&total, &failed)
	}
	if total == 0 {
		return 100, nil
	}
	return float64(total-failed) / float64(total) * 100, nil
}

func (s *Store) errorRateQuery(since string, until string) (float64, error) {
	var total int
	if until == "" {
		s.db.QueryRow(`SELECT COUNT(*) FROM pipeline_runs WHERE created_at >= ? AND status='failed'`, since).Scan(&total)
	} else {
		s.db.QueryRow(`SELECT COUNT(*) FROM pipeline_runs WHERE created_at >= ? AND created_at < ? AND status='failed'`, since, until).Scan(&total)
	}
	var runs int
	if until == "" {
		s.db.QueryRow(`SELECT COUNT(*) FROM pipeline_runs WHERE created_at >= ?`, since).Scan(&runs)
	} else {
		s.db.QueryRow(`SELECT COUNT(*) FROM pipeline_runs WHERE created_at >= ? AND created_at < ?`, since, until).Scan(&runs)
	}
	if runs == 0 {
		return 0, nil
	}
	return float64(total) / float64(runs) * 100, nil
}

func (s *Store) negativeFeedbackRate(since string, until string) (float64, error) {
	var negative int
	if until == "" {
		s.db.QueryRow(`SELECT COUNT(*) FROM feedback WHERE created_at >= ? AND (like_dislike='dislike' OR (rating IS NOT NULL AND rating < 3))`, since).Scan(&negative)
	} else {
		s.db.QueryRow(`SELECT COUNT(*) FROM feedback WHERE created_at >= ? AND created_at < ? AND (like_dislike='dislike' OR (rating IS NOT NULL AND rating < 3))`, since, until).Scan(&negative)
	}
	var total int
	if until == "" {
		s.db.QueryRow(`SELECT COUNT(*) FROM feedback WHERE created_at >= ?`, since).Scan(&total)
	} else {
		s.db.QueryRow(`SELECT COUNT(*) FROM feedback WHERE created_at >= ? AND created_at < ?`, since, until).Scan(&total)
	}
	if total == 0 {
		return 0, nil
	}
	return float64(negative) / float64(total), nil
}

type CacheEntry struct {
	Data       string    `json:"data"`
	ComputedAt time.Time `json:"computedAt"`
	TTLSeconds int       `json:"ttlSeconds"`
}

func (s *Store) GetCache(key string) (string, bool) {
	var data string
	var computedAt string
	var ttl int
	err := s.db.QueryRow(`SELECT data, computed_at, ttl_seconds FROM metrics_cache WHERE cache_key = ?`, key).Scan(&data, &computedAt, &ttl)
	if err != nil {
		return "", false
	}

	computed, err := time.Parse(time.RFC3339, computedAt)
	if err != nil {
		return "", false
	}

	if time.Since(computed).Seconds() > float64(ttl) {
		return "", false
	}

	return data, true
}

func (s *Store) SetCache(key string, data interface{}, ttl int) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("cache marshal: %w", err)
	}

	_, err = s.db.Exec(`INSERT INTO metrics_cache (cache_key, data, computed_at, ttl_seconds) VALUES (?, ?, ?, ?) ON CONFLICT(cache_key) DO UPDATE SET data=excluded.data, computed_at=excluded.computed_at, ttl_seconds=excluded.ttl_seconds`,
		key, string(jsonData), time.Now().Format(time.RFC3339), ttl)
	return err
}

func (s *Store) SubmitFeedback(fb *Feedback) error {
	_, err := s.db.Exec(`
		INSERT INTO feedback (run_id, rating, like_dislike, category, comment, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, fb.RunID, fb.Rating, fb.LikeDislike, fb.Category, fb.Comment, fb.CreatedAt.Format(time.RFC3339))

	if err != nil {
		return fmt.Errorf("submit feedback: %w", err)
	}

	return nil
}

func (s *Store) PenalizeRunScore(runID string, newScore float64) error {
	_, err := s.db.Exec(`UPDATE pipeline_runs SET score = ? WHERE id = ?`, newScore, runID)
	if err != nil {
		return fmt.Errorf("penalize score: %w", err)
	}
	return nil
}

func periodCutoff(period string) string {
	now := time.Now()
	switch period {
	case "7d":
		return now.Add(-7 * 24 * time.Hour).Format(time.RFC3339)
	case "30d":
		return now.Add(-30 * 24 * time.Hour).Format(time.RFC3339)
	case "24h":
		return now.Add(-24 * time.Hour).Format(time.RFC3339)
	default:
		return now.Add(-24 * time.Hour).Format(time.RFC3339)
	}
}

func periodStart(period string) string {
	now := time.Now()
	switch period {
	case "7d":
		return now.Add(-7 * 24 * time.Hour).Format(time.RFC3339)
	case "24h":
		return now.Add(-24 * time.Hour).Format(time.RFC3339)
	default:
		return now.Add(-24 * time.Hour).Format(time.RFC3339)
	}
}

func computeBaselineStart(baseline string) string {
	now := time.Now()
	switch baseline {
	case "48h":
		return now.Add(-48 * time.Hour).Format(time.RFC3339)
	case "7d":
		return now.Add(-14 * 24 * time.Hour).Format(time.RFC3339)
	default:
		return now.Add(-48 * time.Hour).Format(time.RFC3339)
	}
}

func computeBaselineEnd(baseline, duration string) string {
	now := time.Now()
	switch baseline {
	case "48h":
		return now.Add(-24 * time.Hour).Format(time.RFC3339)
	case "7d":
		return now.Add(-7 * 24 * time.Hour).Format(time.RFC3339)
	default:
		return now.Add(-24 * time.Hour).Format(time.RFC3339)
	}
}
