package events

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"

	"media2rag/internal/model"
)

type ProgressEmitter struct {
	ch     chan model.Event
	doneCh chan struct{}
	barCh  chan struct{}

	p   *mpb.Progress
	bar *mpb.Bar

	stateMu    sync.RWMutex
	total      int
	completed  int
	failed     int
	skipped    int
	active     int
	fileName   string
	stage      string
	start      time.Time
	compDur    time.Duration
}

func NewProgressEmitter(total int, concurrency int) *ProgressEmitter {
	e := &ProgressEmitter{
		total:  total,
		ch:     make(chan model.Event, 200),
		doneCh: make(chan struct{}),
		barCh:  make(chan struct{}),
		start:  time.Now(),
		stage:  "starting",
	}
	if total <= 1 {
		close(e.barCh)
		return e
	}

	e.p = mpb.New(
		mpb.WithWidth(120),
		mpb.WithRefreshRate(180*time.Millisecond),
	)
	e.bar = e.p.New(int64(total),
		mpb.BarStyle().Lbound("╢").Filler("▌").Tip("▌").Padding("░").Rbound("╟"),
		mpb.PrependDecorators(
			decor.Any(func(decor.Statistics) string {
				e.stateMu.RLock()
				s := fmt.Sprintf("[%d/%d]", e.completed+e.failed+e.skipped, e.total)
				e.stateMu.RUnlock()
				return s
			}),
			decor.Any(func(decor.Statistics) string {
				e.stateMu.RLock()
				a := e.active
				e.stateMu.RUnlock()
				if a > 0 {
					return fmt.Sprintf("%d active", a)
				}
				return ""
			}, decor.WC{W: 10, C: decor.DindentRight}),
			decor.Any(func(decor.Statistics) string {
				e.stateMu.RLock()
				n := e.fileName
				e.stateMu.RUnlock()
				if len(n) > 40 {
					n = n[:37] + "..."
				}
				return n
			}, decor.WC{W: 43, C: decor.DindentRight}),
			decor.Any(func(decor.Statistics) string {
				e.stateMu.RLock()
				s := e.stage
				e.stateMu.RUnlock()
				return s
			}, decor.WC{W: 18, C: decor.DindentRight}),
			decor.Any(func(decor.Statistics) string {
				e.stateMu.RLock()
				comp := e.completed
				dur := e.compDur
				active := e.active
				total := e.total
				failed := e.failed
				skipped := e.skipped
				e.stateMu.RUnlock()
				return etaStr(comp, dur, total, failed, skipped, active)
			}, decor.WC{W: 14, C: decor.DindentRight}),
		),
	)

	go e.loop()
	return e
}

func (e *ProgressEmitter) Emit(evt model.Event) {
	select {
	case e.ch <- evt:
	default:
	}
}

func (e *ProgressEmitter) Done() {}

func (e *ProgressEmitter) FileStart(name string) {
	e.ch <- model.Event{Type: "file_start", Data: map[string]string{"name": name}}
}

func (e *ProgressEmitter) FileDone(name string) {
	e.ch <- model.Event{Type: "file_done", Data: map[string]string{"name": name}}
}

func (e *ProgressEmitter) FileSkipped(name string) {
	e.ch <- model.Event{Type: "file_skip", Data: map[string]string{"name": name}}
}

func (e *ProgressEmitter) FileError(name string, err error) {
	e.ch <- model.Event{Type: "file_error", Data: map[string]interface{}{
		"name": name, "error": err.Error(),
	}}
}

func (e *ProgressEmitter) Wait() {
	<-e.barCh
	close(e.ch)
	<-e.doneCh
	if e.p != nil {
		e.p.Wait()
	}
}

