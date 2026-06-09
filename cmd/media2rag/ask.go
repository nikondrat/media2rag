package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"media2rag/internal/model"
)

var askCmd = &cobra.Command{
	Use:   "ask [question]",
	Short: "Ask a question via LLM",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		question := args[0]

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
	},
}

func init() {
	rootCmd.AddCommand(askCmd)
}
