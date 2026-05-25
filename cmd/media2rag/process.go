package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"media2rag/internal/events"
	"media2rag/internal/model"
)

var processCmd = &cobra.Command{
	Use:   "process [file|url]",
	Short: "Process a file or URL into RAG-ready Markdown",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var emitter events.EventEmitter
		if jsonOutput {
			emitter = events.NewStdoutEmitter()
		} else {
			emitter = events.NewNoopEmitter()
		}

		emitter.Emit(model.Event{Type: "starting", Data: map[string]string{"source": args[0]}})
		fmt.Fprintf(cmd.OutOrStdout(), "processing: %s\n", args[0])

		emitter.Emit(model.Event{Type: "completed", Data: map[string]string{"source": args[0]}})
		emitter.Done()

		return nil
	},
}

func init() {
	processCmd.Flags().BoolVar(&jsonOutput, "json", false, "output JSON events")
	rootCmd.AddCommand(processCmd)
}
