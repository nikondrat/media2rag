package pipeline

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

var splitDelimiters = []string{"\n\n\n", "\n\n", "\n", ". "}

func (p *Pipeline) splitText(text string) []string {
	if p.checkpointDir != "" {
		chunks := loadChunks(p.checkpointDir)
		if chunks != nil {
			return chunks
		}
	}

	chunks := splitTextPure(text, p.config)

	if p.checkpointDir != "" {
		saveChunks(p.checkpointDir, chunks)
	}

	return chunks
}

func splitTextPure(text string, cfg PipelineConfig) []string {
	if len(text) <= cfg.ChunkSize {
		return []string{text}
	}

	var chunks []string
	start := 0
	for start < len(text) {
		end := start + cfg.ChunkSize
		if end >= len(text) {
			chunks = append(chunks, text[start:])
			break
		}

		splitAt := findSplitPoint(text, start, end)
		chunks = append(chunks, text[start:splitAt])

		start = splitAt - cfg.ChunkOverlap
		if start < 0 {
			start = 0
		}
	}

	return chunks
}

func findSplitPoint(text string, start, end int) int {
	segment := text[start:end]
	for _, delim := range splitDelimiters {
		if idx := strings.LastIndex(segment, delim); idx > 0 {
			return start + idx + len(delim)
		}
	}
	return end
}

func loadChunks(dir string) []string {
	entries, err := os.ReadDir(filepath.Join(dir, "chunks"))
	if err != nil {
		return nil
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), "chunk-") && strings.HasSuffix(e.Name(), ".md") {
			files = append(files, e.Name())
		}
	}
	if len(files) == 0 {
		return nil
	}

	sort.Slice(files, func(i, j int) bool {
		ai, _ := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(files[i], "chunk-"), ".md"))
		aj, _ := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(files[j], "chunk-"), ".md"))
		return ai < aj
	})

	chunks := make([]string, len(files))
	for i, name := range files {
		data, err := os.ReadFile(filepath.Join(dir, "chunks", name))
		if err != nil {
			return nil
		}
		chunks[i] = string(data)
	}
	return chunks
}

func saveChunks(dir string, chunks []string) {
	chunkDir := filepath.Join(dir, "chunks")
	os.MkdirAll(chunkDir, 0755)
	for i, chunk := range chunks {
		name := fmt.Sprintf("chunk-%03d.md", i+1)
		os.WriteFile(filepath.Join(chunkDir, name), []byte(chunk), 0644)
	}
}
