package pipeline

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"media2rag/internal/model"
)

type assembleOpts struct {
	source     string
	docType    string
	author     string
	language   string
	coreThesis string
	domains    []string
}

func assemble(results []ChunkResult, cleaned string, opts assembleOpts) *model.RAGDocument {
	topicFreq := make(map[string]int)
	allClaims := make([]model.Claim, 0)
	allMental := make([]string, 0)
	allKeyTerms := make([]model.KeyTerm, 0)
	allTakeaways := make([]string, 0)
	perChunkThesis := ""

	for _, r := range results {
		for _, t := range r.Topics {
			topicFreq[t]++
		}
		allClaims = append(allClaims, r.Claims...)
		allMental = append(allMental, r.MentalModels...)
		allKeyTerms = append(allKeyTerms, r.KeyTerms...)
		allTakeaways = append(allTakeaways, r.Takeaways...)
		if r.CoreThesis != "" {
			perChunkThesis = r.CoreThesis
		}
	}

	type tc struct {
		topic string
		count int
	}
	var sorted []tc
	for t, c := range topicFreq {
		sorted = append(sorted, tc{t, c})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].count > sorted[j].count
	})

	var topics []string
	for _, t := range sorted {
		topics = append(topics, t.topic)
	}

	title := ""
	for _, r := range results {
		if r.Title != "" {
			title = r.Title
			break
		}
	}
	if title == "" && len(sorted) > 0 {
		title = sorted[0].topic
	}

	var summaries []string
	for _, r := range results {
		if r.Summary != "" {
			summaries = append(summaries, r.Summary)
		}
	}
	summary := strings.Join(summaries, " ")

	wc := wordCount(cleaned)

	coreThesis := opts.coreThesis
	if coreThesis == "" {
		coreThesis = perChunkThesis
	}

	domains := opts.domains

	claims := dedupClaims(allClaims)
	mentalModels := dedupStrings(allMental)
	keyTerms := dedupKeyTerms(allKeyTerms)
	takeaways := dedupStrings(allTakeaways)

	md := model.DocumentMetadata{
		Title:        title,
		Author:       opts.author,
		Source:       opts.source,
		DocType:      opts.docType,
		Language:     opts.language,
		Domains:      domains,
		CoreThesis:   coreThesis,
		MentalModels: mentalModels,
		Claims:       claims,
		Takeaways:    takeaways,
		KeyTerms:     keyTerms,
		Summary:      summary,
		WordCount:    wc,
		Topics:       topics,
	}

	frontmatter := buildYAMLFrontmatter(md)

	var sb strings.Builder
	sb.WriteString("---\n")
	sb.WriteString(frontmatter)
	sb.WriteString("---\n\n")
	sb.WriteString(cleaned)

	return &model.RAGDocument{
		Markdown: sb.String(),
		Metadata: md,
	}
}

func buildYAMLFrontmatter(md model.DocumentMetadata) string {
	var sb strings.Builder
	now := time.Now()

	writeYAMLField(&sb, "title", md.Title)
	writeYAMLField(&sb, "source", md.Source)
	writeYAMLField(&sb, "type", md.DocType)
	writeYAMLField(&sb, "author", md.Author)
	writeYAMLField(&sb, "language", md.Language)
	sb.WriteString(fmt.Sprintf("created_at: %s\n", now.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("word_count: %d\n", md.WordCount))

	writeYAMLList(&sb, "domains", mapSlice(md.Domains))
	writeYAMLField(&sb, "core_thesis", md.CoreThesis)
	writeYAMLList(&sb, "mental_models", mapSlice(md.MentalModels))
	writeYAMLClaimList(&sb, md.Claims)
	writeYAMLList(&sb, "takeaways", mapSlice(md.Takeaways))
	writeYAMLKeyTermList(&sb, md.KeyTerms)
	writeYAMLField(&sb, "summary", md.Summary)
	writeYAMLList(&sb, "topics", mapSlice(md.Topics))

	return sb.String()
}

func writeYAMLField(sb *strings.Builder, key, value string) {
	if value == "" {
		return
	}
	sb.WriteString(fmt.Sprintf("%s: %s\n", key, yamlStr(value)))
}

func writeYAMLList(sb *strings.Builder, key string, items []string) {
	if len(items) == 0 {
		return
	}
	sb.WriteString(fmt.Sprintf("%s:\n", key))
	for _, item := range items {
		sb.WriteString(fmt.Sprintf("  - %s\n", yamlStr(item)))
	}
}

func writeYAMLClaimList(sb *strings.Builder, claims []model.Claim) {
	if len(claims) == 0 {
		return
	}
	sb.WriteString("claims:\n")
	for _, c := range claims {
		sb.WriteString(fmt.Sprintf("  - statement: %s\n", yamlStr(c.Statement)))
		sb.WriteString(fmt.Sprintf("    confidence: %.2f\n", c.Confidence))
		sb.WriteString(fmt.Sprintf("    source: %s\n", yamlStr(c.Source)))
	}
}

func writeYAMLKeyTermList(sb *strings.Builder, terms []model.KeyTerm) {
	if len(terms) == 0 {
		return
	}
	sb.WriteString("key_terms:\n")
	for _, kt := range terms {
		sb.WriteString(fmt.Sprintf("  - term: %s\n", yamlStr(kt.Term)))
		sb.WriteString(fmt.Sprintf("    definition: %s\n", yamlStr(kt.Definition)))
	}
}

func yamlStr(s string) string {
	if s == "" {
		return "\"\""
	}
	if strings.ContainsAny(s, ":\"#{}[]&*!|>'%@`") || strings.HasPrefix(s, "-") {
		return fmt.Sprintf("%q", s)
	}
	return s
}

func wordCount(s string) int {
	if s == "" {
		return 0
	}
	count := 1
	inWord := false
	for _, c := range s {
		if c == ' ' || c == '\n' || c == '\t' {
			inWord = false
		} else {
			if !inWord {
				count++
				inWord = true
			}
		}
	}
	return count - 1
}

func dedupClaims(claims []model.Claim) []model.Claim {
	seen := make(map[string]bool)
	var result []model.Claim
	for _, c := range claims {
		key := strings.TrimSpace(strings.ToLower(c.Statement))
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, c)
	}
	return result
}

func dedupStrings(items []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, item := range items {
		key := strings.TrimSpace(strings.ToLower(item))
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, item)
	}
	return result
}

func dedupKeyTerms(terms []model.KeyTerm) []model.KeyTerm {
	seen := make(map[string]bool)
	var result []model.KeyTerm
	for _, kt := range terms {
		key := strings.TrimSpace(strings.ToLower(kt.Term))
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, kt)
	}
	return result
}

func mapSlice(items []string) []string {
	if items == nil {
		return []string{}
	}
	return items
}
