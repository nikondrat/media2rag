import json

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

    def process(self, extracted: ExtractedContent, source_path: str = "") -> RAGDocument:
        if not extracted.raw_text.strip():
            raise ValueError("No content to process")

        is_large = len(extracted.raw_text) > LARGE_DOC_THRESHOLD

        if is_large:
            self._emit("large_doc_detected", chars=len(extracted.raw_text), mode="metadata_only")
            sample = extracted.raw_text[:15000]
            _, metadata = self._transformer.transform(sample, extracted.metadata)
            structured = Compressor.clean_artifacts(extracted.raw_text)
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
