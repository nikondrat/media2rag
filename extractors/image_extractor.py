import base64
from pathlib import Path

from domain.document import ExtractedContent, DocumentMetadata
from extractors.base import BaseExtractor


class ImageExtractor(BaseExtractor):
    SUPPORTED_EXTENSIONS = {".png", ".jpg", ".jpeg", ".webp", ".bmp", ".tiff"}

    def __init__(self, vision_client):
        self._vision_client = vision_client

    def extract(self, source: Path | str) -> ExtractedContent:
        source_path = Path(source) if isinstance(source, str) else source
        if not source_path.exists():
            raise FileNotFoundError(f"File not found: {source_path}")

        image_b64 = self._encode_image(source_path)
        description = self._vision_client.chat_with_image(
            prompt="Describe this image in detail. Include all text, diagrams, charts, and visual elements. "
                   "If it's a screenshot or document page, extract all readable content.",
            image_b64=image_b64,
            system="You are a document analysis assistant. Describe images thoroughly for RAG knowledge base.",
        )

        raw_text = f"# Image: {source_path.name}\n\n{description}"

        return ExtractedContent(
            raw_text=raw_text,
            metadata=DocumentMetadata(
                title=source_path.stem,
                source=str(source_path),
                doc_type="image",
                word_count=len(raw_text.split()),
            ),
            images=[{"path": str(source_path), "description": description}],
        )

    def supports(self, source: Path | str) -> bool:
        path = Path(source) if isinstance(source, str) else source
        return path.suffix.lower() in self.SUPPORTED_EXTENSIONS

    def _encode_image(self, path: Path) -> str:
        with open(path, "rb") as f:
            return base64.b64encode(f.read()).decode("utf-8")
