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

type VideoExtractor struct{}

func (v *VideoExtractor) ContentType() string {
	return ContentTypeTranscript
}

var videoExtensions = map[string]bool{
	".mp4": true, ".mkv": true, ".avi": true,
	".mov": true, ".webm": true, ".flv": true,
}

func (v *VideoExtractor) Detect(path string) bool {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return false
	}
	ext := strings.ToLower(filepath.Ext(path))
	return videoExtensions[ext]
}

func (v *VideoExtractor) Extract(ctx context.Context, path string) (string, error) {
	if err := checkCommand("ffmpeg"); err != nil {
		return "", err
	}
	if err := checkCommand("whisper"); err != nil {
		return "", err
	}

	tmpDir, err := os.MkdirTemp("", "media2rag-video-*")
	if err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	audioPath := filepath.Join(tmpDir, "audio.wav")
	if err := extractAudio(ctx, path, audioPath); err != nil {
		return "", err
	}

	return transcribeAudio(ctx, audioPath, tmpDir)
}

func extractAudio(ctx context.Context, videoPath, audioPath string) error {
	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-i", videoPath,
		"-vn",
		"-acodec", "pcm_s16le",
		"-ar", "16000",
		"-ac", "1",
		audioPath,
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg failed: %w\nstderr: %s", err, stderr.String())
	}
	return nil
}

func transcribeAudio(ctx context.Context, audioPath, outputDir string) (string, error) {
	cmd := exec.CommandContext(ctx, "whisper", audioPath,
		"--output_format", "txt",
		"--output_dir", outputDir,
		"--model", "base",
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("whisper failed: %w\nstderr: %s", err, stderr.String())
	}

	basename := strings.TrimSuffix(filepath.Base(audioPath), filepath.Ext(audioPath))
	txtPath := filepath.Join(outputDir, basename+".txt")
	data, err := os.ReadFile(txtPath)
	if err != nil {
		return "", fmt.Errorf("read whisper output: %w", err)
	}

	return string(data), nil
}

func (v *VideoExtractor) ExtractImages(ctx context.Context, path string, outDir string) ([]model.ExtractedImage, error) {
	return nil, nil
}
