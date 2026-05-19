import json
import re

from domain.document import ExtractedContent, RAGDocument
from processors.compressor import Compressor
from processors.transformer import Transformer
from processors.generator import Generator

LARGE_DOC_THRESHOLD = 50000  # chars


class CTGPipeline:
    """Compression → Transformation → Generation pipeline."""

    def __init__(self, llm_client, json_mode: bool = False):
        self._compressor = Compressor(llm_client, json_mode=json_mode)
        self._transformer = Transformer(llm_client)
        self._generator = Generator()
        self._json_mode = json_mode

    def _emit(self, status: str, **kwargs):
        if self._json_mode:
            obj = {"status": status, **kwargs}
            print(json.dumps(obj, ensure_ascii=False), flush=True)

    @staticmethod
    def _clean_book_artifacts(text: str) -> str:
        import re

        lines = text.split("\n")
        cleaned = []
        in_front_matter = True

        for line in lines:
            stripped = line.strip().lower()

            if in_front_matter:
                if any(kw in stripped for kw in ["part one", "chapter 1", "### part one", "### chapter"]):
                    in_front_matter = False
                if any(kw in stripped for kw in [
                    "copyright", "all rights reserved", "library of congress",
                    "isbn", "printed in", "typeset", "cataloging-in-publication",
                    "dedication", "to nancy", "to my",
                ]):
                    continue
                if stripped and not stripped.startswith("#") and len(stripped) < 80:
                    if not any(c.isalpha() for c in stripped):
                        continue
                    if stripped.replace(".", "").replace("—", "").replace("-", "").strip() == "":
                        continue

            cleaned.append(line)

        result = "\n".join(cleaned)

        # Fix single-letter line breaks: "I\nN" → "In", "A\nnd" → "And"
        result = re.sub(r'\b([A-Za-z])\n\s+([A-Za-z])\b', r'\1\2', result)

        # Fix title case words split across lines: "The\nory" → "Theory"
        result = re.sub(r'([a-z])\n([A-Z][a-z]{2,})', r'\1 \2', result)

        # Collapse excessive whitespace
        result = re.sub(r"\n{4,}", "\n\n\n", result)
        result = re.sub(r" {2,}", " ", result)

        return result.strip()

    def process(self, extracted: ExtractedContent, source_path: str = "") -> RAGDocument:
        if not extracted.raw_text.strip():
            raise ValueError("No content to process")

        is_large = len(extracted.raw_text) > LARGE_DOC_THRESHOLD

        if is_large:
            self._emit("large_doc_detected", chars=len(extracted.raw_text), mode="metadata_only")
            sample = extracted.raw_text[:15000]
            _, llm_metadata = self._transformer.transform(sample, extracted.metadata)

            metadata = extracted.metadata
            if llm_metadata.topics:
                metadata.topics = llm_metadata.topics
            if llm_metadata.summary:
                metadata.summary = llm_metadata.summary
            if llm_metadata.key_insights:
                metadata.key_insights = llm_metadata.key_insights
            if llm_metadata.title and llm_metadata.title != extracted.metadata.title:
                metadata.title = llm_metadata.title

            structured = self._clean_book_artifacts(extracted.raw_text)
        else:
            self._emit("compression_start", chars=len(extracted.raw_text))
            compressed = self._compressor.compress(extracted.raw_text)
            compressed = Compressor.clean_artifacts(compressed)
            self._emit("compression_done", chars=len(compressed))

            self._emit("transformation_start")
            structured, metadata = self._transformer.transform(compressed, extracted.metadata)

        metadata.source = extracted.metadata.source or metadata.source
        metadata.doc_type = extracted.metadata.doc_type or metadata.doc_type
        self._emit("transformation_done", topics=metadata.topics)

        self._emit("generation_start")
        doc = self._generator.generate(structured, metadata, source_path)
        self._emit("generation_done")

        return doc
