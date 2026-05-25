package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"media2rag/internal/llm"
	"media2rag/internal/model"
	"media2rag/internal/rag"
	"media2rag/internal/service"
)

var askRag bool

var askCmd = &cobra.Command{
	Use:   "ask [question]",
	Short: "Ask a question via LLM or RAG",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		question := args[0]

		if !askRag {
			resp, err := llmClient.Chat(cmd.Context(), model.ChatRequest{
				Messages: []model.Message{
					{Role: "user", Content: question},
				},
			})
			if err != nil {
				return fmt.Errorf("chat failed: %w", err)
			}
			fmt.Fprintln(os.Stdout, resp.Message.Content)
			return nil
		}

		embedClient := llm.NewOllamaClient(cfg.LLM.OllamaURL, cfg.LLM.EmbedModel)
		qdrantSvc := service.NewQdrant(cfg.RAG.Qdrant)
		st, err := qdrantSvc.EnsureRunning(cmd.Context(), embedClient)
		if err != nil {
			return fmt.Errorf("qdrant: %w", err)
		}
		defer qdrantSvc.Stop(cmd.Context())

		engine := rag.NewEngine(rag.EngineConfig{
			Store:         st,
			LLM:           llmClient,
			EmbedClient:   embedClient,
			OllamaURL:     cfg.LLM.OllamaURL,
			EmbedModel:    cfg.LLM.EmbedModel,
			RerankModel:   cfg.RAG.RerankModel,
			RerankEnabled: cfg.RAG.Rerank,
		})

		resp, err := engine.Query(cmd.Context(), rag.RAGQuery{
			Query: question,
			TopK:  5,
		})
		if err != nil {
			return fmt.Errorf("rag query: %w", err)
		}

		fmt.Fprintln(os.Stdout, resp.Answer)
		if len(resp.Sources) > 0 {
			fmt.Fprintln(os.Stdout, "\nSources:")
			for _, s := range resp.Sources {
				fmt.Fprintf(os.Stdout, "  [%d] %s (%s)\n", s.Index, s.Title, s.Type)
			}
		}
		return nil
	},
}

func init() {
	askCmd.Flags().BoolVar(&jsonOutput, "json", false, "output JSON events")
	askCmd.Flags().BoolVarP(&askRag, "rag", "r", false, "use RAG (Qdrant) for answering")
	rootCmd.AddCommand(askCmd)
}
