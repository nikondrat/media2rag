#!/usr/bin/env python3
"""Reindex all workspace directories. Prefers output/final.md (CTG), falls back to intermediate/raw.md."""

import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent.parent))

from domain.document import ExtractedContent, DocumentMetadata
from ingestion.embedder import Embedder
from ingestion.pipeline import IngestionPipeline


WORKSPACE_ROOT = Path.home() / "Documents" / "media2rag"


def find_workspaces(root: Path) -> list[Path]:
    workspaces = []
    for subdir in sorted(root.iterdir()):
        if not subdir.is_dir():
            continue
        if subdir.name.startswith("."):
            continue
        raw_md = subdir / "intermediate" / "raw.md"
        if raw_md.exists():
            workspaces.append(subdir)
    return workspaces


def extract_metadata_from_raw(text: str) -> tuple[str, str]:
    title = ""
    body = text

    if text.startswith("# "):
        nl = text.find("\n")
        if nl > 0:
            title = text[2:nl].strip()
            body = text[nl + 1:]
        else:
            title = text[2:].strip()
            body = ""

    return title, body


def reindex_workspace(workspace: Path, pipeline: IngestionPipeline) -> dict | None:
    final_md = workspace / "output" / "final.md"
    raw_md = workspace / "intermediate" / "raw.md"

    if final_md.exists():
        text = final_md.read_text(encoding="utf-8")
        title, body = extract_metadata_from_raw(text)
        source = workspace.name
        if not body.strip():
            return None
        extracted = ExtractedContent(
            raw_text=body,
            metadata=DocumentMetadata(
                title=title or workspace.name,
                source=source,
                doc_type="document",
                word_count=len(body.split()),
            ),
        )
        result = pipeline.ingest(extracted, source)
        return result

    if raw_md.exists():
        text = raw_md.read_text(encoding="utf-8")
        title, body = extract_metadata_from_raw(text)
        if not body.strip():
            return None
        source = workspace.name
        extracted = ExtractedContent(
            raw_text=body,
            metadata=DocumentMetadata(
                title=title or workspace.name,
                source=source,
                doc_type="video",
                word_count=len(body.split()),
            ),
        )
        result = pipeline.ingest(extracted, source)
        return result

    return None


def main():
    if not WORKSPACE_ROOT.exists():
        print(f"Workspace root not found: {WORKSPACE_ROOT}")
        sys.exit(1)

    workspaces = find_workspaces(WORKSPACE_ROOT)
    print(f"Found {len(workspaces)} workspaces in {WORKSPACE_ROOT}")

    if not workspaces:
        print("Nothing to reindex.")
        return

    embedder = Embedder()
    pipeline = IngestionPipeline(embedder=embedder)

    total_chunks = 0
    total_parents = 0
    total_final = 0
    total_raw = 0
    errors = 0

    for i, ws in enumerate(workspaces, 1):
        name = ws.name
        print(f"[{i}/{len(workspaces)}] {name[:60]}...", end=" ", flush=True)

        try:
            source_type = "final.md" if (ws / "output" / "final.md").exists() else "raw.md"
            result = reindex_workspace(ws, pipeline)
            if result:
                chunks = result["chunks"]
                parents = result["parents"]
                total_chunks += chunks
                total_parents += parents
                if source_type == "final.md":
                    total_final += 1
                else:
                    total_raw += 1
                print(f"OK ({chunks} chunks, {parents} parents, from {source_type})")
            else:
                print("SKIP (empty)")
        except Exception as e:
            errors += 1
            print(f"ERROR: {e}")

    print(f"\nDone: {len(workspaces)} workspaces ({total_final} final.md, {total_raw} raw.md), {total_chunks} chunks, {total_parents} parents, {errors} errors")


if __name__ == "__main__":
    main()
