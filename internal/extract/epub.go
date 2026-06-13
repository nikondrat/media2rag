package extract

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"media2rag/internal/model"
)

type EPUBExtractor struct{}

func (e *EPUBExtractor) ContentType() string {
	return ContentTypeBook
}

func (e *EPUBExtractor) Detect(path string) bool {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return false
	}
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".epub"
}

func (e *EPUBExtractor) Extract(ctx context.Context, path string) (string, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return "", fmt.Errorf("open epub: %w", err)
	}
	defer r.Close()

	contentFiles := findContentFiles(r)
	if len(contentFiles) == 0 {
		return "", fmt.Errorf("no content files found in epub")
	}

	var content strings.Builder
	for _, f := range contentFiles {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}

		text, err := extractHTMLContent(r, f)
		if err != nil {
			continue
		}

		if text == "" || isBoilerplate(filepath.Base(f)) {
			continue
		}

		if content.Len() > 0 {
			content.WriteString("\n\n")
		}
		content.WriteString(text)
	}

	result := content.String()
	if strings.TrimSpace(result) == "" {
		return "", fmt.Errorf("epub contains no readable content")
	}

	return result, nil
}

func findContentFiles(r *zip.ReadCloser) []string {
	var files []string
	for _, f := range r.File {
		name := strings.ToLower(f.Name)
		if strings.HasSuffix(name, ".html") || strings.HasSuffix(name, ".xhtml") || strings.HasSuffix(name, ".htm") {
			if !strings.Contains(name, "toc") && !strings.Contains(name, "nav") {
				files = append(files, f.Name)
			}
		}
	}
	return files
}

func extractHTMLContent(r *zip.ReadCloser, name string) (string, error) {
	for _, f := range r.File {
		if f.Name != name {
			continue
		}

		rc, err := f.Open()
		if err != nil {
			return "", err
		}
		defer rc.Close()

		data, err := io.ReadAll(rc)
		if err != nil {
			return "", err
		}

		return extractTextFromHTML(string(data)), nil
	}
	return "", fmt.Errorf("file not found: %s", name)
}

func isBoilerplate(title string) bool {
	lower := strings.ToLower(strings.TrimSpace(title))
	boilerplate := []string{
		"copyright", "title page", "table of contents",
		"about the author", "acknowledgments", "dedication",
		"also by", "other books", "about this book",
	}

	for _, b := range boilerplate {
		if strings.Contains(lower, b) {
			return true
		}
	}
	return false
}

func extractTextFromHTML(html string) string {
	text := html

	replacements := []struct {
		old, new string
	}{
		{"<br>", "\n"}, {"<br/>", "\n"}, {"<br />", "\n"},
		{"</p>", "\n\n"}, {"</div>", "\n"}, {"</li>", "\n"},
		{"</h1>", "\n\n"}, {"</h2>", "\n\n"}, {"</h3>", "\n\n"},
		{"</h4>", "\n\n"}, {"</tr>", "\n"},
	}

	for _, r := range replacements {
		text = strings.ReplaceAll(text, r.old, r.new)
	}

	var result strings.Builder
	inTag := false
	for _, c := range text {
		switch {
		case c == '<':
			inTag = true
		case c == '>':
			inTag = false
		case !inTag:
			result.WriteRune(c)
		}
	}

	cleaned := result.String()
	cleaned = strings.ReplaceAll(cleaned, "&nbsp;", " ")
	cleaned = strings.ReplaceAll(cleaned, "&amp;", "&")
	cleaned = strings.ReplaceAll(cleaned, "&lt;", "<")
	cleaned = strings.ReplaceAll(cleaned, "&gt;", ">")
	cleaned = strings.ReplaceAll(cleaned, "&quot;", "\"")

	var lines []string
	for _, line := range strings.Split(cleaned, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			lines = append(lines, trimmed)
		}
	}

	return strings.Join(lines, "\n")
}

func (e *EPUBExtractor) ExtractImages(ctx context.Context, path string, outDir string) ([]model.ExtractedImage, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("open epub: %w", err)
	}
	defer r.Close()

	if err := os.MkdirAll(outDir, 0755); err != nil {
		return nil, fmt.Errorf("create images dir: %w", err)
	}

	var images []model.ExtractedImage
	for _, f := range r.File {
		if ctx.Err() != nil {
			return images, ctx.Err()
		}

		name := strings.ToLower(f.Name)
		ext := filepath.Ext(name)
		if ext != ".jpg" && ext != ".jpeg" && ext != ".png" && ext != ".gif" && ext != ".svg" {
			continue
		}

		rc, err := f.Open()
		if err != nil {
			continue
		}

		outPath := filepath.Join(outDir, filepath.Base(f.Name))
		outFile, err := os.Create(outPath)
		if err != nil {
			rc.Close()
			continue
		}

		if _, err := io.Copy(outFile, rc); err != nil {
			rc.Close()
			outFile.Close()
			continue
		}
		rc.Close()
		outFile.Close()

		images = append(images, model.ExtractedImage{
			Path:    outPath,
			AltText: fmt.Sprintf("Image from EPUB: %s", filepath.Base(f.Name)),
		})
	}

	return images, nil
}
