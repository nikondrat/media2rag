package model

import (
	"crypto/sha256"
	"fmt"
)

// Entity types (12)
const (
	NodeTypeProblem      = "Problem"
	NodeTypeSolution     = "Solution"
	NodeTypeOpportunity  = "Opportunity"
	NodeTypeSkill        = "Skill"
	NodeTypeResource     = "Resource"
	NodeTypeMarket       = "Market"
	NodeTypeAudience     = "Audience"
	NodeTypeBusiness     = "Business"
	NodeTypeEvent        = "Event"
	NodeTypeClaim        = "Claim"
	NodeTypeMetric       = "Metric"
	NodeTypeConcept      = "Concept"
)

// ValidNodeTypes returns all valid node types
func ValidNodeTypes() []string {
	return []string{
		NodeTypeProblem, NodeTypeSolution, NodeTypeOpportunity,
		NodeTypeSkill, NodeTypeResource, NodeTypeMarket,
		NodeTypeAudience, NodeTypeBusiness, NodeTypeEvent,
		NodeTypeClaim, NodeTypeMetric, NodeTypeConcept,
	}
}

// Relation types (14)
const (
	EdgeTypeCauses       = "causes"
	EdgeTypeEnables      = "enables"
	EdgeTypePrevents     = "prevents"
	EdgeTypeRequires     = "requires"
	EdgeTypeSolves       = "solves"
	EdgeTypeBlocks       = "blocks"
	EdgeTypeCompetesWith = "competes_with"
	EdgeTypeServes       = "serves"
	EdgeTypeLeverages    = "leverages"
	EdgeTypeLeadsTo      = "leads_to"
	EdgeTypeCorrelates   = "correlates"
	EdgeTypeSupports     = "supports"
	EdgeTypeContradicts  = "contradicts"
	EdgeTypePartOf       = "part_of"
)

// ValidEdgeTypes returns all valid edge types
func ValidEdgeTypes() []string {
	return []string{
		EdgeTypeCauses, EdgeTypeEnables, EdgeTypePrevents,
		EdgeTypeRequires, EdgeTypeSolves, EdgeTypeBlocks,
		EdgeTypeCompetesWith, EdgeTypeServes, EdgeTypeLeverages,
		EdgeTypeLeadsTo, EdgeTypeCorrelates, EdgeTypeSupports,
		EdgeTypeContradicts, EdgeTypePartOf,
	}
}

