package events

import (
	"encoding/json"
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
