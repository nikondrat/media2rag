package extract

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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

	cmd := exec.CommandContext(ctx, "pdftotext", path, "-")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("pdftotext failed: %w\nstderr: %s", err, stderr.String())
	}

	content := stdout.String()
	if strings.TrimSpace(content) == "" {
		return "", fmt.Errorf("pdftotext returned empty output")
	}

	return content, nil
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
		if ext != ".jpg" && ext != ".jpeg" && ext != ".png" && ext != ".ppm" && ext != ".pbm" && ext != ".pgm" {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		images = append(images, model.ExtractedImage{
			Path: filepath.Join(outDir, name),
			AltText: fmt.Sprintf("Image from PDF page"),
			Width:   0,
			Height:  0,
		})
		_ = info
	}

	return images, nil
}
