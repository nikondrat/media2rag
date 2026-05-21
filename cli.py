import argparse
import json
import sys
from pathlib import Path
from datetime import datetime, timezone

import yaml

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


PROJECT_YAML_NAME = ".media2rag.yaml"


def _write_project_yaml(work_dir: Path, data: dict):
    """Write or update .media2rag.yaml in the workspace subdirectory."""
    work_dir.mkdir(parents=True, exist_ok=True)
    yaml_path = work_dir / PROJECT_YAML_NAME
    existing = {}
    if yaml_path.exists():
        try:
            existing = yaml.safe_load(yaml_path.read_text(encoding="utf-8")) or {}
        except Exception:
            pass
    existing.update(data)
    yaml_path.write_text(yaml.dump(existing, allow_unicode=True, default_flow_style=False, sort_keys=False), encoding="utf-8")


def _read_project_yaml(work_dir: Path) -> dict | None:
    """Read .media2rag.yaml from the workspace subdirectory."""
    yaml_path = work_dir / PROJECT_YAML_NAME
    if not yaml_path.exists():
        return None
    try:
        return yaml.safe_load(yaml_path.read_text(encoding="utf-8")) or {}
    except Exception:
        return None


def _init_project_yaml(work_dir: Path, source: str, source_type: str, title: str, backend: str, model: str, extract_only: bool):
    """Create initial .media2rag.yaml at processing start."""
    _write_project_yaml(work_dir, {
        "source": source,
        "source_type": source_type,
        "title": title,
        "backend": backend,
        "model": model,
        "extract_only": extract_only,
        "state": "queued",
        "progress": 0.0,
        "started_at": datetime.now(timezone.utc).isoformat(),
        "completed_at": None,
        "error_message": None,
        "word_count": None,
        "topics": None,
        "summary": None,
        "key_insights": None,
        "chunks_total": None,
        "chunks_done": 0,
    })


def _update_project_yaml(work_dir: Path, **kwargs):
    """Update specific fields in .media2rag.yaml."""
    _write_project_yaml(work_dir, kwargs)


def _resume_processing(work_dir: Path, source: Path | str, cfg: AppConfig, llm_client, json_mode: bool = False):
    """Resume processing from an existing workspace directory."""
    project_data = _read_project_yaml(work_dir)
    intermediate_path = work_dir / "intermediate" / "raw.md"
    if not intermediate_path.exists():
        if json_mode:
            emit_json("error", file=str(source), message="No intermediate file found for resume")
        else:
            print(f"❌ No intermediate file found for resume: {intermediate_path}")
        sys.exit(1)

    raw_content = intermediate_path.read_text(encoding="utf-8")
    title = project_data.get("title") if project_data else None
    newline_idx = -1
    if not title:
        if raw_content.startswith("# "):
            newline_idx = raw_content.find("\n")
            title = raw_content[2:newline_idx].strip() if newline_idx > 0 else raw_content[2:]
        else:
            title = Path(source).stem
            newline_idx = -1
    raw_text = raw_content[newline_idx + 1:] if newline_idx > 0 else raw_content

    _update_project_yaml(work_dir, state="extracting", progress=0.1)

    from domain.document import ExtractedContent, DocumentMetadata, RAGDocument
    original_type = (project_data or {}).get("source_type", "markdown")
    extracted = ExtractedContent(
        raw_text=raw_text,
        metadata=DocumentMetadata(
            title=title,
            source=str(source),
            doc_type=original_type,
            word_count=len(raw_text.split()),
        ),
    )

    if json_mode:
        emit_json("extracted", file=str(source), type="markdown", words=extracted.metadata.word_count)

    if not llm_client:
        doc = RAGDocument(
            markdown=f"# {title}\n\n{raw_text}",
            metadata=extracted.metadata,
        )
    else:
        if json_mode:
            emit_json("map_start", total=len(list((work_dir / "chunks").glob("chunk_*.md"))), work_dir=str(work_dir))
        pipeline = CTGPipeline(llm_client, json_mode=json_mode)
        doc = pipeline.process(extracted, str(source), workspace_dir=work_dir)

    output_path = doc.save(Path(), workspace_dir=work_dir)

    _update_project_yaml(
        work_dir,
        state="completed",
        progress=1.0,
        completed_at=datetime.now(timezone.utc).isoformat(),
        topics=doc.metadata.topics or None,
        summary=doc.metadata.summary or None,
        key_insights=doc.metadata.key_insights or None,
        title=doc.metadata.title,
    )

    if not json_mode:
        print(f"✅ Saved: {output_path}")
    else:
        emit_json("completed", file=str(source), output=str(output_path), intermediate=str(intermediate_path),
                  work_dir=str(work_dir),
                  sections=[p.stem for p in sorted((work_dir / "sections").glob("*.md"))] if (work_dir / "sections").exists() else [],
                  images=[])
    return output_path


