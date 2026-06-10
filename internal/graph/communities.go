package graph

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"media2rag/internal/llm"
	"media2rag/internal/model"
)

// CommunityDetector groups chunks into topic-based communities
type CommunityDetector struct {
	client llm.LLMClient
	model  string
}

// NewCommunityDetector creates a new detector
func NewCommunityDetector(client llm.LLMClient, model string) *CommunityDetector {
	return &CommunityDetector{
		client: client,
		model:  model,
	}
}

// ChunkWithTopic represents a chunk with its topic
type ChunkWithTopic struct {
	ID      string
	Topic   string
	Summary string
	KeyPoints []string
	Content string
}

// DetectGroups chunks into communities by topic
func (d *CommunityDetector) DetectGroups(chunks []ChunkWithTopic) []*model.Community {
	// Group by topic
	byTopic := make(map[string][]ChunkWithTopic)
	for _, chunk := range chunks {
		topic := chunk.Topic
		if topic == "" {
			topic = "uncategorized"
		}
		byTopic[topic] = append(byTopic[topic], chunk)
	}

	communities := make([]*model.Community, 0, len(byTopic))
	for topic, topicChunks := range byTopic {
		chunkIDs := make([]string, len(topicChunks))
		for i, c := range topicChunks {
			chunkIDs[i] = c.ID
		}

		communities = append(communities, &model.Community{
			ID:             model.NodeID(topic, "community"),
			Topic:          topic,
			MemberChunkIDs: chunkIDs,
		})
	}

	// Sort by number of members (largest first)
	sort.Slice(communities, func(i, j int) bool {
		return len(communities[i].MemberChunkIDs) > len(communities[j].MemberChunkIDs)
	})

	return communities
}

// GenerateSummaries generates LLM summaries for each community
func (d *CommunityDetector) GenerateSummaries(ctx context.Context, communities []*model.Community, chunks []ChunkWithTopic) error {
	// Build chunk lookup
	chunkMap := make(map[string]ChunkWithTopic)
	for _, c := range chunks {
		chunkMap[c.ID] = c
	}

	for _, community := range communities {
		if err := d.generateCommunitySummary(ctx, community, chunkMap); err != nil {
			return fmt.Errorf("generate summary for community %s: %w", community.Topic, err)
		}
	}

	return nil
}

// generateCommunitySummary generates an LLM summary for a single community
func (d *CommunityDetector) generateCommunitySummary(ctx context.Context, community *model.Community, chunkMap map[string]ChunkWithTopic) error {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Community: %s\n\n", community.Topic))

	for _, chunkID := range community.MemberChunkIDs {
		chunk, ok := chunkMap[chunkID]
		if !ok {
			continue
		}
		sb.WriteString(fmt.Sprintf("Chunk: %s\n", chunkID))
		if chunk.Summary != "" {
			sb.WriteString(fmt.Sprintf("Summary: %s\n", chunk.Summary))
		}
		if len(chunk.KeyPoints) > 0 {
			sb.WriteString("Key Points:\n")
			for _, kp := range chunk.KeyPoints {
				sb.WriteString(fmt.Sprintf("- %s\n", kp))
			}
		}
		sb.WriteString("\n")
	}

	prompt := fmt.Sprintf(`Write a concise summary synthesizing the key insights from all chunks in this community.
Include the main themes, patterns, and actionable takeaways.
Reference specific chunk IDs for each claim.

%s

Summary:`, sb.String())

	resp, err := d.client.Chat(ctx, model.ChatRequest{
		Model: d.model,
		Messages: []model.Message{
			{Role: "system", Content: "You are an expert at synthesizing insights from multiple sources into a coherent summary."},
			{Role: "user", Content: prompt},
		},
	})
	if err != nil {
		return fmt.Errorf("LLM summary generation: %w", err)
	}

	community.Summary = strings.TrimSpace(resp.Message.Content)

	// Extract key insights
	keyInsights, err := d.extractKeyInsights(ctx, community.Summary)
	if err != nil {
		// Non-fatal
		keyInsights = []string{community.Summary}
	}
	community.KeyInsights = keyInsights

	return nil
}

