package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"media2rag/internal/events"
	"media2rag/internal/llm"
	"media2rag/internal/model"
	"media2rag/internal/pipeline"
	"media2rag/internal/workspace"
)

var (
	processBackend          string
	processModel            string
	processExtractOnly      bool
	processOutput           string
	processOutputDir        string
	processFinalDir         string
	processLogFile          string
	processForce            bool
	processDryRun           bool
	processFileConcurrency  int
	processTotalConcurrency int
)

func effectiveFileConcurrency() int {
	if processFileConcurrency > 0 {
		return processFileConcurrency
	}
	if fc := cfg.Pipeline.MaxFileConcurrency; fc > 0 {
		return fc
	}
	backend := cfg.LLM.DefaultBackend
	if processBackend != "" {
		backend = processBackend
	}
	switch backend {
	case "openrouter":
		return 0
	case "lmstudio":
		return 4
	default:
		return 1
	}
}

func effectiveTotalConcurrency() int {
	if processTotalConcurrency > 0 {
		return processTotalConcurrency
	}
	if tc := cfg.Pipeline.MaxTotalConcurrency; tc > 0 {
		return tc
	}
	backend := cfg.LLM.DefaultBackend
	if processBackend != "" {
		backend = processBackend
	}
	switch backend {
	case "openrouter":
		return 100
	case "lmstudio":
		return 16
	default:
		return 8
	}
}

