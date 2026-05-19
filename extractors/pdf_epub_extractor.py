import subprocess
import tempfile
from pathlib import Path

from config import MarkerConfig
from domain.document import ExtractedContent, DocumentMetadata
from extractors.base import BaseExtractor


class PdfEpubExtractor(BaseExtractor):
    SUPPORTED_EXTENSIONS = {".pdf", ".epub", ".docx", ".pptx", ".xlsx", ".html"}

    def __init__(self, cfg: MarkerConfig):
        self._cfg = cfg

    def extract(self, source: Path | str) -> ExtractedContent:
        source_path = Path(source) if isinstance(source, str) else source
        if not source_path.exists():
            raise FileNotFoundError(f"File not found: {source_path}")

        result = subprocess.run(
            ["marker_single", str(source_path), "--batch_multiplier", str(self._cfg.batch_multiplier)]
            + (["--langs", ",".join(self._cfg.langs)] if self._cfg.langs else [])
            + (["--max_pages", str(self._cfg.max_pages)] if self._cfg.max_pages else []),
            capture_output=True,
            text=True,
            timeout=600,
        )

        if result.returncode != 0:
            raise RuntimeError(f"Marker failed: {result.stderr[:500]}")

        output_dir = Path(tempfile.mkdtemp(prefix="marker_"))
        md_file = output_dir / f"{source_path.stem}.md"
        if not md_file.exists():
            md_file = source_path.parent / f"{source_path.stem}.md"

        raw_text = md_file.read_text(encoding="utf-8") if md_file.exists() else ""
        page_count = self._count_pages(source_path)

        return ExtractedContent(
            raw_text=raw_text,
            metadata=DocumentMetadata(
                title=source_path.stem,
                source=str(source_path),
                doc_type=self._detect_type(source_path),
                original_path=str(source_path),
                word_count=len(raw_text.split()),
            ),
            page_count=page_count,
        )

    def supports(self, source: Path | str) -> bool:
        path = Path(source) if isinstance(source, str) else source
        return path.suffix.lower() in self.SUPPORTED_EXTENSIONS

    def _detect_type(self, path: Path) -> str:
        suffix = path.suffix.lower()
        return {"pdf": "pdf", ".epub": "epub", ".docx": "docx", ".pptx": "pptx", ".xlsx": "xlsx", ".html": "html"}.get(suffix, "document")

    def _count_pages(self, path: Path) -> int:
        if path.suffix.lower() == ".pdf":
            try:
                import fitz
                doc = fitz.open(path)
                count = len(doc)
                doc.close()
                return count
            except ImportError:
                return 0
        return 0
