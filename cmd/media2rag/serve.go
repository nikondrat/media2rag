package main

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	dashEmbed "media2rag/dashboard"
	"media2rag/internal/api"
	"media2rag/internal/dashboard"
	"media2rag/internal/embedcheck"
	"media2rag/internal/judge"
	"media2rag/internal/llm"
)

var (
	serveHost string
	servePort int
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the HTTP API server",
	RunE: func(cmd *cobra.Command, args []string) error {
		host := serveHost
		port := servePort
		if cfg != nil {
			if host == "localhost" && servePort == 8542 {
				host = cfg.Server.Host
				port = cfg.Server.Port
			}
		}

		dashFS, err := fs.Sub(dashEmbed.FS, "dist")
		if err != nil {
			return err
		}

		dbPath := cfg.Dashboard.DBPath
		if dbPath == "" {
			dbPath = "~/.media2rag/dashboard.db"
		}
		if dbPath[0] == '~' {
			home, _ := os.UserHomeDir()
			dbPath = filepath.Join(home, dbPath[1:])
		}
		os.MkdirAll(filepath.Dir(dbPath), 0755)

		store, err := dashboard.NewStore(dbPath)
		if err != nil {
			return fmt.Errorf("dashboard store: %w", err)
		}
		log.Printf("dashboard store: %s", dbPath)

		sse := dashboard.NewSSEBroadcaster()
		tracer := dashboard.NewTracer(store, sse)

		var judgeRunner *judge.Runner
		if cfg.Judge.Enabled {
			judgeModel := cfg.Judge.Model
			if judgeModel == "" {
				judgeModel = cfg.LLMFallback.JudgeModel
			}

			var judgeClient llm.LLMClient
			if len(cfg.LLMFallback.JudgeChain) > 0 {
				judgeClient, err = llm.NewClientFromChain(cmd.Context(),
					cfg.LLMFallback.JudgeChain,
					cfg.LLM.DefaultBackend,
					cfg.LLM.OllamaURL,
					cfg.LLM.OpenRouterURL,
					cfg.LLM.OpenRouterKey,
				)
				if err != nil {
					log.Printf("judge chain init: %v, using default client", err)
					judgeClient = llmClient
				}
			} else {
				judgeClient = llmClient
			}

			judgeRunner = judge.NewRunner(judgeClient, judgeModel, tracer)
			log.Printf("judge enabled: model=%s", judgeModel)
		}

		var embedChecker *embedcheck.Runner
		if cfg.EmbedCheck.Enabled {
			ecfg := embedcheck.DefaultConfig()
			if cfg.EmbedCheck.SampleSize > 0 {
				ecfg.SampleSize = cfg.EmbedCheck.SampleSize
			}
			if cfg.EmbedCheck.SimilarityThreshold > 0 {
				ecfg.SimilarityThreshold = cfg.EmbedCheck.SimilarityThreshold
			}
			if cfg.EmbedCheck.RelevanceThreshold > 0 {
				ecfg.RelevanceThreshold = cfg.EmbedCheck.RelevanceThreshold
			}

			embedClient := llm.NewOllamaClient(cfg.LLM.OllamaURL, cfg.EmbedCheck.Model)
			embedChecker = embedcheck.NewRunner(embedClient, cfg.EmbedCheck.Model, tracer, ecfg)
			log.Printf("embed check enabled: model=%s", cfg.EmbedCheck.Model)
		}

		srv := api.New(api.Options{
			Config:            cfg,
			LLMClient:         llmClient,
			WorkspaceDir:      cfg.Workspace.DataDir,
			ExtractorRegistry: extractorRegistry,
			DashboardFS:       dashFS,
			Store:             store,
			Tracer:            tracer,
			SSE:               sse,
			JudgeRunner:       judgeRunner,
			EmbedChecker:      embedChecker,
		})

		return srv.Start(cmd.Context(), host, port)
	},
}

func init() {
	serveCmd.Flags().StringVar(&serveHost, "host", "localhost", "server host")
	serveCmd.Flags().IntVar(&servePort, "port", 8542, "server port")
	rootCmd.AddCommand(serveCmd)
}
