package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"media2rag/internal/config"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Configure media2rag settings",
	Long: `Interactive configuration wizard for media2rag.

Run without arguments to start the interactive setup.
Use subcommands for specific operations.`,
	RunE: runConfigWizard,
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration",
	RunE:  runConfigShow,
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration value",
	Args:  cobra.ExactArgs(2),
	RunE:  runConfigSet,
}

var configGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Get a configuration value",
	Args:  cobra.ExactArgs(1),
	RunE:  runConfigGet,
}

var configResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset configuration to defaults",
	RunE:  runConfigReset,
}

var configUseCmd = &cobra.Command{
	Use:   "use <provider>",
	Short: "Quick switch to a provider",
	Args:  cobra.ExactArgs(1),
	RunE:  runConfigUse,
}

var configProfileCmd = &cobra.Command{
	Use:   "profile",
	Short: "Manage configuration profiles",
}

var configProfileListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all profiles",
	RunE:  runProfileList,
}

var configProfileCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new profile",
	Args:  cobra.ExactArgs(1),
	RunE:  runProfileCreate,
}

var configProfileUseCmd = &cobra.Command{
	Use:   "use <name>",
	Short: "Switch to a profile",
	Args:  cobra.ExactArgs(1),
	RunE:  runProfileUse,
}

var configProfileDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a profile",
	Args:  cobra.ExactArgs(1),
	RunE:  runProfileDelete,
}

var (
	reader = bufio.NewReader(os.Stdin)
)

func readInput(prompt, defaultVal string) string {
	fmt.Printf("? %s (%s): ", prompt, defaultVal)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if input == "" {
		return defaultVal
	}
	return input
}

func readInputRequired(prompt string) string {
	for {
		fmt.Printf("? %s: ", prompt)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input != "" {
			return input
		}
		fmt.Println("  This field is required")
	}
}

func readInt(prompt string, defaultVal int) int {
	val := readInput(prompt, strconv.Itoa(defaultVal))
	i, err := strconv.Atoi(val)
	if err != nil {
		return defaultVal
	}
	return i
}

func printSuccess(msg string) {
	fmt.Printf("\n  ✅ %s\n", msg)
}

func printError(msg string) {
	fmt.Printf("\n  ❌ %s\n", msg)
}

func runConfigWizard(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load("")
	if err != nil {
		defaultCfg := config.DefaultConfig()
		cfg = &defaultCfg
	}

	fmt.Println("\n  media2rag Configuration Wizard")

	// Select provider
	providerNames := getProviderNames(cfg)
	providerIdx := selectFromList("Provider", providerNames, 0)
	providerName := providerNames[providerIdx]
	provider := cfg.Providers[providerName]

	fmt.Printf("  Selected: %s (%s)\n\n", providerName, provider.URL)

	// Fetch models
	fmt.Println("  Fetching models...")
	models, err := config.FetchModels(provider)
	if err != nil {
		fmt.Printf("  ⚠️  Could not fetch models: %v\n", err)
		fmt.Println("  You can set the model manually later with: media2rag config set model <name>")
	} else if len(models) > 0 {
		modelNames := make([]string, len(models))
		for i, m := range models {
			modelNames[i] = m.Name
		}
		modelIdx := selectFromList("Model", modelNames, 0)
		cfg.LLM.Model = models[modelIdx].ID
		fmt.Println()
	} else {
		fmt.Println("  No models found. Set manually with: media2rag config set model <name>")
	}

	// Set backend
	cfg.LLM.DefaultBackend = provider.Type

	// Output directories
	home := os.Getenv("HOME")
	cfg.Defaults.OutputDir = readInput("Default output dir", cfg.Defaults.OutputDir)
	if cfg.Defaults.OutputDir == "" {
		cfg.Defaults.OutputDir = home + "/Documents/media2rag"
	}
	cfg.Defaults.FinalDir = readInput("Default final dir", cfg.Defaults.FinalDir)
	if cfg.Defaults.FinalDir == "" {
		cfg.Defaults.FinalDir = home + "/Documents"
	}

	// Pipeline settings
	fmt.Println()
	cfg.Pipeline.ChunkSize = readInt("Chunk size", cfg.Pipeline.ChunkSize)
	cfg.Pipeline.MaxConcurrency = readInt("Max concurrency", cfg.Pipeline.MaxConcurrency)

	// Save
	if err := cfg.Save(); err != nil {
		printError(fmt.Sprintf("Save failed: %v", err))
		return err
	}

	printSuccess(fmt.Sprintf("Config saved to %s", config.ConfigPath()))
	return nil
}

