package pipeline

import "strings"

const (
	maxConsecutiveRepeats = 10
	ngramSize             = 3
	maxNgramRepeats       = 5
)

func detectRepetition(content string) bool {
	words := strings.Fields(content)
	if len(words) < 10 {
		return false
	}

	if detectConsecutiveRepeats(words) {
		return true
	}

	if detectNgramRepeats(words) {
		return true
	}

	return false
}

func detectConsecutiveRepeats(words []string) bool {
	current := 1
	for i := 1; i < len(words); i++ {
		if strings.EqualFold(words[i], words[i-1]) {
			current++
			if current > maxConsecutiveRepeats {
				return true
			}
		} else {
			current = 1
		}
	}
	return false
}

func detectNgramRepeats(words []string) bool {
	if len(words) < ngramSize {
		return false
	}

	ngramCounts := make(map[string]int)
	for i := 0; i <= len(words)-ngramSize; i++ {
		ngram := strings.ToLower(strings.Join(words[i:i+ngramSize], " "))
		ngramCounts[ngram]++
		if ngramCounts[ngram] > maxNgramRepeats {
			return true
		}
	}
	return false
}
