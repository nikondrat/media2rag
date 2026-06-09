package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"media2rag/internal/events"
	"media2rag/internal/extract"
	"media2rag/internal/llm"
	"media2rag/internal/model"
	"media2rag/internal/pipeline"
	"media2rag/internal/workspace"
)

var (
	processBackend     string
	processModel       string
	processExtractOnly bool
	processOutput      string
	processOutputDir   string
	processFinalDir    string
	processLogFile     string
	processForce       bool
)

var processCmd = &cobra.Command{
	Use:   "process [file|url|directory]",
	Short: "Process a file, URL, or directory into RAG-ready Markdown",
	Long: `Process content into RAG-ready Markdown with structured metadata.

Accepts a single file (.md), a URL, or a directory of .md files.
When a directory is given, all .md files (non-recursive) are processed.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		source := args[0]

		info, err := os.Stat(source)
		if err == nil && info.IsDir() {
			return processDirectory(cmd, source)
		}

		return processFile(cmd, source)
	},
}

func processDirectory(cmd *cobra.Command, dir string) error {
	outDir := processOutputDir
	if outDir == "" {
		outDir = filepath.Join(dir, "media2rag-output")
	}

	var errs []error
	var processed, skipped, failed int
	total := 0

	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read directory: %w", err)
	}

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
		total++
	}

	fmt.Fprintf(os.Stderr, "found %d files to process\n", total)

	progress := events.NewProgressEmitter(events.NewNoopEmitter(), total)
	logFileFlag := processLogFile

	fileIdx := 0
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

		baseName := name[:len(name)-len(ext)]
		fileOutDir := filepath.Join(outDir, baseName)

		statusFile := filepath.Join(fileOutDir, "status.yaml")
		shouldSkip := false
		if data, err := os.ReadFile(statusFile); err == nil {
			var st struct {
				Stage string `yaml:"stage"`
			}
			if yaml.Unmarshal(data, &st) == nil && st.Stage == "done" {
				shouldSkip = true
			}
		}

		if shouldSkip && !processForce {
			skipped++
			progress.SetFile(fileIdx, name)
			fileIdx++
			continue
		}

		path := filepath.Join(dir, name)
		logPath := filepath.Join(fileOutDir, "process.log")

		progress.SetFile(fileIdx, name)

		var emitter events.EventEmitter = progress
		if logFileFlag != "" {
			tee, teeErr := events.NewTeeEmitter(progress, logFileFlag)
			if teeErr == nil {
				emitter = tee
			}
		} else {
			tee, teeErr := events.NewTeeEmitter(progress, logPath)
			if teeErr == nil {
				emitter = tee
			}
		}

		processOutputDir = fileOutDir

		if err := processFileWithEmitter(cmd, path, emitter); err != nil {
			failed++
			errs = append(errs, fmt.Errorf("%s: %w", name, err))
			fmt.Fprintf(os.Stderr, "\nERROR: %v\n", err)
		} else {
			processed++
		}
		fileIdx++
	}

	fmt.Fprintf(os.Stderr, "\n\ndone: %d processed, %d skipped, %d failed\n", processed, skipped, failed)

	progress.Close()

	if len(errs) > 0 {
		return fmt.Errorf("%d error(s)", len(errs))
	}
	return nil
}

func truncStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

func processFile(cmd *cobra.Command, source string) (err error) {
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

	return runProcessFile(cmd, source, emitter)
}

func processFileWithEmitter(cmd *cobra.Command, source string, emitter events.EventEmitter) (err error) {
	return runProcessFile(cmd, source, emitter)
}

func runProcessFile(cmd *cobra.Command, source string, emitter events.EventEmitter) (err error) {
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

	docAuthor, docLang := "", ""
	if info, err := os.Stat(source); err == nil && !info.IsDir() {
		if raw, err := os.ReadFile(source); err == nil {
			fm := extract.ParseFrontmatter(string(raw))
			if a, ok := fm["author"]; ok {
				docAuthor = a
			}
			if l, ok := fm["language"]; ok {
				docLang = l
			} else if _, ok := fm["lang"]; ok {
				docLang = fm["lang"]
			}
		}
	}

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

	// --- LLM client ---
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
		client, err = llm.NewClient(cmd.Context(), backend, cfg.LLM.OllamaURL, modelName, cfg.LLM.OpenRouterURL, cfg.LLM.OpenRouterKey, cfg.LLM.LMStudioURL, time.Duration(cfg.LLM.Timeout)*time.Second)
		if err != nil {
			emitter.Emit(model.Event{Type: "error", Error: fmt.Sprintf("llm client: %v", err)})
			return fmt.Errorf("llm client: %w", err)
		}
	}

	// --- Pipeline config ---
	pcfg := pipeline.DefaultConfig()
	if cfg.Pipeline.ChunkSize > 0 {
		pcfg.ChunkSize = cfg.Pipeline.ChunkSize
	}
	if cfg.Pipeline.MaxConcurrency > 0 {
		pcfg.MaxConcurrency = cfg.Pipeline.MaxConcurrency
	} else {
		switch cfg.LLM.DefaultBackend {
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

	// --- Workspace ---
	workspaceDir := cfg.Workspace.DataDir
	if workspaceDir == "" {
		workspaceDir = filepath.Join(os.Getenv("HOME"), ".media2rag", "workspace")
	}
	ws, err := workspace.New(workspaceDir)
	if err != nil {
		emitter.Emit(model.Event{Type: "error", Error: fmt.Sprintf("workspace init: %v", err)})
		return fmt.Errorf("workspace init: %w", err)
	}

	sourceType := workspace.SourceType(source)
	wDoc, err := ws.CreateDocument(source, sourceType)
	if err != nil {
		emitter.Emit(model.Event{Type: "error", Error: fmt.Sprintf("create document: %v", err)})
		return fmt.Errorf("create document: %w", err)
	}

	if err := ws.SaveSource(wDoc.Hash, markdown); err != nil {
		emitter.Emit(model.Event{Type: "error", Error: fmt.Sprintf("save source: %v", err)})
		return fmt.Errorf("save source: %w", err)
	}

	pipe.SetCheckpointDir(filepath.Join(wDoc.RootPath, ".pipeline-cache"))
	if processOutputDir != "" {
		pipe.SetOutputDir(processOutputDir)
	}

	ec := model.ExtractedContent{
		Content:   markdown,
		Source:    source,
		DocType:   sourceType,
		Author:    docAuthor,
		Language:  docLang,
		WordCount: wordCount,
	}
	ragDoc, err := pipe.Run(cmd.Context(), ec, emitter)
	if err != nil {
		emitter.Emit(model.Event{Type: "error", Error: fmt.Sprintf("pipeline: %v", err)})
		return fmt.Errorf("pipeline: %w", err)
	}

	// Save version in workspace
	version, err := ws.SaveVersion(wDoc.Hash, ragDoc.Markdown)
	if err != nil {
		emitter.Emit(model.Event{Type: "error", Error: fmt.Sprintf("save version: %v", err)})
		return fmt.Errorf("save version: %w", err)
	}

	// Write final.md (pipeline already wrote intermediate files if outputDir set)
	if processOutputDir != "" {
		if err := exportToDir(processOutputDir, source, markdown, ragDoc, wordCount, version, emitter); err != nil {
			emitter.Emit(model.Event{Type: "error", Error: fmt.Sprintf("export: %v", err)})
			return err
		}
	}

	// Copy final to --final-dir (flat directory with title-based filenames)
	if processFinalDir != "" {
		title := ragDoc.Metadata.Title
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
		if err := os.WriteFile(finalPath, []byte(ragDoc.Markdown), 0644); err != nil {
			return fmt.Errorf("write final: %w", err)
		}
		emitter.Emit(model.Event{Type: "final_written", Data: map[string]string{"path": finalPath}})
	}

	// Determine output path
	outputPath := processOutput
	if outputPath == "" {
		if processOutputDir != "" {
			versionDir := filepath.Join(processOutputDir, fmt.Sprintf("v%d", version))
			outputPath = filepath.Join(versionDir, "final.md")
		} else {
			versionDir := filepath.Join(wDoc.RootPath, "versions", fmt.Sprintf("v%d", version))
			outputPath = filepath.Join(versionDir, "final.md")
		}
	}

	// Write to output path
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		emitter.Emit(model.Event{Type: "error", Error: fmt.Sprintf("create output dir: %v", err)})
		return fmt.Errorf("create output dir: %w", err)
	}
	if err := os.WriteFile(outputPath, []byte(ragDoc.Markdown), 0644); err != nil {
		emitter.Emit(model.Event{Type: "error", Error: fmt.Sprintf("write output: %v", err)})
		return fmt.Errorf("write output: %w", err)
	}

	if processOutputDir != "" {
		if err := exportToDir(processOutputDir, source, markdown, ragDoc, wordCount, version, emitter); err != nil {
			emitter.Emit(model.Event{Type: "error", Error: fmt.Sprintf("export: %v", err)})
			return err
		}
	}

	completedData := map[string]interface{}{
		"hash":        wDoc.Hash,
		"source":      source,
		"output_path": outputPath,
		"word_count":  wordCount,
		"version":     version,
		"title":       ragDoc.Metadata.Title,
		"topics":      ragDoc.Metadata.Topics,
	}
	if processOutputDir != "" {
		completedData["output_dir"] = processOutputDir
	}
	emitter.Emit(model.Event{Type: "completed", Data: completedData})

	return nil
}

func init() {
	processCmd.Flags().BoolVar(&jsonOutput, "json", false, "output JSON events")
	processCmd.Flags().StringVar(&processBackend, "backend", "", "LLM backend (ollama, openrouter, lmstudio)")
	processCmd.Flags().StringVar(&processModel, "model", "", "LLM model name")
	processCmd.Flags().BoolVar(&processExtractOnly, "extract-only", false, "only extract content, skip pipeline")
	processCmd.Flags().StringVarP(&processOutput, "output", "o", "", "output file path (default: workspace)")
	processCmd.Flags().StringVarP(&processOutputDir, "output-dir", "d", "", "processing directory for intermediate files (creates chunks/, intermediate/, results/, output/)")
	processCmd.Flags().StringVar(&processFinalDir, "final-dir", "", "directory to copy final .md files (flat, named by title)")
	processCmd.Flags().StringVar(&processLogFile, "log-file", "", "log file path (default: auto in output dir)")
	processCmd.Flags().BoolVar(&processForce, "force", false, "reprocess files even if output exists")
	rootCmd.AddCommand(processCmd)
}

func unsupportedMsg(source string) string {
	ext := filepath.Ext(source)
	if ext != "" {
		return fmt.Sprintf("unsupported file format: %s (%s support coming in v2)", ext, ext[1:])
	}
	return fmt.Sprintf("unsupported source: %s", source)
}

func exportToDir(dir, source, rawMD string, doc *model.RAGDocument, wordCount, version int, emitter events.EventEmitter) error {
	emitter.Emit(model.Event{Type: "export_start", Data: map[string]string{"dir": dir}})

	title := doc.Metadata.Title
	if title == "" {
		title = "unnamed_document"
	}
	sanitized := pipeline.SanitizeFilename(title)
	if sanitized == "" {
		sanitized = "unnamed_document"
	}

	if err := os.MkdirAll(filepath.Join(dir, "chunks"), 0755); err != nil {
		return fmt.Errorf("create chunks dir: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "intermediate"), 0755); err != nil {
		return fmt.Errorf("create intermediate dir: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "output"), 0755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	// Write raw extracted content
	if err := os.WriteFile(filepath.Join(dir, "intermediate", "raw.md"), []byte(rawMD), 0644); err != nil {
		return fmt.Errorf("write raw.md: %w", err)
	}

	// Write cleaned content (post pre-clean, before chunking)
	if doc.CleanedText != "" {
		if err := os.WriteFile(filepath.Join(dir, "intermediate", "cleaned.md"), []byte(doc.CleanedText), 0644); err != nil {
			return fmt.Errorf("write cleaned.md: %w", err)
		}
	}

	// Write individual chunk files
	sort.Slice(doc.Chunks, func(i, j int) bool {
		return doc.Chunks[i].Index < doc.Chunks[j].Index
	})
	for _, ch := range doc.Chunks {
		if ch.Type == "" && ch.Summary == "" {
			continue
		}
		var cb strings.Builder
		fmt.Fprintf(&cb, "## chunk_%02d\n", ch.Index+1)
		writeChunkField(&cb, "type", ch.Type)
		writeChunkField(&cb, "topic", ch.Topic)
		writeChunkField(&cb, "summary", ch.Summary)
		if len(ch.KeyPoints) > 0 {
			cb.WriteString("key_points:\n")
			for _, kp := range ch.KeyPoints {
				kp = strings.TrimSpace(kp)
				if kp != "" {
					cb.WriteString("- ")
					cb.WriteString(kp)
					cb.WriteString("\n")
				}
			}
		}
		writeChunkField(&cb, "source_quote", ch.SourceQuote)
		writeChunkField(&cb, "my_takeaway", ch.MyTakeaway)
		writeChunkField(&cb, "confidence", pipeline.ConfidenceToString(ch.Confidence))
		writeChunkField(&cb, "applicability", ch.Applicability)
		if len(ch.Steps) > 0 {
			cb.WriteString("steps:\n")
			for _, s := range ch.Steps {
				s = strings.TrimSpace(s)
				if s != "" {
					cb.WriteString("- ")
					cb.WriteString(s)
					cb.WriteString("\n")
				}
			}
		}
		if ch.Content != "" {
			cb.WriteString("\n")
			cb.WriteString(ch.Content)
			cb.WriteString("\n")
		}
		chunkPath := filepath.Join(dir, "chunks", fmt.Sprintf("chunk_%03d.md", ch.Index+1))
		if err := os.WriteFile(chunkPath, []byte(cb.String()), 0644); err != nil {
			return fmt.Errorf("write %s: %w", chunkPath, err)
		}
	}

	// Write output/final.md (stable filename, like main branch)
	finalPath := filepath.Join(dir, "output", "final.md")
	if err := os.WriteFile(finalPath, []byte(doc.Markdown), 0644); err != nil {
		return fmt.Errorf("write output/final.md: %w", err)
	}

	// Write <sanitized-title>.md at root of output-dir (copy named after title)
	titlePath := filepath.Join(dir, sanitized+".md")
	if err := os.WriteFile(titlePath, []byte(doc.Markdown), 0644); err != nil {
		return fmt.Errorf("write %s: %w", titlePath, err)
	}

	// Write .media2rag.yaml metadata
	meta := map[string]interface{}{
		"source":       source,
		"title":        doc.Metadata.Title,
		"word_count":   wordCount,
		"version":      version,
		"topics":       doc.Metadata.Topics,
		"language":     doc.Metadata.Language,
		"author":       doc.Metadata.Author,
		"core_thesis":  doc.Metadata.CoreThesis,
		"domains":      doc.Metadata.Domains,
		"chunks_total": len(doc.Chunks),
		"status":       "completed",
	}
	metaYAML, err := yaml.Marshal(meta)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".media2rag.yaml"), metaYAML, 0644); err != nil {
		return fmt.Errorf("write .media2rag.yaml: %w", err)
	}

	emitter.Emit(model.Event{Type: "export_complete", Data: map[string]interface{}{
		"final_path": finalPath,
		"title_path": titlePath,
		"chunks":     len(doc.Chunks),
	}})

	return nil
}

func writeChunkField(b *strings.Builder, key, value string) {
	if strings.TrimSpace(value) == "" {
		return
	}
	b.WriteString(key)
	b.WriteString(": ")
	b.WriteString(value)
	b.WriteString("\n")
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
