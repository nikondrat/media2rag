package events

import (
	"fmt"
	"sync"
	"time"

	"github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"

	"media2rag/internal/model"
)

type ProgressEmitter struct {
	inner      EventEmitter
	p          *mpb.Progress
	bar        *mpb.Bar
	mu         sync.Mutex
	total      int
	current    int
	fileName   string
	stage      string
	chunkIdx  int
	chunkTot  int
	start     time.Time
	stageStart time.Time
	completed int
}

func NewProgressEmitter(inner EventEmitter, total int) *ProgressEmitter {
	e := &ProgressEmitter{
		inner:      inner,
		total:      total,
		start:      time.Now(),
		stageStart: time.Now(),
	}

	if total <= 1 {
		return e
	}

	e.p = mpb.New(
		mpb.WithWidth(120),
		mpb.WithRefreshRate(180*time.Millisecond),
	)

	e.bar = e.p.New(int64(total),
		mpb.BarStyle().Lbound("╢").Filler("▌").Tip("▌").Padding("░").Rbound("╟"),
		mpb.PrependDecorators(
			decor.Any(func(s decor.Statistics) string {
				e.mu.Lock()
				defer e.mu.Unlock()
				return fmt.Sprintf("[%d/%d]", e.current, e.total)
			}),
			decor.Any(func(s decor.Statistics) string {
				e.mu.Lock()
				defer e.mu.Unlock()
				name := e.fileName
				if len(name) > 50 {
					name = name[:47] + "..."
				}
				return name
			}, decor.WC{W: 53, C: decor.DindentRight}),
			decor.Any(func(s decor.Statistics) string {
				e.mu.Lock()
				defer e.mu.Unlock()
				return e.stage
			}, decor.WC{W: 18, C: decor.DindentRight}),
			decor.Any(func(s decor.Statistics) string {
				e.mu.Lock()
				defer e.mu.Unlock()
				return e.etaStr()
			}, decor.WC{W: 14, C: decor.DindentRight}),
		),
	)

	e.stage = "starting"
	return e
}

func (e *ProgressEmitter) etaStr() string {
	if e.completed < 1 {
		return "ETA ..."
	}
	elapsed := time.Since(e.start)
	perFile := elapsed / time.Duration(e.completed)
	remaining := e.total - e.completed
	eta := time.Duration(remaining) * perFile
	return formatDur(eta)
}

func (e *ProgressEmitter) SetFile(idx int, name string) {
	e.mu.Lock()
	e.current = idx + 1
	e.fileName = name
	e.stage = "starting"
	e.chunkIdx = 0
	e.chunkTot = 0
	e.stageStart = time.Now()
	e.mu.Unlock()

	if e.bar != nil {
		e.bar.Increment()
	}
}

func (e *ProgressEmitter) Emit(evt model.Event) {
	e.mu.Lock()
	switch evt.Type {
	case "pre_clean":
		e.stage = "cleaning"
		e.stageStart = time.Now()
	case "pre_clean_done":
		e.stage = "cleaned"
	case "compression_start":
		e.stage = "compressing"
		e.stageStart = time.Now()
	case "cleaning_part":
		if data, ok := evt.Data.(map[string]int); ok {
			e.stage = fmt.Sprintf("clean %d/%d", data["part"], data["total"])
		}
	case "compression_done":
		e.stage = "compressed"
	case "splitting":
		e.stage = "splitting"
		e.stageStart = time.Now()
	case "split_done":
		if data, ok := evt.Data.(map[string]int); ok {
			e.chunkTot = data["chunks"]
		}
		e.stage = fmt.Sprintf("split %d chunks", e.chunkTot)
	case "processing_start":
		e.stage = "processing"
		e.stageStart = time.Now()
	case "processing_chunk":
		if data, ok := evt.Data.(map[string]int); ok {
			e.chunkIdx = data["chunk"]
			e.chunkTot = data["total"]
		}
		e.stage = fmt.Sprintf("chunk %d/%d", e.chunkIdx, e.chunkTot)
	case "processing_chunk_done":
		if data, ok := evt.Data.(map[string]int); ok {
			e.chunkIdx = data["chunk"]
		}
		e.stage = fmt.Sprintf("chunk %d/%d ✓", e.chunkIdx, e.chunkTot)
	case "processing_done":
		e.stage = "processed"
	case "holistic_analysis":
		e.stage = "holistic"
		e.stageStart = time.Now()
	case "holistic_done":
		e.stage = "holistic ✓"
	case "context_enrichment":
		e.stage = "enriching ctx"
		e.stageStart = time.Now()
	case "context_enrichment_done":
		e.stage = "enriched"
	case "assembling":
		e.stage = "assembling"
	case "completed":
		e.completed++
		e.stage = "done"
	case "error":
		e.completed++
		e.stage = "FAILED"
	case "checkpoint_restore":
		if data, ok := evt.Data.(map[string]string); ok {
			e.stage = fmt.Sprintf("cache: %s", data["stage"])
		}
	}
	e.mu.Unlock()

	e.inner.Emit(evt)
}

func (e *ProgressEmitter) Done() {
	e.inner.Done()
}

func (e *ProgressEmitter) Close() {
	if e.p != nil {
		e.p.Wait()
	}
}

func formatDur(d time.Duration) string {
	d = d.Round(time.Second)
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	totalSec := int(d.Seconds())
	h := totalSec / 3600
	m := (totalSec % 3600) / 60
	s := totalSec % 60
	if h > 0 {
		return fmt.Sprintf("%dh%02dm", h, m)
	}
	return fmt.Sprintf("%dm%02ds", m, s)
}