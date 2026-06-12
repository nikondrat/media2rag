package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"media2rag/internal/graph"
	"media2rag/internal/model"
)

var (
	graphInfoPath string
)

var graphInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show knowledge graph statistics",
	Long: `Display statistics about the knowledge graph.

Usage:
  media2rag graph info
  media2rag graph info --graph-path /path/to/graph.json`,
	RunE: runGraphInfo,
}

func runGraphInfo(cmd *cobra.Command, args []string) error {
	gPath := graphInfoPath
	if gPath == "" {
		home, _ := os.UserHomeDir()
		gPath = filepath.Join(home, ".media2rag", "data", "graph.json")
	}

	if !graph.GraphExists(gPath) {
		return fmt.Errorf("graph not found at %s, run: media2rag index", gPath)
	}

	kg, err := graph.LoadGraph(gPath)
	if err != nil {
		return fmt.Errorf("load graph: %w", err)
	}

	// Compute statistics
	typeCount := make(map[string]int)
	for _, node := range kg.Nodes {
		typeCount[node.Type]++
	}

	relCount := make(map[string]int)
	for _, edge := range kg.Edges {
		relCount[edge.RelationType]++
	}

	// Compute average degree
	degree := make(map[string]int)
	for _, edge := range kg.Edges {
		degree[edge.From]++
		degree[edge.To]++
	}
	var totalDegree int
	for _, d := range degree {
		totalDegree += d
	}
	avgDegree := 0.0
	if len(kg.Nodes) > 0 {
		avgDegree = float64(totalDegree) / float64(len(kg.Nodes))
	}

	// Find top nodes by degree
	type nodeDegree struct {
		Name   string
		Type   string
		Degree int
	}
	var topNodes []nodeDegree
	for _, node := range kg.Nodes {
		topNodes = append(topNodes, nodeDegree{
			Name:   node.Name,
			Type:   node.Type,
			Degree: degree[node.ID],
		})
	}
	// Sort by degree (simple selection sort for top 5)
	for i := 0; i < 5 && i < len(topNodes); i++ {
		maxIdx := i
		for j := i + 1; j < len(topNodes); j++ {
			if topNodes[j].Degree > topNodes[maxIdx].Degree {
				maxIdx = j
			}
		}
		topNodes[i], topNodes[maxIdx] = topNodes[maxIdx], topNodes[i]
	}

	// Load communities info
	cPath := filepath.Join(filepath.Dir(gPath), "graph_communities.json")
	var communities []*model.Community
	if graph.GraphExists(cPath) {
		communities, _ = graph.LoadCommunities(cPath)
	}

	// Print report
	fmt.Println("=== Knowledge Graph Statistics ===")
	fmt.Printf("Nodes:     %d\n", len(kg.Nodes))
	fmt.Printf("Edges:     %d\n", len(kg.Edges))
	fmt.Printf("Avg degree: %.1f\n", avgDegree)
	fmt.Printf("Density:   %.4f\n", graphDensity(len(kg.Nodes), len(kg.Edges)))

	if len(communities) > 0 {
		fmt.Printf("\nCommunities: %d\n", len(communities))
		domainCount := make(map[string]int)
		for _, c := range communities {
			domainCount[c.Domain]++
		}
		fmt.Println("Domains:")
		for domain, count := range domainCount {
			fmt.Printf("  %s: %d communities\n", domain, count)
		}
	}

	fmt.Println("\nNode types:")
	for t, count := range typeCount {
		fmt.Printf("  %s: %d\n", t, count)
	}

	fmt.Println("\nTop relations:")
	for r, count := range relCount {
		fmt.Printf("  %s: %d\n", r, count)
	}

	fmt.Println("\nTop connected nodes:")
	for i, nd := range topNodes {
		if nd.Degree == 0 {
			break
		}
		fmt.Printf("  %d. %s (%s) — %d connections\n", i+1, nd.Name, nd.Type, nd.Degree)
	}

	return nil
}

func graphDensity(nodes, edges int) float64 {
	if nodes <= 1 {
		return 0
	}
	maxEdges := nodes * (nodes - 1)
	return float64(edges) / float64(maxEdges)
}

func init() {
	home, _ := os.UserHomeDir()
	graphInfoCmd.Flags().StringVar(&graphInfoPath, "graph-path", filepath.Join(home, ".media2rag", "data", "graph.json"), "Path to graph.json")

	// Create parent 'graph' command if not exists
	graphCmd := &cobra.Command{
		Use:   "graph",
		Short: "Graph management commands",
	}
	graphCmd.AddCommand(graphInfoCmd)
	rootCmd.AddCommand(graphCmd)
}
