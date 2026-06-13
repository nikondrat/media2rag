package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"media2rag/internal/openapi"
)

var openapiPort int

var openapiCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start HTTP server with OpenAPI endpoint for OpenWebUI",
	Long: `Start an HTTP server that exposes media2rag as OpenAPI tools.

This is the recommended way to connect media2rag to OpenWebUI.

Usage:
  1. Start the server:
     media2rag serve --port 8080

  2. In OpenWebUI, go to Workspace > Tools > Create Tool

  3. Set the OpenAPI URL to:
     http://your-server:8080/openapi.json

  4. The tool will be created with all media2rag endpoints.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		server, err := openapi.NewServer()
		if err != nil {
			return fmt.Errorf("init server: %w", err)
		}

		addr := fmt.Sprintf(":%d", openapiPort)
		fmt.Fprintf(os.Stderr, "media2rag server starting on %s\n", addr)
		fmt.Fprintf(os.Stderr, "OpenAPI spec: http://localhost:%d/openapi.json\n", openapiPort)
		fmt.Fprintf(os.Stderr, "\nConnect to OpenWebUI:\n")
		fmt.Fprintf(os.Stderr, "  1. Go to Workspace > Tools > Create Tool\n")
		fmt.Fprintf(os.Stderr, "  2. Set OpenAPI URL: http://your-server:%d/openapi.json\n", openapiPort)
		fmt.Fprintf(os.Stderr, "  3. Save and use the tool\n")

		return server.Start(addr)
	},
}

func init() {
	openapiCmd.Flags().IntVar(&openapiPort, "port", 8080, "HTTP port")
	rootCmd.AddCommand(openapiCmd)
}
