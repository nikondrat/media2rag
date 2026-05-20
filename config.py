import os
from dataclasses import dataclass, field
from pathlib import Path

from dotenv import load_dotenv

load_dotenv()


@dataclass
class OllamaConfig:
    base_url: str = "http://localhost:11434"
    ctg_model: str = "gemma4:26b"
    vision_model: str = "gemma4:latest"
    timeout: int = 300


@dataclass
class OpenRouterConfig:
    api_key: str = ""
    base_url: str = "https://openrouter.ai/api/v1"
    default_model: str = "qwen/qwen-plus"
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
class AppConfig:
    ollama: OllamaConfig = field(default_factory=OllamaConfig)
    openrouter: OpenRouterConfig = field(default_factory=OpenRouterConfig)
    whisper: WhisperConfig = field(default_factory=WhisperConfig)
    marker: MarkerConfig = field(default_factory=MarkerConfig)
    output_dir: Path = field(default_factory=lambda: Path("output"))
    workspace_dir: Path | None = None
    llm_backend: str = "ollama"  # ollama or openrouter

    @classmethod
    def from_env(cls) -> "AppConfig":
        workspace_env = os.getenv("WORKSPACE")
        return cls(
            ollama=OllamaConfig(
                base_url=os.getenv("OLLAMA_BASE_URL", "http://localhost:11434"),
                ctg_model=os.getenv("OLLAMA_CTG_MODEL", "gemma4:26b"),
                vision_model=os.getenv("OLLAMA_VISION_MODEL", "gemma4:latest"),
            ),
            openrouter=OpenRouterConfig(
                api_key=os.getenv(
                    "OPENROUTER_API_KEY", os.getenv("OPENROUTER_API", "")
                ),
                default_model=os.getenv("OPENROUTER_MODEL", "qwen/qwen-plus"),
            ),
            whisper=WhisperConfig(
                model=os.getenv("WHISPER_MODEL", "large-v3"),
                device=os.getenv("WHISPER_DEVICE", "cpu"),
            ),
            output_dir=Path(os.getenv("OUTPUT_DIR", "output")),
            workspace_dir=Path(workspace_env) if workspace_env else None,
            llm_backend=os.getenv("LLM_BACKEND", "ollama"),
        )
