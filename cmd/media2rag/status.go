package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"media2rag/internal/store"
	"media2rag/internal/workspace"
)

type serviceStatus struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Details string `json:"details,omitempty"`
}

type statusReport struct {
	Services   []serviceStatus `json:"services"`
	Documents  int             `json:"documents"`
	Workspace  string          `json:"workspace"`
	AllHealthy bool            `json:"all_healthy"`
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check service health and workspace status",
	RunE: func(cmd *cobra.Command, args []string) error {
		services := []serviceStatus{
			checkOllama(),
			checkRdrr(),
			checkQdrant(),
			checkWorkspace(),
		}

		allHealthy := true
		for _, s := range services {
			if s.Status != "ok" {
				allHealthy = false
			}
		}

		ws, _ := openWorkspace()
		docCount, _ := ws.DocumentCount()

		workspaceDir := cfg.Workspace.DataDir
		if workspaceDir == "" {
			workspaceDir = filepath.Join(os.Getenv("HOME"), ".media2rag", "workspace")
		}

		if jsonOutput {
			report := statusReport{
				Services:   services,
				Documents:  docCount,
				Workspace:  workspaceDir,
				AllHealthy: allHealthy,
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(report)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Workspace: %s\n", workspaceDir)
		fmt.Fprintf(cmd.OutOrStdout(), "Documents: %d\n\n", docCount)
		fmt.Fprintln(cmd.OutOrStdout(), "Services:")
		for _, s := range services {
			indicator := "✓"
			if s.Status != "ok" {
				indicator = "✗"
			}
			details := ""
			if s.Details != "" {
				details = " (" + s.Details + ")"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "  %s %s%s\n", indicator, s.Name, details)
		}

		return nil
	},
}

func init() {
	statusCmd.Flags().BoolVar(&jsonOutput, "json", false, "output JSON")
	rootCmd.AddCommand(statusCmd)
}

func checkOllama() serviceStatus {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://localhost:11434/api/tags")
	if err != nil {
		return serviceStatus{Name: "Ollama", Status: "error", Details: "not connected (localhost:11434)"}
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return serviceStatus{Name: "Ollama", Status: "error", Details: fmt.Sprintf("HTTP %d", resp.StatusCode)}
	}
	return serviceStatus{Name: "Ollama", Status: "ok", Details: "connected"}
}

func checkRdrr() serviceStatus {
	cmd := exec.Command("npx", "rdrr", "--version")
	if err := cmd.Run(); err != nil {
		return serviceStatus{Name: "rdrr", Status: "error", Details: "not found"}
	}
	return serviceStatus{Name: "rdrr", Status: "ok", Details: "available"}
}

func checkWorkspace() serviceStatus {
	workspaceDir := cfg.Workspace.DataDir
	if workspaceDir == "" {
		workspaceDir = filepath.Join(os.Getenv("HOME"), ".media2rag", "workspace")
	}

	if _, err := os.Stat(workspaceDir); os.IsNotExist(err) {
		return serviceStatus{Name: "Workspace", Status: "ok", Details: "not yet created"}
	}

	ws := &workspace.Workspace{RootPath: workspaceDir}
	count, err := ws.DocumentCount()
	if err != nil {
		return serviceStatus{Name: "Workspace", Status: "ok", Details: fmt.Sprintf("exists (%d docs)", count)}
	}
	return serviceStatus{Name: "Workspace", Status: "ok", Details: fmt.Sprintf("%d documents", count)}
}

func checkQdrant() serviceStatus {
	host := cfg.RAG.Qdrant.Host
	port := cfg.RAG.Qdrant.Port
	if host == "" {
		host = "localhost"
	}
	if port == 0 {
		port = 6334
	}

	st, err := store.New(host, port)
	if err != nil {
		return serviceStatus{Name: "Qdrant", Status: "error", Details: fmt.Sprintf("connect: %v", err)}
	}
	defer st.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cols, err := st.ListCollections(ctx)
	if err != nil {
		return serviceStatus{Name: "Qdrant", Status: "error", Details: fmt.Sprintf("ping: %v", err)}
	}
	return serviceStatus{Name: "Qdrant", Status: "ok", Details: fmt.Sprintf("%d collections", len(cols))}
}
