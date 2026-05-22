import json
import re
from pathlib import Path

from domain.document import ExtractedContent, RAGDocument
from processors.compressor import Compressor
from processors.transformer import Transformer
from processors.chunked_transformer import ChunkedTransformer
from processors.generator import Generator


class CTGPipeline:
    """Compression → Transformation → Generation pipeline."""

    def __init__(self, llm_client, json_mode: bool = False, reasoning: bool = False):
        self._compressor = Compressor(llm_client, json_mode=json_mode, reasoning=reasoning)
        self._transformer = Transformer(llm_client, reasoning=reasoning)
        self._chunked_transformer = ChunkedTransformer(llm_client, json_mode=json_mode, reasoning=reasoning)
        self._generator = Generator()
        self._json_mode = json_mode

    def _emit(self, status: str, **kwargs):
        if self._json_mode:
            obj = {"status": status, **kwargs}
            print(json.dumps(obj, ensure_ascii=False), flush=True)

    def process(self, extracted: ExtractedContent, source_path: str = "", workspace_dir: Path | None = None) -> RAGDocument:
        if not extracted.raw_text.strip():
            raise ValueError("No content to process")

        self._chunked_transformer._work_dir = workspace_dir

        self._emit("compression_start", chars=len(extracted.raw_text))
        compressed = self._compressor.compress(extracted.raw_text)
        compressed = Compressor.clean_artifacts(compressed)
        self._emit("compression_done", chars=len(compressed))

        self._emit("transformation_start")
        structured, metadata = self._chunked_transformer.map_reduce(
            compressed, extracted.metadata, source_path=source_path
        )

        metadata.source = extracted.metadata.source or metadata.source
        metadata.doc_type = extracted.metadata.doc_type or metadata.doc_type
        self._emit("transformation_done", topics=metadata.topics)

        self._emit("generation_start")
        doc = self._generator.generate(structured, metadata, source_path)
        self._emit("generation_done")

        return doc
