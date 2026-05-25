package events

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"media2rag/internal/model"
)

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

type HumanEmitter struct{}

func NewHumanEmitter() *HumanEmitter {
	return &HumanEmitter{}
}

func (e *HumanEmitter) Emit(evt model.Event) {
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
	case "checkpoint_restore":
		data, _ := evt.Data.(map[string]string)
		fmt.Fprintf(os.Stderr, "pipeline: loaded %s from checkpoint\n", data["stage"])
	case "processing_done":
		fmt.Fprintf(os.Stderr, "pipeline: all chunks processed\n")
	case "holistic_analysis":
		fmt.Fprintf(os.Stderr, "pipeline: holistic analysis...\n")
	case "holistic_done":
		fmt.Fprintf(os.Stderr, "pipeline: holistic analysis done\n")
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
