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
	Title    string            `json:"title"`
	Content  string            `json:"content"`
	Type     string            `json:"type"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

func parseRdrrJSON(data []byte) (string, error) {
	var resp rdrrResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", err
	}
	if resp.Content == "" {
		return "", fmt.Errorf("rdrr response has empty content field")
	}
	return resp.Content, nil
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
