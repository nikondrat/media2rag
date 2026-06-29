package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Check and install required dependencies",
	RunE:  runSetup,
}

type dependency struct {
	Name     string
	Command  string
	Required bool
	Install  string
}

func getDependencies() []dependency {
	deps := []dependency{
		{Name: "npx", Command: "npx", Required: true, Install: "Install Node.js: https://nodejs.org"},
		{Name: "pdftotext", Command: "pdftotext", Required: true, Install: installCmd("poppler")},
		{Name: "pdfinfo", Command: "pdfinfo", Required: true, Install: installCmd("poppler")},
		{Name: "pdfimages", Command: "pdfimages", Required: false, Install: installCmd("poppler")},
		{Name: "ocrmypdf", Command: "ocrmypdf", Required: false, Install: "pip3 install ocrmypdf && brew install tesseract tesseract-lang"},
		{Name: "ffmpeg", Command: "ffmpeg", Required: false, Install: installCmd("ffmpeg")},
		{Name: "whisper", Command: "whisper", Required: false, Install: "pip install openai-whisper"},
	}
	return deps
}

func installCmd(tool string) string {
	switch runtime.GOOS {
	case "darwin":
		return fmt.Sprintf("brew install %s", tool)
	case "linux":
		return fmt.Sprintf("sudo apt-get install %s", tool)
	default:
		return fmt.Sprintf("install %s manually", tool)
	}
}

func runSetup(cmd *cobra.Command, args []string) error {
	deps := getDependencies()
	allOk := true

	fmt.Fprintln(os.Stderr, "Checking dependencies...")

	for _, dep := range deps {
		found := checkCommand(dep.Command)
		status := "✅"
		if found != nil {
			status = "❌"
			if dep.Required {
				allOk = false
			}
		}

		req := ""
		if !dep.Required {
			req = " (optional)"
		}

		fmt.Fprintf(os.Stderr, "%s %s%s\n", status, dep.Name, req)

		if found != nil {
			fmt.Fprintf(os.Stderr, "   Install: %s\n", dep.Install)
		}
	}

	if checkCommand("ocrmypdf") == nil {
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Checking OCR language data...")
		checkTesseractLang("eng")
		checkTesseractLang("rus")
	}

	fmt.Fprintln(os.Stderr)

	if !allOk {
		fmt.Fprintln(os.Stderr, "Some required dependencies are missing. Install them and run 'media2rag setup' again.")
		return fmt.Errorf("setup incomplete")
	}

	fmt.Fprintln(os.Stderr, "All required dependencies are installed!")
	return nil
}

func checkTesseractLang(lang string) {
	cmd := exec.Command("tesseract", "--list-langs")
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ tesseract language '%s': cannot check\n", lang)
		return
	}

	if strings.Contains(string(output), lang) {
		fmt.Fprintf(os.Stderr, "✅ tesseract language '%s'\n", lang)
	} else {
		fmt.Fprintf(os.Stderr, "❌ tesseract language '%s': not installed\n", lang)
		fmt.Fprintf(os.Stderr, "   Install: curl -L -o /opt/homebrew/share/tessdata/%s.traineddata https://github.com/tesseract-ocr/tessdata/raw/main/%s.traineddata\n", lang, lang)
	}
}

func checkCommand(name string) error {
	_, err := exec.LookPath(name)
	return err
}

func init() {
	rootCmd.AddCommand(setupCmd)
}
