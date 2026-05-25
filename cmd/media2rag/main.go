package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"media2rag/internal/config"
	"media2rag/internal/llm"
)

var (
	cfgFile    string
	verbose    bool
	jsonOutput bool
	cfg        *config.Config
	llmClient  llm.LLMClient
)

var rootCmd = &cobra.Command{
	Use:   "media2rag",
	Short: "Convert any content into RAG-ready Markdown",
	Long: `media2rag converts URLs, Markdown files, and other content
into RAG-ready Markdown with structured metadata.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		var err error
		cfg, err = config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		if verbose {
			fmt.Fprintf(os.Stderr, "config loaded: backend=%s model=%s\n", cfg.LLM.DefaultBackend, cfg.LLM.Model)
		}

		llmClient, err = llm.NewClient(
			cmd.Context(),
			cfg.LLM.DefaultBackend,
			cfg.LLM.OllamaURL,
			cfg.LLM.Model,
			cfg.LLM.OpenRouterURL,
			cfg.LLM.OpenRouterKey,
		)
		if err != nil {
			return fmt.Errorf("init llm client: %w", err)
		}

		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file path")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
