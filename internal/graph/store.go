package graph

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"media2rag/internal/model"
)

// SaveGraph saves a knowledge graph to a JSON file
func SaveGraph(graph *model.KnowledgeGraph, path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	data, err := json.MarshalIndent(graph, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal graph: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write graph file: %w", err)
	}

	return nil
}

// LoadGraph loads a knowledge graph from a JSON file and builds indexes
func LoadGraph(path string) (*model.KnowledgeGraph, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read graph file: %w", err)
	}

	var graph model.KnowledgeGraph
	if err := json.Unmarshal(data, &graph); err != nil {
		return nil, fmt.Errorf("unmarshal graph: %w", err)
	}

	graph.BuildIndexes()

	if err := graph.Validate(); err != nil {
		return nil, fmt.Errorf("graph validation failed: %w", err)
	}

	return &graph, nil
}

// SaveCommunities saves communities to a JSON file
func SaveCommunities(communities []*model.Community, path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	data, err := json.MarshalIndent(communities, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal communities: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write communities file: %w", err)
	}

	return nil
}

// LoadCommunities loads communities from a JSON file
func LoadCommunities(path string) ([]*model.Community, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read communities file: %w", err)
	}

	var communities []*model.Community
	if err := json.Unmarshal(data, &communities); err != nil {
		return nil, fmt.Errorf("unmarshal communities: %w", err)
	}

	return communities, nil
}

// GraphExists checks if a graph file exists and is valid
func GraphExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
