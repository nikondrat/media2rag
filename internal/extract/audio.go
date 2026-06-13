package extract

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"media2rag/internal/model"
)

type AudioExtractor struct{}

func (a *AudioExtractor) ContentType() string {
	return ContentTypeTranscript
}

var audioExtensions = map[string]bool{
	".mp3": true, ".wav": true, ".flac": true,
	".ogg": true, ".m4a": true, ".aac": true,
}

func (a *AudioExtractor) Detect(path string) bool {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return false
	}
	ext := strings.ToLower(filepath.Ext(path))
	return audioExtensions[ext]
}

func (a *AudioExtractor) Extract(ctx context.Context, path string) (string, error) {
	if err := checkCommand("whisper"); err != nil {
		return "", err
	}

	tmpDir, err := os.MkdirTemp("", "media2rag-audio-*")
	if err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	cmd := exec.CommandContext(ctx, "whisper", path,
		"--output_format", "txt",
		"--output_dir", tmpDir,
		"--model", "base",
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("whisper failed: %w\nstderr: %s", err, stderr.String())
	}

	basename := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	txtPath := filepath.Join(tmpDir, basename+".txt")
	data, err := os.ReadFile(txtPath)
	if err != nil {
		return "", fmt.Errorf("read whisper output: %w", err)
	}

	return string(data), nil
}

func checkCommand(name string) error {
	_, err := exec.LookPath(name)
	if err != nil {
		var pathErr *exec.Error
		if errors.As(err, &pathErr) && pathErr.Err == exec.ErrNotFound {
			return fmt.Errorf("%s not found. Install: media2rag setup", name)
		}
		return err
	}
	return nil
}

func (a *AudioExtractor) ExtractImages(ctx context.Context, path string, outDir string) ([]model.ExtractedImage, error) {
	return nil, nil
}
