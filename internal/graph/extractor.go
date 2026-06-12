package graph

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"media2rag/internal/events"
	"media2rag/internal/llm"
	"media2rag/internal/model"
)

// EntityExtractor extracts entities and relations from chunks using LLM
type EntityExtractor struct {
	client         llm.LLMClient
	concurrency    int
	model          string
	checkpointDir  string
	Saver          func(chunkID string, result *ExtractResult) error
}

// NewEntityExtractor creates a new extractor
func NewEntityExtractor(client llm.LLMClient, concurrency int, model string) *EntityExtractor {
	if concurrency <= 0 {
		concurrency = 5
	}
	return &EntityExtractor{
		client:      client,
		concurrency: concurrency,
		model:       model,
	}
}

// NewEntityExtractorWithCheckpoint creates an extractor with per-chunk checkpointing
func NewEntityExtractorWithCheckpoint(client llm.LLMClient, concurrency int, model string, checkpointDir string) *EntityExtractor {
	e := NewEntityExtractor(client, concurrency, model)
	e.checkpointDir = checkpointDir
	return e
}

// ExtractResult holds entities and relations extracted from a chunk
type ExtractResult struct {
	ChunkID string
	Nodes   []*model.GraphNode
	Edges   []*model.GraphEdge
	Err     error
}

// ExtractChunk extracts entities and relations from a single chunk
func (e *EntityExtractor) ExtractChunk(ctx context.Context, chunkID, content string) (*ExtractResult, error) {
	nodes, edges, err := e.extractAll(ctx, chunkID, content)
	return &ExtractResult{
		ChunkID: chunkID,
		Nodes:   nodes,
		Edges:   edges,
		Err:     err,
	}, err
}

// ExtractBatch extracts entities and relations from multiple chunks in parallel
func (e *EntityExtractor) ExtractBatch(ctx context.Context, chunks []ChunkInput, emitter events.EventEmitter) []*ExtractResult {
	return e.ExtractBatchWithCheckpoint(ctx, chunks, emitter, nil)
}

// ExtractBatchWithCheckpoint extracts with periodic checkpoint callbacks
// Each successful result is saved atomically to checkpointDir/<chunkID>.json
func (e *EntityExtractor) ExtractBatchWithCheckpoint(ctx context.Context, chunks []ChunkInput, emitter events.EventEmitter, checkpoint func(processedIDs []string)) []*ExtractResult {
	results := make([]*ExtractResult, len(chunks))
	sem := make(chan struct{}, e.concurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var processedIDs []string
	doneIDs := make(map[string]bool)

	// Load existing checkpoints
	if e.checkpointDir != "" {
		entries, _ := os.ReadDir(e.checkpointDir)
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
				chunkID := strings.TrimSuffix(entry.Name(), ".json")
				doneIDs[chunkID] = true
			}
		}
		if len(doneIDs) > 0 {
			fmt.Fprintf(os.Stderr, "\r  Found %d existing checkpoints, skipping...\n", len(doneIDs))
		}
	}

	for i, chunk := range chunks {
		chunkID := fmt.Sprintf("%s:chunk_%d", chunk.DocID, chunk.ChunkIndex)

		// Skip if checkpoint exists
		if doneIDs[chunkID] {
			results[i] = &ExtractResult{ChunkID: chunkID}
			mu.Lock()
			processedIDs = append(processedIDs, chunkID)
			if checkpoint != nil && len(processedIDs)%10 == 0 {
				checkpoint(processedIDs)
			}
			mu.Unlock()
			continue
		}

		wg.Add(1)
		go func(idx int, c ChunkInput, cid string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			result, err := e.ExtractChunk(ctx, cid, c.Content)
			if err != nil {
				results[idx] = &ExtractResult{ChunkID: cid, Err: err}
				if emitter != nil {
					emitter.Emit(model.Event{Type: "graph_extract_error", Data: map[string]interface{}{
						"chunk_id": cid,
						"error":    err.Error(),
					}})
				}
				return
			}
			results[idx] = result

			// Save per-chunk checkpoint
			if e.checkpointDir != "" {
				if saveErr := saveExtractionCheckpoint(e.checkpointDir, cid, result); saveErr != nil {
					fmt.Fprintf(os.Stderr, "Warning: save checkpoint for %s: %v\n", cid, saveErr)
				}
			}

			if emitter != nil {
				emitter.Emit(model.Event{Type: "graph_chunk_extracted", Data: map[string]interface{}{
					"chunk_id": cid,
					"nodes":    len(result.Nodes),
					"edges":    len(result.Edges),
				}})
			}

			mu.Lock()
			processedIDs = append(processedIDs, cid)
			if checkpoint != nil && len(processedIDs)%10 == 0 {
				checkpoint(processedIDs)
			}
			mu.Unlock()
		}(i, chunk, chunkID)
	}

	wg.Wait()

	if checkpoint != nil && len(processedIDs)%50 != 0 {
		checkpoint(processedIDs)
	}

	return results
}

// ChunkInput represents a chunk to extract from
type ChunkInput struct {
	DocID      string
	ChunkIndex int
	Content    string
	Topic      string
}

