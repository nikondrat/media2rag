package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"media2rag/internal/graph"
	"media2rag/internal/model"
)

var (
	exportGraphPath  string
	exportOutputPath string
	exportFormat     string
)

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export knowledge graph to various formats",
	Long: `Export the knowledge graph to GraphML, DOT, or other formats for visualization.

Usage:
  media2rag export --format graphml
  media2rag export --format dot --output graph.dot
  media2rag export --format json --output graph-export.json`,
	RunE: runExport,
}

func runExport(cmd *cobra.Command, args []string) error {
	gPath := exportGraphPath
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

	switch exportFormat {
	case "graphml":
		return exportGraphML(kg, exportOutputPath)
	case "dot":
		return exportDOT(kg, exportOutputPath)
	case "json":
		return exportJSON(kg, exportOutputPath)
	default:
		return fmt.Errorf("unsupported format: %s (supported: graphml, dot, json)", exportFormat)
	}
}

type graphML struct {
	XMLName xml.Name        `xml:"http://graphml.graphdrawing.org/xmlns graphml"`
	Graph   graphMLGraph    `xml:"graph"`
}

type graphMLGraph struct {
	XMLName    xml.Name      `xml:"graph"`
	ID         string        `xml:"id,attr"`
	EdgeDefault string       `xml:"edgedefault,attr"`
	Nodes      []graphMLNode `xml:"node"`
	Edges      []graphMLEdge `xml:"edge"`
}

type graphMLNode struct {
	XMLName xml.Name      `xml:"node"`
	ID      string        `xml:"id,attr"`
	Data    []graphMLData `xml:"data"`
}

type graphMLEdge struct {
	XMLName xml.Name `xml:"edge"`
	ID      string   `xml:"id,attr"`
	Source  string   `xml:"source,attr"`
	Target  string   `xml:"target,attr"`
}

type graphMLData struct {
	XMLName xml.Name `xml:"data"`
	Key     string   `xml:"key,attr"`
	Value   string   `xml:",chardata"`
}

func exportGraphML(kg *model.KnowledgeGraph, outputPath string) error {
	gml := graphML{
		Graph: graphMLGraph{
			ID:         "knowledge_graph",
			EdgeDefault: "directed",
		},
	}

	for _, node := range kg.Nodes {
		gmlNode := graphMLNode{
			ID: node.ID,
			Data: []graphMLData{
				{Key: "name", Value: node.Name},
				{Key: "type", Value: node.Type},
				{Key: "description", Value: node.Description},
			},
		}
		if len(node.Aliases) > 0 {
			gmlNode.Data = append(gmlNode.Data, graphMLData{Key: "aliases", Value: fmt.Sprintf("%v", node.Aliases)})
		}
		gml.Graph.Nodes = append(gml.Graph.Nodes, gmlNode)
	}

	for i, edge := range kg.Edges {
		gml.Graph.Edges = append(gml.Graph.Edges, graphMLEdge{
			ID:     fmt.Sprintf("e%d", i),
			Source: edge.From,
			Target: edge.To,
		})
	}

	data, err := xml.MarshalIndent(gml, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal GraphML: %w", err)
	}

	output := []byte(xml.Header + string(data))

	if outputPath == "" {
		fmt.Println(string(output))
		return nil
	}

	if err := os.WriteFile(outputPath, output, 0644); err != nil {
		return fmt.Errorf("write GraphML file: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Exported GraphML to %s\n", outputPath)
	return nil
}

func exportDOT(kg *model.KnowledgeGraph, outputPath string) error {
	var sb string
	sb = "digraph KnowledgeGraph {\n"
	sb += "  rankdir=LR;\n"
	sb += "  node [shape=box, style=filled];\n\n"

	typeColors := map[string]string{
		"Problem":     "#ffcccc",
		"Solution":    "#ccffcc",
		"Opportunity": "#ccccff",
		"Concept":     "#ffffcc",
		"Metric":      "#ffccff",
		"Business":    "#ccffff",
	}

	for _, node := range kg.Nodes {
		color := typeColors[node.Type]
		if color == "" {
			color = "#ffffff"
		}
		label := fmt.Sprintf("%s\\n(%s)", escapeDOT(node.Name), node.Type)
		sb += fmt.Sprintf("  \"%s\" [label=\"%s\", fillcolor=\"%s\"];\n", node.ID, label, color)
	}

	sb += "\n"

	for _, edge := range kg.Edges {
		label := edge.RelationType
		if edge.Mechanism != "" {
			label = fmt.Sprintf("%s\\n%s", edge.RelationType, edge.Mechanism)
		}
		sb += fmt.Sprintf("  \"%s\" -> \"%s\" [label=\"%s\"];\n", edge.From, edge.To, escapeDOT(label))
	}

	sb += "}\n"

	if outputPath == "" {
		fmt.Println(sb)
		return nil
	}

	if err := os.WriteFile(outputPath, []byte(sb), 0644); err != nil {
		return fmt.Errorf("write DOT file: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Exported DOT to %s\n", outputPath)
	return nil
}

func exportJSON(kg *model.KnowledgeGraph, outputPath string) error {
	data, err := json.MarshalIndent(kg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal JSON: %w", err)
	}

	if outputPath == "" {
		fmt.Println(string(data))
		return nil
	}

	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("write JSON file: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Exported JSON to %s\n", outputPath)
	return nil
}

func escapeDOT(s string) string {
	result := ""
	for _, c := range s {
		switch c {
		case '"':
			result += "\\\""
		case '\\':
			result += "\\\\"
		case '\n':
			result += "\\n"
		default:
			result += string(c)
		}
	}
	return result
}

func init() {
	home, _ := os.UserHomeDir()
	defaultGraphPath := filepath.Join(home, ".media2rag", "data", "graph.json")

	exportCmd.Flags().StringVar(&exportGraphPath, "graph-path", defaultGraphPath, "Path to graph.json")
	exportCmd.Flags().StringVar(&exportOutputPath, "output", "", "Output file path (default: stdout)")
	exportCmd.Flags().StringVar(&exportFormat, "format", "graphml", "Export format: graphml, dot, json")
	rootCmd.AddCommand(exportCmd)
}
