import argparse
import json
import sys
from pathlib import Path

from config import AppConfig
from clients.ollama_client import OllamaClient
from clients.openrouter_client import OpenRouterClient
from extractors.base import BaseExtractor
from extractors.pdf_epub_extractor import PdfEpubExtractor
from extractors.video_extractor import VideoExtractor
from extractors.audio_extractor import AudioExtractor
from extractors.image_extractor import ImageExtractor
from extractors.markdown_extractor import MarkdownExtractor
from extractors.url_extractor import URLExtractor
from extractors.telegram_extractor import TelegramExtractor
from processors.ctg_pipeline import CTGPipeline


def get_llm_client(cfg: AppConfig, model: str = ""):
    if cfg.llm_backend == "openrouter" or cfg.openrouter.api_key:
        client = OpenRouterClient(cfg.openrouter)
        if client.is_available():
            if model:
                client._model = model
            return client
        print("⚠️  OpenRouter API key not set")

    client = OllamaClient(cfg.ollama)
    if client.is_available():
        if model:
            client._ctg_model = model
        return client
    print("⚠️  Ollama not available at", cfg.ollama.base_url)

    print("   No LLM backend — saving raw extraction")
    return None


def get_extractors(cfg: AppConfig, llm_client) -> list[BaseExtractor]:
    extractors = [
        TelegramExtractor(),
        URLExtractor(),
        MarkdownExtractor(),
        PdfEpubExtractor(cfg.marker),
        VideoExtractor(cfg.whisper),
        AudioExtractor(cfg.whisper),
    ]
    if llm_client:
        if isinstance(llm_client, OllamaClient):
            extractors.append(ImageExtractor(llm_client))
    return extractors


def find_extractor(source: Path | str, extractors: list[BaseExtractor]) -> BaseExtractor | None:
    for ext in extractors:
        if ext.supports(source):
            return ext
    return None


def emit_json(status: str, **kwargs):
    obj = {"status": status, **kwargs}
    print(json.dumps(obj, ensure_ascii=False), flush=True)


def process_single(source: Path | str, cfg: AppConfig, llm_client, output_dir: Path, json_mode: bool = False):
    extractors = get_extractors(cfg, llm_client)
    extractor = find_extractor(source, extractors)

    if not extractor:
        if json_mode:
            emit_json("error", file=str(source), message="No extractor for this file type")
        else:
            print(f"❌ No extractor for: {source}")
        sys.exit(1)

    source_str = str(source)
    if not json_mode:
        print(f"📄 Extracting: {source}")

    if json_mode:
        emit_json("extracting", file=source_str, type=extractor.__class__.__name__)

    extracted = extractor.extract(source)
    doc_type = extracted.metadata.doc_type
    word_count = extracted.metadata.word_count

    if not json_mode:
        print(f"   Type: {doc_type}, Words: {word_count}")
    else:
        emit_json("extracted", file=source_str, type=doc_type, words=word_count)

    if not llm_client:
        from domain.document import RAGDocument, DocumentMetadata
        doc = RAGDocument(
            markdown=f"# {extracted.metadata.title}\n\n{extracted.raw_text}",
            metadata=extracted.metadata,
        )
    else:
        pipeline = CTGPipeline(llm_client, json_mode=json_mode)
        doc = pipeline.process(extracted, source_str)

    output_path = doc.save(output_dir)
    if not json_mode:
        print(f"✅ Saved: {output_path}")
    else:
        emit_json("completed", file=source_str, output=str(output_path))
    return output_path


def process_directory(input_dir: Path, cfg: AppConfig, llm_client, output_dir: Path, json_mode: bool = False):
    extractors = get_extractors(cfg, llm_client)
    files = [f for f in input_dir.rglob("*") if f.is_file()]
    processed = 0
    errors = 0
    total = 0

    for f in sorted(files):
        extractor = find_extractor(f, extractors)
        if not extractor:
            continue
        total += 1

    if json_mode:
        emit_json("batch_start", total=total, directory=str(input_dir))

    for i, f in enumerate(sorted(files)):
        extractor = find_extractor(f, extractors)
        if not extractor:
            continue

        if json_mode:
            emit_json("batch_progress", current=i + 1, total=total, file=str(f))

        try:
            process_single(f, cfg, llm_client, output_dir, json_mode=json_mode)
            processed += 1
        except Exception as e:
            if json_mode:
                emit_json("error", file=str(f), message=str(e))
            else:
                print(f"❌ Error processing {f}: {e}")
            errors += 1

    if json_mode:
        emit_json("batch_complete", processed=processed, errors=errors)
    else:
        print(f"\n📊 Done: {processed} processed, {errors} errors")


def main():
    parser = argparse.ArgumentParser(description="Convert media to RAG-ready Markdown")
    parser.add_argument("source", help="File path, directory, or URL")
    parser.add_argument("-o", "--output", default="output", help="Output directory")
    parser.add_argument("--backend", choices=["ollama", "openrouter"], help="LLM backend")
    parser.add_argument("--model", help="LLM model name (overrides default)")
    parser.add_argument("--extract-only", action="store_true", help="Skip CTG, save raw extraction")
    parser.add_argument("--batch", action="store_true", help="Process all files in directory")
    parser.add_argument("--json", action="store_true", help="Output progress as JSON (for GUI)")
    args = parser.parse_args()

    cfg = AppConfig.from_env()
    if args.backend:
        cfg.llm_backend = args.backend

    llm_client = None if args.extract_only else get_llm_client(cfg, args.model)

    output_dir = Path(args.output)

    if args.source.startswith("http"):
        source = args.source
    else:
        source = Path(args.source)

    if args.batch or (isinstance(source, Path) and source.is_dir()):
        process_directory(source, cfg, llm_client, output_dir, json_mode=args.json)
    else:
        process_single(source, cfg, llm_client, output_dir, json_mode=args.json)


if __name__ == "__main__":
    main()
