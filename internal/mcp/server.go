package mcp

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type Server struct {
	mcpServer *server.MCPServer
}

func NewServer() *Server {
	s := &Server{}
	s.mcpServer = server.NewMCPServer(
		"media2rag",
		"1.0.0",
		server.WithToolCapabilities(true),
	)
	s.registerTools()
	return s
}

func (s *Server) registerTools() {
	s.mcpServer.AddTool(
		mcp.NewTool("process",
			mcp.WithDescription("Process content (URL, file, or directory) through the CTG pipeline to create RAG-ready Markdown"),
			mcp.WithString("source", mcp.Description("URL or file path to process"), mcp.Required()),
			mcp.WithString("output_dir", mcp.Description("Output directory")),
			mcp.WithBoolean("force", mcp.Description("Force reprocessing")),
		),
		s.handleProcess,
	)

	s.mcpServer.AddTool(
		mcp.NewTool("rag",
			mcp.WithDescription("Search indexed document chunks using hybrid vector search"),
			mcp.WithString("query", mcp.Description("Search query"), mcp.Required()),
			mcp.WithNumber("top", mcp.Description("Number of results to return")),
			mcp.WithString("format", mcp.Description("Output format (text, json)")),
		),
		s.handleRAG,
	)

	s.mcpServer.AddTool(
		mcp.NewTool("index",
			mcp.WithDescription("Index processed document chunks into Qdrant vector database"),
			mcp.WithString("directory", mcp.Description("Directory containing processed chunks"), mcp.Required()),
		),
		s.handleIndex,
	)

	s.mcpServer.AddTool(
		mcp.NewTool("documents",
			mcp.WithDescription("Manage processed documents (list, show, delete)"),
			mcp.WithString("action", mcp.Description("Action: list, show, delete"), mcp.Required()),
			mcp.WithString("id", mcp.Description("Document ID for show/delete")),
		),
		s.handleDocuments,
	)

	s.mcpServer.AddTool(
		mcp.NewTool("health",
			mcp.WithDescription("Check LLM backend health and availability"),
		),
		s.handleHealth,
	)

	s.mcpServer.AddTool(
		mcp.NewTool("preprocess",
			mcp.WithDescription("Optimize a query for RAG search using LLM"),
			mcp.WithString("query", mcp.Description("Query to optimize"), mcp.Required()),
			mcp.WithNumber("variants", mcp.Description("Number of query variants")),
		),
		s.handlePreprocess,
	)
}

func (s *Server) handleProcess(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	source := req.GetString("source", "")
	outputDir := req.GetString("output_dir", "")
	force := req.GetBool("force", false)

	args := []string{"process", source}
	if outputDir != "" {
		args = append(args, "-o", outputDir)
	}
	if force {
		args = append(args, "--force")
	}

	return runCommand(ctx, args)
}

func (s *Server) handleRAG(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	query := req.GetString("query", "")
	top := req.GetFloat("top", 5)
	format := req.GetString("format", "text")

	args := []string{"rag", query}
	if top > 0 {
		args = append(args, "--top", fmt.Sprintf("%.0f", top))
	}
	if format != "" {
		args = append(args, "--format", format)
	}

	return runCommand(ctx, args)
}

func (s *Server) handleIndex(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	directory := req.GetString("directory", "")
	args := []string{"index", directory}
	return runCommand(ctx, args)
}

func (s *Server) handleDocuments(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	action := req.GetString("action", "")
	id := req.GetString("id", "")

	args := []string{"documents", action}
	if id != "" {
		args = append(args, id)
	}

	return runCommand(ctx, args)
}

func (s *Server) handleHealth(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return runCommand(ctx, []string{"health"})
}

func (s *Server) handlePreprocess(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	query := req.GetString("query", "")
	variants := req.GetFloat("variants", 1)

	args := []string{"preprocess", query}
	if variants > 1 {
		args = append(args, "--variants", fmt.Sprintf("%.0f", variants))
	}

	return runCommand(ctx, args)
}

func runCommand(ctx context.Context, args []string) (*mcp.CallToolResult, error) {
	binary, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("get executable: %w", err)
	}

	cmd := exec.CommandContext(ctx, binary, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("command failed: %s\n%s", err, string(output))), nil
	}

	return mcp.NewToolResultText(string(output)), nil
}

func (s *Server) ServeStdio() error {
	return server.ServeStdio(s.mcpServer)
}

func (s *Server) ServeHTTP(port int) error {
	httpServer := server.NewStreamableHTTPServer(s.mcpServer)
	addr := fmt.Sprintf(":%d", port)
	return httpServer.Start(addr)
}
