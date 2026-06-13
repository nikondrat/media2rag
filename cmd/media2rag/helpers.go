package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"media2rag/internal/extract"
)

func dryRunDirectory(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read directory: %w", err)
	}

	fmt.Fprintf(os.Stderr, "files to process in %s:\n", dir)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := filepath.Ext(entry.Name())
		if ext != ".md" && ext != ".markdown" {
			continue
		}
		if strings.HasPrefix(entry.Name(), "._") {
			continue
		}
		fmt.Fprintf(os.Stderr, "  %s\n", entry.Name())
	}
	return nil
}

func unsupportedMsg(source string) string {
	ext := filepath.Ext(source)
	if ext != "" {
		return fmt.Sprintf("unsupported file format: %s (%s support coming in v2)", ext, ext[1:])
	}
	return fmt.Sprintf("unsupported source: %s", source)
}

func countWords(s string) int {
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

func parseFileMetadata(source string) (author, language string) {
	if info, err := os.Stat(source); err == nil && !info.IsDir() {
		if raw, err := os.ReadFile(source); err == nil {
			fm := extract.ParseFrontmatter(string(raw))
			if a, ok := fm["author"]; ok {
				author = a
			}
			if l, ok := fm["language"]; ok {
				language = l
			} else if _, ok := fm["lang"]; ok {
				language = fm["lang"]
			}
		}
	}
	return
}