// extractKeyInsights extracts bullet-point insights from a summary
func (d *CommunityDetector) extractKeyInsights(ctx context.Context, summary string) ([]string, error) {
	prompt := fmt.Sprintf(`Extract 3-5 key insights from this summary.

Return in this exact format:
insight: <first insight>
insight: <second insight>
insight: <third insight>

Summary:
%s`, summary)

	resp, err := d.client.Chat(ctx, model.ChatRequest{
		Model: d.model,
		Messages: []model.Message{
			{Role: "system", Content: "Extract key insights. Use the exact format specified."},
			{Role: "user", Content: prompt},
		},
	})
	if err != nil {
		return nil, err
	}

	var insights []string
	lines := strings.Split(resp.Message.Content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToLower(trimmed), "insight:") {
			val := strings.TrimPrefix(trimmed, "insight:")
			val = strings.TrimSpace(val)
			if val != "" {
				insights = append(insights, val)
			}
		}
	}

	if len(insights) == 0 {
		return []string{summary}, nil
	}
	return insights, nil
}

// GenerateDomainHierarchy assigns domains to communities using LLM
func (d *CommunityDetector) GenerateDomainHierarchy(ctx context.Context, communities []*model.Community) error {
	if len(communities) == 0 {
		return nil
	}

	topics := make([]string, len(communities))
	for i, c := range communities {
		topics[i] = c.Topic
	}

	prompt := fmt.Sprintf(`Group these topics into logical domains.

Return in this exact format:
domain: <domain_name> | <topic1>, <topic2>, <topic3>

Example:
domain: sales | Воронка продаж, CRM
domain: devops | Kubernetes, Docker

Topics: %s`, strings.Join(topics, ", "))

	resp, err := d.client.Chat(ctx, model.ChatRequest{
		Model: d.model,
		Messages: []model.Message{
			{Role: "system", Content: "Group topics into domains. Use the exact format specified."},
			{Role: "user", Content: prompt},
		},
	})
	if err != nil {
		return fmt.Errorf("LLM domain hierarchy: %w", err)
	}

	domains := make(map[string][]string)
	lines := strings.Split(resp.Message.Content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToLower(trimmed), "domain:") {
			val := strings.TrimPrefix(trimmed, "domain:")
			val = strings.TrimSpace(val)
			parts := splitPipe(val, 2)
			if len(parts) < 2 {
				continue
			}
			domainName := strings.TrimSpace(parts[0])
			topicList := strings.TrimSpace(parts[1])
			for _, t := range strings.Split(topicList, ",") {
				t = strings.TrimSpace(t)
				if t != "" {
					domains[domainName] = append(domains[domainName], t)
				}
			}
		}
	}

	// Assign domains to communities
	for _, community := range communities {
		for domain, domainTopics := range domains {
			for _, dt := range domainTopics {
				if dt == community.Topic {
					community.Domain = domain
					break
				}
			}
		}
		if community.Domain == "" {
			community.Domain = "general"
		}
	}

	return nil
}

// LinkCommunitiesToGraph links communities to graph nodes
func LinkCommunitiesToGraph(communities []*model.Community, graph *model.KnowledgeGraph) {
	for _, community := range communities {
		nodeIDs := make([]string, 0)
		for _, chunkID := range community.MemberChunkIDs {
			// Find nodes that reference this chunk
			for _, node := range graph.Nodes {
				for _, sc := range node.SourceChunks {
					if sc == chunkID {
						found := false
						for _, nid := range nodeIDs {
							if nid == node.ID {
								found = true
								break
							}
						}
						if !found {
							nodeIDs = append(nodeIDs, node.ID)
						}
					}
				}
			}
		}
		community.MemberNodeIDs = nodeIDs
	}
}
