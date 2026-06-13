package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

type LLMConfig struct {
	DefaultBackend string `mapstructure:"default_backend"`
	OllamaURL      string `mapstructure:"ollama_url"`
	LMStudioURL    string `mapstructure:"lmstudio_url"`
	OpenRouterKey  string `mapstructure:"openrouter_key"`
	OpenRouterURL  string `mapstructure:"openrouter_url"`
	Model          string `mapstructure:"model"`
	Timeout        int    `mapstructure:"timeout"`
}

type PipelineConfig struct {
	MaxTokens           int  `mapstructure:"max_tokens"`
	ChunkSize           int  `mapstructure:"chunk_size"`
	MaxConcurrency      int  `mapstructure:"max_concurrency"`
	MaxFileConcurrency  int  `mapstructure:"max_file_concurrency"`
	MaxTotalConcurrency int  `mapstructure:"max_total_concurrency"`
	HolisticAnalysis    *bool `mapstructure:"holistic_analysis"`
}

type WorkspaceConfig struct {
	DataDir  string `mapstructure:"data_dir"`
	CacheDir string `mapstructure:"cache_dir"`
}

type Provider struct {
	Type string `yaml:"type" mapstructure:"type"`
	URL  string `yaml:"url" mapstructure:"url"`
	Key  string `yaml:"key,omitempty" mapstructure:"key"`
}

type Profile struct {
	Backend  string `yaml:"backend" mapstructure:"backend"`
	Provider string `yaml:"provider,omitempty" mapstructure:"provider"`
	Model    string `yaml:"model" mapstructure:"model"`
	Output   string `yaml:"output,omitempty" mapstructure:"output"`
	FinalDir string `yaml:"final_dir,omitempty" mapstructure:"final_dir"`
	Timeout  int    `yaml:"timeout,omitempty" mapstructure:"timeout"`
}

type LastUsed struct {
	Source   string `yaml:"source,omitempty"`
	Output   string `yaml:"output,omitempty"`
	FinalDir string `yaml:"final_dir,omitempty"`
}

type Defaults struct {
	OutputDir string `yaml:"output_dir,omitempty" mapstructure:"output_dir"`
	FinalDir  string `yaml:"final_dir,omitempty" mapstructure:"final_dir"`
}

type Config struct {
	LLM           LLMConfig            `yaml:"llm" mapstructure:"llm"`
	Pipeline      PipelineConfig       `yaml:"pipeline" mapstructure:"pipeline"`
	Workspace     WorkspaceConfig      `yaml:"workspace" mapstructure:"workspace"`
	Providers     map[string]Provider  `yaml:"providers,omitempty"`
	Profiles      map[string]Profile   `yaml:"profiles,omitempty"`
	ActiveProfile string               `yaml:"active_profile,omitempty"`
	Defaults      Defaults             `yaml:"defaults,omitempty"`
	LastUsed      LastUsed             `yaml:"last_used,omitempty"`
}

func DefaultConfig() Config {
	return Config{
		LLM: LLMConfig{
			DefaultBackend: "lmstudio",
			OllamaURL:      "http://localhost:11434",
			LMStudioURL:    "http://localhost:1234",
			Model:          "",
			Timeout:        600,
		},
		Pipeline: PipelineConfig{
			MaxTokens:    4096,
			ChunkSize:    1500,
		},
		Providers: map[string]Provider{
			"lmstudio":   {Type: "lmstudio", URL: "http://localhost:1234"},
			"ollama":     {Type: "ollama", URL: "http://localhost:11434"},
			"openrouter": {Type: "openrouter", URL: "https://openrouter.ai/api"},
		},
	}
}

func ConfigPath() string {
	return filepath.Join(os.Getenv("HOME"), ".media2rag", "config.yaml")
}

func Load(configPath string) (*Config, error) {
	cfg := DefaultConfig()

	v := viper.New()

	defaultYAML := ConfigPath()
	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		v.SetConfigFile(defaultYAML)
	}

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			if configPath != "" {
				return nil, fmt.Errorf("failed to read config: %w", err)
			}
		}
	}

	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	godotenv.Load()

	if v := os.Getenv("OPENROUTER_API"); v != "" {
		cfg.LLM.OpenRouterKey = v
	}

	envOverrides := map[string]*string{
		"MEDIA2RAG_LLM_DEFAULT_BACKEND": &cfg.LLM.DefaultBackend,
		"MEDIA2RAG_LLM_OLLAMA_URL":      &cfg.LLM.OllamaURL,
		"MEDIA2RAG_LLM_OPENROUTER_URL":  &cfg.LLM.OpenRouterURL,
		"MEDIA2RAG_LLM_OPENROUTER_KEY":  &cfg.LLM.OpenRouterKey,
		"MEDIA2RAG_LLM_MODEL":           &cfg.LLM.Model,
		"MEDIA2RAG_LLM_TIMEOUT":         nil,
	}

	for envKey, ptr := range envOverrides {
		if val, ok := os.LookupEnv(envKey); ok {
			if ptr != nil {
				*ptr = val
			}
		}
	}

	if timeoutStr, ok := os.LookupEnv("MEDIA2RAG_LLM_TIMEOUT"); ok {
		fmt.Sscanf(timeoutStr, "%d", &cfg.LLM.Timeout)
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (c *Config) Save() error {
	path := ConfigPath()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	return os.WriteFile(path, data, 0644)
}

func (c *Config) Validate() error {
	if c.LLM.DefaultBackend != "ollama" && c.LLM.DefaultBackend != "openrouter" && c.LLM.DefaultBackend != "lmstudio" && c.LLM.DefaultBackend != "openai-compatible" {
		return fmt.Errorf("DefaultBackend must be \"ollama\", \"openrouter\", \"lmstudio\", or \"openai-compatible\", got %q",
			c.LLM.DefaultBackend)
	}
	return nil
}

func (c *Config) ActiveProfileConfig() *Profile {
	if c.ActiveProfile == "" {
		return nil
	}
	p, ok := c.Profiles[c.ActiveProfile]
	if !ok {
		return nil
	}
	return &p
}

func (c *Config) GetProvider(name string) *Provider {
	p, ok := c.Providers[name]
	if !ok {
		return nil
	}
	return &p
}

func (c *Config) ResolveBackend() string {
	if p := c.ActiveProfileConfig(); p != nil && p.Backend != "" {
		return p.Backend
	}
	return c.LLM.DefaultBackend
}

func (c *Config) ResolveModel() string {
	if p := c.ActiveProfileConfig(); p != nil && p.Model != "" {
		return p.Model
	}
	return c.LLM.Model
}

func (c *Config) ResolveOutput() string {
	if p := c.ActiveProfileConfig(); p != nil && p.Output != "" {
		return p.Output
	}
	return c.Defaults.OutputDir
}

func (c *Config) ResolveFinalDir() string {
	if p := c.ActiveProfileConfig(); p != nil && p.FinalDir != "" {
		return p.FinalDir
	}
	return c.Defaults.FinalDir
}
