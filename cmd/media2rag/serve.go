package main

import (
	"github.com/spf13/cobra"

	"media2rag/internal/api"
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

		srv := api.New(api.Options{
			Config:            cfg,
			LLMClient:         llmClient,
			WorkspaceDir:      cfg.Workspace.DataDir,
			ExtractorRegistry: extractorRegistry,
		})

		return srv.Start(cmd.Context(), host, port)
	},
}

func init() {
	serveCmd.Flags().StringVar(&serveHost, "host", "localhost", "server host")
	serveCmd.Flags().IntVar(&servePort, "port", 8542, "server port")
	rootCmd.AddCommand(serveCmd)
}
