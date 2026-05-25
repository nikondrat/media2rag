package events

import (
	"bytes"
	"encoding/json"
	"testing"

	"media2rag/internal/model"
)

func TestStdoutEmitter_EmitsJSONLines(t *testing.T) {
	var buf bytes.Buffer
	e := &StdoutEmitter{enc: json.NewEncoder(&buf)}

	e.Emit(model.Event{Type: "start", Progress: 0})
	e.Emit(model.Event{Type: "progress", Progress: 0.5})
	e.Emit(model.Event{Type: "done", Progress: 1.0})

	lines := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n"))
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}

	var evt model.Event
	for i, line := range lines {
		if err := json.Unmarshal(line, &evt); err != nil {
			t.Fatalf("line %d: invalid JSON: %v", i, err)
		}
	}
}

func TestNoopEmitter(t *testing.T) {
	e := NewNoopEmitter()
	e.Emit(model.Event{Type: "test"})
	e.Done()
}
