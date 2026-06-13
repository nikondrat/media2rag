package extract

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

type PDFExtractor struct{}

func (p *PDFExtractor) Detect(path string) bool {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return false
	}
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".pdf"
}

func (p *PDFExtractor) Extract(ctx context.Context, path string) (string, error) {
	if err := checkCommand("pdftotext"); err != nil {
		return "", err
	}

	cmd := exec.CommandContext(ctx, "pdftotext", path, "-")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("pdftotext failed: %w\nstderr: %s", err, stderr.String())
	}

	content := stdout.String()
	if strings.TrimSpace(content) == "" {
		return "", fmt.Errorf("pdftotext returned empty output")
	}

	return content, nil
}
