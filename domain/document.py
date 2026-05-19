from dataclasses import dataclass, field
from pathlib import Path
from typing import Optional


@dataclass
class DocumentMetadata:
    title: str
    source: str
    doc_type: str  # video, audio, pdf, epub, image
    author: str = ""
    topics: list[str] = field(default_factory=list)
    summary: str = ""
    key_insights: list[str] = field(default_factory=list)
    original_path: str = ""
    word_count: int = 0


@dataclass
class ExtractedContent:
    raw_text: str
    metadata: DocumentMetadata
    images: list[dict] = field(default_factory=list)  # [{"path": ..., "description": ...}]
    page_count: int = 0
    duration_seconds: float = 0.0


@dataclass
class RAGDocument:
    markdown: str
    metadata: DocumentMetadata
    chunks: list[str] = field(default_factory=list)

    def save(self, output_dir: Path) -> Path:
        output_dir.mkdir(parents=True, exist_ok=True)
        safe_name = _sanitize_filename(self.metadata.title) or "untitled"
        filepath = output_dir / f"{safe_name}.md"
        filepath.write_text(self.markdown, encoding="utf-8")
        return filepath


def _sanitize_filename(title: str) -> str:
    import re
    s = title.strip().replace("/", " ").replace("\\", " ").replace(":", " -")
    s = re.sub(r'[<>"|?*]', "", s)
    s = re.sub(r"\s+", " ", s)
    return s[:120].rstrip(" .")