func runConfigShow(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load("")
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	fmt.Println("\n  Current Configuration")

	if cfg.ActiveProfile != "" {
		fmt.Printf("  Active Profile: %s\n", cfg.ActiveProfile)
	}

	fmt.Printf("  Backend:   %s\n", cfg.ResolveBackend())
	fmt.Printf("  Model:     %s\n", cfg.ResolveModel())
	fmt.Printf("  Output:    %s\n", cfg.ResolveOutput())
	fmt.Printf("  Final Dir: %s\n", cfg.ResolveFinalDir())
	fmt.Printf("  Timeout:   %d\n", cfg.LLM.Timeout)
	fmt.Println()

	if len(cfg.Profiles) > 0 {
		fmt.Println("  Profiles:")
		for name, p := range cfg.Profiles {
			marker := "  "
			if name == cfg.ActiveProfile {
				marker = "* "
			}
			fmt.Printf("    %s%s (%s / %s)\n", marker, name, p.Backend, p.Model)
		}
		fmt.Println()
	}

	if len(cfg.Providers) > 0 {
		fmt.Println("  Providers:")
		for name, p := range cfg.Providers {
			fmt.Printf("    %s: %s (%s)\n", name, p.Type, p.URL)
		}
	}

	if cfg.LastUsed.Source != "" {
		fmt.Printf("\n  Last Used:\n")
		fmt.Printf("    Source:   %s\n", cfg.LastUsed.Source)
		fmt.Printf("    Output:   %s\n", cfg.LastUsed.Output)
		fmt.Printf("    Final:    %s\n", cfg.LastUsed.FinalDir)
	}

	return nil
}

func runConfigSet(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load("")
	if err != nil {
		defaultCfg := config.DefaultConfig()
		cfg = &defaultCfg
	}

	key, value := args[0], args[1]

	switch key {
	case "backend":
		cfg.LLM.DefaultBackend = value
	case "model":
		cfg.LLM.Model = value
	case "output":
		cfg.Defaults.OutputDir = value
	case "final-dir":
		cfg.Defaults.FinalDir = value
	case "timeout":
		if v, err := strconv.Atoi(value); err == nil {
			cfg.LLM.Timeout = v
		}
	case "chunk-size":
		if v, err := strconv.Atoi(value); err == nil {
			cfg.Pipeline.ChunkSize = v
		}
	case "concurrency":
		if v, err := strconv.Atoi(value); err == nil {
			cfg.Pipeline.MaxConcurrency = v
		}
	case "lmstudio-url":
		if p, ok := cfg.Providers["lmstudio"]; ok {
			p.URL = value
			cfg.Providers["lmstudio"] = p
		}
	case "ollama-url":
		if p, ok := cfg.Providers["ollama"]; ok {
			p.URL = value
			cfg.Providers["ollama"] = p
		}
	case "openrouter-key":
		cfg.LLM.OpenRouterKey = value
	default:
		return fmt.Errorf("unknown key: %s", key)
	}

	if err := cfg.Save(); err != nil {
		return err
	}

	printSuccess(fmt.Sprintf("Set %s = %s", key, value))
	return nil
}

func runConfigGet(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load("")
	if err != nil {
		return err
	}

	switch args[0] {
	case "backend":
		fmt.Println(cfg.ResolveBackend())
	case "model":
		fmt.Println(cfg.ResolveModel())
	case "output":
		fmt.Println(cfg.ResolveOutput())
	case "final-dir":
		fmt.Println(cfg.ResolveFinalDir())
	case "timeout":
		fmt.Println(cfg.LLM.Timeout)
	default:
		return fmt.Errorf("unknown key: %s", args[0])
	}

	return nil
}

func runConfigReset(cmd *cobra.Command, args []string) error {
	cfg := config.DefaultConfig()
	if err := cfg.Save(); err != nil {
		return err
	}
	printSuccess("Config reset to defaults")
	return nil
}

func runConfigUse(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load("")
	if err != nil {
		return err
	}

	providerName := args[0]
	provider, ok := cfg.Providers[providerName]
	if !ok {
		return fmt.Errorf("unknown provider: %s (available: %s)", providerName, getProviderNamesStr(cfg))
	}

	cfg.LLM.DefaultBackend = provider.Type
	if provider.Key != "" {
		cfg.LLM.OpenRouterKey = provider.Key
	}

	if err := cfg.Save(); err != nil {
		return err
	}

	printSuccess(fmt.Sprintf("Switched to %s", providerName))
	return nil
}

