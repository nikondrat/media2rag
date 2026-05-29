package model

type RAGDocument struct {
	Markdown string           `json:"markdown"`
	Metadata DocumentMetadata `json:"metadata"`
}

type DocumentMetadata struct {
	Title        string   `json:"title"`
	Author       string   `json:"author"`
	Source       string   `json:"source"`
	DocType      string   `json:"doc_type"`
	Language     string   `json:"language"`
	Domains      []string `json:"domains"`
	CoreThesis   string   `json:"core_thesis"`
	MentalModels []string `json:"mental_models"`
	Claims       []Claim  `json:"claims"`
	Takeaways    []string `json:"takeaways"`
	KeyTerms     []KeyTerm `json:"key_terms"`
	Summary      string   `json:"summary"`
	KeyInsights  []string `json:"key_insights"`
	WordCount    int      `json:"word_count"`
	Topics       []string `json:"topics"`
}

type Claim struct {
	Statement  string  `json:"statement"`
	Confidence float64 `json:"confidence"`
	Source     string  `json:"source"`
}

type KeyTerm struct {
	Term       string `json:"term"`
	Definition string `json:"definition"`
}
