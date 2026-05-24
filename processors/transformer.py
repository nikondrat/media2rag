import json
import re
from typing import Optional
from unidecode import unidecode

from domain.document import Claim, DocumentMetadata
from processors.output_parser import MarkdownOutputParser


class Transformer:
    """Structure cleaned content into knowledge blocks with typed metadata."""

    _EXTRACT_STRUCTURE_PROMPT = (
        "You are a metadata extractor. Analyze the content and extract ONLY metadata.\n\n"
        "OUTPUT (ALL IN ENGLISH, regardless of source language):\n"
        "1. title — concise English title\n"
        "2. author — author/speaker name, or 'Unknown'\n"
        "3. language — ISO 639-1 code of the SOURCE language (e.g. 'ru', 'en')\n"
        "4. domains — 2-4 domain tags: investing, entrepreneurship, marketing, trading, psychology, etc.\n"
        "5. core_thesis — ONE sentence: the single most important argument\n"
        "6. mental_models — thinking frameworks used\n"
        "7. claims — key statements with type (argument/fact/framework/prediction) and confidence\n"
        "8. takeaways — actionable items\n"
        "9. key_terms — 5-8 keywords for embedding retrieval (in source language)\n"
        "10. section_outline — list of H1 sections with brief descriptions\n\n"
        "Output valid YAML frontmatter between --- delimiters.\n"
        "Start directly with --- (no preamble, no markdown)."
    )

    _WRITE_CONTENT_PROMPT = (
        "You are a content writer. Produce structured body content based on the outline.\n\n"
        "CRITICAL: Write the ENTIRE body in the SOURCE LANGUAGE specified in metadata. "
        "NEVER switch to English unless the source content itself contains English.\n\n"
        "SECTIONS (use ONLY these H1 headings as applicable):\n"
        "- # Thesis — the core argument\n"
        "- # Mechanism — how the system/process works\n"
        "- # Pattern — recurring sequences or rules\n"
        "- # Evidence — data, examples, case studies\n"
        "- # Framework — decision models, mental models, how-to\n"
        "- # Steps — numbered actionable steps\n"
        "- # Definitions — key terms explained\n"
        "- # Quotes — verbatim impactful statements\n\n"
        "RULES:\n"
        "- Remove ALL: greetings, sign-offs, CTA, @mentions, filler, self-promotion\n"
        "- Preserve ALL: facts, numbers, examples, frameworks, quotes, names\n"
        "- Each section must contain ONLY signal — no water, no transitions\n"
        "- Keep the ORIGINAL language of the source for the body\n"
        "- Use ## for sub-sections with descriptive names (NOT the reserved H1 names)\n"
        "- NEVER use reserved H1 names as ## sub-headings\n"
        "- NEVER add commentary, summaries, or 'merged version' text\n"
        "- NEVER output 'Here is', 'The merged', 'Below is', or any framing\n"
        "- If a section has nothing to contribute, omit it entirely\n"
        "- Output ONLY the body content — no preamble, no postamble"
    )

    SYSTEM_PROMPT = (
        "You are a knowledge extraction and structuring specialist. "
        "Analyze the content and produce TWO outputs:\n\n"

        "## METADATA (ALL IN ENGLISH, regardless of source language)\n"
        "1. title — concise English title\n"
        "2. author — author/speaker name, or 'Unknown'\n"
        "3. language — ISO 639-1 code of the SOURCE language (e.g. 'ru', 'en')\n"
        "4. domains — 2-4 domain tags: investing, entrepreneurship, marketing, trading, psychology, etc.\n"
        "5. core_thesis — ONE sentence: the single most important argument\n"
        "6. mental_models — thinking frameworks used: systems-thinking, capital-allocation, first-principles, etc.\n"
        "7. claims — extract key statements with type and confidence (ALL IN ENGLISH):\n"
        "   - type: 'argument' (author's opinion), 'fact' (verifiable), 'framework' (decision model), 'prediction'\n"
        "   - confidence: 'strong', 'moderate', 'speculative'\n"
        "8. takeaways — actionable items the reader can apply\n"
        "9. key_terms — 5-8 keywords for embedding retrieval (in source language for better search)\n\n"

        "## BODY (PRESERVE SOURCE LANGUAGE)\n"
        "Restructure content into typed knowledge blocks. Use ONLY these H1 sections as applicable:\n"
        "- # Thesis — the core argument\n"
        "- # Mechanism — how the system/process works\n"
        "- # Pattern — recurring sequences or rules\n"
        "- # Evidence — data, examples, case studies\n"
        "- # Framework — decision models, mental models, how-to\n"
        "- # Steps — numbered actionable steps\n"
        "- # Definitions — key terms explained\n"
        "- # Quotes — verbatim impactful statements worth remembering\n\n"

        "Use ## for sub-sections within each H1 section when needed.\n"
        "NEVER use 'Thesis', 'Mechanism', 'Pattern', 'Evidence', 'Framework', 'Steps', "
        "'Definitions', or 'Quotes' as ## sub-headings — those names are reserved for H1-only.\n"
        "Use descriptive sub-headings specific to the content (e.g., ## Уровень смысла, ## Hiring Process).\n\n"

        "RULES FOR BODY:\n"
        "- Remove ALL: greetings, sign-offs, CTA, @mentions, 'in my previous post', 'subscribe', 'save this'\n"
        "- Remove ALL: rhetorical questions, filler, self-promotion, links to other content\n"
        "- Preserve ALL: facts, numbers, examples, frameworks, quotes, specific names\n"
        "- Each section must contain ONLY signal — no water, no transitions between sections\n"
        "- Keep the ORIGINAL language of the source for the body\n"
        "- If a section has nothing to contribute, omit it entirely\n\n"

        "CRITICAL:\n"
        "- ALL metadata values MUST be in English\n"
        "- structured_content body MUST be in the source language\n"
        "- Output valid YAML frontmatter between --- delimiters\n"
        "- Use inline arrays for simple lists: [item1, item2]\n"
        "- Use nested YAML for claims with attributes\n"
        "- Start output directly with --- (no markdown, no preamble)\n"
        "- End frontmatter with --- then write the body"
    )

    def __init__(self, llm_client, json_mode: bool = False, reasoning: bool = False):
        self._client = llm_client
        self._parser = MarkdownOutputParser()
        self._json_mode = json_mode
        self._reasoning = reasoning

    def _emit(self, status: str, **kwargs):
        if self._json_mode:
            obj = {"status": status, **kwargs}
            print(json.dumps(obj, ensure_ascii=False), flush=True)
        else:
            messages = {
                "extract_structure_start": "    🔍 Extracting structure & metadata...",
                "extract_structure_done": f"    ✅ Structure extracted: {kwargs.get('fields', 'metadata + outline')}",
                "write_content_start": "    ✍️  Writing structured content...",
                "write_content_done": f"    ✅ Content written ({kwargs.get('chars', 0)} chars)",
            }
            msg = messages.get(status, f"    [{status}] {kwargs}")
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

    def _extract_structure(self, text: str, content_classification: dict = None) -> tuple[dict, str]:
        """Extract metadata and section outline from text.

        Returns (metadata_dict, section_outline_string).
        """
        self._emit("extract_structure_start")
        system = self._EXTRACT_STRUCTURE_PROMPT
        if content_classification:
            from processors.content_router import TYPE_MODIFIERS
            modifier = TYPE_MODIFIERS.get(content_classification.get("content_type", "monologue"), "")
            system = system + modifier

        prompt = f"Extract metadata and section outline from this content:\n\n{text}"
        response = self._chat_with_streaming(prompt=prompt, system=system)
        parsed = self._parser.parse(response)

        outline = ""
        if "section_outline" in (parsed.metadata or {}):
            outline_items = parsed.metadata.get("section_outline", [])
            if isinstance(outline_items, list):
                outline = "\n".join(f"- {item}" if isinstance(item, str) else f"- {item}" for item in outline_items)

        self._emit("extract_structure_done", fields=f"title={parsed.metadata.get('title', 'N/A')}, domains={parsed.metadata.get('domains', [])}")
        return parsed.metadata or {}, outline

    def _write_content(self, text: str, outline: str, metadata: dict, content_classification: dict = None) -> str:
        """Write structured body content using the outline as context.

        Returns the structured body as a string.
        """
        self._emit("write_content_start")
        system = self._WRITE_CONTENT_PROMPT
        if content_classification:
            from processors.content_router import TYPE_MODIFIERS
            modifier = TYPE_MODIFIERS.get(content_classification.get("content_type", "monologue"), "")
            system = system + modifier

        prompt = (
            f"## Section Outline\n{outline}\n\n"
            f"## Metadata\n"
            f"- Title: {metadata.get('title', '')}\n"
            f"- Core Thesis: {metadata.get('core_thesis', '')}\n"
            f"- Domains: {', '.join(metadata.get('domains', []))}\n\n"
            f"## Source Content\n{text}\n\n"
            "Produce the structured body content following the outline above."
        )

        result = self._chat_with_streaming(prompt=prompt, system=system)
        self._emit("write_content_done", chars=len(result))
        return result

    def transform(self, compressed_text: str, existing_metadata: DocumentMetadata = None) -> tuple[str, DocumentMetadata]:
        return self._transform_with_context(compressed_text, existing_metadata)

    def transform_chunk_content(
        self,
        chunk_text: str,
        content_classification: dict = None,
    ) -> str:
        """Transform chunk — write content only, skip metadata extraction."""
        self._emit("write_content_start")
        system = self._WRITE_CONTENT_PROMPT
        if content_classification:
            from processors.content_router import TYPE_MODIFIERS
            modifier = TYPE_MODIFIERS.get(content_classification.get("content_type", "monologue"), "")
            system = system + modifier

        prompt = (
            f"## Source Content\n{chunk_text}\n\n"
            "Produce the structured body content in the source language."
        )
        result = self._chat_with_streaming(prompt=prompt, system=system)
        self._emit("write_content_done", chars=len(result))
        return result

    def transform_chunk(
        self,
        chunk_text: str,
        chunk_index: int,
        total_chunks: int,
        shared_metadata: DocumentMetadata = None,
        content_classification: dict = None,
    ) -> tuple[str, DocumentMetadata]:
        body = self.transform_chunk_content(chunk_text, content_classification)
        if not body:
            return chunk_text, shared_metadata or DocumentMetadata(title="", source="", doc_type="")
        structured = self._format_content(body, shared_metadata or DocumentMetadata(title="", source="", doc_type=""))
        return structured, shared_metadata or DocumentMetadata(title="", source="", doc_type="")

    def extract_global_metadata(
        self,
        full_text: str,
        content_classification: dict = None,
    ) -> tuple[dict, DocumentMetadata]:
        """Extract global metadata from full document text.

        Returns (metadata_dict, DocumentMetadata).
        """
        metadata_dict, outline = self._extract_structure(full_text, content_classification)

        if not metadata_dict:
            return {}, DocumentMetadata(title="", source="", doc_type="")

        claims = []
        for c in metadata_dict.get("claims", []):
            if isinstance(c, dict):
                claims.append(Claim(
                    text=c.get("text", ""),
                    type=c.get("type", "argument"),
                    confidence=c.get("confidence", "strong"),
                ))

        domains = metadata_dict.get("domains", [])
        title = metadata_dict.get("title", "")
        if title:
            title = unidecode(title)
        metadata = DocumentMetadata(
            title=title,
            author=metadata_dict.get("author", "Unknown"),
            language=metadata_dict.get("language", ""),
            domains=domains,
            topics=domains,
            core_thesis=metadata_dict.get("core_thesis", ""),
            mental_models=metadata_dict.get("mental_models", []),
            claims=claims,
            takeaways=metadata_dict.get("takeaways", []),
            key_terms=metadata_dict.get("key_terms", []),
            source="",
            doc_type="",
            summary=metadata_dict.get("core_thesis", ""),
            key_insights=[c.text for c in claims if c.type in ("framework", "prediction")],
        )
        return metadata_dict, metadata

    def _transform_with_context(
        self,
        text: str,
        existing_metadata: DocumentMetadata = None,
        chunk_context: str = None,
        content_classification: dict = None,
    ) -> tuple[str, DocumentMetadata]:
        metadata_dict, outline = self._extract_structure(text, content_classification)

        if not metadata_dict:
            return text, existing_metadata or DocumentMetadata(title="", source="", doc_type="")

        body = self._write_content(text, outline, metadata_dict, content_classification)

        claims = []
        for c in metadata_dict.get("claims", []):
            if isinstance(c, dict):
                claims.append(Claim(
                    text=c.get("text", ""),
                    type=c.get("type", "argument"),
                    confidence=c.get("confidence", "strong"),
                ))

        domains = metadata_dict.get("domains", [])
        title = metadata_dict.get("title", existing_metadata.title if existing_metadata else "")
        if title:
            title = unidecode(title)
        metadata = DocumentMetadata(
            title=title,
            author=metadata_dict.get("author", existing_metadata.author if existing_metadata else "Unknown"),
            language=metadata_dict.get("language", ""),
            domains=domains,
            topics=domains,
            core_thesis=metadata_dict.get("core_thesis", ""),
            mental_models=metadata_dict.get("mental_models", []),
            claims=claims,
            takeaways=metadata_dict.get("takeaways", []),
            key_terms=metadata_dict.get("key_terms", []),
            source=existing_metadata.source if existing_metadata else "",
            doc_type=existing_metadata.doc_type if existing_metadata else "",
            summary=metadata_dict.get("core_thesis", ""),
            key_insights=[c.text for c in claims if c.type in ("framework", "prediction")],
        )

        structured = self._format_content(body, metadata)
        return structured, metadata

    def _format_content(self, content: str, metadata: DocumentMetadata) -> str:
        if not content.startswith("#"):
            title = metadata.title or "Untitled"
            content = f"# {title}\n\n{content}"

        content = re.sub(r'\n{3,}', '\n\n', content)
        return content.strip()
