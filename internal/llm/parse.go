package llm

import (
	"strings"

	"media2rag/internal/model"
)

func ParseOutput(text string) ([]model.TypedBlock, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, nil
	}

	var blocks []model.TypedBlock
	lines := strings.Split(text, "\n")

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if isBlockStart(line) {
			blockType, params := parseBlockHeader(line)
			i++
			var contentLines []string
			for i < len(lines) {
				if strings.TrimSpace(lines[i]) == "<" {
					break
				}
				if isBlockStart(lines[i]) {
					break
				}
				contentLines = append(contentLines, lines[i])
				i++
			}
			blocks = append(blocks, model.TypedBlock{
				Type:    blockType,
				Params:  params,
				Content: strings.TrimSpace(strings.Join(contentLines, "\n")),
			})
		}
	}

	if len(blocks) == 0 {
		blocks = append(blocks, model.TypedBlock{
			Type:    "text",
			Params:  nil,
			Content: text,
		})
	}

	return blocks, nil
}

func isBlockStart(line string) bool {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "> ") {
		return false
	}
	rest := strings.TrimSpace(trimmed[2:])
	if rest == "" {
		return false
	}
	return true
}

func parseBlockHeader(line string) (string, map[string]string) {
	trimmed := strings.TrimSpace(line)
	rest := strings.TrimSpace(trimmed[2:])

	colonIdx := strings.Index(rest, ": ")
	if colonIdx == -1 {
		return rest, nil
	}

	blockType := rest[:colonIdx]
	paramsStr := rest[colonIdx+2:]

	if paramsStr == "" {
		return blockType, nil
	}

	params := make(map[string]string)
	pairs := strings.Split(paramsStr, ", ")
	for _, pair := range pairs {
		if eqIdx := strings.Index(pair, "="); eqIdx != -1 {
			key := strings.TrimSpace(pair[:eqIdx])
			value := strings.TrimSpace(pair[eqIdx+1:])
			if key != "" {
				params[key] = value
			}
		}
	}

	if len(params) == 0 {
		return blockType, nil
	}

	return blockType, params
}
