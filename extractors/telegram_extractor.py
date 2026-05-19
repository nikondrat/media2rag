import re
from pathlib import Path
from urllib.parse import urlparse

import requests
from bs4 import BeautifulSoup

from domain.document import ExtractedContent, DocumentMetadata
from extractors.base import BaseExtractor


class TelegramExtractor(BaseExtractor):
    TELEGRAM_PATTERNS = ["t.me/", "telegram.me/"]

    def __init__(self):
        self._session = requests.Session()
        self._session.headers.update({
            "User-Agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36"
        })

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
                word_count=len(text.split()),
            ),
        )

    def _extract_channel_posts(self, channel: str, url: str) -> ExtractedContent:
        fetch_url = f"https://t.me/s/{channel}"
        resp = self._session.get(fetch_url, timeout=30)
        resp.raise_for_status()

        soup = BeautifulSoup(resp.text, "html.parser")
        posts = soup.find_all("div", class_="tgme_widget_message_text")

        parts = []
        for post in posts:
            text = post.get_text(separator="\n", strip=True)
            if text:
                parts.append(text)

        content = "\n\n---\n\n".join(parts)
        title_elem = soup.find("title")
        channel_name = title_elem.get_text(strip=True) if title_elem else channel

        return ExtractedContent(
            raw_text=content,
            metadata=DocumentMetadata(
                title=channel_name,
                source=url,
                doc_type="telegram",
                word_count=len(content.split()),
            ),
        )