// GraphNode represents an entity in the knowledge graph
type GraphNode struct {
	ID            string                 `json:"id"`
	Name          string                 `json:"name"`
	Type          string                 `json:"type"`
	Description   string                 `json:"description"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	SourceChunks  []string               `json:"source_chunks,omitempty"`
	Embedding     []float32              `json:"embedding,omitempty"`
	Aliases       []string               `json:"aliases,omitempty"`
}

// NewGraphNode creates a node with a deterministic ID from name+type
func NewGraphNode(name, nodeType, description string) *GraphNode {
	return &GraphNode{
		ID:          NodeID(name, nodeType),
		Name:        name,
		Type:        nodeType,
		Description: description,
		Metadata:    make(map[string]interface{}),
	}
}

// NodeID generates a deterministic ID from name and type
func NodeID(name, nodeType string) string {
	h := sha256.Sum256([]byte(name + "::" + nodeType))
	return fmt.Sprintf("%x", h[:8])
}

// GraphEdge represents a relation between two nodes
type GraphEdge struct {
	From          string                 `json:"from"`
	To            string                 `json:"to"`
	RelationType  string                 `json:"relation_type"`
	Mechanism     string                 `json:"mechanism,omitempty"`
	Confidence    float64                `json:"confidence"`
	SourceChunk   string                 `json:"source_chunk"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// KnowledgeGraph holds the full graph with indexes
type KnowledgeGraph struct {
	Nodes []*GraphNode `json:"nodes"`
	Edges []*GraphEdge `json:"edges"`

	// Indexes (built after load)
	ByName     map[string][]*GraphNode   `json:"-"`
	ByType     map[string][]*GraphNode   `json:"-"`
	ByRelation map[string][]*GraphEdge   `json:"-"`
	ByNodeID   map[string]*GraphNode     `json:"-"`
	OutEdges   map[string][]*GraphEdge   `json:"-"`
	InEdges    map[string][]*GraphEdge   `json:"-"`
}

// NewKnowledgeGraph creates an empty graph
func NewKnowledgeGraph() *KnowledgeGraph {
	return &KnowledgeGraph{
		Nodes:      make([]*GraphNode, 0),
		Edges:      make([]*GraphEdge, 0),
		ByName:     make(map[string][]*GraphNode),
		ByType:     make(map[string][]*GraphNode),
		ByRelation: make(map[string][]*GraphEdge),
		ByNodeID:   make(map[string]*GraphNode),
		OutEdges:   make(map[string][]*GraphEdge),
		InEdges:    make(map[string][]*GraphEdge),
	}
}

// AddNode adds a node to the graph (deduplicates by ID)
func (g *KnowledgeGraph) AddNode(node *GraphNode) {
	if _, exists := g.ByNodeID[node.ID]; exists {
		// Merge source chunks
		existing := g.ByNodeID[node.ID]
		for _, sc := range node.SourceChunks {
			found := false
			for _, esc := range existing.SourceChunks {
				if esc == sc {
					found = true
					break
				}
			}
			if !found {
				existing.SourceChunks = append(existing.SourceChunks, sc)
			}
		}
		return
	}
	g.Nodes = append(g.Nodes, node)
	g.ByNodeID[node.ID] = node
	g.ByName[node.Name] = append(g.ByName[node.Name], node)
	g.ByType[node.Type] = append(g.ByType[node.Type], node)
}

// AddEdge adds an edge to the graph
func (g *KnowledgeGraph) AddEdge(edge *GraphEdge) {
	g.Edges = append(g.Edges, edge)
	g.ByRelation[edge.RelationType] = append(g.ByRelation[edge.RelationType], edge)
	g.OutEdges[edge.From] = append(g.OutEdges[edge.From], edge)
	g.InEdges[edge.To] = append(g.InEdges[edge.To], edge)
}

// BuildIndexes rebuilds all indexes (useful after loading from JSON)
func (g *KnowledgeGraph) BuildIndexes() {
	g.ByName = make(map[string][]*GraphNode)
	g.ByType = make(map[string][]*GraphNode)
	g.ByRelation = make(map[string][]*GraphEdge)
	g.ByNodeID = make(map[string]*GraphNode)
	g.OutEdges = make(map[string][]*GraphEdge)
	g.InEdges = make(map[string][]*GraphEdge)

	for _, node := range g.Nodes {
		g.ByNodeID[node.ID] = node
		g.ByName[node.Name] = append(g.ByName[node.Name], node)
		g.ByType[node.Type] = append(g.ByType[node.Type], node)
	}

	for _, edge := range g.Edges {
		g.ByRelation[edge.RelationType] = append(g.ByRelation[edge.RelationType], edge)
		g.OutEdges[edge.From] = append(g.OutEdges[edge.From], edge)
		g.InEdges[edge.To] = append(g.InEdges[edge.To], edge)
	}
}

// GetNodeByID returns a node by ID
func (g *KnowledgeGraph) GetNodeByID(id string) (*GraphNode, bool) {
	node, ok := g.ByNodeID[id]
	return node, ok
}

// GetNodesByName returns all nodes with the given name
func (g *KnowledgeGraph) GetNodesByName(name string) []*GraphNode {
	return g.ByName[name]
}

// GetNodesByType returns all nodes of the given type
func (g *KnowledgeGraph) GetNodesByType(nodeType string) []*GraphNode {
	return g.ByType[nodeType]
}

// GetEdgesByRelation returns all edges of the given relation type
func (g *KnowledgeGraph) GetEdgesByRelation(relationType string) []*GraphEdge {
	return g.ByRelation[relationType]
}

// GetOutEdges returns all outgoing edges from a node
func (g *KnowledgeGraph) GetOutEdges(nodeID string) []*GraphEdge {
	return g.OutEdges[nodeID]
}

// GetInEdges returns all incoming edges to a node
func (g *KnowledgeGraph) GetInEdges(nodeID string) []*GraphEdge {
	return g.InEdges[nodeID]
}

// Validate checks graph integrity
func (g *KnowledgeGraph) Validate() error {
	validTypes := make(map[string]bool)
	for _, t := range ValidNodeTypes() {
		validTypes[t] = true
	}
	validRelations := make(map[string]bool)
	for _, t := range ValidEdgeTypes() {
		validRelations[t] = true
	}

	for _, node := range g.Nodes {
		if !validTypes[node.Type] {
			return fmt.Errorf("invalid node type: %s (node: %s)", node.Type, node.ID)
		}
	}

	for _, edge := range g.Edges {
		if !validRelations[edge.RelationType] {
			return fmt.Errorf("invalid edge type: %s", edge.RelationType)
		}
		if _, ok := g.ByNodeID[edge.From]; !ok {
			return fmt.Errorf("orphan edge: from node %s not found", edge.From)
		}
		if _, ok := g.ByNodeID[edge.To]; !ok {
			return fmt.Errorf("orphan edge: to node %s not found", edge.To)
		}
	}

	return nil
}

// Community represents a topic-based cluster of chunks
type Community struct {
	ID              string   `json:"id"`
	Topic           string   `json:"topic"`
	Domain          string   `json:"domain,omitempty"`
	Summary         string   `json:"summary"`
	MemberChunkIDs  []string `json:"member_chunk_ids"`
	MemberNodeIDs   []string `json:"member_node_ids,omitempty"`
	KeyInsights     []string `json:"key_insights,omitempty"`
}

// GraphQuery represents a structured query for graph traversal
type GraphQuery struct {
	Entities []string `json:"entities"`
	Pattern  string   `json:"pattern"`
	Relations []string `json:"relations,omitempty"`
	Mode     string   `json:"mode"`     // local, global, drift, auto
	Depth    int      `json:"depth"`
}

// Query patterns
const (
	PatternRootCause    = "root_cause"
	PatternCounterfactual = "counterfactual"
	PatternPrerequisites = "prerequisites"
	PatternCommonality  = "commonality"
	PatternGlobal       = "global"
	PatternDRIFT        = "drift"
)

// Search modes
const (
	ModeLocal  = "local"
	ModeGlobal = "global"
	ModeDRIFT  = "drift"
	ModeAuto   = "auto"
)

// SearchResult represents a single result from RAG or GraphRAG
type SearchResult struct {
	ChunkID     string  `json:"chunk_id"`
	File        string  `json:"file"`
	Score       float64 `json:"score"`
	Topic       string  `json:"topic"`
	Summary     string  `json:"summary"`
	KeyPoints   []string `json:"key_points,omitempty"`
	Content     string  `json:"content,omitempty"`
}

// GraphRAGResult represents a GraphRAG response with chains and provenance
type GraphRAGResult struct {
	Query       string          `json:"query"`
	Entities    []string        `json:"entities"`
	Pattern     string          `json:"pattern"`
	Mode        string          `json:"mode"`
	Chains      []ReasoningChain `json:"chains"`
	Opportunities []string      `json:"opportunities,omitempty"`
	Provenance  []string        `json:"provenance"`
	Answer      string          `json:"answer"`
}

// ReasoningChain represents a causal path through the graph
type ReasoningChain struct {
	Path       []string  `json:"path"`
	Relations  []string  `json:"relations"`
	Confidence float64   `json:"confidence"`
	Mechanisms []string  `json:"mechanisms"`
	SourceChunks []string `json:"source_chunks"`
}
