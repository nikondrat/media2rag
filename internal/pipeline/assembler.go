package pipeline

import (
	"fmt"
	"sort"
	"strings"

	"golang.org/x/text/unicode/norm"

	"media2rag/internal/model"
)

type assembleOpts struct {
	source          string
	docType         string
	author          string
	language        string
	domains         []string
	coreThesis      string
	causalChains    []model.CausalLink
	preconditions   []string
	counterfactuals []string
}

type AssemblyInput struct {
	Content          string
	Chunks           []ChunkResult
	HolisticAnalysis string
	SourcePath       string
	SourceHash       string
	SourceType       string
	Frontmatter      map[string]interface{}
}

func assemble(results []ChunkResult, opts assembleOpts) *model.RAGDocument {
	fm := map[string]interface{}{}

	if opts.author != "" {
		fm["author"] = opts.author
	}
	fm["language"] = opts.language
	if len(opts.domains) > 0 {
		topics := make([]interface{}, len(opts.domains))
		for i, d := range opts.domains {
			topics[i] = d
		}
		fm["topics"] = topics
	}

	input := &AssemblyInput{
		Chunks:     results,
		SourcePath: opts.source,
		SourceType: opts.docType,
		Frontmatter: fm,
	}

	title := CollectTitle(input)
	fm["title"] = title

	holisticText := ""
	if opts.coreThesis != "" {
		holisticText = "## Holistic Analysis\n\n" + opts.coreThesis + "\n"
	}
	if len(opts.causalChains) > 0 {
		holisticText += "\n## Causal Chains\n\n" + formatCausalMarkdown(opts.causalChains)
	}
	if len(opts.preconditions) > 0 {
		holisticText += "\n## Preconditions\n\n" + bulletList(opts.preconditions)
	}
	if len(opts.counterfactuals) > 0 {
		holisticText += "\n## Counterfactuals\n\n" + bulletList(opts.counterfactuals)
	}
	input.HolisticAnalysis = holisticText

	markdown := assembleOutput(input)

	var docChunks []model.Chunk
	for _, ch := range results {
		docChunks = append(docChunks, model.Chunk{
			Index:         ch.Index,
			Type:          ch.Type,
			Topic:         ch.Topic,
			Summary:       ch.Summary,
			Content:       ch.Content,
			Context:       ch.Context,
			KeyPoints:     ch.KeyPoints,
			SourceQuote:   ch.SourceQuote,
			MyTakeaway:    ch.MyTakeaway,
			Confidence:    ch.Confidence,
			Applicability: ch.Applicability,
			Steps:         ch.Steps,
		})
	}

	topicSet := map[string]bool{}
	for _, ch := range results {
		if ch.Topic != "" {
			topicSet[ch.Topic] = true
		}
		for _, t := range ch.Topics {
			topicSet[t] = true
		}
	}
	for _, d := range opts.domains {
		topicSet[d] = true
	}
	var topics []string
	for t := range topicSet {
		topics = append(topics, t)
	}
	sort.Strings(topics)

	var takeaways []string
	for _, ch := range results {
		if ch.MyTakeaway != "" {
			takeaways = append(takeaways, ch.MyTakeaway)
		}
	}

	doc := &model.RAGDocument{
		Markdown: markdown,
		Chunks:   docChunks,
		Metadata: model.DocumentMetadata{
			Title:           title,
			Author:          opts.author,
			Source:          opts.source,
			DocType:         opts.docType,
			Language:        opts.language,
			Domains:         opts.domains,
			CoreThesis:      opts.coreThesis,
			Topics:          topics,
			Takeaways:       takeaways,
			Status:          "processed",
			CausalChains:    opts.causalChains,
			Preconditions:   opts.preconditions,
			Counterfactuals: opts.counterfactuals,
		},
	}

	wordCount := 0
	for _, ch := range results {
		wordCount += len(strings.Fields(ch.Content))
	}
	doc.Metadata.WordCount = wordCount

	return doc
}

func assembleOutput(input *AssemblyInput) string {
	var b strings.Builder

	writeFrontmatter(&b, input)
	b.WriteString("---\n\n")

	chunks := input.Chunks
	sort.Slice(chunks, func(i, j int) bool {
		return chunks[i].Index < chunks[j].Index
	})

	for i, ch := range chunks {
		if ch.Type == "" && ch.Summary == "" {
			continue
		}
		if i > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "## chunk_%02d\n", ch.Index+1)
		if ch.Context != "" {
			writeField(&b, "context", ch.Context)
		}
		writeField(&b, "type", ch.Type)
		writeField(&b, "topic", ch.Topic)
		writeField(&b, "summary", ch.Summary)

		if len(ch.KeyPoints) > 0 {
			b.WriteString("key_points:\n")
			for _, kp := range ch.KeyPoints {
				kp = strings.TrimSpace(kp)
				if kp != "" {
					b.WriteString("- ")
					b.WriteString(kp)
					b.WriteString("\n")
				}
			}
		}

		writeField(&b, "source_quote", ch.SourceQuote)
		writeField(&b, "my_takeaway", ch.MyTakeaway)
		writeField(&b, "confidence", ConfidenceToString(ch.Confidence))
		writeField(&b, "applicability", ch.Applicability)

		if len(ch.Steps) > 0 {
			b.WriteString("steps:\n")
			for _, s := range ch.Steps {
				s = strings.TrimSpace(s)
				if s != "" {
					b.WriteString("- ")
					b.WriteString(s)
					b.WriteString("\n")
				}
			}
		}
	}

	if input.HolisticAnalysis != "" {
		b.WriteString("\n")
		b.WriteString(input.HolisticAnalysis)
	}

	return b.String()
}

