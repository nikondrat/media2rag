package extract

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode"

	"media2rag/internal/model"
)

type PDFExtractor struct{}

func (p *PDFExtractor) ContentType() string {
	return ContentTypeBook
}

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
	if err := checkCommand("pdfinfo"); err != nil {
		return "", err
	}

	content := p.extractText(ctx, path)

	if needsOCR(content) {
		ocrPath, cleanup, err := p.runOCR(ctx, path)
		if err == nil {
			defer cleanup()
			content = p.extractText(ctx, ocrPath)
		}
	}

	if strings.TrimSpace(content) == "" {
		return "", fmt.Errorf("pdftotext returned empty output")
	}

	return content, nil
}

func (p *PDFExtractor) extractText(ctx context.Context, path string) string {
	pageCount := getPageCount(path)
	if pageCount == 0 {
		cmd := exec.CommandContext(ctx, "pdftotext", path, "-")
		var stdout bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Run()
		return stdout.String()
	}

	var pages []string
	for i := 1; i <= pageCount; i++ {
		pageText := extractPage(path, i)
		if !isTOCPage(pageText) && !isBoilerplatePage(pageText) {
			pages = append(pages, pageText)
		}
	}

	return strings.Join(pages, "\n\n")
}

func needsOCR(text string) bool {
	if len(text) < 100 {
		return true
	}

	runes := []rune(text)

	uniqueLetters := map[rune]bool{}
	letterCount := 0
	for _, r := range runes {
		if unicode.IsLetter(r) {
			uniqueLetters[r] = true
			letterCount++
		}
	}

	if letterCount == 0 {
		return true
	}

	diversity := float64(len(uniqueLetters)) / float64(letterCount)
	if diversity < 0.05 {
		return true
	}

	repeated := 0
	for i := 1; i < len(runes); i++ {
		if runes[i] == runes[i-1] {
			repeated++
		}
	}
	repeatRatio := float64(repeated) / float64(len(runes))

	if repeatRatio > 0.3 {
		return true
	}

	return false
}

func (p *PDFExtractor) runOCR(ctx context.Context, path string) (string, func(), error) {
	if _, err := exec.LookPath("ocrmypdf"); err != nil {
		return "", nil, fmt.Errorf("ocrmypdf not found: %w", err)
	}

	tmp, err := os.CreateTemp("", "ocr-*.pdf")
	if err != nil {
		return "", nil, err
	}
	tmp.Close()
	os.Remove(tmp.Name())

	cmd := exec.CommandContext(ctx, "ocrmypdf",
		"--skip-text",
		"--deskew",
		"--language", "rus+eng",
		path, tmp.Name())
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		os.Remove(tmp.Name())
		return "", nil, fmt.Errorf("ocrmypdf failed: %w\nstderr: %s", err, stderr.String())
	}

	cleanup := func() { os.Remove(tmp.Name()) }
	return tmp.Name(), cleanup, nil
}

func (p *PDFExtractor) ExtractImages(ctx context.Context, path string, outDir string) ([]model.ExtractedImage, error) {
	if err := checkCommand("pdfimages"); err != nil {
		return nil, nil
	}

	if err := os.MkdirAll(outDir, 0755); err != nil {
		return nil, fmt.Errorf("create images dir: %w", err)
	}

	prefix := filepath.Join(outDir, "img")
	cmd := exec.CommandContext(ctx, "pdfimages", "-j", path, prefix)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("pdfimages failed: %w\nstderr: %s", err, stderr.String())
	}

	entries, err := os.ReadDir(outDir)
	if err != nil {
		return nil, fmt.Errorf("read images dir: %w", err)
	}

	var images []model.ExtractedImage
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if ext != ".jpg" && ext != ".jpeg" && ext != ".png" {
			continue
		}

		images = append(images, model.ExtractedImage{
			Path:    filepath.Join(outDir, name),
			AltText: "Image from PDF page",
			Width:   0,
			Height:  0,
		})
	}

	convertPPMFiles(outDir)

	return images, nil
}
