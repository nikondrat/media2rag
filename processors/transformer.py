import json
import re
from typing import Optional

from domain.document import DocumentMetadata


class Transformer:
    """Structure compressed content by topics with metadata extraction."""

    SYSTEM_PROMPT = (
        "You are a knowledge structuring specialist. Analyze the content and:\n"
        "1. Extract: title, author/speaker, main topics (3-5 keywords), 2-3 sentence summary, 3-5 key insights\n"
        "2. Restructure the content into clear sections with H2/H3 headings\n"
        "3. Group related ideas together under thematic headings\n"
        "4. Remove any remaining conversational filler\n"
        "5. Preserve all concrete examples, numbers, frameworks, and actionable steps\n\n"
        "Respond in JSON format:\n"
        '{"title": "...", "author": "...", "topics": ["..."], "summary": "...", "key_insights": ["..."], "structured_content": "..."}'
    )

    def __init__(self, llm_client):
        self._client = llm_client

    def transform(self, compressed_text: str) -> tuple[str, DocumentMetadata]:
        response = self._client.chat(
            prompt=f"Structure this content:\n\n{compressed_text}",
            system=self.SYSTEM_PROMPT,
        )

        parsed = self._parse_json_response(response)
        if not parsed:
            return compressed_text, DocumentMetadata(title="", source="", doc_type="")

        metadata = DocumentMetadata(
            title=parsed.get("title", ""),
            author=parsed.get("author", ""),
            topics=parsed.get("topics", []),
            summary=parsed.get("summary", ""),
            key_insights=parsed.get("key_insights", []),
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
