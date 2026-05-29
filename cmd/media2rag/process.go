package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"media2rag/internal/dashboard"
	"media2rag/internal/events"
	"media2rag/internal/llm"
	"media2rag/internal/model"
	"media2rag/internal/pipeline"
	"media2rag/internal/rag"
	"media2rag/internal/service"
	"media2rag/internal/workspace"
)

var (
	processBackend     string
	processModel       string
	processExtractOnly bool
)

var processCmd = &cobra.Command{
	Use:   "process [file|url]",
	Short: "Process a file or URL into RAG-ready Markdown",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		source := args[0]

		var emitter events.EventEmitter
		if jsonOutput {
			emitter = events.NewStdoutEmitter()
		} else {
			emitter = events.NewHumanEmitter()
		}

		emitter.Emit(model.Event{Type: "extracting", Data: map[string]string{"source": source}})

		extractor, err := extractorRegistry.Find(source)
		if err != nil {
			msg := unsupportedMsg(source)
			emitter.Emit(model.Event{Type: "error", Error: msg})
			emitter.Done()
			return fmt.Errorf("%s", msg)
		}

		markdown, err := extractor.Extract(cmd.Context(), source)
		if err != nil {
			emitter.Emit(model.Event{Type: "error", Error: err.Error()})
			emitter.Done()
			return fmt.Errorf("extract: %w", err)
		}

		wordCount := countWords(markdown)
		emitter.Emit(model.Event{Type: "extracted", Data: map[string]interface{}{
			"word_count": wordCount,
		}})

		if processExtractOnly {
			fmt.Println(markdown)
			return nil
		}

		workspaceDir := cfg.Workspace.DataDir
		if workspaceDir == "" {
			workspaceDir = filepath.Join(os.Getenv("HOME"), ".media2rag", "workspace")
		}

		ws, err := workspace.New(workspaceDir)
		if err != nil {
			emitter.Emit(model.Event{Type: "error", Error: fmt.Sprintf("workspace init: %v", err)})
			emitter.Done()
			return fmt.Errorf("workspace init: %w", err)
		}

		emitter.Emit(model.Event{Type: "saving", Data: map[string]string{"source": source}})

		sourceType := workspace.SourceType(source)
		wDoc, err := ws.CreateDocument(source, sourceType)
		if err != nil {
			emitter.Emit(model.Event{Type: "error", Error: fmt.Sprintf("create document: %v", err)})
			emitter.Done()
			return fmt.Errorf("create document: %w", err)
		}

		if err := ws.SaveSource(wDoc.Hash, markdown); err != nil {
			emitter.Emit(model.Event{Type: "error", Error: fmt.Sprintf("save source: %v", err)})
			emitter.Done()
			return fmt.Errorf("save source: %w", err)
		}

		client := llmClient
		if processModel != "" || processBackend != "" {
			backend := cfg.LLM.DefaultBackend
			modelName := cfg.LLM.Model
			if processBackend != "" {
				backend = processBackend
			}
			if processModel != "" {
				modelName = processModel
			}
			client, err = llm.NewClient(cmd.Context(), backend, cfg.LLM.OllamaURL, modelName, cfg.LLM.OpenRouterURL, cfg.LLM.OpenRouterKey)
			if err != nil {
				emitter.Emit(model.Event{Type: "error", Error: fmt.Sprintf("llm client: %v", err)})
				emitter.Done()
				return fmt.Errorf("llm client: %w", err)
			}
		}

		pcfg := pipeline.DefaultConfig()
		if cfg.Pipeline.ChunkSize > 0 {
			pcfg.ChunkSize = cfg.Pipeline.ChunkSize
		}
		if cfg.Pipeline.ChunkOverlap > 0 {
			pcfg.ChunkOverlap = cfg.Pipeline.ChunkOverlap
		}
		pcfg.MaxConcurrency = 3
		pcfg.LLMTimeout = time.Duration(cfg.LLM.Timeout) * time.Second
		pcfg.ExtractClaims = boolVal(cfg.Pipeline.ExtractClaims, pcfg.ExtractClaims)
		pcfg.ExtractMentalModels = boolVal(cfg.Pipeline.ExtractMentalModels, pcfg.ExtractMentalModels)
		pcfg.ExtractKeyTerms = boolVal(cfg.Pipeline.ExtractKeyTerms, pcfg.ExtractKeyTerms)
		pcfg.ExtractCoreThesis = boolVal(cfg.Pipeline.ExtractCoreThesis, pcfg.ExtractCoreThesis)
		pcfg.ExtractTakeaways = boolVal(cfg.Pipeline.ExtractTakeaways, pcfg.ExtractTakeaways)
		pcfg.HolisticAnalysis = boolVal(cfg.Pipeline.HolisticAnalysis, pcfg.HolisticAnalysis)
		pipe := pipeline.New(pcfg, client)
		pipe.SetCheckpointDir(filepath.Join(wDoc.RootPath, ".pipeline-cache"))

		dbPath := cfg.Dashboard.DBPath
		if dbPath == "" {
			dbPath = "~/.media2rag/dashboard.db"
		}
		if len(dbPath) > 0 && dbPath[0] == '~' {
			home, _ := os.UserHomeDir()
			dbPath = filepath.Join(home, dbPath[1:])
		}

		var store *dashboard.Store
		var tracer *dashboard.Tracer
		if dbPath != "" {
			os.MkdirAll(filepath.Dir(dbPath), 0755)
			store, err = dashboard.NewStore(dbPath)
			if err != nil {
				log.Printf("warning: dashboard store: %v (tracing disabled)", err)
			} else {
				defer store.Close()
				sse := dashboard.NewSSEBroadcaster()
				tracer = dashboard.NewTracer(store, sse)
			}
		}

		runID := dashboard.GenerateID()
		startTime := time.Now()

		if tracer != nil {
			tracer.SaveRunStart(runID, source, sourceType)
			tracer.BroadcastEvent("pipeline_start", map[string]interface{}{
				"run_id": runID, "source": source, "timestamp": startTime.Unix(),
			})
			pipe.SetRunID(runID)
			pipe.SetTracer(tracer)
		}

		ec := model.ExtractedContent{
			Content:   markdown,
			Source:    source,
			DocType:   sourceType,
			WordCount: wordCount,
		}
		ragDoc, err := pipe.Run(cmd.Context(), ec, emitter)
		if err != nil {
			if tracer != nil {
				tracer.BroadcastEvent("pipeline_complete", map[string]interface{}{
					"run_id": runID, "status": "failed", "error": err.Error(),
				})
			}
			emitter.Emit(model.Event{Type: "error", Error: fmt.Sprintf("pipeline: %v", err)})
			emitter.Done()
			return fmt.Errorf("pipeline: %w", err)
		}

		version, err := ws.SaveVersion(wDoc.Hash, ragDoc.Markdown)
		if err != nil {
			emitter.Emit(model.Event{Type: "error", Error: fmt.Sprintf("save version: %v", err)})
			emitter.Done()
			return fmt.Errorf("save version: %w", err)
		}

		versionDir := filepath.Join(wDoc.RootPath, "versions", fmt.Sprintf("v%d", version))
		copyPipelineArtifacts(wDoc.RootPath, versionDir)

		outputPath := filepath.Join(versionDir, "final.md")

	if tracer != nil {
		tokenEstimate := len(ragDoc.Markdown) / 4
		tracer.SaveRunComplete(runID, 0.85, tokenEstimate, int(time.Since(startTime).Milliseconds()), 0, "")
			tracer.BroadcastEvent("pipeline_complete", map[string]interface{}{
				"run_id": runID, "score": 0.85, "status": "completed",
			})
		}

		emitter.Emit(model.Event{Type: "completed", Data: map[string]interface{}{
			"hash":        wDoc.Hash,
			"source":      source,
			"output_path": outputPath,
			"word_count":  wordCount,
			"version":     version,
			"title":       ragDoc.Metadata.Title,
			"topics":      ragDoc.Metadata.Topics,
			"run_id":      runID,
		}})

		embedClient := llm.NewOllamaClient(cfg.LLM.OllamaURL, cfg.LLM.EmbedModel)
		qdrantSvc := service.NewQdrant(cfg.RAG.Qdrant)
		if st, err := qdrantSvc.EnsureRunning(cmd.Context(), embedClient); err == nil {
			indexer := rag.NewIndexer(st, embedClient)
			_ = indexer.IndexDocument(cmd.Context(), wDoc.Hash, ragDoc.Markdown)
			qdrantSvc.Stop(cmd.Context())
		}

		emitter.Done()
		return nil
	},
}

