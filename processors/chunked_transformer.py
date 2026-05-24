import json
import re
from pathlib import Path
from typing import Optional
from unidecode import unidecode

import yaml

from domain.document import Claim, DocumentMetadata
from processors.chunker import SemanticChunker
from processors.transformer import Transformer
from processors.output_parser import MarkdownOutputParser
from processors.evaluator import ContentEvaluator
from processors.content_router import ContentRouter


class ChunkedTransformer:
    """Incremental map-reduce transformer for large documents.

    Each chunk is processed independently and saved to disk.
    Final merge+dedup reads all intermediate files.
    Supports resume: already-processed chunks are skipped.
    """

    DEDUP_SYSTEM_PROMPT = (
        "You are merging content blocks from the same document section. "
        "Merge them into a single coherent block without repetition or filler.\n\n"
        "CRITICAL — OUTPUT STRUCTURE RULES:\n"
        "- Remove ALL sub-headings named 'Thesis', 'Mechanism', 'Pattern', 'Evidence', 'Framework', "
        "'Steps', 'Definitions', or 'Quotes'. These are H1-level section names and must NOT appear "
        "as H2 sub-headings anywhere. Convert their content into plain paragraphs.\n"
        "- If the content has natural sub-topics, use descriptive ## sub-headings "
        "that are specific to the content (e.g., ## Уровень смысла, ## Hiring Process)\n"
        "- Never repeat the same sub-heading name twice — each ## must be unique within the section\n"
        "- If the content is a simple list or paragraph, output it without any sub-headings at all\n"
        "- NEVER use # headings — only ## if absolutely needed\n"
        "- Output ONLY the merged content — NO preamble, NO commentary, NO meta-text\n"
        "- DO NOT start with 'Here is', 'The merged', 'Based on', 'Готово', or any framing\n"
        "- Preserve ALL facts, numbers, examples, frameworks, quotes, names\n"
        "- Maintain the source language\n"
        "- If nothing meaningful remains, output nothing"
    )

    POST_CLEANUP_PROMPT = (
        "You are a quality gate for a markdown document. Review and fix ONLY structural issues:\n\n"
        "1. Remove any preamble or commentary from an AI assistant (lines like 'Here is the merged version', "
        "'Here's the **merged**', 'Let me know if you', 'I'd be happy to', 'Ready for use', etc.) — "
        "including preceding '---' separator if present\n"
        "2. NEVER use 'Thesis', 'Mechanism', 'Pattern', 'Evidence', 'Framework', "
        "'Steps', 'Definitions', or 'Quotes' as ## sub-headings — they must only be # H1 headings\n"
        "3. If there are no H1 (#) headings, add them starting with # Thesis\n"
        "4. Remove excessive blank lines (more than 2 consecutive)\n"
        "5. Preserve ALL content — facts, numbers, examples, quotes, names. "
        "DO NOT rewrite, rephrase, or summarize anything.\n"
        "6. Output ONLY in the SAME language as the input document. "
        "If input is Russian, output MUST be in Russian. "
        "NEVER switch languages.\n"
        "7. Output ONLY the cleaned markdown — no explanations, no commentary"
    )

    def __init__(self, llm_client, json_mode: bool = False, work_dir: Path | None = None, reasoning: bool = False,
                 evaluator: Optional[ContentEvaluator] = None, router: Optional[ContentRouter] = None):
        self._transformer = Transformer(llm_client, json_mode=json_mode, reasoning=reasoning)
        self._chunker = SemanticChunker()
        self._client = llm_client
        self._json_mode = json_mode
        self._work_dir = work_dir
        self._parser = MarkdownOutputParser()
        self._reasoning = reasoning
        self._evaluator = evaluator
        self._router = router

    def _emit(self, status: str, **kwargs):
        if self._json_mode:
            obj = {"status": status, **kwargs}
            print(json.dumps(obj, ensure_ascii=False), flush=True)
        else:
            messages = {
                "map_start": f"  📦 Map-reduce: {kwargs.get('total_chunks')} chunks to process",
                "map_chunk": f"  📄 Processing chunk {kwargs.get('current')}/{kwargs.get('total')} ({kwargs.get('chars')} chars)",
                "map_skip": f"  ⏭️  Skipping chunk {kwargs.get('current')}/{kwargs.get('total')} (already processed)",
                "map_chunk_done": f"  ✅ Chunk {kwargs.get('current')}/{kwargs.get('total')} done",
                "map_chunk_error": f"  ❌ Error on chunk {kwargs.get('current')}: {kwargs.get('error')}",
                "map_done": f"  📦 Map phase complete ({kwargs.get('total_chunks')} chunks)",
                "router_start": "  🎯 Classifying content type...",
                "router_done": f"  📋 Content type: {kwargs.get('content_type')}",
                "quality_check_start": "  🔍 Quality check: evaluating document...",
                "quality_check_done": f"  📊 Quality check complete: overall={kwargs.get('overall')}, revision={'needed' if kwargs.get('needs_revision') else 'passed'}",
                "reduce_start": f"  🔀 Reduce: merging {kwargs.get('total_chunks')} chunks...",
                "reduce_done": "  🔀 Reduce complete",
                "sections_saved": f"  📁 Sections saved: {', '.join(kwargs.get('sections', []))}",
                "post_process_start": "  🧹 Post-processing cleanup...",
                "post_process_done": "  ✅ Post-processing complete",
                "post_process_warn": f"  ⚠️  Post-process warnings: {kwargs.get('issues')}",
                "merge_subsection": f"  🔀 Merging sub {kwargs.get('part')}/{kwargs.get('total')} of {kwargs.get('section')}",
            }
            msg = messages.get(status, f"  [{status}] {kwargs}")
            print(msg, flush=True)

    def _chat_with_streaming(self, prompt: str, system: str = "") -> str:
        """Call LLM with streaming and emit tokens as events."""
        result_parts = []
        token_count = 0
        batch_size = 20

        try:
            for token in self._client.chat_stream(prompt=prompt, system=system, reasoning=self._reasoning):
                result_parts.append(token)
                token_count += 1
                if token_count % batch_size == 0:
                    self._emit("llm_token", tokens="".join(result_parts[-batch_size:]))
        except Exception:
            if not result_parts:
                result_parts.append(self._client.chat(prompt=prompt, system=system, reasoning=self._reasoning))

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
        classification = None

        if self._router:
            self._emit("router_start")
            classification = self._router.classify(text)
            self._emit("router_done", content_type=classification["content_type"])

        for i, chunk in enumerate(chunks):
            if self._is_processed(chunk_dir, i):
                self._emit("map_skip", current=i + 1, total=total)
                continue

            self._emit("map_chunk", current=i + 1, total=total, chars=len(chunk.text), chunk_id=i)
            try:
                structured, meta = self._transformer.transform_chunk(
                    chunk.text, chunk.index, chunk.total, shared_meta,
                    content_classification=classification,
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

        content, meta = self._reduce(chunk_dir, total, existing_metadata)
        content = self._post_process(content)
        content = self._llm_post_process(content)

        metadata_dict, global_meta = self._transformer.extract_global_metadata(content, classification)
        if metadata_dict:
            self._emit("metadata_extracted", title=global_meta.title, domains=global_meta.domains)
            global_meta.source = meta.source or (existing_metadata.source if existing_metadata else "")
            global_meta.doc_type = meta.doc_type or (existing_metadata.doc_type if existing_metadata else "")
            meta = global_meta

        return content, meta

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
                cached = merged_file.read_text(encoding="utf-8")
                cleaned = self._strip_bad_subheadings(cached)
                if cleaned != cached:
                    merged_file.write_text(cleaned, encoding="utf-8")
                parts.append(cleaned)
                self._emit("reduce_skip", section=section_name)
                continue

            combined = "\n\n".join(self._strip_bad_subheadings(c) for c in contents if c)
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
        cleaned_content = self._strip_bad_subheadings(content)
        if len(cleaned_content) < 500:
            return f"# {section_name}\n\n{cleaned_content}"
        try:
            response = self._chat_with_streaming(
                prompt=f"Merge and deduplicate these content blocks:\n\n{cleaned_content}",
                system=self.DEDUP_SYSTEM_PROMPT,
            )
            return f"# {section_name}\n\n{response.strip()}"
        except Exception:
            return f"# {section_name}\n\n{cleaned_content}"

    def _merge_large_section(self, content: str, section_name: str, threshold: int, chunk_dir: Path) -> str:
        chunks = self._split_by_paragraphs(self._strip_bad_subheadings(content), threshold)
        merged_parts = []

        for i, chunk in enumerate(chunks):
            sub_file = chunk_dir / f"merged_sub_{re.sub(r'[^a-zA-Z0-9а-яА-ЯёЁ_-]', '_', section_name)}_{i}.md"
            if sub_file.exists():
                merged_parts.append(self._strip_bad_subheadings(sub_file.read_text(encoding="utf-8")))
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
        return f"# {section_name}\n\n{result}"

    def _strip_bad_subheadings(self, text: str) -> str:
        BAD = re.compile(r"^##\s+(Thesis|Mechanism|Pattern|Evidence|Framework|Steps|Definitions|Quotes)\s*$", re.MULTILINE)
        return BAD.sub("", text)

    def _post_process(self, content: str) -> str:
        stripped = self._strip_bad_subheadings(content)

        issues = []
        if not re.search(r"^# ", stripped, re.MULTILINE):
            issues.append("no H1 headers")
        if re.search(r"(?i)^(let me know|here.*(?:merged|deduplicate)|i.d be happy|would you like|ready for use)", stripped, re.MULTILINE):
            issues.append("possible LLM preamble")

        if issues:
            self._emit("post_process_warn", issues=", ".join(issues))
            # Auto-fix: strip lines matching commentary patterns
            lines = stripped.split("\n")
            filtered = []
            COMMENTARY = re.compile(
                r"(?i)^(let me know if you|"
                r"here.s\s+(?:a|the)\s*\*{0,2}merged[\s,]+deduplicated|"
                r"here is\s+(?:a|the)\s*\*{0,2}merged[\s,]+deduplicated|"
                r"i.d be happy to|would you like me to|ready for use)"
            )
            skip = False
            for i, line in enumerate(lines):
                if skip:
                    skip = False
                    continue
                s = line.strip()
                if COMMENTARY.match(s):
                    skip = True
                    continue
                if s == "---" and i + 1 < len(lines):
                    nxt = lines[i + 1].strip()
                    if COMMENTARY.match(nxt):
                        skip = True
                        continue
                filtered.append(line)
            stripped = "\n".join(filtered)

        if not re.search(r"^# ", stripped, re.MULTILINE):
            stripped = "# Thesis\n\n" + stripped.strip()

        stripped = re.sub(r'\n{4,}', '\n\n\n', stripped)
        return stripped.strip()

    def _llm_post_process(self, content: str) -> str:
        if len(content) < 500:
            return content
        BAD = re.compile(r"^##\s+(Thesis|Mechanism|Pattern|Evidence|Framework|Steps|Definitions|Quotes)\s*$", re.MULTILINE)
        has_issues = bool(BAD.search(content))
        if not has_issues:
            PREAMBLE = re.compile(
                r"(?i)^(let me know if you|"
                r"here.s\s+(?:a|the)\s*\*{0,2}merged[\s,]+deduplicated|"
                r"here is\s+(?:a|the)\s*\*{0,2}merged[\s,]+deduplicated|"
                r"i.d be happy to|would you like me to|ready for use)"
            )
            has_issues = bool(PREAMBLE.search(content))
        if not has_issues:
            return content
        try:
            self._emit("post_process_start")
            response = self._chat_with_streaming(
                prompt=f"Review and fix structural issues in this markdown:\n\n{content}",
                system=self.POST_CLEANUP_PROMPT,
            )
            cleaned = re.sub(r'\n{4,}', '\n\n\n', response.strip())
            if abs(len(cleaned) - len(content)) / max(len(content), 1) > 0.30:
                self._emit("post_process_warn", issues="length guard triggered (>30% diff), using original")
                return content
            if not re.search(r"^# ", cleaned, re.MULTILINE):
                cleaned = "# Thesis\n\n" + cleaned
            self._emit("post_process_done")
            return cleaned
        except Exception:
            return content

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
            match = re.match(r"^# (.+)$", line.strip())
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
