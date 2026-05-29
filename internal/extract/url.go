package extract

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

type URLExtractor struct{}

func (u *URLExtractor) Detect(path string) bool {
	return strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://")
}

func (u *URLExtractor) Extract(ctx context.Context, path string) (string, error) {
	cmd := exec.CommandContext(ctx, "npx", "rdrr", path, "--json")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if isNotFound(err) {
			return "", fmt.Errorf("rdrr not found. Install: npx rdrr")
		}
		return "", fmt.Errorf("rdrr execution failed: %w\nstderr: %s", err, stderr.String())
	}

	output := stdout.Bytes()
	if len(output) == 0 {
		return "", fmt.Errorf("rdrr returned empty output")
	}

	content, err := parseRdrrJSON(output)
	if err != nil {
		return stdout.String(), nil
	}

	return content, nil
}

type rdrrResponse struct {
	Title       string            `json:"title"`
	Content     string            `json:"content"`
	Description string            `json:"description"`
	Type        string            `json:"type"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

func parseRdrrJSON(data []byte) (string, error) {
	var resp rdrrResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", err
	}
	return pickContent(resp.Content, resp.Description), nil
}

func wordCount(s string) int {
	if s == "" {
		return 0
	}
	count := 1
	inWord := false
	for _, c := range s {
		if c == ' ' || c == '\n' || c == '\t' {
			inWord = false
		} else {
			if !inWord {
				count++
				inWord = true
			}
		}
	}
	return count - 1
}

func pickContent(content, description string) string {
	if content == "" && description == "" {
		return ""
	}
	if description == "" {
		return content
	}
	if content == "" {
		return description
	}

	contentWords := wordCount(content)
	descWords := wordCount(description)

	threshold := descWords * 30 / 100
	if contentWords < threshold {
		return description
	}
	return content
}

func isNotFound(err error) bool {
	if errors.Is(err, exec.ErrNotFound) {
		return true
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode() == 127
	}
	return false
}
