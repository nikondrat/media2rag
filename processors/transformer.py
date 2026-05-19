import json
import re
from typing import Optional

from domain.document import DocumentMetadata


class Transformer:
    """Structure cleaned content by topics with metadata extraction."""

    SYSTEM_PROMPT = (
        "You are a knowledge structuring specialist. Analyze the content and:\n"
        "1. Extract metadata: title, author/speaker, main topics (3-5 keywords), 2-3 sentence summary, 3-5 key insights\n"
        "2. Restructure into clear H2/H3 sections by theme\n"
        "3. Preserve ALL facts, numbers, examples, frameworks, quotes, and actionable advice\n"
        "4. Remove conversational filler, greetings, sign-offs\n\n"
        "Adapt section names to the actual content. Common sections: Key Principles, Practical Steps, Examples, Metrics, Frameworks.\n\n"
        "Respond in JSON format ONLY:\n"
        '{"title": "...", "author": "...", "topics": ["topic1", "topic2", "topic3"], "summary": "...", "key_insights": ["insight1", "insight2"], "structured_content": "..."}\n\n'
        "IMPORTANT:\n"
        "- structured_content must be the FULL cleaned content organized with H2/H3 headings\n"
        "- Do NOT add any meta-text like 'Here is the structured content'\n"
        "- Do NOT add separators with commentary\n"
        "- Escape all quotes in structured_content"
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

        metadata = DocumentMetadata(
            title=parsed.get("title", existing_metadata.title if existing_metadata else ""),
            author=parsed.get("author", existing_metadata.author if existing_metadata else ""),
            topics=parsed.get("topics", []),
            summary=parsed.get("summary", ""),
            key_insights=parsed.get("key_insights", []),
            source=existing_metadata.source if existing_metadata else "",
            doc_type=existing_metadata.doc_type if existing_metadata else "",
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
            content = f"# {metadata.title}\n\n{content}"

        content = re.sub(r'\n{3,}', '\n\n', content)
        return content.strip()
