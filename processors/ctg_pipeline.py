from domain.document import ExtractedContent, RAGDocument
from processors.compressor import Compressor
from processors.transformer import Transformer
from processors.generator import Generator


class CTGPipeline:
    """Compression → Transformation → Generation pipeline."""

    def __init__(self, llm_client):
        self._compressor = Compressor(llm_client)
        self._transformer = Transformer(llm_client)
        self._generator = Generator()

    def process(self, extracted: ExtractedContent, source_path: str = "") -> RAGDocument:
        if not extracted.raw_text.strip():
            raise ValueError("No content to process")

        print(f"  [1/3] Compression: {len(extracted.raw_text)} chars → ...")
        compressed = self._compressor.compress(extracted.raw_text)
        compressed = Compressor.clean_artifacts(compressed)
        print(f"  [1/3] Compression: done ({len(compressed)} chars)")

        print(f"  [2/3] Transformation: structuring by topics...")
        structured, metadata = self._transformer.transform(compressed, extracted.metadata)
        metadata.source = extracted.metadata.source or metadata.source
        metadata.doc_type = extracted.metadata.doc_type or metadata.doc_type
        print(f"  [2/3] Transformation: done (topics: {metadata.topics})")

        print(f"  [3/3] Generation: assembling RAG markdown...")
        doc = self._generator.generate(structured, metadata, source_path)
        print(f"  [3/3] Generation: done")

        return doc
