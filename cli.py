import argparse
import json
import sys
from pathlib import Path

from config import AppConfig
from clients.ollama_client import OllamaClient
from clients.openrouter_client import OpenRouterClient
from domain.document import ExtractedContent, DocumentMetadata
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
    if cfg.llm_backend == "openrouter" and cfg.openrouter.api_key:
        client = OpenRouterClient(cfg.openrouter)
        if client.is_available():
            if model:
                client._model = model
            return client
        print("⚠️  OpenRouter API key not set or unavailable")

    if cfg.llm_backend == "ollama":
        client = OllamaClient(cfg.ollama)
        if client.is_available():
            if model:
                client._ctg_model = model
            return client
        print("⚠️  Ollama not available at", cfg.ollama.base_url)

    if cfg.openrouter.api_key:
        client = OpenRouterClient(cfg.openrouter)
        if client.is_available():
            if model:
                client._model = model
            return client

    print("   No LLM backend — saving raw extraction")
    return None


def get_extractors(cfg: AppConfig, llm_client) -> list[BaseExtractor]:
    extractors = [
        TelegramExtractor(llm_client=llm_client),
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

    # Special handling for Telegram channels — process each post individually
    if isinstance(extractor, TelegramExtractor) and isinstance(source, str):
        channel, post_id = extractor._parse_telegram_url(source)
        if channel and not post_id:
            return _process_telegram_channel(extractor, channel, source, cfg, llm_client, output_dir, json_mode)

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
        from domain.document import RAGDocument
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


def _process_telegram_channel(extractor, channel: str, url: str, cfg, llm_client, output_dir, json_mode):
    """Process Telegram channel: scrape all posts, filter, process each through pipeline."""
    if not json_mode:
        print(f"📡 Scraping channel: {channel}")

    posts = extractor.extract_all_posts(url)
    total = len(posts)

    if not json_mode:
        print(f"   Found {total} posts after filtering")
    else:
        emit_json("telegram_channel", channel=channel, total_posts=total, url=url)

    if not posts:
        if json_mode:
            emit_json("completed", file=url, output="", message="No posts after filtering")
        return None

    processed = 0
    for i, post in enumerate(posts):
        post_url = post.url
        if not json_mode:
            print(f"   [{i+1}/{total}] Post #{post.id} ({post.date[:10] if post.date else '?'})")
        else:
            emit_json("telegram_progress", current=i + 1, total=total, post_id=post.id, post_url=post_url)

        extracted = ExtractedContent(
            raw_text=post.text,
            metadata=DocumentMetadata(
                title=f"Post #{post.id}",
                source=post_url,
                doc_type="telegram",
                word_count=len(post.text.split()),
            ),
        )

        if not llm_client:
            from domain.document import RAGDocument
            doc = RAGDocument(
                markdown=f"# Post #{post.id}\n\n{post.text}",
                metadata=extracted.metadata,
            )
        else:
            pipeline = CTGPipeline(llm_client, json_mode=json_mode)
            doc = pipeline.process(extracted, post_url)

        output_path = doc.save(output_dir)
        processed += 1

        if json_mode:
            emit_json("completed", file=post_url, output=str(output_path))
        elif not json_mode:
            print(f"      ✅ {output_path.name}")

    if not json_mode:
        print(f"✅ Channel done: {processed}/{total} posts processed")
    else:
        emit_json("telegram_complete", channel=channel, processed=processed, total=total)
    return output_dir


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
