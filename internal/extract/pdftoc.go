package extract

import (
	"bytes"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

func getPageCount(path string) int {
	cmd := exec.Command("pdfinfo", path)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return 0
	}

	re := regexp.MustCompile(`Pages:\s+(\d+)`)
	match := re.FindStringSubmatch(stdout.String())
	if len(match) < 2 {
		return 0
	}

	n, _ := strconv.Atoi(match[1])
	return n
}

func extractPage(path string, pageNum int) string {
	cmd := exec.Command("pdftotext",
		"-f", strconv.Itoa(pageNum),
		"-l", strconv.Itoa(pageNum),
		path, "-")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Run()
	return stdout.String()
}

var (
	chapterPattern     = regexp.MustCompile(`(?i)(глава|chapter|раздел|section|часть|part|unit|module)\s+\d+`)
	dottedLinePattern  = regexp.MustCompile(`\.{3,}\s*\d+\s*$`)
	pageNumPattern     = regexp.MustCompile(`\s\d{1,4}\s*$`)
)

func isTOCPage(text string) bool {
	lines := strings.Split(text, "\n")
	if len(lines) < 5 {
		return false
	}

	structuralCount := 0
	pageNumCount := 0
	nonEmptyLines := 0
	totalLineLen := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		nonEmptyLines++
		totalLineLen += len(trimmed)

		if chapterPattern.MatchString(trimmed) && dottedLinePattern.MatchString(trimmed) {
			structuralCount++
		}

		if pageNumPattern.MatchString(trimmed) {
			pageNumCount++
		}
	}

	if nonEmptyLines == 0 {
		return false
	}

	signals := 0

	if float64(structuralCount)/float64(nonEmptyLines) > 0.2 {
		signals++
	}

	if float64(pageNumCount)/float64(nonEmptyLines) > 0.4 {
		signals++
	}

	avgLineLen := totalLineLen / nonEmptyLines
	if avgLineLen < 40 {
		signals++
	}

	return signals >= 2
}

func isBoilerplatePage(text string) bool {
	trimmed := strings.TrimSpace(text)

	if len(trimmed) < 50 {
		return true
	}

	if matched, _ := regexp.MatchString(`^\d+$`, trimmed); matched {
		return true
	}

	return false
}
