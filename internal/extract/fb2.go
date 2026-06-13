package extract

import (
	"archive/zip"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type FB2Extractor struct{}

func (f *FB2Extractor) Detect(path string) bool {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return false
	}
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".fb2" || ext == ".zip"
}

func (f *FB2Extractor) Extract(ctx context.Context, path string) (string, error) {
	data, err := readFB2Data(path)
	if err != nil {
		return "", err
	}

	var book FictionBook
	if err := xml.Unmarshal(data, &book); err != nil {
		return "", fmt.Errorf("parse fb2: %w", err)
	}

	return extractFB2Content(&book), nil
}

func readFB2Data(path string) ([]byte, error) {
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".zip" {
		return readFromZip(path)
	}
	return os.ReadFile(path)
}

func readFromZip(path string) ([]byte, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("open zip: %w", err)
	}
	defer r.Close()

	for _, f := range r.File {
		if strings.HasSuffix(strings.ToLower(f.Name), ".fb2") {
			rc, err := f.Open()
			if err != nil {
				return nil, fmt.Errorf("open fb2 in zip: %w", err)
			}
			defer rc.Close()
			return io.ReadAll(rc)
		}
	}

	return nil, fmt.Errorf("no .fb2 file found in zip")
}

type FictionBook struct {
	Body []Body `xml:"body"`
}

type Body struct {
	Sections []Section `xml:"section"`
}

type Section struct {
	Title   *Title   `xml:"title"`
	Paragraphs []Paragraph `xml:"p"`
	Sections []Section `xml:"section"`
}

type Title struct {
	Paragraphs []Paragraph `xml:"p"`
}

type Paragraph struct {
	Content string `xml:",chardata"`
}

func extractFB2Content(book *FictionBook) string {
	var content strings.Builder

	for _, body := range book.Body {
		for _, section := range body.Sections {
			text := extractSectionText(&section, 0)
			if text != "" {
				content.WriteString(text)
				content.WriteString("\n\n")
			}
		}
	}

	result := content.String()
	if strings.TrimSpace(result) == "" {
		return ""
	}

	return strings.TrimSpace(result)
}

func extractSectionText(section *Section, depth int) string {
	var text strings.Builder

	if section.Title != nil {
		level := depth + 2
		if level > 6 {
			level = 6
		}
		prefix := strings.Repeat("#", level)
		for _, p := range section.Title.Paragraphs {
			trimmed := strings.TrimSpace(p.Content)
			if trimmed != "" && !isFB2Boilerplate(trimmed) {
				text.WriteString(prefix)
				text.WriteString(" ")
				text.WriteString(trimmed)
				text.WriteString("\n\n")
			}
		}
	}

	for _, p := range section.Paragraphs {
		trimmed := strings.TrimSpace(p.Content)
		if trimmed != "" {
			text.WriteString(trimmed)
			text.WriteString("\n\n")
		}
	}

	for _, sub := range section.Sections {
		subText := extractSectionText(&sub, depth+1)
		if subText != "" {
			text.WriteString(subText)
		}
	}

	return text.String()
}

func isFB2Boilerplate(text string) bool {
	lower := strings.ToLower(text)
	boilerplate := []string{
		"содержание", "оглавление", "об авторе", "обложка",
		"титул", "посвящение", "благодарности",
		"contents", "about the author", "cover", "title page",
	}

	for _, b := range boilerplate {
		if strings.Contains(lower, b) {
			return true
		}
	}
	return false
}
