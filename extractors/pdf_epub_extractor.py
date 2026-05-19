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

        suffix = source_path.suffix.lower()
        if suffix == ".pdf":
            return self._extract_pdf(source_path)
        if suffix == ".epub":
            return self._extract_epub(source_path)

        return self._extract_fallback(source_path)

    def _extract_pdf(self, source_path: Path) -> ExtractedContent:
        try:
            return self._extract_with_pymupdf(source_path)
        except ImportError:
            return self._extract_fallback(source_path)

    def _extract_with_pymupdf(self, source_path: Path) -> ExtractedContent:
        import fitz

        doc = fitz.open(source_path)
        pages = []
        for page in doc:
            blocks = page.get_text("dict")["blocks"]
            for block in blocks:
                if block.get("type") == 0:
                    for line in block.get("lines", []):
                        text = "".join(span["text"] for span in line["spans"])
                        if text.strip():
                            size = line["spans"][0].get("size", 12)
                            if size > 16:
                                pages.append(f"# {text.strip()}")
                            elif size > 13:
                                pages.append(f"## {text.strip()}")
                            else:
                                pages.append(text.strip())
        page_count = len(doc)
        doc.close()

        raw_text = "\n\n".join(pages)
        return ExtractedContent(
            raw_text=raw_text,
            metadata=DocumentMetadata(
                title=source_path.stem,
                source=str(source_path),
                doc_type="pdf",
                word_count=len(raw_text.split()),
            ),
            page_count=page_count,
        )

    def _extract_epub(self, source_path: Path) -> ExtractedContent:
        import ebooklib
        from ebooklib import epub
        from bs4 import BeautifulSoup

        book = epub.read_epub(source_path)
        chapters = []
        title = book.get_metadata("DC", "title")
        book_title = title[0][0] if title else source_path.stem

        for item in book.get_items_of_type(ebooklib.ITEM_DOCUMENT):
            content = item.get_content().decode("utf-8")
            soup = BeautifulSoup(content, "html.parser")
            text = soup.get_text(separator="\n", strip=True)
            if text.strip():
                chapters.append(text)

        raw_text = "\n\n".join(chapters)
        return ExtractedContent(
            raw_text=raw_text,
            metadata=DocumentMetadata(
                title=book_title,
                source=str(source_path),
                doc_type="epub",
                word_count=len(raw_text.split()),
            ),
        )

    def _extract_fallback(self, source_path: Path) -> ExtractedContent:
        with tempfile.TemporaryDirectory() as tmpdir:
            result = subprocess.run(
                ["marker_single", str(source_path), tmpdir, "--langs", ",".join(self._cfg.langs)],
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
            return ExtractedContent(
                raw_text=raw_text,
                metadata=DocumentMetadata(
                    title=source_path.stem,
                    source=str(source_path),
                    doc_type=self._detect_type(source_path),
                    word_count=len(raw_text.split()),
                ),
                page_count=0,
            )

    def supports(self, source: Path | str) -> bool:
        path = Path(source) if isinstance(source, str) else source
        return path.suffix.lower() in self.SUPPORTED_EXTENSIONS

    def _detect_type(self, path: Path) -> str:
        suffix = path.suffix.lower()
        return {".pdf": "pdf", ".epub": "epub", ".docx": "docx", ".pptx": "pptx", ".xlsx": "xlsx", ".html": "html"}.get(suffix, "document")
