package graph

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"media2rag/internal/events"
	"media2rag/internal/llm"
	"media2rag/internal/model"
)

// EntityExtractor extracts entities and relations from chunks using LLM
type EntityExtractor struct {
	client      llm.LLMClient
	concurrency int
	model       string
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
	results := make([]*ExtractResult, len(chunks))
	sem := make(chan struct{}, e.concurrency)
	var wg sync.WaitGroup

	for i, chunk := range chunks {
		wg.Add(1)
		go func(idx int, c ChunkInput) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			chunkID := fmt.Sprintf("%s:chunk_%d", c.DocID, c.ChunkIndex)
			result, err := e.ExtractChunk(ctx, chunkID, c.Content)
			if err != nil {
				results[idx] = &ExtractResult{ChunkID: chunkID, Err: err}
				if emitter != nil {
					emitter.Emit(model.Event{Type: "graph_extract_error", Data: map[string]interface{}{
						"chunk_id": chunkID,
						"error":    err.Error(),
					}})
				}
				return
			}
			results[idx] = result
			if emitter != nil {
				emitter.Emit(model.Event{Type: "graph_chunk_extracted", Data: map[string]interface{}{
					"chunk_id": chunkID,
					"nodes":    len(result.Nodes),
					"edges":    len(result.Edges),
				}})
			}
		}(i, chunk)
	}

	wg.Wait()
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
