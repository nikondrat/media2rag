package pipeline

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"media2rag/internal/events"
	"media2rag/internal/llm"
	"media2rag/internal/model"
)

const visionDescribePrompt = `Describe this image in detail for use in a RAG (Retrieval Augmented Generation) system.
Focus on:
- What the image shows (objects, people, scenes, diagrams)
- Any text visible in the image (OCR)
- Key data points, numbers, labels, or annotations
- Relationships between elements

Write in the same language as the surrounding document.
Be concise but comprehensive. 2-4 sentences maximum.`

type visionJob struct {
	index int
	image model.ExtractedImage
}

type ImageDescription struct {
	Image       model.ExtractedImage
	Description string
}

type VisionProcessor struct {
	client  llm.LLMClient
	maxConc int
}

func NewVisionProcessor(client llm.LLMClient, maxConc int) *VisionProcessor {
	if maxConc <= 0 {
		maxConc = 3
	}
	return &VisionProcessor{
		client:  client,
		maxConc: maxConc,
	}
}

func (v *VisionProcessor) Process(ctx context.Context, images []model.ExtractedImage, emitter events.EventEmitter) ([]ImageDescription, error) {
	if len(images) == 0 {
		return nil, nil
	}

	emitter.Emit(model.Event{Type: EventVisionStart, Data: map[string]int{"images": len(images)}})

	results := make([]ImageDescription, len(images))

	pool := &WorkerPool[visionJob]{
		NumWorkers: v.maxConc,
		ProcessFn: func(ctx context.Context, job visionJob) error {
			emitter.Emit(model.Event{Type: EventVisionImage, Data: map[string]int{
				"image": job.index + 1,
				"total": len(images),
			}})

			desc, err := v.describeImage(ctx, job.image)
			if err != nil {
				return fmt.Errorf("describe image %d/%d: %w", job.index+1, len(images), err)
			}

			results[job.index] = ImageDescription{
				Image:       job.image,
				Description: desc,
			}
			return nil
		},
	}

	jobs := make([]visionJob, len(images))
	for i, img := range images {
		jobs[i] = visionJob{index: i, image: img}
	}

	if err := pool.Run(ctx, jobs); err != nil {
		return results, err
	}

	emitter.Emit(model.Event{Type: EventVisionDone, Data: map[string]int{"images": len(images)}})
	return results, nil
}

func (v *VisionProcessor) describeImage(ctx context.Context, img model.ExtractedImage) (string, error) {
	imgPath := img.Path

	if isPPMFormat(imgPath) {
		converted, cleanup, err := convertToJPEG(imgPath)
		if err != nil {
			return "", fmt.Errorf("convert PPM: %w", err)
		}
		defer cleanup()
		imgPath = converted
	}

	data, err := os.ReadFile(imgPath)
	if err != nil {
		return "", fmt.Errorf("read image: %w", err)
	}

	mimeType := http.DetectContentType(data)
	if mimeType == "application/octet-stream" {
		mimeType = "image/jpeg"
	}

	b64 := base64.StdEncoding.EncodeToString(data)
	imageData := fmt.Sprintf("data:%s;base64,%s", mimeType, b64)

	callCtx, cancel := v.timeoutCtx(ctx)
	callCtx = llm.WithStage(callCtx, "vision")
	defer cancel()

	resp, err := v.client.Chat(callCtx, model.ChatRequest{
		Messages: []model.Message{
			{Role: "user", Content: visionDescribePrompt},
		},
		Images: []string{imageData},
	})
	if err != nil {
		return "", fmt.Errorf("llm chat: %w", err)
	}

	content := strings.TrimSpace(resp.Message.Content)
	if content == "" {
		return "", fmt.Errorf("llm returned empty response")
	}

	return content, nil
}

func (v *VisionProcessor) timeoutCtx(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, 60*time.Second)
}

const (
	EventVisionStart = "vision_start"
	EventVisionImage = "vision_image"
	EventVisionDone  = "vision_done"
)

func isPPMFormat(path string) bool {
	ext := strings.ToLower(path)
	return strings.HasSuffix(ext, ".ppm") || strings.HasSuffix(ext, ".pbm") || strings.HasSuffix(ext, ".pgm")
}

func convertToJPEG(srcPath string) (string, func(), error) {
	if _, err := exec.LookPath("convert"); err != nil {
		return "", nil, fmt.Errorf("ImageMagick not found: %w", err)
	}

	ext := filepath.Ext(srcPath)
	dstPath := strings.TrimSuffix(srcPath, ext) + ".jpg"

	cmd := exec.Command("convert", srcPath, "-quality", "90", dstPath)
	if err := cmd.Run(); err != nil {
		return "", nil, fmt.Errorf("convert failed: %w", err)
	}

	cleanup := func() { os.Remove(dstPath) }
	return dstPath, cleanup, nil
}