func (e *ProgressEmitter) loop() {
	fileStartTimes := make(map[string]time.Time)

	for evt := range e.ch {
		switch evt.Type {
		case "file_start":
			name, _ := evt.Data.(map[string]string)["name"]
			e.stateMu.Lock()
			e.active++
			e.fileName = name
			e.stage = "starting"
			fileStartTimes[name] = time.Now()
			e.stateMu.Unlock()

		case "file_done":
			name, _ := evt.Data.(map[string]string)["name"]
			e.stateMu.Lock()
			e.completed++
			e.active--
			if t, ok := fileStartTimes[name]; ok {
				e.compDur += time.Since(t)
				delete(fileStartTimes, name)
			}
			done := e.completed+e.failed+e.skipped >= e.total
			e.stateMu.Unlock()
			if e.bar != nil {
				e.bar.Increment()
			}
			if done {
				close(e.barCh)
			}

		case "file_skip":
			e.stateMu.Lock()
			e.skipped++
			done := e.completed+e.failed+e.skipped >= e.total
			e.stateMu.Unlock()
			if e.bar != nil {
				e.bar.Increment()
			}
			if done {
				close(e.barCh)
			}

		case "file_error":
			data := evt.Data.(map[string]interface{})
			name, _ := data["name"].(string)
			errStr, _ := data["error"].(string)
			e.stateMu.Lock()
			e.failed++
			e.active--
			if t, ok := fileStartTimes[name]; ok {
				e.compDur += time.Since(t)
				delete(fileStartTimes, name)
			}
			done := e.completed+e.failed+e.skipped >= e.total
			e.stateMu.Unlock()
			if e.bar != nil {
				e.bar.Increment()
			}
			fmt.Fprintf(os.Stderr, "\nERROR: %s: %s\n", name, errStr)
			if done {
				close(e.barCh)
			}

		default:
			e.stateMu.Lock()
			e.stage = stageFromEvent(evt, e.stage)
			if name, ok := eventFileName(evt); ok && name != "" {
				e.fileName = name
			}
			e.stateMu.Unlock()
		}
	}

	close(e.doneCh)
}

func stageFromEvent(evt model.Event, current string) string {
	switch evt.Type {
	case "pipeline_start":
		return "pipeline"
	case "pre_clean", "pre_clean_done":
		return "cleaning"
	case "splitting", "split_done":
		if data, ok := evt.Data.(map[string]int); ok && evt.Type == "split_done" {
			return fmt.Sprintf("%d chunks", data["chunks"])
		}
		return "splitting"
	case "processing_start":
		return "processing"
	case "processing_chunk":
		if data, ok := evt.Data.(map[string]int); ok {
			return fmt.Sprintf("chunk %d/%d", data["chunk"], data["total"])
		}
	case "processing_retry":
		if data, ok := evt.Data.(map[string]interface{}); ok {
			chunk, _ := data["chunk"].(int)
			attempt, _ := data["attempt"].(int)
			return fmt.Sprintf("chunk %d retry %d", chunk, attempt)
		}
	case "holistic_analysis", "holistic_done":
		return "holistic"
	case "causal_extraction", "causal_done":
		return "causal"
	case "context_enrichment", "context_enrichment_done":
		return "enriching"
	case "assembling":
		return "assembling"
	case "completed":
		return "done"
	}
	return current
}

func eventFileName(evt model.Event) (string, bool) {
	switch d := evt.Data.(type) {
	case map[string]string:
		if n, ok := d["source"]; ok {
			return n, true
		}
	case map[string]interface{}:
		if n, ok := d["source"].(string); ok {
			return n, true
		}
	}
	return "", false
}

func etaStr(completed int, compDur time.Duration, total, failed, skipped, active int) string {
	if completed < 1 || active == 0 {
		return "ETA ..."
	}
	remaining := total - completed - failed - skipped
	if remaining <= 0 {
		return "done"
	}
	perFile := compDur / time.Duration(completed)
	eta := time.Duration(remaining) * perFile / time.Duration(active)

	d := eta.Round(time.Second)
	if d < time.Minute {
		return fmt.Sprintf("ETA %ds", int(d.Seconds()))
	}
	totalSec := int(d.Seconds())
	h := totalSec / 3600
	m := (totalSec % 3600) / 60
	s := totalSec % 60
	if h > 0 {
		return fmt.Sprintf("ETA %dh%02dm", h, m)
	}
	if s > 0 {
		return fmt.Sprintf("ETA %dm%02ds", m, s)
	}
	return fmt.Sprintf("ETA %dm", m)
}
