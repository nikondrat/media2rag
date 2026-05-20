import subprocess
import tempfile
from pathlib import Path

import whisper

from config import WhisperConfig
from domain.document import ExtractedContent, DocumentMetadata
from extractors.base import BaseExtractor


class AudioExtractor(BaseExtractor):
    SUPPORTED_EXTENSIONS = {".mp3", ".wav", ".m4a", ".flac", ".ogg", ".aac"}

    def __init__(self, cfg: WhisperConfig):
        self._cfg = cfg
        self._model = None

    def _load_model(self):
        if self._model is None:
            self._model = whisper.load_model(self._cfg.model, device=self._cfg.device)
        return self._model

    def extract(self, source: Path | str, workspace_dir: Path | None = None) -> ExtractedContent:
        source_path = Path(source) if isinstance(source, str) else source
        if not source_path.exists():
            raise FileNotFoundError(f"File not found: {source_path}")

        model = self._load_model()
        lang = self._cfg.language or None
        result = model.transcribe(str(source_path), language=lang)
        raw_text = result.get("text", "").strip()

        duration = self._get_duration(source_path)

        return ExtractedContent(
            raw_text=raw_text,
            metadata=DocumentMetadata(
                title=source_path.stem,
                source=str(source_path),
                doc_type="audio",
                language=self._cfg.language or "",
                word_count=len(raw_text.split()),
            ),
            duration_seconds=duration,
        )

    def supports(self, source: Path | str) -> bool:
        path = Path(source) if isinstance(source, str) else source
        return path.suffix.lower() in self.SUPPORTED_EXTENSIONS

    def _get_duration(self, path: Path) -> float:
        try:
            result = subprocess.run(
                ["ffprobe", "-v", "error", "-show_entries", "format=duration",
                 "-of", "default=noprint_wrappers=1:nokey=1", str(path)],
                capture_output=True, text=True, timeout=10,
            )
            return float(result.stdout.strip())
        except Exception:
            return 0.0
