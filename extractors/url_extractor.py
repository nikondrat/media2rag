import re
from pathlib import Path
from urllib.parse import urlparse

import requests
from bs4 import BeautifulSoup

from domain.document import ExtractedContent, DocumentMetadata
from extractors.base import BaseExtractor


class URLExtractor(BaseExtractor):
    def __init__(self):
        self._session = requests.Session()
        self._session.headers.update({
            "User-Agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36"
        })

    def extract(self, source: Path | str) -> ExtractedContent:
        url = source if isinstance(source, str) else str(source)
        if not url.startswith("http"):
            raise ValueError(f"Not a URL: {url}")

        resp = self._session.get(url, timeout=30)
        resp.raise_for_status()

        soup = BeautifulSoup(resp.text, "html.parser")

        title = self._extract_title(soup)
        content = self._extract_content(soup)

        if not content.strip():
            content = soup.get_text(separator="\n", strip=True)

        return ExtractedContent(
            raw_text=content,
            metadata=DocumentMetadata(
                title=title or urlparse(url).netloc,
                source=url,
                doc_type="article",
                word_count=len(content.split()),
            ),
        )

    def supports(self, source: Path | str) -> bool:
        if isinstance(source, str):
            return source.startswith("http")
        return False

    def _extract_title(self, soup: BeautifulSoup) -> str:
        for tag in ["h1", "title", "meta[property='og:title']"]:
            if tag == "meta[property='og:title']":
                el = soup.find("meta", property="og:title")
                if el and el.get("content"):
                    return el["content"].strip()
            else:
                el = soup.find(tag)
                if el and el.get_text(strip=True):
                    return el.get_text(strip=True)
        return ""

    def _extract_content(self, soup: BeautifulSoup) -> str:
        for tag in ["script", "style", "nav", "header", "footer", "aside"]:
            for el in soup.find_all(tag):
                el.decompose()

        article = soup.find("article") or soup.find("main") or soup.find("body")
        if not article:
            return ""

        paragraphs = article.find_all(["p", "h1", "h2", "h3", "h4", "h5", "h6", "li", "pre", "blockquote"])
        if not paragraphs:
            return article.get_text(separator="\n", strip=True)

        parts = []
        for p in paragraphs:
            text = p.get_text(strip=True)
            if text:
                if p.name.startswith("h"):
                    level = int(p.name[1])
                    parts.append(f"{'#' * level} {text}")
                elif p.name == "blockquote":
                    parts.append(f"> {text}")
                elif p.name == "pre":
                    parts.append(f"```\n{text}\n```")
                else:
                    parts.append(text)

        return "\n\n".join(parts)
