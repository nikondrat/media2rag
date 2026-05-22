import json
import re
from pathlib import Path
from unidecode import unidecode

import yaml

from domain.document import Claim, DocumentMetadata
from processors.chunker import SemanticChunker
from processors.transformer import Transformer
from processors.output_parser import MarkdownOutputParser


class ChunkedTransformer:
    """Incremental map-reduce transformer for large documents.

    Each chunk is processed independently and saved to disk.
    Final merge+dedup reads all intermediate files.
    Supports resume: already-processed chunks are skipped.
    """

    DEDUP_SYSTEM_PROMPT = (
        "You are merging structured knowledge blocks from multiple chunks of the same document. "
        "Merge sections of the same type (## Thesis, ## Mechanism, etc.) into coherent, non-repetitive content.\n\n"
        "RULES:\n"
        "- Remove duplicate information across sections\n"
        "- Combine overlapping points into single comprehensive statements\n"
        "- Preserve ALL facts, numbers, examples, frameworks, quotes\n"
        "- Maintain the source language\n"
        "- Keep ## heading structure, use only ## headings\n"
        "- If a section becomes empty after dedup, omit it\n"
        "- Output ONLY the merged markdown content\n"
        "- DO NOT add any introductory text, conclusions, summaries about the merging process\n"
        "- DO NOT write phrases like 'Here is the merged text', 'Ready for use', 'Based on the provided fragments'\n"
        "- DO NOT add any meta-commentary — start directly with the first ## heading\n"
        "- The output must look like a final document section, not a response to a request"
    )

    def __init__(self, llm_client, json_mode: bool = False, work_dir: Path | None = None):
        self._transformer = Transformer(llm_client, json_mode=json_mode)
        self._chunker = SemanticChunker()
        self._client = llm_client
        self._json_mode = json_mode
        self._work_dir = work_dir
        self._parser = MarkdownOutputParser()

    def _emit(self, status: str, **kwargs):
        if self._json_mode:
            obj = {"status": status, **kwargs}
            print(json.dumps(obj, ensure_ascii=False), flush=True)

    def _chat_with_streaming(self, prompt: str, system: str = "") -> str:
        """Call LLM with streaming and emit tokens as events."""
        result_parts = []
        token_count = 0
        batch_size = 20

        try:
            for token in self._client.chat_stream(prompt=prompt, system=system):
                result_parts.append(token)
                token_count += 1
                if token_count % batch_size == 0:
                    self._emit("llm_token", tokens="".join(result_parts[-batch_size:]))
        except Exception:
            if not result_parts:
                result_parts.append(self._client.chat(prompt=prompt, system=system))

        return "".join(result_parts)

    def _chunk_dir(self, source_path: str) -> Path:
        return (self._work_dir / "chunks") if self._work_dir else Path("chunks")

    def _chunk_file(self, chunk_dir: Path, index: int) -> Path:
        return chunk_dir / f"chunk_{index:03d}.md"

    def _meta_file(self, chunk_dir: Path) -> Path:
        return chunk_dir / "metadata.yaml"

    def _is_processed(self, chunk_dir: Path, index: int) -> bool:
        return self._chunk_file(chunk_dir, index).exists()

    def _save_chunk_result(self, chunk_dir: Path, index: int, structured: str, meta: DocumentMetadata):
        chunk_dir.mkdir(parents=True, exist_ok=True)
        self._chunk_file(chunk_dir, index).write_text(structured, encoding="utf-8")

        meta_dict = {
            "title": unidecode(meta.title) if meta.title else "",
            "author": meta.author,
            "language": meta.language,
            "domains": meta.domains,
            "topics": meta.topics,
            "core_thesis": meta.core_thesis,
            "mental_models": meta.mental_models,
            "claims": [{"text": c.text, "type": c.type, "confidence": c.confidence} for c in meta.claims],
            "takeaways": meta.takeaways,
            "key_terms": meta.key_terms,
            "summary": meta.summary,
            "key_insights": meta.key_insights,
        }
        existing = {}
        if self._meta_file(chunk_dir).exists():
            existing = yaml.safe_load(self._meta_file(chunk_dir).read_text(encoding="utf-8")) or {}
        existing[str(index)] = meta_dict
        self._meta_file(chunk_dir).write_text(yaml.dump(existing, allow_unicode=True, default_flow_style=False), encoding="utf-8")

    def _load_all_chunks(self, chunk_dir: Path, total: int) -> list[tuple[str, dict]]:
        results = []
        for i in range(total):
            path = self._chunk_file(chunk_dir, i)
            if path.exists():
                results.append((path.read_text(encoding="utf-8"), None))
        return results

    def _load_all_metadata(self, chunk_dir: Path) -> list[dict]:
        if not self._meta_file(chunk_dir).exists():
            return []
        data = yaml.safe_load(self._meta_file(chunk_dir).read_text(encoding="utf-8")) or {}
        indices = sorted(int(k) for k in data.keys())
        return [data[str(i)] for i in indices]

    def map_reduce(self, text: str, existing_metadata: DocumentMetadata = None, source_path: str = "") -> tuple[str, DocumentMetadata]:
        chunk_dir = self._chunk_dir(source_path)
        chunk_dir.mkdir(parents=True, exist_ok=True)

        chunks = self._chunker.split(text)
        total = len(chunks)

        self._emit("map_start", total_chunks=total, work_dir=str(chunk_dir))

        shared_meta = existing_metadata

        for i, chunk in enumerate(chunks):
            if self._is_processed(chunk_dir, i):
                self._emit("map_skip", current=i + 1, total=total)
                continue

            self._emit("map_chunk", current=i + 1, total=total, chars=len(chunk.text), chunk_id=i)
            try:
                structured, meta = self._transformer.transform_chunk(
                    chunk.text, chunk.index, chunk.total, shared_meta
                )
                self._save_chunk_result(chunk_dir, i, structured, meta)
                if not shared_meta and meta.domains:
                    shared_meta = meta
                self._emit("map_chunk_done", current=i + 1, total=total, chunk_id=i)
            except Exception as e:
                self._emit("map_chunk_error", current=i + 1, error=str(e), chunk_id=i)
                error_file = self._chunk_file(chunk_dir, i).with_suffix(".error")
                error_file.write_text(str(e), encoding="utf-8")

        self._emit("map_done", total_chunks=total)

        return self._reduce(chunk_dir, total, existing_metadata)

    def _merged_section_file(self, chunk_dir: Path, section_name: str) -> Path:
        safe = re.sub(r'[^a-zA-Z0-9а-яА-ЯёЁ_-]', '_', section_name)
        return chunk_dir / f"merged_{safe}.md"

    def _reduce(self, chunk_dir: Path, total: int, existing_metadata: DocumentMetadata = None) -> tuple[str, DocumentMetadata]:
        meta_results = self._load_all_metadata(chunk_dir)
        if not meta_results:
            raise ValueError("No processed chunks found")

        merged_meta = self._merge_metadata(meta_results, existing_metadata)

        self._emit("reduce_start", total_chunks=total)

        sections = self._collect_all_sections(chunk_dir, total)
        final_content = self._merge_sections_with_threshold(sections, chunk_dir)

        # Save sections to workspace_dir / "sections"
        workspace_dir = self._work_dir
        if workspace_dir:
            sections_dir = workspace_dir / "sections"
            sections_dir.mkdir(parents=True, exist_ok=True)
            section_names = []
            for section_name, contents in sections.items():
                if section_name == "_preamble":
                    continue
                safe = re.sub(r'[^a-zA-Z0-9а-яА-ЯёЁ_-]', '_', section_name)
                section_file = sections_dir / f"{safe}.md"
                combined = "\n\n".join(c for c in contents if c)
                if combined:
                    section_file.write_text(f"## {section_name}\n\n{combined}", encoding="utf-8")
                    section_names.append(section_name)
            if section_names:
                self._emit("sections_saved", sections=section_names)

        self._emit("reduce_done")
        return final_content, merged_meta

    def _collect_all_sections(self, chunk_dir: Path, total: int) -> dict[str, list[str]]:
        sections = {}
        for i in range(total):
            path = self._chunk_file(chunk_dir, i)
            if path.exists():
                text = path.read_text(encoding="utf-8")
                self._extract_sections(text, sections)
        return sections

    def _merge_sections_with_threshold(self, sections: dict[str, list[str]], chunk_dir: Path, threshold: int = 8000) -> str:
        parts = []
        for section_name, contents in sections.items():
            merged_file = self._merged_section_file(chunk_dir, section_name)
            if merged_file.exists():
                self._emit("reduce_skip", section=section_name)
                parts.append(merged_file.read_text(encoding="utf-8"))
                continue

            combined = "\n\n".join(c for c in contents if c)
            if not combined:
                continue

            if len(combined) <= threshold:
                merged = self._merge_single_section(combined, section_name)
            else:
                merged = self._merge_large_section(combined, section_name, threshold, chunk_dir)

            if merged:
                merged_file.write_text(merged, encoding="utf-8")
                parts.append(merged)

        return "\n\n".join(parts).strip()

    def _merge_single_section(self, content: str, section_name: str) -> str:
        if section_name == "_preamble":
            return content
        if len(content) < 500:
            return f"## {section_name}\n\n{content}"
        try:
            response = self._chat_with_streaming(
                prompt=f"Merge and deduplicate these content blocks:\n\n{content}",
                system=self.DEDUP_SYSTEM_PROMPT,
            )
            cleaned = self._strip_meta_commentary(response.strip())
            return f"## {section_name}\n\n{cleaned}"
        except Exception:
            return f"## {section_name}\n\n{content}"

    def _merge_large_section(self, content: str, section_name: str, threshold: int, chunk_dir: Path) -> str:
        chunks = self._split_by_paragraphs(content, threshold)
        merged_parts = []

        for i, chunk in enumerate(chunks):
            sub_file = chunk_dir / f"merged_sub_{re.sub(r'[^a-zA-Z0-9а-яА-ЯёЁ_-]', '_', section_name)}_{i}.md"
            if sub_file.exists():
                merged_parts.append(sub_file.read_text(encoding="utf-8"))
                self._emit("merge_subsection_skip", section=section_name, part=i + 1)
                continue

            self._emit("merge_subsection", section=section_name, part=i + 1, total=len(chunks))
            try:
                response = self._chat_with_streaming(
                    prompt=f"Merge and deduplicate these content blocks:\n\n{chunk}",
                    system=self.DEDUP_SYSTEM_PROMPT,
                )
                merged_parts.append(response.strip())
                sub_file.write_text(response.strip(), encoding="utf-8")
            except Exception:
                merged_parts.append(chunk)

        if len(merged_parts) == 1:
            result = merged_parts[0]
        else:
            combined = "\n\n".join(merged_parts)
            if len(combined) <= threshold:
                try:
                    response = self._chat_with_streaming(
                        prompt=f"Merge these subsections into one:\n\n{combined}",
                        system=self.DEDUP_SYSTEM_PROMPT,
                    )
                    result = response.strip()
                except Exception:
                    result = combined
            else:
                result = combined

        if section_name == "_preamble":
            return result
        return f"## {section_name}\n\n{result}"

    def _strip_meta_commentary(self, text: str) -> str:
        lines = text.split("\n")
        filtered = []
        skip_prefixes = (
            "готово",
            "вот объедин",
            "вот merged",
            "вот результат",
            "here is the merged",
            "here is the combined",
            "here is the merged and deduplicated",
            "ready for use",
            "готово для использования",
            "based on the provided",
            "на основе предоставленных",
            "предоставленн",
            "удалены повторы",
            "eliminated duplicates",
            "preserved all",
            "сохранены все",
            "all key ideas",
            "все ключевые",
            "объединены в логичные",
            "merged into logical",
            "без потери",
            "without losing",
            "упорядочиванием",
            "reorganized",
            "сохранением структуры",
            "preserving the structure",
            "без потери ни одного",
            "final version",
            "финальная версия",
            "итоговый текст",
            "final text",
            "merged text",
            "объединённый текст",
            "дедуплицированный",
            "deduplicated",
        )
        for line in lines:
            stripped = line.strip().lower()
            if not stripped:
                continue
            if any(stripped.startswith(p) for p in skip_prefixes):
                continue
            if stripped.startswith("---") and not stripped.startswith("##"):
                continue
            filtered.append(line)

        return "\n".join(filtered).strip()

    def _split_by_paragraphs(self, text: str, max_size: int) -> list[str]:
        paragraphs = text.split("\n\n")
        chunks = []
        current = []
        current_size = 0

        for p in paragraphs:
            p_size = len(p)
            if current_size + p_size > max_size and current:
                chunks.append("\n\n".join(current))
                current = [p]
                current_size = p_size
            else:
                current.append(p)
                current_size += p_size

        if current:
            chunks.append("\n\n".join(current))
        return chunks

    def _merge_metadata(self, meta_results: list[dict], existing_metadata: DocumentMetadata) -> DocumentMetadata:
        merged = existing_metadata or DocumentMetadata(title="", source="", doc_type="")
        all_domains = set()
        all_mental_models = set()
        all_takeaways = []
        all_key_terms = set()
        all_claims = []

        all_topics = set()

        for meta in meta_results:
            if meta.get("domains"):
                all_domains.update(meta["domains"])
            if meta.get("topics"):
                all_topics.update(meta["topics"])
            if meta.get("mental_models"):
                all_mental_models.update(meta["mental_models"])
            if meta.get("takeaways"):
                for tw in meta["takeaways"]:
                    all_takeaways.append(tw.get("text", tw) if isinstance(tw, dict) else tw)
            if meta.get("key_terms"):
                all_key_terms.update(meta["key_terms"])
            if meta.get("claims"):
                all_claims.extend([Claim(**c) for c in meta["claims"]])
            if meta.get("core_thesis") and not merged.core_thesis:
                merged.core_thesis = meta["core_thesis"]
            if meta.get("language") and not merged.language:
                merged.language = meta["language"]
            if meta.get("author") and not merged.author:
                merged.author = meta["author"]
            if meta.get("title") and not merged.title:
                merged.title = unidecode(meta["title"])

        merged.domains = list(all_domains)
        merged.topics = list(all_topics) if all_topics else list(all_domains)
        merged.mental_models = list(all_mental_models)
        merged.takeaways = self._dedup_list(all_takeaways)
        merged.key_terms = list(all_key_terms)
        merged.claims = self._dedup_claims(all_claims)

        return merged

    def _extract_sections(self, structured: str, sections: dict[str, list[str]]):
        current_section = "_preamble"
        current_content = []

        for line in structured.split("\n"):
            match = re.match(r"^## (.+)$", line.strip())
            if match:
                if current_content:
                    sections.setdefault(current_section, []).append("\n".join(current_content).strip())
                current_section = match.group(1).strip()
                current_content = []
            else:
                if line.strip():
                    current_content.append(line)

        if current_content:
            sections.setdefault(current_section, []).append("\n".join(current_content).strip())

    def _count_sections(self, content: str) -> int:
        return len(re.findall(r"^## ", content, re.MULTILINE))

    def _dedup_list(self, items: list[str]) -> list[str]:
        seen = set()
        result = []
        for item in items:
            if isinstance(item, dict):
                text = item.get("text", item.get("takeaway", str(item)))
            else:
                text = str(item)
            normalized = text.strip().lower()
            if normalized not in seen:
                seen.add(normalized)
                result.append(text)
        return result

    def _dedup_claims(self, claims: list[Claim]) -> list[Claim]:
        seen = set()
        result = []
        for claim in claims:
            normalized = claim.text.strip().lower()[:100]
            if normalized not in seen:
                seen.add(normalized)
                result.append(claim)
        return result

    def cleanup(self, source_path: str):
        chunk_dir = self._chunk_dir(source_path)
        if chunk_dir.exists():
            import shutil
            shutil.rmtree(chunk_dir)
