package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"media2rag/internal/model"
)

var (
	preprocessVariants  int
	preprocessContextFile string
	preprocessFormat    string
)

var preprocessCmd = &cobra.Command{
	Use:   "preprocess <query>",
	Short: "Optimize a query for RAG search",
	Long: `Optimize a user query using LLM to improve retrieval from a knowledge base.

The preprocessor rewrites queries to include relevant keywords, synonyms,
and context that help find relevant documents in vector search.

Examples:
  media2rag preprocess "what is RAG?"
  media2rag preprocess "how to scale business" --variants 3
  media2rag preprocess "project status" --context-file context.md`,
	Args: cobra.ExactArgs(1),
	RunE: runPreprocess,
}

const preprocessSystemPrompt = `You are a search query optimizer. Given a user question, rewrite it to improve retrieval from a knowledge base.

Rules:
1. Expand abbreviations and technical terms
2. Add relevant keywords and synonyms
3. Preserve the original intent
4. Write in the same language as the query
5. Return ONLY the optimized query, no explanation`

const preprocessVariantsPrompt = `You are a search query optimizer. Given a user question, generate multiple search queries to improve retrieval from a knowledge base.

Generate %d different queries that:
1. Cover different aspects of the original question
2. Use different phrasings and synonyms
3. Include relevant technical terms
4. Preserve the original intent
5. Write in the same language as the query

Return each query on a new line, numbered from 1.`

func runPreprocess(cmd *cobra.Command, args []string) error {
	query := args[0]

	ctx := cmd.Context()

	var contextContent string
	if preprocessContextFile != "" {
		data, err := os.ReadFile(preprocessContextFile)
		if err != nil {
			return fmt.Errorf("read context file: %w", err)
		}
		contextContent = string(data)
	}

	if preprocessVariants > 1 {
		return runPreprocessVariants(ctx, query, contextContent)
	}

	return runPreprocessSingle(ctx, query, contextContent)
}

func runPreprocessSingle(ctx context.Context, query, context string) error {
	prompt := preprocessSystemPrompt
	if context != "" {
		prompt += "\n\nContext:\n" + context
	}

	resp, err := llmClient.Chat(ctx, model.ChatRequest{
		Messages: []model.Message{
			{Role: "system", Content: prompt},
			{Role: "user", Content: query},
		},
	})
	if err != nil {
		return fmt.Errorf("preprocess: %w", err)
	}

	optimized := strings.TrimSpace(resp.Message.Content)

	switch preprocessFormat {
	case "md":
		fmt.Printf("## Optimized Query\n%s\n", optimized)
	default:
		fmt.Println(optimized)
	}

	return nil
}

func runPreprocessVariants(ctx context.Context, query, context string) error {
	prompt := fmt.Sprintf(preprocessVariantsPrompt, preprocessVariants)
	if context != "" {
		prompt += "\n\nContext:\n" + context
	}

	resp, err := llmClient.Chat(ctx, model.ChatRequest{
		Messages: []model.Message{
			{Role: "system", Content: prompt},
			{Role: "user", Content: query},
		},
	})
	if err != nil {
		return fmt.Errorf("preprocess: %w", err)
	}

	variants := parseVariants(resp.Message.Content)

	switch preprocessFormat {
	case "md":
		fmt.Println("## Optimized Queries")
		for i, v := range variants {
			fmt.Printf("%d. %s\n", i+1, v)
		}
	default:
		for _, v := range variants {
			fmt.Println(v)
		}
	}

	return nil
}

func parseVariants(content string) []string {
	var variants []string
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if len(line) > 2 && line[0] >= '0' && line[0] <= '9' {
			if idx := strings.Index(line, "."); idx > 0 && idx < 5 {
				line = strings.TrimSpace(line[idx+1:])
			}
		}

		if line != "" {
			variants = append(variants, line)
		}
	}
	return variants
}

func init() {
	preprocessCmd.Flags().IntVar(&preprocessVariants, "variants", 1, "Number of query variants to generate")
	preprocessCmd.Flags().StringVar(&preprocessContextFile, "context-file", "", "File with context to improve query optimization")
	preprocessCmd.Flags().StringVar(&preprocessFormat, "format", "text", "Output format (text, md)")
	rootCmd.AddCommand(preprocessCmd)
}
