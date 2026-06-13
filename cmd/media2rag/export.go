package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"media2rag/internal/events"
	"media2rag/internal/model"
	"media2rag/internal/pipeline"
)

func exportToDir(dir, source, rawMD string, doc *model.RAGDocument, wordCount, version int, emitter events.EventEmitter) error {
	emitter.Emit(model.Event{Type: "export_start", Data: map[string]string{"dir": dir}})

	title := doc.Metadata.Title
	if title == "" {
		title = "unnamed_document"
	}
	sanitized := pipeline.SanitizeFilename(title)
	if sanitized == "" {
		sanitized = "unnamed_document"
	}

	if err := createExportDirs(dir); err != nil {
		return err
	}

	if err := writeExportFiles(dir, rawMD, doc, sanitized); err != nil {
		return err
	}

	if err := writeMetadata(dir, source, doc, wordCount, version); err != nil {
		return err
	}

	emitter.Emit(model.Event{Type: "export_complete", Data: map[string]interface{}{
		"final_path": filepath.Join(dir, "output", "final.md"),
		"title_path": filepath.Join(dir, sanitized+".md"),
		"chunks":     len(doc.Chunks),
	}})

	return nil
}

func createExportDirs(dir string) error {
	dirs := []string{"chunks", "intermediate", "output"}
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(dir, d), 0755); err != nil {
			return fmt.Errorf("create %s dir: %w", d, err)
		}
	}
	return nil
}

func writeExportFiles(dir, rawMD string, doc *model.RAGDocument, sanitized string) error {
	if err := os.WriteFile(filepath.Join(dir, "intermediate", "raw.md"), []byte(rawMD), 0644); err != nil {
		return fmt.Errorf("write raw.md: %w", err)
	}

	if doc.CleanedText != "" {
		if err := os.WriteFile(filepath.Join(dir, "intermediate", "cleaned.md"), []byte(doc.CleanedText), 0644); err != nil {
			return fmt.Errorf("write cleaned.md: %w", err)
		}
	}

	if err := writeChunks(dir, doc); err != nil {
		return err
	}

	finalPath := filepath.Join(dir, "output", "final.md")
	if err := os.WriteFile(finalPath, []byte(doc.Markdown), 0644); err != nil {
		return fmt.Errorf("write output/final.md: %w", err)
	}

	titlePath := filepath.Join(dir, sanitized+".md")
	if err := os.WriteFile(titlePath, []byte(doc.Markdown), 0644); err != nil {
		return fmt.Errorf("write %s: %w", titlePath, err)
	}

	return nil
}

func writeChunks(dir string, doc *model.RAGDocument) error {
	sort.Slice(doc.Chunks, func(i, j int) bool {
		return doc.Chunks[i].Index < doc.Chunks[j].Index
	})

	for _, ch := range doc.Chunks {
		if ch.Type == "" && ch.Summary == "" {
			continue
		}

		var cb strings.Builder
		fmt.Fprintf(&cb, "## chunk_%02d\n", ch.Index+1)
		writeChunkField(&cb, "type", ch.Type)
		writeChunkField(&cb, "topic", ch.Topic)
		writeChunkField(&cb, "summary", ch.Summary)

		if len(ch.KeyPoints) > 0 {
			cb.WriteString("key_points:\n")
			for _, kp := range ch.KeyPoints {
				kp = strings.TrimSpace(kp)
				if kp != "" {
					cb.WriteString("- ")
					cb.WriteString(kp)
					cb.WriteString("\n")
				}
			}
		}

		writeChunkField(&cb, "source_quote", ch.SourceQuote)
		writeChunkField(&cb, "my_takeaway", ch.MyTakeaway)
		writeChunkField(&cb, "confidence", pipeline.ConfidenceToString(ch.Confidence))
		writeChunkField(&cb, "applicability", ch.Applicability)

		if len(ch.Steps) > 0 {
			cb.WriteString("steps:\n")
			for _, s := range ch.Steps {
				s = strings.TrimSpace(s)
				if s != "" {
					cb.WriteString("- ")
					cb.WriteString(s)
					cb.WriteString("\n")
				}
			}
		}

		if ch.Content != "" {
			cb.WriteString("\n")
			cb.WriteString(ch.Content)
			cb.WriteString("\n")
		}

		chunkPath := filepath.Join(dir, "chunks", fmt.Sprintf("chunk_%03d.md", ch.Index+1))
		if err := os.WriteFile(chunkPath, []byte(cb.String()), 0644); err != nil {
			return fmt.Errorf("write %s: %w", chunkPath, err)
		}
	}

	return nil
}

func writeChunkField(b *strings.Builder, key, value string) {
	if strings.TrimSpace(value) == "" {
		return
	}
	b.WriteString(key)
	b.WriteString(": ")
	b.WriteString(value)
	b.WriteString("\n")
}

func writeMetadata(dir, source string, doc *model.RAGDocument, wordCount, version int) error {
	meta := map[string]interface{}{
		"source":       source,
		"title":        doc.Metadata.Title,
		"word_count":   wordCount,
		"version":      version,
		"topics":       doc.Metadata.Topics,
		"language":     doc.Metadata.Language,
		"author":       doc.Metadata.Author,
		"core_thesis":  doc.Metadata.CoreThesis,
		"domains":      doc.Metadata.Domains,
		"chunks_total": len(doc.Chunks),
		"status":       "completed",
	}

	metaYAML, err := yaml.Marshal(meta)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}

	if err := os.WriteFile(filepath.Join(dir, ".media2rag.yaml"), metaYAML, 0644); err != nil {
		return fmt.Errorf("write .media2rag.yaml: %w", err)
	}

	return nil
}
