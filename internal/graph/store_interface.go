package graph

import (
	"media2rag/internal/model"
)

// GraphStore abstracts the graph storage layer.
// JSON implementation is default; Kuzu can be swapped in later.
type GraphStore interface {
	// Load reads the full graph into memory
	Load(path string) (*model.KnowledgeGraph, error)

	// Save persists the graph to storage
	Save(kg *model.KnowledgeGraph, path string) error

	// Exists checks if a graph exists at the given path
	Exists(path string) bool

	// Merge merges new nodes/edges into an existing graph (dedup by ID)
	Merge(kg *model.KnowledgeGraph, nodes []*model.GraphNode, edges []*model.GraphEdge)

	// LoadCommunities reads communities from storage
	LoadCommunities(path string) ([]*model.Community, error)

	// SaveCommunities persists communities to storage
	SaveCommunities(communities []*model.Community, path string) error

	// CommunitiesExists checks if communities file exists
	CommunitiesExists(path string) bool
}

// JSONStore implements GraphStore using JSON files
type JSONStore struct{}

// NewJSONStore creates a JSON-backed graph store
func NewJSONStore() *JSONStore {
	return &JSONStore{}
}

func (s *JSONStore) Load(path string) (*model.KnowledgeGraph, error) {
	return LoadGraph(path)
}

func (s *JSONStore) Save(kg *model.KnowledgeGraph, path string) error {
	return SaveGraph(kg, path)
}

func (s *JSONStore) Exists(path string) bool {
	return GraphExists(path)
}

func (s *JSONStore) Merge(kg *model.KnowledgeGraph, nodes []*model.GraphNode, edges []*model.GraphEdge) {
	for _, node := range nodes {
		kg.AddNode(node)
	}
	for _, edge := range edges {
		if _, fromOk := kg.GetNodeByID(edge.From); fromOk {
			if _, toOk := kg.GetNodeByID(edge.To); toOk {
				kg.AddEdge(edge)
			}
		}
	}
}

func (s *JSONStore) LoadCommunities(path string) ([]*model.Community, error) {
	return LoadCommunities(path)
}

func (s *JSONStore) SaveCommunities(communities []*model.Community, path string) error {
	return SaveCommunities(communities, path)
}

func (s *JSONStore) CommunitiesExists(path string) bool {
	return GraphExists(path)
}