const entityRelationPrompt = `Extract entities and relations from the text.

Entity types: Problem, Solution, Opportunity, Skill, Resource, Market, Audience, Business, Event, Claim, Metric, Concept
Relation types: causes, enables, prevents, requires, solves, blocks, competes_with, serves, leverages, leads_to, correlates, supports, contradicts, part_of

Return in this exact format:
entity: <name> | <type> | <brief description>
relation: <from_name> | <to_name> | <relation_type> | <mechanism> | <confidence 0.0-1.0>

Example:
entity: Отсутствие CRM | Problem | нет системы управления клиентами
entity: потеря 30% лидов | Metric | треть потенциальных клиентов не конвертируется
relation: Отсутствие CRM | потеря 30% лидов | causes | без CRM лиды теряются в процессе | 0.9

Return only filled values, no empty lines.`

// extractAll extracts both entities and relations in a single LLM call
func (e *EntityExtractor) extractAll(ctx context.Context, chunkID, content string) ([]*model.GraphNode, []*model.GraphEdge, error) {
	resp, err := e.client.Chat(ctx, model.ChatRequest{
		Model: e.model,
		Messages: []model.Message{
			{Role: "system", Content: "Extract entities and relations from text. Use the exact format specified."},
			{Role: "user", Content: entityRelationPrompt + "\n\nText:\n" + content},
		},
	})
	if err != nil {
		return nil, nil, fmt.Errorf("LLM extraction: %w", err)
	}

	nodes, edges := parseEntityRelationResponse(resp.Message.Content, chunkID)
	return nodes, edges, nil
}

func parseEntityRelationResponse(response, chunkID string) ([]*model.GraphNode, []*model.GraphEdge) {
	var nodes []*model.GraphNode
	var edges []*model.GraphEdge
	nodeMap := make(map[string]*model.GraphNode)

	for _, line := range strings.Split(response, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		if strings.HasPrefix(strings.ToLower(trimmed), "entity:") {
			val := strings.TrimPrefix(trimmed, "entity:")
			val = strings.TrimSpace(val)
			parts := splitPipe(val, 3)
			if len(parts) < 3 {
				continue
			}
			name := strings.TrimSpace(parts[0])
			nodeType := strings.TrimSpace(parts[1])
			desc := strings.TrimSpace(parts[2])

			if !isValidNodeType(nodeType) {
				continue
			}

			node := model.NewGraphNode(name, nodeType, desc)
			node.SourceChunks = []string{chunkID}
			nodeMap[name] = node
			nodes = append(nodes, node)
		}

		if strings.HasPrefix(strings.ToLower(trimmed), "relation:") {
			val := strings.TrimPrefix(trimmed, "relation:")
			val = strings.TrimSpace(val)
			parts := splitPipe(val, 5)
			if len(parts) < 5 {
				continue
			}
			fromName := strings.TrimSpace(parts[0])
			toName := strings.TrimSpace(parts[1])
			relType := strings.TrimSpace(parts[2])
			mechanism := strings.TrimSpace(parts[3])
			confStr := strings.TrimSpace(parts[4])

			fromNode, fromOk := nodeMap[fromName]
			toNode, toOk := nodeMap[toName]
			if !fromOk || !toOk {
				continue
			}
			if !isValidEdgeType(relType) {
				continue
			}

			conf, _ := strconv.ParseFloat(confStr, 64)
			if conf <= 0 {
				conf = 0.5
			}

			edges = append(edges, &model.GraphEdge{
				From:         fromNode.ID,
				To:           toNode.ID,
				RelationType: relType,
				Mechanism:    mechanism,
				Confidence:   conf,
				SourceChunk:  chunkID,
			})
		}
	}

	return nodes, edges
}

func splitPipe(s string, n int) []string {
	var parts []string
	for len(parts) < n-1 {
		idx := strings.Index(s, "|")
		if idx < 0 {
			break
		}
		parts = append(parts, s[:idx])
		s = s[idx+1:]
	}
	parts = append(parts, s)
	return parts
}

func isValidNodeType(t string) bool {
	for _, v := range model.ValidNodeTypes() {
		if v == t {
			return true
		}
	}
	return false
}

func isValidEdgeType(t string) bool {
	for _, v := range model.ValidEdgeTypes() {
		if v == t {
			return true
		}
	}
	return false
}

// CheckpointResult is the serializable form of ExtractResult for checkpoint files
type CheckpointResult struct {
	ChunkID string                `json:"chunk_id"`
	Nodes   []*model.GraphNode   `json:"nodes"`
	Edges   []*model.GraphEdge   `json:"edges"`
}

func saveExtractionCheckpoint(dir, chunkID string, result *ExtractResult) error {
	cp := CheckpointResult{
		ChunkID: chunkID,
		Nodes:   result.Nodes,
		Edges:   result.Edges,
	}
	data, err := json.Marshal(cp)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	safeID := strings.NewReplacer("/", "_", "\\", "_", ":", "_").Replace(chunkID)
	return os.WriteFile(filepath.Join(dir, safeID+".json"), data, 0644)
}

// LoadExtractionCheckpoints loads all saved checkpoint results from a directory
func LoadExtractionCheckpoints(dir string) (map[string]*CheckpointResult, error) {
	results := make(map[string]*CheckpointResult)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return results, nil
		}
		return nil, err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		var cp CheckpointResult
		if err := json.Unmarshal(data, &cp); err != nil {
			continue
		}
		results[cp.ChunkID] = &cp
	}
	return results, nil
}
