package model

type RAGDocument struct {
	Markdown      string           `json:"markdown"`
	CleanedText   string           `json:"cleaned_text,omitempty"`
	Metadata      DocumentMetadata `json:"metadata"`
	Chunks        []Chunk          `json:"chunks,omitempty"`
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
	Confidence   float64  `json:"confidence,omitempty"`
	Status       string   `json:"status,omitempty"`
	MyRelevance  string   `json:"my_relevance,omitempty"`
	Tags         []string `json:"tags,omitempty"`
	ID           string   `json:"id,omitempty"`
}

type Chunk struct {
	Index         int      `json:"index"`
	Type          string   `json:"type"`
	Topic         string   `json:"topic"`
	Summary       string   `json:"summary"`
	Content       string   `json:"content,omitempty"`
	Context       string   `json:"context,omitempty"`
	KeyPoints     []string `json:"key_points,omitempty"`
	SourceQuote   string   `json:"source_quote,omitempty"`
	MyTakeaway    string   `json:"my_takeaway,omitempty"`
	Confidence    float64  `json:"confidence,omitempty"`
	Applicability string   `json:"applicability,omitempty"`
	Steps         []string `json:"steps,omitempty"`
}

const (
	TypeIdea        = "idea"
	TypeFramework   = "framework"
	TypePrinciple   = "principle"
	TypeExample     = "example"
	TypeCaseStudy   = "case_study"
	TypeTool        = "tool"
	TypeWarning     = "warning"
	TypeActionStep  = "action_step"
	TypeQuote       = "quote"
	TypeQuestion    = "question"
	TypePersonalNote = "personal_note"
)

type Claim struct {
	Statement  string  `json:"statement"`
	Confidence float64 `json:"confidence"`
	Source     string  `json:"source"`
}

type KeyTerm struct {
	Term       string `json:"term"`
	Definition string `json:"definition"`
}
