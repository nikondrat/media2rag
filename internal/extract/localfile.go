package extract

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type LocalFileExtractor struct{}

func (l *LocalFileExtractor) ContentType() string {
	return ContentTypeClean
}

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

func ParseFrontmatter(content string) map[string]string {
	meta := make(map[string]string)
	content = strings.TrimLeft(content, "\n\r\t ")
	if !strings.HasPrefix(content, "---") {
		return meta
	}
	rest := content[3:]
	idx := strings.Index(rest, "---")
	if idx < 0 {
		return meta
	}
	fm := rest[:idx]
	for _, line := range strings.Split(fm, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		val = strings.Trim(val, "\"'")
		meta[key] = val
	}
	return meta
}
