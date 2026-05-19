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
        from bs4 import BeautifulSoup, NavigableString, Tag
        import hashlib
        import re

        book = epub.read_epub(source_path)

        title = book.get_metadata("DC", "title")
        book_title = title[0][0] if title else source_path.stem

        author = book.get_metadata("DC", "creator")
        author_name = author[0][0] if author else ""

        chapters = []
        items = list(book.get_items_of_type(ebooklib.ITEM_DOCUMENT))

        # Extract images first
        image_items = book.get_items_of_type(ebooklib.ITEM_IMAGE)
        image_map = {}  # filename -> saved path
        output_dir = source_path.parent / f"{source_path.stem}_images"
        output_dir.mkdir(exist_ok=True)

        for img_item in image_items:
            img_data = img_item.get_content()
            img_hash = hashlib.md5(img_data).hexdigest()[:8]
            ext = Path(img_item.file_name).suffix or ".jpg"
            filename = f"img_{img_hash}{ext}"
            img_path = output_dir / filename
            img_path.write_bytes(img_data)
            image_map[img_item.file_name] = str(img_path.relative_to(source_path.parent))

        skip_title_patterns = [
            "copyright", "acknowledgment", "acknowledgement", "dedication",
            "about the author", "about this book",
            "preface", "foreword", "introduction", "prologue", "epilogue",
        ]

        skip_content_patterns = [
            "all rights reserved", "library of congress cataloging",
            "printed in the united states", "typeset", "isbn",
            "reproduction of this book", "permission to reproduce",
        ]

        for item in items:
            content = item.get_content().decode("utf-8")
            soup = BeautifulSoup(content, "html.parser")

            full_text = soup.get_text(" ", strip=True)
            lower_text = full_text.lower()

            title_el = soup.find(["h1", "h2", "title"])
            if title_el:
                title_text = title_el.get_text(strip=True).lower()
                if any(p in title_text for p in skip_title_patterns):
                    continue

            if any(p in lower_text for p in skip_content_patterns):
                continue

            if lower_text.strip().startswith("contents") or lower_text.strip().startswith("table of contents"):
                continue

            for h in soup.find_all(["h1", "h2", "h3", "h4", "h5", "h6"]):
                level = int(h.name[1])
                prefix = "#" * min(level + 1, 6)
                heading_text = h.get_text(strip=True)
                h.clear()
                h.string = f"{prefix} {heading_text}"

            for br in soup.find_all("br"):
                br.replace_with("\n")

            # Track image positions before unwrapping
            img_positions = []
            for img in soup.find_all("img"):
                src = img.get("src", "")
                if src in image_map:
                    parent = img.parent
                    parent_text = parent.get_text(" ", strip=True) if isinstance(parent, Tag) else ""
                    img_positions.append({
                        "src": src,
                        "path": image_map[src],
                        "context": "inline" if parent and parent.name == "p" else "block",
                        "parent_text_preview": parent_text[:100] if parent_text else "",
                    })
                img.replace_with(f"\n![image]({image_map.get(src, src)})\n")

            # Unwrap inline tags without creating line breaks
            for tag in soup.find_all(["em", "i", "strong", "b", "span", "a"]):
                tag.unwrap()

            blocks = []
            block_tags = {"p", "div", "blockquote", "ul", "ol", "li", "h1", "h2", "h3", "h4", "h5", "h6", "body", "html"}
            root = soup.find("body") or soup

            def extract_blocks(element):
                children = [c for c in element.children if hasattr(c, "name")]
                has_block_child = any(c.name in block_tags for c in children)
                if has_block_child:
                    for child in children:
                        if child.name in block_tags:
                            extract_blocks(child)
                else:
                    text = element.get_text(" ", strip=True)
                    if text and len(text) > 10:
                        blocks.append(text)

            extract_blocks(root)

            text = "\n\n".join(blocks)

            # Fix punctuation spacing from tag unwrapping
            text = re.sub(r" +([,.;:])", r"\1", text)
            text = re.sub(r"([,.;:]) +", r"\1 ", text)

            # ALL-CAPS header detection
            lines = text.split("\n\n")
            processed_lines = []
            caps_pattern = re.compile(r'^[A-Z][A-Z\s\-\'"]{14,}$')
            caps_start_pattern = re.compile(r'^([A-Z][A-Z\s\-\'"]{14,})\s+(.+)$')

            for line in lines:
                stripped = line.strip()
                if caps_pattern.match(stripped):
                    words = stripped.split()
                    if len(words) >= 3:
                        processed_lines.append(f"## {stripped}")
                        continue
                caps_match = caps_start_pattern.match(stripped)
                if caps_match:
                    header = caps_match.group(1)
                    rest = caps_match.group(2)
                    words = header.split()
                    if len(words) >= 3:
                        processed_lines.append(f"## {header}")
                        processed_lines.append(rest)
                        continue
                processed_lines.append(line)

            text = "\n\n".join(processed_lines)
            text = re.sub(r"\n{3,}", "\n\n", text)
            text = re.sub(r" {2,}", " ", text)

            if text.strip() and len(text.strip()) > 200:
                chapters.append(text)

        raw_text = "\n\n".join(chapters)

        images = []
        for img_info in image_map.values():
            images.append({"path": img_info, "position": "embedded"})

        return ExtractedContent(
            raw_text=raw_text,
            metadata=DocumentMetadata(
                title=book_title,
                source=str(source_path),
                doc_type="epub",
                author=author_name,
                word_count=len(raw_text.split()),
            ),
            images=images,
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
