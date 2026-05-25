package extract

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type LocalFileExtractor struct{}

func (l *LocalFileExtractor) Detect(path string) bool {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return false
	}
	ext := filepath.Ext(path)
	return ext == ".md" || ext == ".markdown"
}

func (l *LocalFileExtractor) Extract(ctx context.Context, path string) (string, error) {
	ext := filepath.Ext(path)
	if ext != ".md" && ext != ".markdown" {
		return "", fmt.Errorf("unsupported file format: %s (%s support coming in v2)", ext, strings.TrimPrefix(ext, "."))
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}

	content := string(data)
	content = stripFrontmatter(content)

	return content, nil
}

func stripFrontmatter(content string) string {
	content = strings.TrimLeft(content, "\n\r\t ")
	if strings.HasPrefix(content, "---") {
		idx := strings.Index(content[3:], "---")
		if idx >= 0 {
			return strings.TrimLeft(content[3+idx+3:], "\n\r")
		}
	}
	return content
}
