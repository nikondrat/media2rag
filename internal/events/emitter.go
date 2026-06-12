package events

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"media2rag/internal/model"
)

const teeFileFlags = os.O_APPEND | os.O_CREATE | os.O_WRONLY

type EventEmitter interface {
	Emit(model.Event)
	Done()
}

type StdoutEmitter struct {
	mu     sync.Mutex
	enc    *json.Encoder
}

func NewStdoutEmitter() *StdoutEmitter {
	return &StdoutEmitter{
		enc: json.NewEncoder(os.Stdout),
	}
}

func (e *StdoutEmitter) Emit(evt model.Event) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.enc.Encode(evt)
}

func (e *StdoutEmitter) Done() {}

type NoopEmitter struct{}

func NewNoopEmitter() *NoopEmitter {
	return &NoopEmitter{}
}

func (e *NoopEmitter) Emit(model.Event) {}

func (e *NoopEmitter) Done() {}

type TeeEmitter struct {
	inner EventEmitter
	file  *os.File
	mu    sync.Mutex
}

func NewTeeEmitter(inner EventEmitter, logPath string) (*TeeEmitter, error) {
	if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
		return nil, fmt.Errorf("create log dir: %w", err)
	}
	f, err := os.OpenFile(logPath, teeFileFlags, 0644)
	if err != nil {
		return nil, fmt.Errorf("create log file: %w", err)
	}
	return &TeeEmitter{inner: inner, file: f}, nil
}

func (e *TeeEmitter) Emit(evt model.Event) {
	e.mu.Lock()
	defer e.mu.Unlock()

	line := fmt.Sprintf("[%s] %s", time.Now().Format(time.RFC3339), formatEvent(&evt))
	fmt.Fprintln(e.file, line)

	e.inner.Emit(evt)
}

func (e *TeeEmitter) Done() {
	e.inner.Done()
	if e.file != nil {
		e.file.Close()
	}
}

func formatEvent(evt *model.Event) string {
	if evt.Error != "" {
		return "ERROR: " + evt.Error
	}
	detail := evt.Type
	switch d := evt.Data.(type) {
	case map[string]int:
		for k, v := range d {
			detail += fmt.Sprintf(" %s=%d", k, v)
		}
	case map[string]string:
		for k, v := range d {
			if len(v) > 80 {
				v = v[:80] + "..."
			}
			detail += fmt.Sprintf(" %s=%s", k, v)
		}
	case map[string]interface{}:
		for k, v := range d {
			detail += fmt.Sprintf(" %s=%v", k, v)
		}
	}
	return detail
}



type HumanEmitter struct {
	mu sync.Mutex
}

func NewHumanEmitter() *HumanEmitter {
	return &HumanEmitter{}
}

func (e *HumanEmitter) Emit(evt model.Event) {
	e.mu.Lock()
	defer e.mu.Unlock()

	switch evt.Type {
	case "extracting":
		source, _ := evt.Data.(map[string]string)["source"]
		fmt.Fprintf(os.Stderr, "extracting: %s\n", source)
	case "extracted":
		fmt.Fprintf(os.Stderr, "extracted successfully\n")
	case "saving":
		fmt.Fprintf(os.Stderr, "saving to workspace...\n")
	case "pipeline_start":
		fmt.Fprintf(os.Stderr, "pipeline: starting...\n")
	case "pre_clean":
		fmt.Fprintf(os.Stderr, "pipeline: pre-cleaning document...\n")
	case "pre_clean_done":
		if data, ok := evt.Data.(map[string]int); ok {
			fmt.Fprintf(os.Stderr, "pipeline: pre-cleaned %d chars\n", data["text_length"])
		} else {
			fmt.Fprintf(os.Stderr, "pipeline: pre-clean done\n")
		}
	case "compression_start":
		fmt.Fprintf(os.Stderr, "pipeline: compressing...\n")
	case "cleaning_part":
		data, _ := evt.Data.(map[string]int)
		fmt.Fprintf(os.Stderr, "pipeline: cleaning part %d/%d\n", data["part"], data["total"])
	case "compression_done":
		fmt.Fprintf(os.Stderr, "pipeline: compression done\n")
	case "splitting":
		fmt.Fprintf(os.Stderr, "pipeline: splitting...\n")
	case "split_done":
		data, _ := evt.Data.(map[string]int)
		fmt.Fprintf(os.Stderr, "pipeline: %d chunks\n", data["chunks"])
	case "processing_start":
		data, _ := evt.Data.(map[string]int)
		fmt.Fprintf(os.Stderr, "pipeline: processing %d chunks...\n", data["total"])
	case "processing_chunk":
		data, _ := evt.Data.(map[string]int)
		fmt.Fprintf(os.Stderr, "pipeline: processing chunk %d/%d\n", data["chunk"], data["total"])
	case "processing_chunk_done":
		data, _ := evt.Data.(map[string]int)
		fmt.Fprintf(os.Stderr, "pipeline: chunk %d done\n", data["chunk"])
	case "processing_retry":
		data, _ := evt.Data.(map[string]interface{})
		chunk, _ := data["chunk"].(int)
		attempt, _ := data["attempt"].(int)
		errMsg, _ := data["error"].(string)
		fmt.Fprintf(os.Stderr, "pipeline: chunk %d retry %d: %s\n", chunk, attempt, errMsg)
	case "checkpoint_restore":
		data, _ := evt.Data.(map[string]string)
		fmt.Fprintf(os.Stderr, "pipeline: loaded %s from checkpoint\n", data["stage"])
	case "processing_done":
		fmt.Fprintf(os.Stderr, "pipeline: all chunks processed\n")
	case "holistic_analysis":
		fmt.Fprintf(os.Stderr, "pipeline: holistic analysis...\n")
	case "holistic_done":
		fmt.Fprintf(os.Stderr, "pipeline: holistic analysis done\n")
	case "context_enrichment":
		fmt.Fprintf(os.Stderr, "pipeline: enriching chunks with context...\n")
	case "context_enrichment_done":
		if data, ok := evt.Data.(map[string]int); ok {
			fmt.Fprintf(os.Stderr, "pipeline: chunk %d context enriched\n", data["chunk"])
		} else {
			fmt.Fprintf(os.Stderr, "pipeline: context enrichment done\n")
		}
	case "assembling":
		fmt.Fprintf(os.Stderr, "pipeline: assembling...\n")
	case "pipeline_completed":
		data, _ := evt.Data.(map[string]interface{})
		title, _ := data["title"].(string)
		fmt.Fprintf(os.Stderr, "pipeline: completed title=%s\n", title)
	case "completed":
		data, _ := evt.Data.(map[string]interface{})
		hash, _ := data["hash"].(string)
		outputPath, _ := data["output_path"].(string)
		fmt.Fprintf(os.Stderr, "completed: hash=%s path=%s\n", hash, outputPath)
	case "error":
		fmt.Fprintf(os.Stderr, "error: %s\n", evt.Error)
	}
}

func (e *HumanEmitter) Done() {
	fmt.Fprintf(os.Stderr, "done.\n")
}
