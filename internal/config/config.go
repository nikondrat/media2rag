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
	OpenRouterKey  string `mapstructure:"openrouter_key"`
	OpenRouterURL  string `mapstructure:"openrouter_url"`
	EmbedModel     string `mapstructure:"embed_model"`
	Model          string `mapstructure:"model"`
	Timeout        int    `mapstructure:"timeout"`
}

type PipelineConfig struct {
	MaxTokens    int `mapstructure:"max_tokens"`
	ChunkSize    int `mapstructure:"chunk_size"`
	ChunkOverlap int `mapstructure:"chunk_overlap"`
}

type WorkspaceConfig struct {
	DataDir string `mapstructure:"data_dir"`
	CacheDir string `mapstructure:"cache_dir"`
}

type ServerConfig struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
}

type Config struct {
	LLM       LLMConfig       `mapstructure:"llm"`
	Pipeline  PipelineConfig  `mapstructure:"pipeline"`
	Workspace WorkspaceConfig `mapstructure:"workspace"`
	Server    ServerConfig    `mapstructure:"server"`
}

func DefaultConfig() Config {
	return Config{
		LLM: LLMConfig{
			DefaultBackend: "ollama",
			OllamaURL:      "http://localhost:11434",
			Model:          "llama3.2",
			EmbedModel:     "nomic-embed-text",
			Timeout:        30,
		},
		Pipeline: PipelineConfig{
			MaxTokens:    4096,
			ChunkSize:    2000,
			ChunkOverlap: 200,
		},
		Server: ServerConfig{
			Host: "localhost",
			Port: 8542,
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

	envOverrides := map[string]*string{
		"MEDIA2RAG_LLM_DEFAULT_BACKEND": &cfg.LLM.DefaultBackend,
		"MEDIA2RAG_LLM_OLLAMA_URL":      &cfg.LLM.OllamaURL,
		"MEDIA2RAG_LLM_OPENROUTER_KEY":  &cfg.LLM.OpenRouterKey,
		"MEDIA2RAG_LLM_MODEL":           &cfg.LLM.Model,
		"MEDIA2RAG_LLM_EMBED_MODEL":     &cfg.LLM.EmbedModel,
		"MEDIA2RAG_LLM_TIMEOUT":         nil,
		"MEDIA2RAG_SERVER_HOST":         &cfg.Server.Host,
		"MEDIA2RAG_SERVER_PORT":         nil,
	}

	for envKey, ptr := range envOverrides {
		if val, ok := os.LookupEnv(envKey); ok {
			if ptr != nil {
				*ptr = val
			}
		}
	}

	if portStr, ok := os.LookupEnv("MEDIA2RAG_SERVER_PORT"); ok {
		fmt.Sscanf(portStr, "%d", &cfg.Server.Port)
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
	if c.LLM.DefaultBackend != "ollama" && c.LLM.DefaultBackend != "openrouter" {
		return fmt.Errorf("%w: DefaultBackend must be \"ollama\" or \"openrouter\", got %q",
			model.ErrConfigInvalid, c.LLM.DefaultBackend)
	}
	return nil
}
