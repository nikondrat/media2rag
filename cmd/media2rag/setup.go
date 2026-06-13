package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"

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
		{Name: "ffmpeg", Command: "ffmpeg", Required: false, Install: installCmd("ffmpeg")},
		{Name: "whisper", Command: "whisper", Required: false, Install: "pip install openai-whisper"},
		{Name: "pdftotext", Command: "pdftotext", Required: false, Install: installCmd("poppler")},
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
			allOk = false
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

	fmt.Fprintln(os.Stderr)

	if !allOk {
		fmt.Fprintln(os.Stderr, "Some dependencies are missing. Install them and run 'media2rag setup' again.")
		return fmt.Errorf("setup incomplete")
	}

	fmt.Fprintln(os.Stderr, "All dependencies are installed!")
	return nil
}

func checkCommand(name string) error {
	_, err := exec.LookPath(name)
	return err
}

func init() {
	rootCmd.AddCommand(setupCmd)
}
