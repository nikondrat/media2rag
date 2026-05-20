import re
from pathlib import Path

from domain.document import ExtractedContent, DocumentMetadata
from extractors.base import BaseExtractor


class MarkdownExtractor(BaseExtractor):
    def extract(self, source: Path | str, workspace_dir: Path | None = None) -> ExtractedContent:
        source_path = Path(source) if isinstance(source, str) else source
        if not source_path.exists():
            raise FileNotFoundError(f"File not found: {source_path}")

        raw_text = source_path.read_text(encoding="utf-8")
        metadata = self._parse_frontmatter(raw_text, source_path)
        clean_text = self._strip_frontmatter(raw_text)

        return ExtractedContent(
            raw_text=clean_text,
            metadata=metadata,
        )

    def supports(self, source: Path | str) -> bool:
        path = Path(source) if isinstance(source, str) else source
        return path.suffix.lower() == ".md"

    def _parse_frontmatter(self, text: str, source_path: Path) -> DocumentMetadata:
        match = re.match(r"^---\n(.*?)\n---", text, re.DOTALL)
        if not match:
            return DocumentMetadata(
                title=source_path.stem,
                source=str(source_path),
                doc_type="transcript",
            )

        import yaml
        try:
            fm = yaml.safe_load(match.group(1))
            return DocumentMetadata(
                title=fm.get("title", source_path.stem),
                source=fm.get("source", str(source_path)),
                doc_type=fm.get("type", "transcript"),
                author=fm.get("author", ""),
                language=fm.get("language", ""),
                word_count=fm.get("word_count", 0),
            )
        except yaml.YAMLError:
            return DocumentMetadata(
                title=source_path.stem,
                source=str(source_path),
                doc_type="transcript",
            )

    def _strip_frontmatter(self, text: str) -> str:
        return re.sub(r"^---\n.*?\n---\n?", "", text, count=1, flags=re.DOTALL).strip()