var processCmd = &cobra.Command{
	Use:   "process [file|url|directory]",
	Short: "Process a file, URL, or directory into RAG-ready Markdown",
	Long: `Process content into RAG-ready Markdown with structured metadata.

Accepts a single file (.md), a URL, or a directory of .md files.
When a directory is given, all .md files (non-recursive) are processed.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		source := args[0]

		if processDryRun {
			info, err := os.Stat(source)
			if err == nil && info.IsDir() {
				return dryRunDirectory(source)
			}
			fmt.Fprintf(os.Stderr, "would process: %s\n", source)
			return nil
		}

		info, err := os.Stat(source)
		if err == nil && info.IsDir() {
			return processDirectory(cmd, source)
		}

		return processFile(cmd, source)
	},
}

func processFile(cmd *cobra.Command, source string) error {
	var emitter events.EventEmitter
	if jsonOutput {
		emitter = events.NewStdoutEmitter()
	} else {
		emitter = events.NewHumanEmitter()
	}

	if processLogFile != "" {
		tee, teeErr := events.NewTeeEmitter(emitter, processLogFile)
		if teeErr == nil {
			emitter = tee
		}
	}

	outputDir := processOutputDir
	if outputDir != "" {
		stem := filepath.Base(source)
		if ext := filepath.Ext(stem); ext != "" {
			stem = stem[:len(stem)-len(ext)]
		}
		outputDir = filepath.Join(outputDir, stem)
	}

	return runProcessFile(cmd, source, emitter, outputDir)
}

func runProcessFile(cmd *cobra.Command, source string, emitter events.EventEmitter, outputDir string) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("PANIC: %v", r)
			emitter.Emit(model.Event{Type: "error", Error: err.Error()})
		}
		emitter.Done()
	}()

	emitter.Emit(model.Event{Type: "extracting", Data: map[string]string{"source": source}})

	extractor, err := extractorRegistry.Find(source)
	if err != nil {
		msg := unsupportedMsg(source)
		emitter.Emit(model.Event{Type: "error", Error: msg})
		return fmt.Errorf("%s", msg)
	}

	markdown, err := extractor.Extract(cmd.Context(), source)
	if err != nil {
		emitter.Emit(model.Event{Type: "error", Error: err.Error()})
		return fmt.Errorf("extract: %w", err)
	}

	wordCount := countWords(markdown)
	docAuthor, docLang := parseFileMetadata(source)

	emitter.Emit(model.Event{Type: "extracted", Data: map[string]interface{}{
		"word_count": wordCount,
	}})

	if processExtractOnly {
		emitter.Emit(model.Event{Type: "completed", Data: map[string]interface{}{
			"source":     source,
			"word_count": wordCount,
		}})
		fmt.Println(markdown)
		return nil
	}

	client, err := setupLLMClient(cmd, emitter)
	if err != nil {
		return err
	}

	pipe, jsonlRec := setupPipeline(client, outputDir)
	if jsonlRec != nil {
		defer jsonlRec.Close()
	}

	wDoc, ws, err := setupWorkspace(source, emitter)
	if err != nil {
		return err
	}

	pipe.SetCheckpointDir(filepath.Join(wDoc.RootPath, ".pipeline-cache"))
	if outputDir != "" {
		pipe.SetOutputDir(outputDir)
	}

	ec := model.ExtractedContent{
		Content:   markdown,
		Source:    source,
		DocType:   workspace.SourceType(source),
		Author:    docAuthor,
		Language:  docLang,
		WordCount: wordCount,
	}

	ragDoc, err := pipe.Run(cmd.Context(), ec, emitter)
	if err != nil {
		emitter.Emit(model.Event{Type: "error", Error: fmt.Sprintf("pipeline: %v", err)})
		return fmt.Errorf("pipeline: %w", err)
	}

	if outputDir != "" && ragDoc.Metadata.Title != "" {
		titleDir := filepath.Join(filepath.Dir(outputDir), pipeline.SanitizeFilename(ragDoc.Metadata.Title))
		if titleDir != outputDir {
			if err := os.Rename(outputDir, titleDir); err == nil {
				outputDir = titleDir
			}
		}
	}

	version, err := ws.SaveVersion(wDoc.Hash, ragDoc.Markdown)
	if err != nil {
		emitter.Emit(model.Event{Type: "error", Error: fmt.Sprintf("save version: %v", err)})
		return fmt.Errorf("save version: %w", err)
	}

	if outputDir != "" {
		if err := exportToDir(outputDir, source, markdown, ragDoc, wordCount, version, emitter); err != nil {
			emitter.Emit(model.Event{Type: "error", Error: fmt.Sprintf("export: %v", err)})
			return err
		}
	}

	if processFinalDir != "" {
		if err := copyToFinalDir(ragDoc, emitter); err != nil {
			return err
		}
	}

	outputPath := determineOutputPath(outputDir, wDoc.RootPath, version)
	if err := writeOutput(outputPath, ragDoc.Markdown, emitter); err != nil {
		return err
	}

	emitter.Emit(model.Event{Type: "completed", Data: map[string]interface{}{
		"hash":        wDoc.Hash,
		"source":      source,
		"output_path": outputPath,
		"word_count":  wordCount,
		"version":     version,
		"title":       ragDoc.Metadata.Title,
		"topics":      ragDoc.Metadata.Topics,
	}})

	return nil
}

func setupLLMClient(cmd *cobra.Command, emitter events.EventEmitter) (llm.LLMClient, error) {
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
		var err error
		client, err = llm.NewClient(cmd.Context(), backend, cfg.LLM.OllamaURL, modelName, cfg.LLM.OpenRouterURL, cfg.LLM.OpenRouterKey, cfg.LLM.LMStudioURL, time.Duration(cfg.LLM.Timeout)*time.Second)
		if err != nil {
			emitter.Emit(model.Event{Type: "error", Error: fmt.Sprintf("llm client: %v", err)})
			return nil, fmt.Errorf("llm client: %w", err)
		}
	}

	totalConc := effectiveTotalConcurrency()
	client = llm.NewRateLimitedClient(client, totalConc)

	if err := llm.LoadPricing(); err != nil {
		emitter.Emit(model.Event{Type: "error", Error: fmt.Sprintf("load pricing: %v", err)})
	}

	return client, nil
}

func setupPipeline(client llm.LLMClient, outputDir string) (*pipeline.Pipeline, *pipeline.JSONLRecorder) {
	pcfg := pipeline.DefaultConfig()
	if cfg.Pipeline.ChunkSize > 0 {
		pcfg.ChunkSize = cfg.Pipeline.ChunkSize
	}
	if cfg.Pipeline.MaxConcurrency > 0 {
		pcfg.MaxConcurrency = cfg.Pipeline.MaxConcurrency
	} else {
		backend := cfg.LLM.DefaultBackend
		if processBackend != "" {
			backend = processBackend
		}
		switch backend {
		case "lmstudio":
			pcfg.MaxConcurrency = 4
		case "openrouter":
			pcfg.MaxConcurrency = 10
		}
	}
	pcfg.LLMTimeout = time.Duration(cfg.LLM.Timeout) * time.Second
	pcfg.HolisticAnalysis = true
	if cfg.Pipeline.HolisticAnalysis != nil {
		pcfg.HolisticAnalysis = *cfg.Pipeline.HolisticAnalysis
	}

	pipe := pipeline.New(pcfg, client)

	var jsonlRec *pipeline.JSONLRecorder
	if outputDir != "" {
		var err error
		jsonlRec, err = pipeline.NewJSONLRecorder(outputDir)
		if err == nil {
			pipe.SetRecorder(jsonlRec)
		}
	}

	return pipe, jsonlRec
}

func setupWorkspace(source string, emitter events.EventEmitter) (*workspace.Document, *workspace.Workspace, error) {
	workspaceDir := cfg.Workspace.DataDir
	if workspaceDir == "" {
		workspaceDir = filepath.Join(os.Getenv("HOME"), ".media2rag", "workspace")
	}

	ws, err := workspace.New(workspaceDir)
	if err != nil {
		emitter.Emit(model.Event{Type: "error", Error: fmt.Sprintf("workspace init: %v", err)})
		return nil, nil, fmt.Errorf("workspace init: %w", err)
	}

	sourceType := workspace.SourceType(source)
	wDoc, err := ws.CreateDocument(source, sourceType)
	if err != nil {
		emitter.Emit(model.Event{Type: "error", Error: fmt.Sprintf("create document: %v", err)})
		return nil, nil, fmt.Errorf("create document: %w", err)
	}

	return wDoc, ws, nil
}

func copyToFinalDir(doc *model.RAGDocument, emitter events.EventEmitter) error {
	title := doc.Metadata.Title
	if title == "" {
		title = "unnamed_document"
	}
	sanitized := pipeline.SanitizeFilename(title)
	if sanitized == "" {
		sanitized = "unnamed_document"
	}

	if err := os.MkdirAll(processFinalDir, 0755); err != nil {
		return fmt.Errorf("create final dir: %w", err)
	}

	finalPath := filepath.Join(processFinalDir, sanitized+".md")
	if err := os.WriteFile(finalPath, []byte(doc.Markdown), 0644); err != nil {
		return fmt.Errorf("write final: %w", err)
	}

	emitter.Emit(model.Event{Type: "final_written", Data: map[string]string{"path": finalPath}})
	return nil
}

func determineOutputPath(outputDir, workspaceRoot string, version int) string {
	if processOutput != "" {
		return processOutput
	}

	if outputDir != "" {
		return filepath.Join(outputDir, fmt.Sprintf("v%d", version), "final.md")
	}

	return filepath.Join(workspaceRoot, "versions", fmt.Sprintf("v%d", version), "final.md")
}

func writeOutput(outputPath, content string, emitter events.EventEmitter) error {
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		emitter.Emit(model.Event{Type: "error", Error: fmt.Sprintf("create output dir: %v", err)})
		return fmt.Errorf("create output dir: %w", err)
	}

	if err := os.WriteFile(outputPath, []byte(content), 0644); err != nil {
		emitter.Emit(model.Event{Type: "error", Error: fmt.Sprintf("write output: %v", err)})
		return fmt.Errorf("write output: %w", err)
	}

	return nil
}

func init() {
	processCmd.Flags().BoolVar(&jsonOutput, "json", false, "output JSON events")
	processCmd.Flags().StringVar(&processBackend, "backend", "", "LLM backend (ollama, openrouter, lmstudio)")
	processCmd.Flags().StringVar(&processModel, "model", "", "LLM model name")
	processCmd.Flags().BoolVar(&processExtractOnly, "extract-only", false, "only extract content, skip pipeline")
	processCmd.Flags().StringVarP(&processOutput, "output", "o", "", "output file path (default: workspace)")
	processCmd.Flags().StringVarP(&processOutputDir, "output-dir", "d", "", "processing directory for intermediate files")
	processCmd.Flags().StringVar(&processFinalDir, "final-dir", "", "directory to copy final .md files")
	processCmd.Flags().StringVar(&processLogFile, "log-file", "", "log file path")
	processCmd.Flags().BoolVar(&processForce, "force", false, "reprocess files even if output exists")
	processCmd.Flags().BoolVar(&processDryRun, "dry-run", false, "list files that would be processed")
	processCmd.Flags().IntVar(&processFileConcurrency, "file-concurrency", 0, "max files to process in parallel")
	processCmd.Flags().IntVar(&processTotalConcurrency, "total-concurrency", 0, "max concurrent LLM requests")
	rootCmd.AddCommand(processCmd)
}
