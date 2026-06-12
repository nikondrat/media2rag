package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"

	"media2rag/internal/model"
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
	MaxTokens            int  `mapstructure:"max_tokens"`
	ChunkSize            int  `mapstructure:"chunk_size"`
	MaxConcurrency       int  `mapstructure:"max_concurrency"`
	MaxFileConcurrency   int  `mapstructure:"max_file_concurrency"`
	MaxTotalConcurrency  int  `mapstructure:"max_total_concurrency"`
	HolisticAnalysis     *bool `mapstructure:"holistic_analysis"`
}

type WorkspaceConfig struct {
	DataDir  string `mapstructure:"data_dir"`
	CacheDir string `mapstructure:"cache_dir"`
}

type Config struct {
	LLM       LLMConfig       `mapstructure:"llm"`
	Pipeline  PipelineConfig  `mapstructure:"pipeline"`
	Workspace WorkspaceConfig `mapstructure:"workspace"`
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
	}
}

func Load(configPath string) (*Config, error) {
	cfg := DefaultConfig()

	v := viper.New()

	defaultYAML := filepath.Join(os.Getenv("HOME"), ".media2rag", "config.yaml")
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

func (c *Config) Validate() error {
	if c.LLM.DefaultBackend != "ollama" && c.LLM.DefaultBackend != "openrouter" && c.LLM.DefaultBackend != "lmstudio" {
		return fmt.Errorf("%w: DefaultBackend must be \"ollama\", \"openrouter\", or \"lmstudio\", got %q",
			model.ErrConfigInvalid, c.LLM.DefaultBackend)
	}
	return nil
}
