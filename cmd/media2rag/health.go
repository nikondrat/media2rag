package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"
)

var healthCmd = &cobra.Command{
	Use:   "health",
	Short: "Check LLM backend health and model availability",
	RunE: func(cmd *cobra.Command, args []string) error {
		backend := cfg.LLM.DefaultBackend
		model := cfg.LLM.Model
		ok := true

		fmt.Fprintf(os.Stderr, "backend: %s\n", backend)
		fmt.Fprintf(os.Stderr, "model:   %s\n\n", model)

		switch backend {
		case "lmstudio":
			url := cfg.LLM.LMStudioURL
			fmt.Fprintf(os.Stderr, "checking %s...\n", url)
			if err := checkLMStudio(url, model); err != nil {
				fmt.Fprintf(os.Stderr, "❌ %v\n", err)
				ok = false
			} else {
				fmt.Fprintf(os.Stderr, "✅ LMStudio responding, model loaded\n")
			}
		case "ollama":
			url := cfg.LLM.OllamaURL
			fmt.Fprintf(os.Stderr, "checking %s...\n", url)
			if err := checkOllama(url, model); err != nil {
				fmt.Fprintf(os.Stderr, "❌ %v\n", err)
				ok = false
			} else {
				fmt.Fprintf(os.Stderr, "✅ Ollama responding, model available\n")
			}
		case "openrouter":
			fmt.Fprintf(os.Stderr, "checking OpenRouter...\n")
			if cfg.LLM.OpenRouterKey == "" {
				fmt.Fprintf(os.Stderr, "❌ no OPENROUTER_API key set\n")
				ok = false
			} else {
				fmt.Fprintf(os.Stderr, "✅ API key configured\n")
			}
		}

		fmt.Fprintf(os.Stderr, "\n")
		if !ok {
			return fmt.Errorf("health check failed")
		}
		fmt.Fprintf(os.Stderr, "all checks passed\n")
		return nil
	},
}

func checkLMStudio(baseURL, model string) error {
	client := &http.Client{Timeout: 10 * time.Second}

	resp, err := client.Get(baseURL + "/v1/models")
	if err != nil {
		return fmt.Errorf("LMStudio not responding at %s: %w (is it running?)", baseURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("LMStudio returned status %d", resp.StatusCode)
	}

	var modelsResp struct {
		Data []struct {
			ID     string `json:"id"`
			Loaded bool   `json:"loaded"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		return fmt.Errorf("decode models: %w", err)
	}

	if len(modelsResp.Data) == 0 {
		return fmt.Errorf("no models loaded in LMStudio — load a model first")
	}

	found := false
	for _, m := range modelsResp.Data {
		if model == "" || m.ID == model {
			found = true
			break
		}
	}
	if !found {
		loaded := modelsResp.Data[0].ID
		return fmt.Errorf("model %q not loaded. Loaded: %s", model, loaded)
	}

	return nil
}

func checkOllama(baseURL, model string) error {
	client := &http.Client{Timeout: 10 * time.Second}

	resp, err := client.Get(baseURL + "/api/tags")
	if err != nil {
		return fmt.Errorf("Ollama not responding at %s: %w (is it running?)", baseURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Ollama returned status %d", resp.StatusCode)
	}

	var tagsResp struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tagsResp); err != nil {
		return fmt.Errorf("decode tags: %w", err)
	}

	if len(tagsResp.Models) == 0 {
		return fmt.Errorf("no models installed in Ollama")
	}

	for _, m := range tagsResp.Models {
		if m.Name == model {
			return nil
		}
	}

	names := make([]string, len(tagsResp.Models))
	for i, m := range tagsResp.Models {
		names[i] = m.Name
	}
	return fmt.Errorf("model %q not found. Available: %s", model, names)
}

func init() {
	rootCmd.AddCommand(healthCmd)
}
