import json
import re
from typing import Optional
from unidecode import unidecode

from domain.document import Claim, DocumentMetadata
from processors.output_parser import MarkdownOutputParser


class Transformer:
    """Structure cleaned content into knowledge blocks with typed metadata."""

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
        "Restructure content into typed knowledge blocks. Use ONLY these H2 sections as applicable:\n"
        "- ## Thesis — the core argument\n"
        "- ## Mechanism — how the system/process works\n"
        "- ## Pattern — recurring sequences or rules\n"
        "- ## Evidence — data, examples, case studies\n"
        "- ## Framework — decision models, mental models, how-to\n"
        "- ## Steps — numbered actionable steps\n"
        "- ## Definitions — key terms explained\n"
        "- ## Quotes — verbatim impactful statements worth remembering\n\n"

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

    def __init__(self, llm_client, json_mode: bool = False):
        self._client = llm_client
        self._parser = MarkdownOutputParser()
        self._json_mode = json_mode

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

    def transform(self, compressed_text: str, existing_metadata: DocumentMetadata = None) -> tuple[str, DocumentMetadata]:
        return self._transform_with_context(compressed_text, existing_metadata)

    def transform_chunk(
        self,
        chunk_text: str,
        chunk_index: int,
        total_chunks: int,
        shared_metadata: DocumentMetadata = None,
    ) -> tuple[str, DocumentMetadata]:
        context = f"This is chunk {chunk_index + 1} of {total_chunks} from a larger document."
        if shared_metadata:
            if shared_metadata.domains:
                context += f" Document domains: {', '.join(shared_metadata.domains)}."
            if shared_metadata.language:
                context += f" Source language: {shared_metadata.language}."
        return self._transform_with_context(chunk_text, shared_metadata, chunk_context=context)

    def _transform_with_context(
        self,
        text: str,
        existing_metadata: DocumentMetadata = None,
        chunk_context: str = None,
    ) -> tuple[str, DocumentMetadata]:
        prompt = f"Structure this content:"
        if chunk_context:
            prompt = f"{chunk_context}\n\n{prompt}"
        prompt += f"\n\n{text}"

        response = self._chat_with_streaming(
            prompt=prompt,
            system=self.SYSTEM_PROMPT,
        )

        parsed = self._parser.parse(response)
        if not parsed.metadata:
            return text, existing_metadata or DocumentMetadata(title="", source="", doc_type="")

        claims = []
        for c in parsed.metadata.get("claims", []):
            if isinstance(c, dict):
                claims.append(Claim(
                    text=c.get("text", ""),
                    type=c.get("type", "argument"),
                    confidence=c.get("confidence", "strong"),
                ))

        domains = parsed.metadata.get("domains", [])
        title = parsed.metadata.get("title", existing_metadata.title if existing_metadata else "")
        if title:
            title = unidecode(title)
        metadata = DocumentMetadata(
            title=title,
            author=parsed.metadata.get("author", existing_metadata.author if existing_metadata else "Unknown"),
            language=parsed.metadata.get("language", ""),
            domains=domains,
            topics=domains,
            core_thesis=parsed.metadata.get("core_thesis", ""),
            mental_models=parsed.metadata.get("mental_models", []),
            claims=claims,
            takeaways=parsed.metadata.get("takeaways", []),
            key_terms=parsed.metadata.get("key_terms", []),
            source=existing_metadata.source if existing_metadata else "",
            doc_type=existing_metadata.doc_type if existing_metadata else "",
            summary=parsed.metadata.get("core_thesis", ""),
            key_insights=[c.text for c in claims if c.type in ("framework", "prediction")],
        )

        structured = self._format_content(parsed.content, metadata)
        return structured, metadata

    def _format_content(self, content: str, metadata: DocumentMetadata) -> str:
        if not content.startswith("#"):
            title = metadata.title or "Untitled"
            content = f"# {title}\n\n{content}"

        content = re.sub(r'\n{3,}', '\n\n', content)
        return content.strip()
