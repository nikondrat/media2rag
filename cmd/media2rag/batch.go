package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"media2rag/internal/events"
	"media2rag/internal/pipeline"
)

type fileResult struct {
	name       string
	fileOutDir string
	err        error
	skipped    bool
}

func processDirectory(cmd *cobra.Command, dir string) error {
	outDir := processOutputDir
	if outDir == "" {
		outDir = filepath.Join(dir, "media2rag-output")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read directory: %w", err)
	}

	type fileJob struct {
		path string
		name string
	}
	var jobs []fileJob
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		ext := filepath.Ext(name)
		if ext != ".md" && ext != ".markdown" {
			continue
		}
		if strings.HasPrefix(name, "._") {
			continue
		}
		jobs = append(jobs, fileJob{path: filepath.Join(dir, name), name: name})
	}

	total := len(jobs)
	fileConcurrency := effectiveFileConcurrency()
	numWorkers := fileConcurrency
	if numWorkers <= 0 || numWorkers > total {
		numWorkers = total
	}

	var progress *events.ProgressEmitter
	if !jsonOutput && total > 1 {
		conc := fileConcurrency
		if conc <= 0 {
			conc = numWorkers
		}
		progress = events.NewProgressEmitter(total, conc)
	}

	fmt.Fprintf(os.Stderr, "processing %d files", total)
	if numWorkers > 1 {
		fmt.Fprintf(os.Stderr, " (%d at a time)", numWorkers)
	}
	fmt.Fprintln(os.Stderr)

	jobsCh := make(chan fileJob, total)
	resultCh := make(chan fileResult, total)
	batchStart := time.Now()

	var wg sync.WaitGroup
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobsCh {
				baseName := job.name[:len(job.name)-len(filepath.Ext(job.name))]
				fileOutDir := filepath.Join(outDir, baseName)

				if shouldSkip(fileOutDir) {
					if progress != nil {
						progress.FileSkipped(job.name)
					}
					resultCh <- fileResult{name: job.name, skipped: true}
					continue
				}

				tee := createEmitter(job.name, fileOutDir, progress)
				err := runProcessFile(cmd, job.path, tee, fileOutDir)

				if progress != nil {
					if err != nil {
						progress.FileError(job.name, err)
					} else {
						progress.FileDone(job.name)
					}
				}

				resultCh <- fileResult{name: job.name, fileOutDir: fileOutDir, err: err}
			}
		}()
	}

	for _, j := range jobs {
		jobsCh <- j
	}
	close(jobsCh)

	go func() {
		wg.Wait()
		close(resultCh)
	}()

	if progress != nil {
		progress.Wait()
	}

	return collectResults(resultCh, batchStart)
}

func shouldSkip(fileOutDir string) bool {
	statusFile := filepath.Join(fileOutDir, "status.yaml")
	data, err := os.ReadFile(statusFile)
	if err != nil {
		return false
	}
	var st struct {
		Stage string `yaml:"stage"`
	}
	if yaml.Unmarshal(data, &st) != nil {
		return false
	}
	return st.Stage == "done" && !processForce
}

func createEmitter(name, fileOutDir string, progress *events.ProgressEmitter) events.EventEmitter {
	logPath := filepath.Join(fileOutDir, "process.log")

	if progress != nil {
		progress.FileStart(name)
		tee, _ := events.NewTeeEmitter(progress, logPath)
		return tee
	}

	var inner events.EventEmitter
	if jsonOutput {
		inner = events.NewStdoutEmitter()
	} else {
		inner = events.NewHumanEmitter()
	}
	tee, _ := events.NewTeeEmitter(inner, logPath)
	return tee
}

func collectResults(resultCh <-chan fileResult, batchStart time.Time) error {
	var processed, skipped, failed int
	var errs []error
	var totalCost float64
	var totalTokensIn, totalTokensOut int

	for r := range resultCh {
		switch {
		case r.skipped:
			skipped++
		case r.err != nil:
			failed++
			errs = append(errs, fmt.Errorf("%s: %w", r.name, r.err))
		default:
			processed++
			if r.fileOutDir != "" {
				if st := pipeline.LoadStatus(r.fileOutDir); st != nil && st.Stage == pipeline.StageDone {
					totalCost += st.TotalCost
					totalTokensIn += st.TotalTokensIn
					totalTokensOut += st.TotalTokensOut
				}
			}
		}
	}

	batchDur := time.Since(batchStart).Round(time.Second)

	fmt.Fprintf(os.Stderr, "\ndone: %d processed, %d skipped, %d failed\n", processed, skipped, failed)
	fmt.Fprintf(os.Stderr, "time: %s\n", batchDur)
	if totalCost > 0 {
		fmt.Fprintf(os.Stderr, "tokens: %d in / %d out\n", totalTokensIn, totalTokensOut)
		fmt.Fprintf(os.Stderr, "cost: $%.4f\n", totalCost)
	}

	if len(errs) > 0 {
		return fmt.Errorf("%d error(s)", len(errs))
	}
	return nil
}
