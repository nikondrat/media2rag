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

const minChunkSize = 200

func (p *Pipeline) splitText(text string) ([]string, error) {
	if p.checkpointDir != "" {
		chunks := loadChunks(p.checkpointDir)
		if chunks != nil {
			return chunks, nil
		}
	}

	chunks := splitTextPure(text, p.config)

	if p.checkpointDir != "" {
		if err := saveChunks(p.checkpointDir, chunks); err != nil {
			return chunks, err
		}
	}

	return chunks, nil
}

func splitTextPure(text string, cfg PipelineConfig) []string {
	if len(text) <= cfg.ChunkSize {
		return []string{text}
	}

	overlap := cfg.ChunkOverlap
	if overlap >= cfg.ChunkSize {
		overlap = cfg.ChunkSize / 4
	}
	if overlap < minChunkSize/2 {
		overlap = minChunkSize / 2
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

		if splitAt <= start {
			splitAt = start + minChunkSize
			if splitAt > len(text) {
				splitAt = len(text)
			}
		}

		if splitAt > len(text) {
			splitAt = len(text)
		}

		chunks = append(chunks, text[start:splitAt])

		nextStart := splitAt - overlap
		if nextStart <= start {
			nextStart = splitAt
		}
		if nextStart >= len(text) {
			break
		}
		start = nextStart
	}

	return dedupChunkOverlap(chunks, overlap)
}

func dedupChunkOverlap(chunks []string, overlap int) []string {
	if len(chunks) <= 1 {
		return chunks
	}

	deduped := make([]string, len(chunks))
	copy(deduped, chunks)

	for i := 1; i < len(deduped); i++ {
		prev := deduped[i-1]
		curr := deduped[i]
		if overlap > 0 && len(prev) >= overlap {
			tail := prev[len(prev)-overlap:]
			if strings.HasPrefix(curr, tail) {
				deduped[i] = curr[len(tail):]
			}
		}
	}

	return deduped
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

func saveChunks(dir string, chunks []string) error {
	chunkDir := filepath.Join(dir, "chunks")
	if err := os.MkdirAll(chunkDir, 0755); err != nil {
		return fmt.Errorf("create chunk dir: %w", err)
	}
	for i, chunk := range chunks {
		name := fmt.Sprintf("chunk-%03d.md", i+1)
		if err := os.WriteFile(filepath.Join(chunkDir, name), []byte(chunk), 0644); err != nil {
			return fmt.Errorf("write %s: %w", name, err)
		}
	}
	return nil
}

func (p *Pipeline) SaveChunks(dir string, chunks []string) error {
	if dir == "" {
		return nil
	}
	return saveChunks(dir, chunks)
}