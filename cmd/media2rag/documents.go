package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"media2rag/internal/workspace"
)

var (
	docShowVersion  int
	docShowVersions bool
	docDeleteForce  bool
)

var documentsCmd = &cobra.Command{
	Use:   "documents",
	Short: "Manage processed documents",
}

var documentsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all processed documents",
	RunE: func(cmd *cobra.Command, args []string) error {
		ws, err := openWorkspace()
		if err != nil {
			return err
		}

		docs, err := ws.ListDocuments()
		if err != nil {
			return fmt.Errorf("list documents: %w", err)
		}

		if len(docs) == 0 {
			fmt.Println("No documents in workspace")
			return nil
		}

		fmt.Printf("%-8s  %-30s  %-20s  %s\n", "Hash", "Title", "Source", "Versions")
		fmt.Println("--------  " + "------------------------------" + "  " + "--------------------" + "  --------")
		for _, doc := range docs {
			title := doc.Title
			if title == "" {
				title = "-"
			}
			if len(title) > 30 {
				title = title[:27] + "..."
			}
			source := doc.Source
			if len(source) > 20 {
				source = source[:17] + "..."
			}
			fmt.Printf("%-8s  %-30s  %-20s  %d\n", doc.Hash, title, source, doc.Versions)
		}
		return nil
	},
}

var documentsShowCmd = &cobra.Command{
	Use:   "show <hash>",
	Short: "Show document details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		hash := args[0]
		ws, err := openWorkspace()
		if err != nil {
			return err
		}

		doc, err := ws.GetDocument(hash)
		if err != nil {
			return fmt.Errorf("get document: %w", err)
		}

		if docShowVersion > 0 {
			content, err := ws.ReadVersion(hash, docShowVersion)
			if err != nil {
				return err
			}
			fmt.Println(content)
			return nil
		}

		fmt.Printf("Hash:      %s\n", doc.Hash)
		fmt.Printf("Source:    %s\n", doc.Metadata.Source)
		fmt.Printf("Type:      %s\n", doc.Metadata.SourceType)
		if doc.Metadata.Title != "" {
			fmt.Printf("Title:     %s\n", doc.Metadata.Title)
		}
		fmt.Printf("Created:   %d\n", doc.Metadata.CreatedAt)
		fmt.Printf("Updated:   %d\n", doc.Metadata.UpdatedAt)

		if docShowVersions {
			fmt.Println("\nVersions:")
			for _, v := range doc.Metadata.Versions {
				fmt.Printf("  v%d\n", v)
			}
		}

		return nil
	},
}

var documentsDeleteCmd = &cobra.Command{
	Use:   "delete <hash>",
	Short: "Delete a document",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		hash := args[0]
		ws, err := openWorkspace()
		if err != nil {
			return err
		}

		if !docDeleteForce {
			fmt.Printf("Are you sure you want to delete document %s? (y/N): ", hash)
			var response string
			fmt.Scanf("%s", &response)
			if response != "y" && response != "Y" && response != "yes" && response != "Yes" {
				fmt.Println("Deletion cancelled")
				return nil
			}
		}

		if err := ws.DeleteDocument(hash); err != nil {
			return fmt.Errorf("delete document: %w", err)
		}

		fmt.Printf("Document %s deleted\n", hash)
		return nil
	},
}

func init() {
	documentsShowCmd.Flags().BoolVar(&docShowVersions, "versions", false, "list all versions")
	documentsShowCmd.Flags().IntVar(&docShowVersion, "version", 0, "show specific version content")
	documentsDeleteCmd.Flags().BoolVar(&docDeleteForce, "force", false, "delete without confirmation")

	documentsCmd.AddCommand(documentsListCmd)
	documentsCmd.AddCommand(documentsShowCmd)
	documentsCmd.AddCommand(documentsDeleteCmd)
	rootCmd.AddCommand(documentsCmd)
}

func openWorkspace() (*workspace.Workspace, error) {
	workspaceDir := cfg.Workspace.DataDir
	if workspaceDir == "" {
		workspaceDir = filepath.Join(os.Getenv("HOME"), ".media2rag", "workspace")
	}
	return workspace.New(workspaceDir)
}
