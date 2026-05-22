import re
from dataclasses import dataclass
from typing import Any

import yaml


@dataclass
class ParsedOutput:
    metadata: dict[str, Any]
    content: str
    raw: str


class MarkdownOutputParser:
    """Единый парсер Markdown frontmatter для всех LLM выводов."""

    def parse(self, response: str, required_fields: list[str] | None = None) -> ParsedOutput:
        if not response.startswith('---'):
            return ParsedOutput(metadata={}, content=response.strip(), raw=response)

        parts = response.split('---', 2)
        if len(parts) < 3:
            return ParsedOutput(metadata={}, content=response.strip(), raw=response)

        try:
            metadata = yaml.safe_load(parts[1]) or {}
        except yaml.YAMLError:
            metadata = {}

        content = parts[2].strip()

        if required_fields:
            metadata = self._ensure_fields(metadata, required_fields)

        return ParsedOutput(metadata=metadata, content=content, raw=response)

    def _ensure_fields(self, metadata: dict, required: list[str]) -> dict:
        for field in required:
            if field not in metadata:
                metadata[field] = '' if not isinstance(metadata.get(field), list) else []
        return metadata

    def format_prompt(self, system_prompt: str, example: str = '') -> str:
        return (
            f"{system_prompt}\n\n"
            f"## OUTPUT FORMAT\n"
            f"---\n"
            f"title: English Title\n"
            f"author: Author Name\n"
            f"language: ru\n"
            f"domains: [investing, psychology]\n"
            f"core_thesis: Single sentence thesis\n"
            f"mental_models: [systems-thinking, first-principles]\n"
            f"claims:\n"
            f"  - text: Claim text\n"
            f"    type: argument\n"
            f"    confidence: strong\n"
            f"takeaways: [action 1, action 2]\n"
            f"key_terms: [term1, term2]\n"
            f"---\n\n"
            f"{example}"
            f"## BODY (preserve source language)\n"
            f"[structured content with # Thesis, # Mechanism, etc.]\n"
        )
