from dataclasses import dataclass, field
from pathlib import Path
from typing import Optional


@dataclass
class Claim:
    text: str
    type: str  # argument | fact | framework | prediction
    confidence: str = "strong"  # strong | moderate | speculative


@dataclass
class DocumentMetadata:
    title: str
    source: str
    doc_type: str
    author: str = ""
    language: str = ""
    domains: list[str] = field(default_factory=list)
    core_thesis: str = ""
    mental_models: list[str] = field(default_factory=list)
    claims: list[Claim] = field(default_factory=list)
    takeaways: list[str] = field(default_factory=list)
    key_terms: list[str] = field(default_factory=list)
    summary: str = ""
    key_insights: list[str] = field(default_factory=list)
    topics: list[str] = field(default_factory=list)
    word_count: int = 0


@dataclass
class ExtractedContent:
    raw_text: str
    metadata: DocumentMetadata
    images: list[dict] = field(default_factory=list)  # [{"path": ..., "description": ...}]
    image_paths: list[Path] = field(default_factory=list)
    page_count: int = 0
    duration_seconds: float = 0.0


@dataclass
class RAGDocument:
    markdown: str
    metadata: DocumentMetadata
    chunks: list[str] = field(default_factory=list)

    def save(self, output_dir: Path, workspace_dir: Path | None = None) -> Path:
        if workspace_dir:
            output_subdir = workspace_dir / "output"
            output_subdir.mkdir(parents=True, exist_ok=True)
            filepath = output_subdir / "final.md"
            filepath.write_text(self.markdown, encoding="utf-8")
            return filepath
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
