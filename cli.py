import argparse
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

    print("   No LLM backend available. Use --extract-only for extraction without CTG.")
    return None


def get_extractors(cfg: AppConfig, llm_client) -> list[BaseExtractor]:
    extractors = [
        MarkdownExtractor(),
        PdfEpubExtractor(cfg.marker),
        VideoExtractor(cfg.whisper),
        AudioExtractor(cfg.whisper),
    ]
    if llm_client:
        from clients.ollama_client import OllamaClient
        if isinstance(llm_client, OllamaClient):
            extractors.append(ImageExtractor(llm_client))
    return extractors


def find_extractor(source: Path | str, extractors: list[BaseExtractor]) -> BaseExtractor | None:
    for ext in extractors:
        if ext.supports(source):
            return ext
    return None


def process_single(source: Path | str, cfg: AppConfig, llm_client, output_dir: Path):
    extractors = get_extractors(cfg, llm_client)
    extractor = find_extractor(source, extractors)

    if not extractor:
        print(f"❌ No extractor for: {source}")
        sys.exit(1)

    print(f"📄 Extracting: {source}")
    extracted = extractor.extract(source)
    print(f"   Type: {extracted.metadata.doc_type}, Words: {extracted.metadata.word_count}")

    if not llm_client:
        print("   No LLM backend — saving raw extraction")
        from domain.document import RAGDocument, DocumentMetadata
        doc = RAGDocument(
            markdown=f"# {extracted.metadata.title}\n\n{extracted.raw_text}",
            metadata=extracted.metadata,
        )
    else:
        pipeline = CTGPipeline(llm_client)
        doc = pipeline.process(extracted, str(source))

    output_path = doc.save(output_dir)
    print(f"✅ Saved: {output_path}")
    return output_path


def process_directory(input_dir: Path, cfg: AppConfig, llm_client, output_dir: Path):
    extractors = get_extractors(cfg, llm_client)
    files = [f for f in input_dir.rglob("*") if f.is_file()]
    processed = 0
    errors = 0

    for f in sorted(files):
        extractor = find_extractor(f, extractors)
        if not extractor:
            continue

        try:
            process_single(f, cfg, llm_client, output_dir)
            processed += 1
        except Exception as e:
            print(f"❌ Error processing {f}: {e}")
            errors += 1

    print(f"\n📊 Done: {processed} processed, {errors} errors")


def main():
    parser = argparse.ArgumentParser(description="Convert media to RAG-ready Markdown")
    parser.add_argument("source", help="File path, directory, or URL")
    parser.add_argument("-o", "--output", default="output", help="Output directory")
    parser.add_argument("--backend", choices=["ollama", "openrouter"], help="LLM backend")
    parser.add_argument("--model", help="LLM model name (overrides default)")
    parser.add_argument("--extract-only", action="store_true", help="Skip CTG, save raw extraction")
    parser.add_argument("--batch", action="store_true", help="Process all files in directory")
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
        process_directory(source, cfg, llm_client, output_dir)
    else:
        process_single(source, cfg, llm_client, output_dir)


if __name__ == "__main__":
    main()
