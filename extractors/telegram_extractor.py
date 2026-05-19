"""Telegram channel scraper with pagination and content filtering."""

import re
from dataclasses import dataclass
from pathlib import Path
from urllib.parse import urlparse

import requests
from bs4 import BeautifulSoup

from domain.document import ExtractedContent, DocumentMetadata
from extractors.base import BaseExtractor
from processors.post_filter import PostFilter


@dataclass
class TelegramPost:
    """Single post with metadata."""
    id: str
    url: str
    date: str
    text: str
    quality: float


class TelegramExtractor(BaseExtractor):
    TELEGRAM_PATTERNS = ["t.me/", "telegram.me/"]
    BASE_URL = "https://t.me/s/{channel}"
    PAGINATION_URL = "https://t.me/s/{channel}?before={post_id}"
    MAX_PAGES = 50  # Safety limit: ~1000 posts

    def __init__(self, llm_client=None):
        self._session = requests.Session()
        self._session.headers.update({
            "User-Agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36"
        })
        self._filter = PostFilter(llm_client=llm_client)
        self._progress_callback = None

    def set_progress_callback(self, callback):
        """Set callback for pagination progress: callback(current_page, total_posts_found)."""
        self._progress_callback = callback

    def extract(self, source: Path | str) -> ExtractedContent:
        url = source if isinstance(source, str) else str(source)
        if not self._is_telegram_url(url):
            raise ValueError(f"Not a Telegram URL: {url}")

        channel, post_id = self._parse_telegram_url(url)
        if not channel:
            raise ValueError(f"Invalid Telegram URL: {url}")

        if post_id:
            return self._extract_single_post(channel, post_id, url)
        return self._extract_channel_posts(channel, url)

    def extract_all_posts(self, source: Path | str) -> list[TelegramPost]:
        """Return all filtered posts as individual objects (for batch processing)."""
        url = source if isinstance(source, str) else str(source)
        channel, post_id = self._parse_telegram_url(url)
        if not channel:
            raise ValueError(f"Invalid Telegram URL: {url}")
        if post_id:
            raise ValueError("Single post URL — use extract() instead")
        return self._scrape_posts(channel)

    def supports(self, source: Path | str) -> bool:
        if isinstance(source, str):
            return self._is_telegram_url(source)
        return False

    def _is_telegram_url(self, url: str) -> bool:
        return url.startswith("http") and any(p in url for p in self.TELEGRAM_PATTERNS)

    def _parse_telegram_url(self, url: str) -> tuple[str | None, str | None]:
        if "t.me/" not in url:
            return None, None

        match = re.search(r"t\.me/([^/]+)(?:/(\d+))?", url)
        if not match:
            return None, None

        channel = match.group(1)
        post_id = match.group(2)
        return channel, post_id

    def _extract_single_post(self, channel: str, post_id: str, url: str) -> ExtractedContent:
        fetch_url = f"https://t.me/s/{channel}/{post_id}"
        resp = self._session.get(fetch_url, timeout=30)
        resp.raise_for_status()

        soup = BeautifulSoup(resp.text, "html.parser")
        post = soup.find("div", class_="tgme_widget_message", attrs={"data-post": f"{channel}/{post_id}"})
        if post:
            text_elem = post.find("div", class_="tgme_widget_message_text")
            text = text_elem.get_text(separator="\n", strip=True) if text_elem else ""
        else:
            text = ""
        title = soup.find("title")
        channel_name = title.get_text(strip=True) if title else channel

        return ExtractedContent(
            raw_text=text,
            metadata=DocumentMetadata(
                title=channel_name,
                source=url,
                doc_type="telegram",
                language=self._detect_language(text),
                word_count=len(text.split()),
            ),
        )

    def _extract_channel_posts(self, channel: str, url: str) -> ExtractedContent:
        """Fallback: combine all posts into single ExtractedContent."""
        posts = self._scrape_posts(channel)

        if not posts:
            return ExtractedContent(
                raw_text="",
                metadata=DocumentMetadata(
                    title=channel,
                    source=url,
                    doc_type="telegram",
                    word_count=0,
                ),
            )

        parts = []
        for post in posts:
            header = f"## Post #{post.id}"
            if post.date:
                header += f" ({post.date[:10]})"
            header += f"\nSource: {post.url}"
            parts.append(f"{header}\n\n{post.text}")

        content = "\n\n---\n\n".join(parts)

        return ExtractedContent(
            raw_text=content,
            metadata=DocumentMetadata(
                title=f"{channel} — {len(posts)} posts",
                source=url,
                doc_type="telegram",
                language=self._detect_language(content),
                word_count=len(content.split()),
            ),
        )

    def _scrape_posts(self, channel: str) -> list[TelegramPost]:
        """Scrape all posts from channel with pagination and filtering."""
        all_posts = []
        before_id = None
        page = 0

        while page < self.MAX_PAGES:
            if before_id:
                fetch_url = f"https://t.me/s/{channel}?before={before_id}"
            else:
                fetch_url = f"https://t.me/s/{channel}"

            resp = self._session.get(fetch_url, timeout=30)
            resp.raise_for_status()

            soup = BeautifulSoup(resp.text, "html.parser")
            messages = soup.find_all("div", class_="tgme_widget_message")

            if not messages:
                break

            oldest_id = None
            for msg in messages:
                post_id = msg.get("data-post", "")
                if not post_id:
                    continue

                pid = post_id.split("/")[-1] if "/" in post_id else post_id
                try:
                    pid_int = int(pid)
                except ValueError:
                    continue

                if oldest_id is None or pid_int < oldest_id:
                    oldest_id = pid_int

                text_elem = msg.find("div", class_="tgme_widget_message_text")
                if not text_elem:
                    continue

                text = text_elem.get_text(separator="\n", strip=True)
                if not text:
                    continue

                link_count = len(msg.find_all("a", href=True))
                quality = self._filter.filter(text, url_count=link_count)

                if quality.passed:
                    post_url = f"https://t.me/{channel}/{pid}"
                    date_elem = msg.find("time")
                    date_str = date_elem.get("datetime", "") if date_elem else ""

                    all_posts.append(TelegramPost(
                        id=pid,
                        url=post_url,
                        date=date_str,
                        text=text,
                        quality=quality.score,
                    ))

            if oldest_id is None:
                break

            before_id = oldest_id
            page += 1

            if self._progress_callback:
                self._progress_callback(page, len(all_posts))

        return all_posts

    @staticmethod
    def _detect_language(text: str) -> str:
        import re
        cyrillic = len(re.findall(r"[\u0400-\u04FF]", text))
        latin = len(re.findall(r"[A-Za-z]", text))
        if cyrillic > latin:
            return "ru"
        if latin > 0:
            return "en"
        return ""