func writeFrontmatter(b *strings.Builder, input *AssemblyInput) {
	fm := input.Frontmatter

	writeField(b, "id", input.SourceHash)
	writeFieldFromMap(b, "title", fm, "title", "")
	writeFieldFromMap(b, "source_type", fm, "source_type", input.SourceType)

	if input.SourcePath != "" {
		writeField(b, "source", input.SourcePath)
	}

	writeFieldFromMap(b, "author", fm, "author", "")
	writeFieldFromMap(b, "date", fm, "date", "")
	writeFieldFromMap(b, "language", fm, "language", "ru")

	if topics, ok := extractStringList(fm, "topics"); ok {
		b.WriteString("topics: [")
		b.WriteString(strings.Join(topics, ", "))
		b.WriteString("]\n")
	}

	writeFieldFromMap(b, "confidence", fm, "confidence", "")
	writeFieldFromMap(b, "status", fm, "status", "processed")
	writeFieldFromMap(b, "my_relevance", fm, "my_relevance", "")

	if tags, ok := extractStringList(fm, "tags"); ok {
		b.WriteString("tags: [")
		b.WriteString(strings.Join(tags, ", "))
		b.WriteString("]\n")
	}
}

func writeField(b *strings.Builder, key, value string) {
	if strings.TrimSpace(value) == "" {
		return
	}
	if strings.Contains(value, "\n") {
		b.WriteString(key)
		b.WriteString(": |\n")
		for _, line := range strings.Split(value, "\n") {
			b.WriteString("  ")
			b.WriteString(line)
			b.WriteString("\n")
		}
	} else {
		b.WriteString(key)
		b.WriteString(": ")
		b.WriteString(value)
		b.WriteString("\n")
	}
}

func writeFieldFromMap(b *strings.Builder, key string, fm map[string]interface{}, mapKey string, fallback string) {
	if fm == nil {
		if fallback != "" {
			writeField(b, key, fallback)
		}
		return
	}
	v, ok := fm[mapKey]
	if !ok {
		v, ok = fm[key]
	}
	if ok {
		s := fmt.Sprintf("%v", v)
		if strings.TrimSpace(s) != "" {
			writeField(b, key, s)
			return
		}
	}
	if fallback != "" {
		writeField(b, key, fallback)
	}
}

func extractStringList(fm map[string]interface{}, key string) ([]string, bool) {
	if fm == nil {
		return nil, false
	}
	v, ok := fm[key]
	if !ok {
		return nil, false
	}

	switch val := v.(type) {
	case []interface{}:
		result := make([]string, 0, len(val))
		for _, item := range val {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				result = append(result, strings.TrimSpace(s))
			}
		}
		return result, true
	case string:
		cleaned := strings.Trim(val, "[]")
		parts := strings.Split(cleaned, ",")
		result := make([]string, 0, len(parts))
		for _, p := range parts {
			t := strings.TrimSpace(p)
			if t != "" {
				result = append(result, t)
			}
		}
		return result, true
	}
	return nil, false
}

func CollectTitle(input *AssemblyInput) string {
	if input.Frontmatter != nil {
		if t, ok := input.Frontmatter["title"]; ok {
			if s, ok := t.(string); ok && strings.TrimSpace(s) != "" {
				return strings.TrimSpace(s)
			}
		}
	}
	if input.SourcePath != "" {
		parts := strings.Split(input.SourcePath, "/")
		filename := parts[len(parts)-1]
		if idx := strings.LastIndex(filename, "."); idx > 0 {
			filename = filename[:idx]
		}
		return SanitizeFilename(filename)
	}
	return "unnamed_document"
}

func SanitizeFilename(name string) string {
	replacer := strings.NewReplacer(
		"/", "-",
		"\\", "-",
		":", "-",
		"*", "-",
		"?", "-",
		"\"", "'",
		"<", "-",
		">", "-",
		"|", "-",
		"\x00", "",
	)
	name = replacer.Replace(name)
	name = norm.NFC.String(name)
	name = strings.Join(strings.Fields(name), " ")
	return strings.TrimSpace(name)
}

func ConfidenceToString(c float64) string {
	switch {
	case c >= 0.7:
		return "high"
	case c >= 0.4:
		return "medium"
	default:
		return "low"
	}
}

func formatCausalMarkdown(chains []model.CausalLink) string {
	var b strings.Builder
	for _, c := range chains {
		if c.Mechanism != "" {
			b.WriteString(fmt.Sprintf("- **%s** → %s → **%s** *(%s)*\n", c.Cause, c.Mechanism, c.Effect, c.Relation))
		} else {
			b.WriteString(fmt.Sprintf("- **%s** → **%s** *(%s)*\n", c.Cause, c.Effect, c.Relation))
		}
	}
	return b.String()
}

func bulletList(items []string) string {
	var b strings.Builder
	for _, item := range items {
		b.WriteString("- ")
		b.WriteString(item)
		b.WriteString("\n")
	}
	return b.String()
}
