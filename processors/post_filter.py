"""Two-stage post filter: heuristic rules + optional LLM quality scoring."""

import re
from dataclasses import dataclass

from processors.output_parser import MarkdownOutputParser


@dataclass
class PostQuality:
    score: float  # 0.0–1.0
    reason: str
    passed: bool


# Patterns that indicate low-value posts
CTA_PATTERNS = [
    r"(?:подписывайтесь|subscribe|join us|присоединяйтесь)",
    r"(?:читать (?:тут|здесь|по ссылке)|read (?:here|more))",
    r"(?:ссылка (?:в описании|в био)|link in (?:bio|description))",
    r"(?:перейти по ссылке|click the link)",
    r"(?:оставить заявку|register now|sign up)",
    r"(?:узнать больше|learn more)",
    r"(?:подробнее (?:тут|здесь|на сайте))",
    r"(?:вступайте в (?:наш|наш канал|сообщество))",
    r"(?:читайте (?:тут|здесь|по ссылке|далее))",
    r"(?:полный текст|full article|полная версия)",
]

SELF_PROMO_PATTERNS = [
    r"горжусь (?:командой|результатом|нашими)",
    r"лично курировал",
    r"(?:наша команда сделала|our team built|we built (?:this|it))",
    r"(?:мы сделали|we (?:made|did|built|created)).{0,40}(?:исследование|research|рейтинг|rating)",
    r"(?:сам (?:курировал|делал|писал)|personally (?:curated|oversaw|wrote))",
]

AD_PATTERNS = [
    r"(?:реклама|advertisement|ad\b|sponsored|партнёрский материал)",
    r"(?:промокод|promo code|скидка \d+%|discount \d+%)",
    r"(?:акция|special offer|бесплатно (?:до|until))",
    r"(?:купи|buy now|закажи|order now)",
    r"(?:рекламная интеграция|paid partnership)",
]


class PostFilter:
    """Filters Telegram posts using heuristic rules + optional LLM scoring."""

    def __init__(self, llm_client=None):
        self._llm = llm_client
        self._cta_regex = [re.compile(p, re.IGNORECASE) for p in CTA_PATTERNS]
        self._promo_regex = [re.compile(p, re.IGNORECASE) for p in SELF_PROMO_PATTERNS]
        self._ad_regex = [re.compile(p, re.IGNORECASE) for p in AD_PATTERNS]

    def filter(self, text: str, url_count: int = 0) -> PostQuality:
        """Return quality assessment. passed=True means keep the post."""
        if not text or len(text.strip()) < 30:
            return PostQuality(0.0, "Слишком короткий пост", False)

        cleaned = text.strip()
        word_count = len(cleaned.split())

        if word_count < 15:
            return PostQuality(0.1, f"Мало содержимого ({word_count} слов)", False)

        url_ratio = url_count / max(word_count, 1)
        if url_ratio > 0.3 and word_count < 50:
            return PostQuality(0.2, "Слишком много ссылок", False)

        cta_matches = sum(1 for r in self._cta_regex if r.search(cleaned))
        promo_matches = sum(1 for r in self._promo_regex if r.search(cleaned))
        ad_matches = sum(1 for r in self._ad_regex if r.search(cleaned))
        total_noise = cta_matches + promo_matches + ad_matches

        if promo_matches >= 1 and cta_matches >= 1 and url_count >= 1:
            return PostQuality(0.1, "Самопиар + CTA + ссылка", False)

        if total_noise >= 2:
            return PostQuality(0.15, f"Шум: {total_noise} паттернов", False)

        if ad_matches >= 1:
            return PostQuality(0.1, "Реклама", False)

        has_copyright = re.search(r"©|all rights reserved", cleaned, re.IGNORECASE)
        if has_copyright and word_count < 40:
            return PostQuality(0.1, "Копирайт/футер", False)

        lines = cleaned.split("\n")
        link_lines = sum(1 for l in lines if l.strip().startswith("http"))
        if link_lines >= 3 and word_count < 60:
            return PostQuality(0.2, "Список ссылок", False)

        if word_count < 25 and url_count >= 1:
            return PostQuality(0.3, "Короткий пост со ссылкой", False)

        if self._llm:
            return self._llm_score(cleaned)

        return PostQuality(0.7, "Heuristic pass", True)

    def _llm_score(self, text: str) -> PostQuality:
        """Use LLM to assess borderline posts."""
        parser = MarkdownOutputParser()
        prompt = (
            "Evaluate this Telegram post for informational value. "
            "Score 0-1 based on: factual content, analysis, insights, actionable information. "
            "Deduct points for: self-promotion, CTAs, ads, reposts without commentary, "
            "emotional reactions without substance.\n\n"
            f"Post:\n{text}"
        )

        try:
            response = self._llm.chat(
                prompt=prompt,
                system=(
                    "You are a content quality evaluator.\n\n"
                    "## OUTPUT FORMAT\n"
                    "---\n"
                    "score: 0.7\n"
                    "reason: Brief explanation\n"
                    "keep: true\n"
                    "---"
                ),
            )
            parsed = parser.parse(response)
            return PostQuality(
                score=float(parsed.metadata.get("score", 0.5)),
                reason=parsed.metadata.get("reason", "LLM assessment"),
                passed=bool(parsed.metadata.get("keep", True)),
            )
        except Exception:
            pass

        return PostQuality(0.5, "LLM fallback — borderline", True)
