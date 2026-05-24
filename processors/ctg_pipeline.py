import json
import re
from pathlib import Path
from typing import Optional

from domain.document import ExtractedContent, RAGDocument
from processors.compressor import Compressor
from processors.transformer import Transformer
from processors.chunked_transformer import ChunkedTransformer
from processors.generator import Generator
from processors.evaluator import ContentEvaluator
from processors.content_router import ContentRouter


class CTGPipeline:
    """Compression → Transformation → Generation pipeline."""

    def __init__(self, llm_client, json_mode: bool = False, reasoning: bool = False, quality_check: bool = False,
                 use_router: bool = True):
        self._compressor = Compressor(llm_client, json_mode=json_mode, reasoning=reasoning)
        self._transformer = Transformer(llm_client, reasoning=reasoning)
        self._evaluator = ContentEvaluator(llm_client, json_mode=json_mode, reasoning=reasoning) if quality_check else None
        router = ContentRouter(llm_client, json_mode=json_mode, reasoning=reasoning) if use_router else None
        self._chunked_transformer = ChunkedTransformer(llm_client, json_mode=json_mode, reasoning=reasoning,
                                                       evaluator=self._evaluator, router=router)
        self._generator = Generator()
        self._json_mode = json_mode
        self._quality_check = quality_check

    def _emit(self, status: str, **kwargs):
        if self._json_mode:
            obj = {"status": status, **kwargs}
            print(json.dumps(obj, ensure_ascii=False), flush=True)
        else:
            messages = {
                "compression_start": f"  📝 Compression: {kwargs.get('chars')} chars of raw text",
                "compression_done": f"  ✅ Compressed to {kwargs.get('chars')} chars",
                "transformation_start": "  🔄 Transformation: starting map-reduce...",
                "transformation_done": f"  ✅ Transformation complete | topics: {kwargs.get('topics', [])}",
                "generation_start": "  📄 Generating final document...",
                "generation_done": "  ✅ Generation complete",
            }
            msg = messages.get(status, f"  [{status}] {kwargs}")
            print(msg, flush=True)

    def process(self, extracted: ExtractedContent, source_path: str = "", workspace_dir: Path | None = None) -> RAGDocument:
        if not extracted.raw_text.strip():
            raise ValueError("No content to process")

        if not self._json_mode:
            print(f"\n{'='*60}")
            print(f"  CTG Pipeline: {extracted.metadata.title or 'Untitled'}")
            print(f"  Source: {source_path}")
            print(f"  {'='*60}\n")

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

        if self._evaluator and self._quality_check:
            self._emit("quality_check_start")
            improved, eval_result = self._evaluator.evaluate_and_optimize(doc.markdown, workspace_dir=workspace_dir)
            doc.markdown = improved
            if hasattr(doc.metadata, 'quality_score'):
                doc.metadata.quality_score = eval_result.get("overall")
            self._emit("quality_check_done", overall=eval_result.get("overall"),
                       needs_revision=eval_result.get("needs_revision"),
                       critique_preview=(eval_result.get("critique", "")[:200] if eval_result.get("critique") else ""),
                       scores={k: eval_result.get(k) for k in ["structure", "completeness", "signal_to_noise", "actionability", "language"] if eval_result.get(k) is not None})

        if not self._json_mode:
            print(f"\n{'='*60}")
            print(f"  Pipeline complete")
            print(f"  Title: {metadata.title}")
            print(f"  Topics: {', '.join(metadata.topics) if metadata.topics else 'N/A'}")
            print(f"  Language: {metadata.language or 'N/A'}")
            print(f"  {'='*60}\n")

        return doc