func runProfileList(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load("")
	if err != nil {
		return err
	}

	if len(cfg.Profiles) == 0 {
		fmt.Println("\n  No profiles configured")
		return nil
	}

	fmt.Println("\n  Profiles:")
	for name, p := range cfg.Profiles {
		marker := "  "
		if name == cfg.ActiveProfile {
			marker = "* "
		}
		fmt.Printf("    %s%s\n", marker, name)
		fmt.Printf("      Backend: %s\n", p.Backend)
		fmt.Printf("      Model:   %s\n", p.Model)
		if p.Output != "" {
			fmt.Printf("      Output:  %s\n", p.Output)
		}
	}

	return nil
}

func runProfileCreate(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load("")
	if err != nil {
		defaultCfg := config.DefaultConfig()
		cfg = &defaultCfg
	}

	name := args[0]

	if cfg.Profiles == nil {
		cfg.Profiles = make(map[string]config.Profile)
	}

	if _, exists := cfg.Profiles[name]; exists {
		return fmt.Errorf("profile %s already exists", name)
	}

	// Select provider
	providerNames := getProviderNames(cfg)
	providerIdx := selectFromList("Provider", providerNames, 0)
	providerName := providerNames[providerIdx]
	provider := cfg.Providers[providerName]

	// Fetch models
	fmt.Println("  Fetching models...")
	models, err := config.FetchModels(provider)
	var model string
	if err != nil {
		model = readInputRequired("Model name")
	} else if len(models) > 0 {
		modelNames := make([]string, len(models))
		for i, m := range models {
			modelNames[i] = m.Name
		}
		modelIdx := selectFromList("Model", modelNames, 0)
		model = models[modelIdx].ID
	} else {
		model = readInputRequired("Model name")
	}

	output := readInput("Default output dir", cfg.Defaults.OutputDir)
	finalDir := readInput("Default final dir", cfg.Defaults.FinalDir)

	cfg.Profiles[name] = config.Profile{
		Backend:  provider.Type,
		Provider: providerName,
		Model:    model,
		Output:   output,
		FinalDir: finalDir,
	}

	if err := cfg.Save(); err != nil {
		return err
	}

	printSuccess(fmt.Sprintf("Profile %s created", name))
	return nil
}

func runProfileUse(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load("")
	if err != nil {
		return err
	}

	name := args[0]
	if _, ok := cfg.Profiles[name]; !ok {
		return fmt.Errorf("profile %s not found", name)
	}

	cfg.ActiveProfile = name
	if err := cfg.Save(); err != nil {
		return err
	}

	printSuccess(fmt.Sprintf("Switched to profile %s", name))
	return nil
}

func runProfileDelete(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load("")
	if err != nil {
		return err
	}

	name := args[0]
	if _, ok := cfg.Profiles[name]; !ok {
		return fmt.Errorf("profile %s not found", name)
	}

	delete(cfg.Profiles, name)
	if cfg.ActiveProfile == name {
		cfg.ActiveProfile = ""
	}

	if err := cfg.Save(); err != nil {
		return err
	}

	printSuccess(fmt.Sprintf("Profile %s deleted", name))
	return nil
}

func getProviderNames(cfg *config.Config) []string {
	var names []string
	for name := range cfg.Providers {
		names = append(names, name)
	}
	return names
}

func getProviderNamesStr(cfg *config.Config) string {
	return strings.Join(getProviderNames(cfg), ", ")
}

func selectFromList(prompt string, options []string, defaultIdx int) int {
	fmt.Printf("? %s:\n", prompt)
	for i, opt := range options {
		marker := "  "
		if i == defaultIdx {
			marker = "> "
		}
		fmt.Printf("    %s%s\n", marker, opt)
	}

	for {
		fmt.Print("  Enter number (or press Enter for default): ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		if input == "" {
			return defaultIdx
		}

		if idx, err := strconv.Atoi(input); err == nil && idx >= 0 && idx < len(options) {
			return idx
		}

		fmt.Println("  Invalid selection, try again")
	}
}

func init() {
	configProfileCmd.AddCommand(configProfileListCmd)
	configProfileCmd.AddCommand(configProfileCreateCmd)
	configProfileCmd.AddCommand(configProfileUseCmd)
	configProfileCmd.AddCommand(configProfileDeleteCmd)

	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configResetCmd)
	configCmd.AddCommand(configUseCmd)
	configCmd.AddCommand(configProfileCmd)

	rootCmd.AddCommand(configCmd)
}