def process_single(source: Path | str, cfg: AppConfig, llm_client, workspace_dir: Path, json_mode: bool = False, existing_work_dir: Path | None = None, extract_only: bool = False, model: str = ""):
    if existing_work_dir and existing_work_dir.exists():
        return _resume_processing(existing_work_dir, source, cfg, llm_client, json_mode)

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
            return _process_telegram_channel(extractor, channel, source, cfg, llm_client, workspace_dir, json_mode)

    source_str = str(source)
    if not json_mode:
        print(f"📄 Extracting: {source}")

    if json_mode:
        emit_json("extracting", file=source_str, type=extractor.__class__.__name__)

    extracted = extractor.extract(source, workspace_dir=workspace_dir)
    doc_type = extracted.metadata.doc_type
    word_count = extracted.metadata.word_count

    if not json_mode:
        print(f"   Type: {doc_type}, Words: {word_count}")
    else:
        emit_json("extracted", file=source_str, type=doc_type, words=word_count)

    from domain.document import _sanitize_filename
    file_stem = _sanitize_filename(extracted.metadata.title) or Path(source_str).stem
    file_workspace = workspace_dir / file_stem
    file_workspace.mkdir(parents=True, exist_ok=True)
    for subdir in ["chunks", "images", "sections", "intermediate", "output"]:
        (file_workspace / subdir).mkdir(exist_ok=True)

    _init_project_yaml(
        file_workspace,
        source=source_str,
        source_type=doc_type,
        title=extracted.metadata.title,
        backend=cfg.llm_backend,
        model=model,
        extract_only=extract_only,
    )

    _update_project_yaml(file_workspace, state="extracted", progress=0.2, word_count=word_count)

    intermediate_path = file_workspace / "intermediate" / "raw.md"
    intermediate_path.write_text(
        f"# {extracted.metadata.title}\n\n{extracted.raw_text}",
        encoding="utf-8",
    )
    intermediate_str = str(intermediate_path)

    if not llm_client:
        from domain.document import RAGDocument
        doc = RAGDocument(
            markdown=f"# {extracted.metadata.title}\n\n{extracted.raw_text}",
            metadata=extracted.metadata,
        )
    else:
        pipeline = CTGPipeline(llm_client, json_mode=json_mode)
        doc = pipeline.process(extracted, source_str, workspace_dir=file_workspace)

    output_path = doc.save(Path(), workspace_dir=file_workspace)

    _update_project_yaml(
        file_workspace,
        state="completed",
        progress=1.0,
        completed_at=datetime.now(timezone.utc).isoformat(),
        topics=doc.metadata.topics or None,
        summary=doc.metadata.summary or None,
        key_insights=doc.metadata.key_insights or None,
        title=doc.metadata.title,
    )

    if not json_mode:
        print(f"✅ Saved: {output_path}")
    else:
        emit_json("completed", file=source_str, output=str(output_path), intermediate=intermediate_str,
                  work_dir=str(file_workspace),
                  sections=[p.stem for p in sorted((file_workspace / "sections").glob("*.md"))] if (file_workspace / "sections").exists() else [],
                  images=[str(p) for p in extracted.image_paths])
    return output_path


def _process_telegram_channel(extractor, channel: str, url: str, cfg, llm_client, workspace_dir, json_mode):
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
    output_paths = []
    total_words = 0

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
            doc = pipeline.process(extracted, post_url, workspace_dir=workspace_dir)

        output_path = doc.save(Path(), workspace_dir=workspace_dir)
        output_paths.append(str(output_path))
        total_words += len(post.text.split())
        processed += 1

        if not json_mode:
            print(f"      ✅ {output_path.name}")

    if not json_mode:
        print(f"✅ Channel done: {processed}/{total} posts processed")
    else:
        emit_json("telegram_complete", channel=channel, processed=processed, total=total,
                  output_files=output_paths, words=total_words)
        emit_json("completed", file=url, output=output_paths[0] if output_paths else "", words=total_words)
    return workspace_dir


def process_directory(input_dir: Path, cfg: AppConfig, llm_client, workspace_dir: Path, json_mode: bool = False, extract_only: bool = False, model: str = ""):
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
            process_single(f, cfg, llm_client, workspace_dir, json_mode=json_mode, extract_only=extract_only, model=model)
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
    parser.add_argument("-o", "--output", "--workspace", dest="workspace", default=None, help="Workspace directory")
    parser.add_argument("--backend", choices=["ollama", "openrouter"], help="LLM backend")
    parser.add_argument("--model", help="LLM model name (overrides default)")
    parser.add_argument("--extract-only", action="store_true", help="Skip CTG, save raw extraction")
    parser.add_argument("--batch", action="store_true", help="Process all files in directory")
    parser.add_argument("--json", action="store_true", help="Output progress as JSON (for GUI)")
    parser.add_argument("--work-dir", dest="work_dir", default=None, help="Existing workspace subdirectory to resume (skip extraction)")
    args = parser.parse_args()

    cfg = AppConfig.from_env()
    if args.backend:
        cfg.llm_backend = args.backend

    llm_client = None if args.extract_only else get_llm_client(cfg, args.model)

    workspace_dir = _resolve_workspace(args.workspace, cfg)

    if args.source.startswith("http"):
        source = args.source
    else:
        source = Path(args.source)

    if args.batch or (isinstance(source, Path) and source.is_dir()):
        process_directory(source, cfg, llm_client, workspace_dir, json_mode=args.json, extract_only=args.extract_only, model=args.model or "")
    else:
        existing_work_dir = Path(args.work_dir) if args.work_dir else None
        process_single(source, cfg, llm_client, workspace_dir, json_mode=args.json, existing_work_dir=existing_work_dir, extract_only=args.extract_only, model=args.model or "")


def _resolve_workspace(cli_arg: str | None, cfg: AppConfig) -> Path:
    if cli_arg:
        return Path(cli_arg)
    if cfg.workspace_dir:
        return cfg.workspace_dir
    if cfg.output_dir and cfg.output_dir != Path("output"):
        return cfg.output_dir
    return Path.home() / "Documents" / "media2rag"


if __name__ == "__main__":
    main()
