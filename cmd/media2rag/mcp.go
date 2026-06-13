package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"media2rag/internal/mcp"
)

var (
	mcpPort int
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Start MCP server (stdio transport)",
	Long: `Start the MCP server using stdio transport.
This is used for IDE and CLI integration with AI agents.

Example:
  media2rag mcp | your-mcp-client`,
	RunE: func(cmd *cobra.Command, args []string) error {
		server := mcp.NewServer()
		return server.ServeStdio()
	},
}

var mcpHTTPCmd = &cobra.Command{
	Use:   "mcp-serve",
	Short: "Start MCP server (HTTP transport)",
	Long: `Start the MCP server using HTTP transport.
This is used for network integration with AI agents.

Example:
  media2rag mcp-serve --port 8080`,
	RunE: func(cmd *cobra.Command, args []string) error {
		server := mcp.NewServer()
		fmt.Fprintf(os.Stderr, "MCP server starting on port %d...\n", mcpPort)
		return server.ServeHTTP(mcpPort)
	},
}

func init() {
	mcpHTTPCmd.Flags().IntVar(&mcpPort, "port", 8080, "HTTP port for MCP server")
	rootCmd.AddCommand(mcpCmd)
	rootCmd.AddCommand(mcpHTTPCmd)
}
