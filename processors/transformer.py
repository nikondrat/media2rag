import json
import re
from typing import Optional

from domain.document import Claim, DocumentMetadata


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

        "Respond in JSON format ONLY:\n"
        '{"title": "...", "author": "...", "language": "ru", "domains": ["investing"], '
        '"core_thesis": "...", "mental_models": ["systems-thinking"], '
        '"claims": [{"text": "...", "type": "argument", "confidence": "strong"}], '
        '"takeaways": ["..."], "key_terms": ["..."], '
        '"structured_content": "## Thesis\\n...\\n\\n## Mechanism\\n..."}\n\n'

        "CRITICAL:\n"
        "- ALL metadata values MUST be in English\n"
        "- structured_content body MUST be in the source language\n"
        "- Escape all quotes and newlines in structured_content\n"
        "- structured_content must use ## headings only (no # or ###)"
    )

    def __init__(self, llm_client):
        self._client = llm_client

    def transform(self, compressed_text: str, existing_metadata: DocumentMetadata = None) -> tuple[str, DocumentMetadata]:
        response = self._client.chat(
            prompt=f"Structure this content:\n\n{compressed_text}",
            system=self.SYSTEM_PROMPT,
        )

        parsed = self._parse_json_response(response)
        if not parsed:
            return compressed_text, existing_metadata or DocumentMetadata(title="", source="", doc_type="")

        claims = []
        for c in parsed.get("claims", []):
            claims.append(Claim(
                text=c.get("text", ""),
                type=c.get("type", "argument"),
                confidence=c.get("confidence", "strong"),
            ))

        metadata = DocumentMetadata(
            title=parsed.get("title", existing_metadata.title if existing_metadata else ""),
            author=parsed.get("author", existing_metadata.author if existing_metadata else "Unknown"),
            language=parsed.get("language", ""),
            domains=parsed.get("domains", []),
            core_thesis=parsed.get("core_thesis", ""),
            mental_models=parsed.get("mental_models", []),
            claims=claims,
            takeaways=parsed.get("takeaways", []),
            key_terms=parsed.get("key_terms", []),
            source=existing_metadata.source if existing_metadata else "",
            doc_type=existing_metadata.doc_type if existing_metadata else "",
            summary=parsed.get("core_thesis", ""),
            key_insights=[c.text for c in claims if c.type in ("framework", "prediction")],
        )

        structured = self._format_content(parsed.get("structured_content", compressed_text), metadata)
        return structured, metadata

    def _parse_json_response(self, response: str) -> Optional[dict]:
        match = re.search(r'\{.*\}', response, re.DOTALL)
        if not match:
            return None
        try:
            return json.loads(match.group())
        except json.JSONDecodeError:
            return None

    def _format_content(self, content: str, metadata: DocumentMetadata) -> str:
        if not content.startswith("#"):
            title = metadata.title or "Untitled"
            content = f"# {title}\n\n{content}"

        content = re.sub(r'\n{3,}', '\n\n', content)
        return content.strip()