func init() {
	processCmd.Flags().BoolVar(&jsonOutput, "json", false, "output JSON events")
	processCmd.Flags().StringVar(&processBackend, "backend", "", "LLM backend (ollama, openrouter)")
	processCmd.Flags().StringVar(&processModel, "model", "", "LLM model name")
	processCmd.Flags().BoolVar(&processExtractOnly, "extract-only", false, "only extract content, skip workspace save")
	rootCmd.AddCommand(processCmd)
}

func copyPipelineArtifacts(docRoot string, versionDir string) {
	cacheDir := filepath.Join(docRoot, ".pipeline-cache")

	if src := filepath.Join(cacheDir, "compressed.md"); fileExists(src) {
		os.WriteFile(filepath.Join(versionDir, "compressed.md"), readFile(src), 0644)
	}

	if srcChunks := filepath.Join(cacheDir, "chunks"); dirExists(srcChunks) {
		dst := filepath.Join(versionDir, "chunks")
		os.MkdirAll(dst, 0755)
		entries, _ := os.ReadDir(srcChunks)
		for _, e := range entries {
			if !e.IsDir() {
				copyFile(filepath.Join(srcChunks, e.Name()), filepath.Join(dst, e.Name()))
			}
		}
	}

	if src := filepath.Join(cacheDir, "results.json"); fileExists(src) {
		os.WriteFile(filepath.Join(versionDir, "results.json"), readFile(src), 0644)
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func readFile(path string) []byte {
	data, _ := os.ReadFile(path)
	return data
}

func copyFile(src, dst string) {
	s, _ := os.Open(src)
	if s == nil {
		return
	}
	defer s.Close()
	d, _ := os.Create(dst)
	if d == nil {
		return
	}
	defer d.Close()
	io.Copy(d, s)
}

func unsupportedMsg(source string) string {
	ext := filepath.Ext(source)
	if ext != "" {
		return fmt.Sprintf("unsupported file format: %s (%s support coming in v2)", ext, ext[1:])
	}
	return fmt.Sprintf("unsupported source: %s", source)
}

func boolVal(ptr *bool, def bool) bool {
	if ptr != nil {
		return *ptr
	}
	return def
}

func countWords(s string) int {
	if s == "" {
		return 0
	}
	count := 1
	inWord := false
	for _, c := range s {
		if c == ' ' || c == '\n' || c == '\t' {
			inWord = false
		} else {
			if !inWord {
				count++
				inWord = true
			}
		}
	}
	return count - 1
}
