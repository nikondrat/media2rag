import json
from typing import Optional

from processors.output_parser import MarkdownOutputParser


CONTENT_TYPES = ["interview", "lecture", "podcast", "monologue", "panel", "case_study"]

TYPE_MODIFIERS = {
    "interview": (
        "\n\n## INTERVIEW-SPECIFIC RULES\n"
        "- Extract advice, frameworks, and case studies from Q&A dialogue\n"
        "- Preserve speaker attribution when quoting (e.g., 'Михаил Гребенюк: ...')\n"
        "- Convert conversational exchanges into structured knowledge blocks\n"
        "- Identify and extract mental models mentioned by speakers\n"
        "- Separate questions (context) from answers (content) — focus on answers\n"
    ),
    "lecture": (
        "\n\n## LECTURE-SPECIFIC RULES\n"
        "- Preserve structured teaching flow: concepts → examples → exercises\n"
        "- Highlight frameworks, formulas, and step-by-step processes\n"
        "- Keep educational progression intact (beginner → advanced)\n"
        "- Extract key definitions and explain them clearly\n"
        "- Preserve any exercises, homework, or practice recommendations\n"
    ),
    "podcast": (
        "\n\n## PODCAST-SPECIFIC RULES\n"
        "- Heavily filter small talk, banter, and off-topic tangents\n"
        "- Consolidate multiple viewpoints into unified insights\n"
        "- Extract actionable advice from casual conversation\n"
        "- Remove promotional segments and sponsor reads\n"
        "- Preserve key stories and anecdotes that illustrate points\n"
    ),
    "monologue": (
        "\n\n## MONOLOGUE-SPECIFIC RULES\n"
        "- Preserve the speaker's argumentative flow and logic\n"
        "- Extract key claims and supporting evidence\n"
        "- Structure as: thesis → arguments → conclusion\n"
        "- Remove repetitive statements and filler\n"
        "- Keep the original tone and perspective\n"
    ),
    "panel": (
        "\n\n## PANEL-SPECIFIC RULES\n"
        "- Capture diverse perspectives and points of disagreement\n"
        "- Organize by topic/theme, not by speaker\n"
        "- Highlight consensus points and contrasting views\n"
        "- Extract frameworks and insights from each panelist\n"
        "- Remove cross-talk and moderating chatter\n"
    ),
    "case_study": (
        "\n\n## CASE STUDY-SPECIFIC RULES\n"
        "- Preserve the problem → action → result structure\n"
        "- Keep specific numbers, timelines, and outcomes\n"
        "- Extract lessons learned and applicable principles\n"
        "- Highlight what worked and what didn't\n"
        "- Maintain context: industry, company size, market conditions\n"
    ),
}


class ContentRouter:
    """Classify content type and provide type-specific prompt modifiers."""

    CLASSIFY_SYSTEM_PROMPT = (
        "You are a content classifier. Analyze the text and determine its type.\n\n"
        "TYPES:\n"
        "- interview: Q&A format with multiple speakers, questions and answers\n"
        "- lecture: structured teaching, frameworks, exercises, educational progression\n"
        "- podcast: conversational, multiple hosts/guests, informal discussion\n"
        "- monologue: single speaker presenting ideas, opinions, or analysis\n"
        "- panel: multiple experts discussing topics, diverse viewpoints\n"
        "- case_study: specific business/project example with problem, action, result\n\n"
        "OUTPUT: YAML frontmatter with content_type, focus_area, and structure_hint.\n"
        "Start directly with --- (no preamble)."
    )

    def __init__(self, llm_client, json_mode: bool = False, reasoning: bool = False):
        self._client = llm_client
        self._json_mode = json_mode
        self._reasoning = reasoning
        self._parser = MarkdownOutputParser()

    def _emit(self, status: str, **kwargs):
        if self._json_mode:
            obj = {"status": status, **kwargs}
            print(json.dumps(obj, ensure_ascii=False), flush=True)
        else:
            messages = {
                "router_start": "  🎯 Classifying content type...",
                "router_done": f"  📋 Content type: {kwargs.get('content_type')} | focus: {kwargs.get('focus_area', 'N/A')}",
            }
            msg = messages.get(status, f"  [{status}] {kwargs}")
            print(msg, flush=True)

    def _chat(self, prompt: str, system: str = "") -> str:
        try:
            return self._client.chat(prompt=prompt, system=system, reasoning=self._reasoning)
        except Exception:
            return self._client.chat(prompt=prompt, system=system, reasoning=False)

    def classify(self, compressed_text: str) -> dict:
        """Classify content type from compressed text.

        Returns dict with: content_type, focus_area, structure_hint.
        Uses first 3000 characters for classification.
        """
        sample = compressed_text[:3000]

        prompt = (
            "Classify this content:\n\n"
            f"{sample}\n\n"
            "Produce YAML with content_type, focus_area, and structure_hint."
        )

        response = self._chat(prompt=prompt, system=self.CLASSIFY_SYSTEM_PROMPT)
        parsed = self._parser.parse(response)

        content_type = parsed.metadata.get("content_type", "monologue")
        if content_type not in CONTENT_TYPES:
            content_type = "monologue"

        return {
            "content_type": content_type,
            "focus_area": parsed.metadata.get("focus_area", ""),
            "structure_hint": parsed.metadata.get("structure_hint", ""),
        }

    def get_transformer_prompt_modifier(self, content_type: str) -> str:
        """Get the type-specific prompt modifier for the transformer."""
        return TYPE_MODIFIERS.get(content_type, TYPE_MODIFIERS["monologue"])
