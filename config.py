import os
from dataclasses import dataclass, field
from pathlib import Path

from dotenv import load_dotenv

load_dotenv()


@dataclass
class OllamaConfig:
    base_url: str = "http://localhost:11434"
    ctg_model: str = "qwen3.5:27b"
    vision_model: str = "qwen3.5:27b"
    timeout: int = 300


@dataclass
class OpenRouterConfig:
    api_key: str = ""
    base_url: str = "https://openrouter.ai/api/v1"
    default_model: str = "qwen/qwen-plus"
    vision_model: str = "qwen/qwen-vl-plus"
    timeout: int = 300


@dataclass
class WhisperConfig:
    model: str = "large-v3"
    device: str = "cpu"  # cpu, cuda, mps
    language: str = ""  # auto-detect


@dataclass
class MarkerConfig:
    batch_multiplier: int = 2
    max_pages: int = 0  # 0 = no limit
    langs: list[str] = field(default_factory=lambda: ["en", "ru"])


@dataclass
class EmbeddingConfig:
    model: str = "qwen3-embedding:0.6b"
    dimensions: int = 1024
    child_tokens: int = 256
    parent_tokens: int = 1024


@dataclass
class AppConfig:
    ollama: OllamaConfig = field(default_factory=OllamaConfig)
    openrouter: OpenRouterConfig = field(default_factory=OpenRouterConfig)
    whisper: WhisperConfig = field(default_factory=WhisperConfig)
    marker: MarkerConfig = field(default_factory=MarkerConfig)
    embedding: EmbeddingConfig = field(default_factory=EmbeddingConfig)
    output_dir: Path = field(default_factory=lambda: Path("output"))
    workspace_dir: Path | None = None
    llm_backend: str = "ollama"  # ollama or openrouter

    @classmethod
    def from_env(cls) -> "AppConfig":
        workspace_env = os.getenv("WORKSPACE")
        return cls(
            ollama=OllamaConfig(
                base_url=os.getenv("OLLAMA_BASE_URL", "http://localhost:11434"),
                ctg_model=os.getenv("OLLAMA_CTG_MODEL", "qwen3.5:27b"),
                vision_model=os.getenv("OLLAMA_VISION_MODEL", "qwen3.5:27b"),
            ),
            openrouter=OpenRouterConfig(
                api_key=os.getenv(
                    "OPENROUTER_API_KEY", os.getenv("OPENROUTER_API", "")
                ),
                default_model=os.getenv("OPENROUTER_MODEL", "qwen/qwen-plus"),
                vision_model=os.getenv("OPENROUTER_VISION_MODEL", "qwen/qwen-vl-plus"),
            ),
            whisper=WhisperConfig(
                model=os.getenv("WHISPER_MODEL", "large-v3"),
                device=os.getenv("WHISPER_DEVICE", "cpu"),
            ),
            embedding=EmbeddingConfig(
                model=os.getenv("EMBED_MODEL", "qwen3-embedding:0.6b"),
                dimensions=int(os.getenv("EMBED_DIMENSIONS", "1024")),
                child_tokens=int(os.getenv("CHUNK_CHILD_TOKENS", "256")),
                parent_tokens=int(os.getenv("CHUNK_PARENT_TOKENS", "1024")),
            ),
            output_dir=Path(os.getenv("OUTPUT_DIR", "output")),
            workspace_dir=Path(workspace_env) if workspace_env else None,
            llm_backend=os.getenv("LLM_BACKEND", "ollama"),
        )
