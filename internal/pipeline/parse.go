package pipeline

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var templateRe = regexp.MustCompile(`<\w+>`)

var knownKeys = []string{
	"type", "title", "topic", "summary", "key_points",
	"source_quote", "my_takeaway", "confidence", "applicability",
}

var multilineKeys = map[string]bool{
	"summary": true, "source_quote": true,
	"my_takeaway": true, "applicability": true,
}

type ValidationError struct {
	Field  string
	Reason string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation failed: %s: %s", e.Field, e.Reason)
}

func validateChunkResult(r ChunkResult) error {
	if r.Type == "" {
		return &ValidationError{Field: "type", Reason: "empty"}
	}
	if templateRe.MatchString(r.Type) {
		return &ValidationError{Field: "type", Reason: "contains template placeholder"}
	}
	if hasTemplateContent(r) {
		return &ValidationError{Field: "multiple", Reason: "response contains template placeholders"}
	}
	return nil
}

func hasTemplateContent(r ChunkResult) bool {
	fields := []string{r.Type, r.Topic, r.Summary, r.MyTakeaway, r.Applicability}
	templateCount := 0
	for _, f := range fields {
		if templateRe.MatchString(f) {
			templateCount++
		}
	}
	return templateCount >= 2
}

func parsePromptResult(response string) ChunkResult {
	var r ChunkResult
	var currentKey string

	lines := strings.Split(response, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)

		key := knownKey(lower)
		if key != "" {
			currentKey = key
			val := safeFieldValue(line, len(key)+1)
			parseField(&r, key, val)
			continue
		}

		if trimmed == "" {
			currentKey = ""
			continue
		}

		if isMultilineKey(currentKey) && trimmed != "" {
			appendMultilineField(&r, currentKey, trimmed)
		}
	}
	return r
}

func parseField(r *ChunkResult, key, val string) {
	switch key {
	case "type":
		r.Type = val
	case "title":
		r.Title = val
	case "topic":
		r.Topic = val
		r.Topics = parseCommaList(val)
	case "summary":
		r.Summary = val
	case "key_points":
		r.KeyPoints = parseCommaList(val)
	case "source_quote":
		r.SourceQuote = val
	case "my_takeaway":
		r.MyTakeaway = val
	case "confidence":
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			r.Confidence = f
		}
	case "applicability":
		r.Applicability = val
	}
}

func appendMultilineField(r *ChunkResult, key, line string) {
	switch key {
	case "summary":
		if r.Summary != "" {
			r.Summary += " " + line
		}
	case "source_quote":
		if r.SourceQuote != "" {
			r.SourceQuote += " " + line
		}
	case "my_takeaway":
		if r.MyTakeaway != "" {
			r.MyTakeaway += " " + line
		}
	case "applicability":
		if r.Applicability != "" {
			r.Applicability += " " + line
		}
	}
}

func safeFieldValue(line string, prefixLen int) string {
	if len(line) <= prefixLen {
		return ""
	}
	return strings.TrimSpace(line[prefixLen:])
}

func isMultilineKey(key string) bool {
	return multilineKeys[key]
}

func knownKey(lower string) string {
	for _, k := range knownKeys {
		prefix := k + ":"
		if strings.HasPrefix(lower, prefix) {
			return k
		}
	}
	return ""
}

func parseCommaList(raw string) []string {
	var items []string
	for _, item := range strings.Split(raw, ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			items = append(items, item)
		}
	}
	return items
}

func mergePromptResult(dst *ChunkResult, src ChunkResult) {
	if dst.Title == "" {
		dst.Title = src.Title
	}
	if dst.Type == "" {
		dst.Type = src.Type
	}
	if dst.Topic == "" {
		dst.Topic = src.Topic
	}
	dst.Topics = append(dst.Topics, src.Topics...)
	if dst.Summary == "" {
		dst.Summary = src.Summary
	}
	dst.KeyPoints = append(dst.KeyPoints, src.KeyPoints...)
	if dst.SourceQuote == "" {
		dst.SourceQuote = src.SourceQuote
	}
	if dst.MyTakeaway == "" {
		dst.MyTakeaway = src.MyTakeaway
	}
	if dst.Confidence == 0 {
		dst.Confidence = src.Confidence
	}
	if dst.Applicability == "" {
		dst.Applicability = src.Applicability
	}
	dst.Steps = append(dst.Steps, src.Steps...)
}

func loadResultFromCheckpoint(dir string, index int) (ChunkResult, error) {
	if dir == "" {
		return ChunkResult{}, fmt.Errorf("no checkpoint dir")
	}
	all := loadResults(dir)
	for _, r := range all {
		if r.Index == index && r.Summary != "" {
			return r, nil
		}
	}
	return ChunkResult{}, fmt.Errorf("not found")
}

func loadResults(dir string) []ChunkResult {
	data, err := os.ReadFile(filepath.Join(dir, "results.json"))
	if err != nil {
		return nil
	}
	var results []ChunkResult
	if err := json.Unmarshal(data, &results); err != nil {
		return nil
	}
	return results
}

func saveResults(dir string, results []ChunkResult) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create checkpoint dir: %w", err)
	}
	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal results: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "results.json"), data, 0644); err != nil {
		return fmt.Errorf("write results: %w", err)
	}
	return nil
}
