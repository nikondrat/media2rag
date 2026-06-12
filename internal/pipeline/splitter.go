package pipeline

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const minChunkSize = 200
const minMergeSize = 500

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

	paragraphs := splitIntoParagraphs(text)

	groups := groupParagraphs(paragraphs, cfg.ChunkSize)
	groups = mergeSmallChunks(groups)
	groups = filterEmptyChunks(groups)

	return groups
}

func splitIntoParagraphs(text string) []string {
	var paragraphs []string
	for _, p := range strings.Split(text, "\n\n") {
		p = strings.TrimSpace(p)
		if p != "" {
			paragraphs = append(paragraphs, p)
		}
	}
	return paragraphs
}

func groupParagraphs(paragraphs []string, maxLen int) []string {
	if len(paragraphs) == 0 {
		return nil
	}

	var groups []string
	var current strings.Builder

	for _, p := range paragraphs {
		if len(p) > maxLen && current.Len() == 0 {
			groups = append(groups, p)
			continue
		}

		if current.Len()+len(p)+2 > maxLen && current.Len() > 0 {
			groups = append(groups, current.String())
			current.Reset()
		}

		if current.Len() > 0 {
			current.WriteString("\n\n")
		}
		current.WriteString(p)
	}

	if current.Len() > 0 {
		groups = append(groups, current.String())
	}

	return groups
}

func mergeSmallChunks(chunks []string) []string {
	if len(chunks) <= 1 {
		return chunks
	}

	var merged []string
	pending := ""

	for _, chunk := range chunks {
		if len(chunk) < minMergeSize {
			if pending == "" {
				pending = chunk
			} else {
				pending += "\n\n" + chunk
			}
			continue
		}

		if pending != "" {
			chunk = pending + "\n\n" + chunk
			pending = ""
		}
		merged = append(merged, chunk)
	}

	if pending != "" {
		if len(merged) > 0 {
			merged[len(merged)-1] += "\n\n" + pending
		} else {
			merged = append(merged, pending)
		}
	}

	return merged
}

func filterEmptyChunks(chunks []string) []string {
	var filtered []string
	for _, c := range chunks {
		if strings.TrimSpace(c) != "" {
			filtered = append(filtered, c)
		}
	}
	return filtered
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

	var chunks []string
	for _, name := range files {
		data, err := os.ReadFile(filepath.Join(dir, "chunks", name))
		if err != nil {
			return nil
		}
		content := string(data)
		if strings.TrimSpace(content) != "" {
			chunks = append(chunks, content)
		}
	}
	if len(chunks) == 0 {
		return nil
	}
	return chunks
}

func saveChunks(dir string, chunks []string) error {
	chunkDir := filepath.Join(dir, "chunks")
	if err := os.RemoveAll(chunkDir); err != nil {
		return fmt.Errorf("clean chunk dir: %w", err)
	}
	if err := os.MkdirAll(chunkDir, 0755); err != nil {
		return fmt.Errorf("create chunk dir: %w", err)
	}
	idx := 1
	for _, chunk := range chunks {
		if strings.TrimSpace(chunk) == "" {
			continue
		}
		name := fmt.Sprintf("chunk-%03d.md", idx)
		if err := os.WriteFile(filepath.Join(chunkDir, name), []byte(chunk), 0644); err != nil {
			return fmt.Errorf("write %s: %w", name, err)
		}
		idx++
	}
	return nil
}

func (p *Pipeline) SaveChunks(dir string, chunks []string) error {
	if dir == "" {
		return nil
	}
	return saveChunks(dir, chunks)
}