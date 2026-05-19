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

        try:
            return self._extract_with_marker_python(source_path)
        except ImportError:
            return self._extract_with_cli(source_path)

    def _extract_with_marker_python(self, source_path: Path) -> ExtractedContent:
        from marker.converters.pdf import PDFConverter
        from marker.models import create_model_dict
        from marker.config.parser import ConfigParser

        config_parser = ConfigParser({"langs": self._cfg.langs})
        converter = PDFConverter(
            model_dict=create_model_dict(),
            config=config_parser.generate_config_dict(),
        )

        raw_text = converter(source_path).markdown
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

    def _extract_with_cli(self, source_path: Path) -> ExtractedContent:
        with tempfile.TemporaryDirectory() as tmpdir:
            result = subprocess.run(
                [
                    "marker_single",
                    str(source_path),
                    tmpdir,
                    "--batch_multiplier", str(self._cfg.batch_multiplier),
                ]
                + (["--langs", ",".join(self._cfg.langs)] if self._cfg.langs else [])
                + (["--max_pages", str(self._cfg.max_pages)] if self._cfg.max_pages else []),
                capture_output=True,
                text=True,
                timeout=600,
            )

            if result.returncode != 0:
                raise RuntimeError(f"Marker failed: {result.stderr[:500]}")

            output_files = list(Path(tmpdir).glob("*.md"))
            if not output_files:
                output_files = list(Path(tmpdir).rglob("*.md"))
            if not output_files:
                raise RuntimeError("Marker produced no output")

            raw_text = output_files[0].read_text(encoding="utf-8")
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
        return {".pdf": "pdf", ".epub": "epub", ".docx": "docx", ".pptx": "pptx", ".xlsx": "xlsx", ".html": "html"}.get(suffix, "document")

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
