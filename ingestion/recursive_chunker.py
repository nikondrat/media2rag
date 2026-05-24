import hashlib
import re

from domain.chunk import Chunk, ParentChunk
from ingestion.normalizer import NormalizedDocument, Section


class RecursiveChunker:
    SEPARATORS = ["\n\n", "\n", ". ", " ", ""]
    CHILD_TOKENS = 256
    PARENT_TOKENS = 1024
    OVERLAP_RATIO = 0.15

    def __init__(self, child_tokens: int = CHILD_TOKENS, parent_tokens: int = PARENT_TOKENS):
        self.child_tokens = child_tokens
        self.parent_tokens = parent_tokens

    def chunk(self, doc: NormalizedDocument, document_id: str) -> tuple[list[Chunk], list[ParentChunk]]:
        all_chunks: list[Chunk] = []
        all_parents: list[ParentChunk] = []

        if doc.sections:
            for section in doc.sections:
                section_name = section.heading or "Introduction"
                parents, chunks = self._chunk_section(section, section_name, document_id)
                all_parents.extend(parents)
                all_chunks.extend(chunks)
        else:
            parents, chunks = self._chunk_section_text(doc.text, "", document_id)
            all_parents.extend(parents)
            all_chunks.extend(chunks)

        for table in doc.tables:
            parent_id = self._make_id(document_id, f"table_{len(all_parents)}")
            parent = ParentChunk(
                id=parent_id,
                document_id=document_id,
                content=table,
                section="Table",
                metadata={"content_type": "table"},
            )
            chunk = Chunk(
                id=self._make_id(document_id, f"chunk_table_{len(all_chunks)}"),
                document_id=document_id,
                parent_id=parent_id,
                content=table,
                section="Table",
                chunk_index=len(all_chunks),
                content_type="table",
            )
            all_parents.append(parent)
            all_chunks.append(chunk)

        return all_chunks, all_parents

    def _chunk_section(
        self, section: Section, section_name: str, document_id: str
    ) -> tuple[list[ParentChunk], list[Chunk]]:
        return self._chunk_section_text(section.content, section_name, document_id)

    def _chunk_section_text(
        self, text: str, section_name: str, document_id: str
    ) -> tuple[list[ParentChunk], list[Chunk]]:
        parents: list[ParentChunk] = []
        chunks: list[Chunk] = []

        parent_texts = self._split_text(text, self.parent_tokens, self.SEPARATORS)

        for pi, parent_text in enumerate(parent_texts):
            parent_id = self._make_id(document_id, f"parent_{section_name}_{pi}")
            parent = ParentChunk(
                id=parent_id,
                document_id=document_id,
                content=parent_text,
                section=section_name,
            )
            parents.append(parent)

            child_texts = self._split_text(parent_text, self.child_tokens, self.SEPARATORS)
            overlap_chars = int(self.child_tokens * 4 * self.OVERLAP_RATIO)

            for ci, child_text in enumerate(child_texts):
                if ci > 0 and overlap_chars > 0:
                    prev_text = child_texts[ci - 1]
                    overlap = prev_text[-overlap_chars:]
                    child_text = overlap + "\n" + child_text

                chunk = Chunk(
                    id=self._make_id(document_id, f"chunk_{section_name}_{pi}_{ci}"),
                    document_id=document_id,
                    parent_id=parent_id,
                    content=child_text.strip(),
                    section=section_name,
                    chunk_index=len(chunks),
                    content_type="text",
                )
                chunks.append(chunk)

        return parents, chunks

    def _split_text(self, text: str, max_tokens: int, separators: list[str]) -> list[str]:
        char_limit = max_tokens * 4

        if len(text) <= char_limit:
            return [text] if text.strip() else []

        sep = separators[0]
        remaining_seps = separators[1:]

        if sep == "":
            return [text[:char_limit]]

        parts = text.split(sep) if sep else list(text)

        chunks: list[str] = []
        current_parts: list[str] = []
        current_len = 0

        for part in parts:
            part_len = len(part) + len(sep)
            if current_len + part_len > char_limit and current_parts:
                chunks.append(sep.join(current_parts))
                current_parts = [part]
                current_len = part_len
            else:
                current_parts.append(part)
                current_len += part_len

        if current_parts:
            chunks.append(sep.join(current_parts))

        result: list[str] = []
        for chunk in chunks:
            if len(chunk) > char_limit and remaining_seps:
                sub_chunks = self._split_text(chunk, max_tokens, remaining_seps)
                result.extend(sub_chunks)
            elif chunk.strip():
                result.append(chunk)

        return result

    @staticmethod
    def _make_id(document_id: str, suffix: str) -> str:
        raw = f"{document_id}:{suffix}"
        return hashlib.sha256(raw.encode()).hexdigest()[:16]
