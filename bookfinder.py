import os
import re
import sys
import time
import socket
import logging
from dataclasses import dataclass, field
from typing import Optional
from urllib.parse import urlparse, unquote

import requests
from bs4 import BeautifulSoup


logger = logging.getLogger(__name__)


@dataclass
class BookMeta:
    title: str
    author: str = ""
    isbn_10: str = ""
    isbn_13: str = ""
    asin: str = ""
    source_url: str = ""


@dataclass
class FoundBook:
    title: str
    author: str = ""
    extension: str = ""
    size: str = ""
    mirror_links: list[str] = field(default_factory=list)
    source: str = ""
    confidence: int = 0


LIBGEN_MIRRORS = [
    "https://libgen.li",
    "https://libgen.is",
    "https://libgen.rs",
    "https://libgen.st",
    "https://libgen.pm",
]

USER_AGENT = (
    "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) "
    "AppleWebKit/537.36 (KHTML, like Gecko) "
    "Chrome/125.0.0.0 Safari/537.36"
)


def _detect_proxy() -> Optional[str]:
    for var in ("HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "http_proxy", "ALL_PROXY", "all_proxy"):
        val = os.environ.get(var)
        if val:
            return val
    for host, port in [("127.0.0.1", 9050), ("127.0.0.1", 9150)]:
        try:
            s = socket.socket()
            s.settimeout(1)
            s.connect((host, port))
            s.close()
            return f"socks5h://{host}:{port}"
        except (OSError, socket.error):
            continue
    for host, port in [("127.0.0.1", 8118), ("127.0.0.1", 8123)]:
        try:
            s = socket.socket()
            s.settimeout(1)
            s.connect((host, port))
            s.close()
            return f"http://{host}:{port}"
        except (OSError, socket.error):
            continue
    return None


def _session(proxy: Optional[str] = None, no_auto: bool = False) -> tuple[requests.Session, Optional[str]]:
    s = requests.Session()
    s.headers.update({"User-Agent": USER_AGENT})
    if not no_auto:
        proxy = proxy or _detect_proxy()
    if proxy:
        s.proxies.update({"https": proxy, "http": proxy})
    adapter = requests.adapters.HTTPAdapter(
        max_retries=0,
        pool_connections=2,
    )
    s.mount("https://", adapter)
    s.mount("http://", adapter)
    return s, proxy


def extract_asin(url: str) -> Optional[str]:
    patterns = [
        r"/dp/([A-Z0-9]{10})",
        r"/product/([A-Z0-9]{10})",
        r"/gp/product/([A-Z0-9]{10})",
        r"/exec/obidos/ASIN/([A-Z0-9]{10})",
        r"asin=([A-Z0-9]{10})",
    ]
    for p in patterns:
        m = re.search(p, url, re.IGNORECASE)
        if m:
            return m.group(1)
    # maybe it's already an ASIN/ISBN
    path = urlparse(url).path.strip("/")
    if re.match(r"^[A-Z0-9]{10}$", path, re.IGNORECASE):
        return path.upper()
    return None


def meta_from_isbn(session: requests.Session, isbn: str) -> Optional[BookMeta]:
    try:
        r = session.get(
            "https://openlibrary.org/api/books",
            params={"bibkeys": f"ISBN:{isbn}", "format": "json", "jscmd": "data"},
            timeout=15,
        )
        if r.status_code != 200:
            return None
        data = r.json()
        key = f"ISBN:{isbn}"
        info = data.get(key)
        if not info:
            return None
        title = info.get("title", "")
        authors = [a["name"] for a in info.get("authors", [])]
        isbns = info.get("identifiers", {})
        return BookMeta(
            title=title,
            author=", ".join(authors) if authors else "",
            isbn_10=next(
                (i for i in (isbns.get("isbn_10") or []) if i), ""
            ),
            isbn_13=next(
                (i for i in (isbns.get("isbn_13") or []) if i), ""
            ),
        )
    except Exception as e:
        logger.debug("meta_from_isbn(%s) failed: %s", isbn, e)
        return None


def meta_from_search(
    session: requests.Session, title: str, author: str = ""
) -> Optional[BookMeta]:
    try:
        query = title
        if author:
            query += f" {author}"
        r = session.get(
            "https://openlibrary.org/search.json",
            params={"q": query, "limit": 3},
            timeout=15,
        )
        if r.status_code != 200:
            return None
        data = r.json()
        docs = data.get("docs", [])
        if not docs:
            return None
        doc = docs[0]
        isbns = doc.get("isbn", [])
        return BookMeta(
            title=doc.get("title", title),
            author=", ".join(doc.get("author_name", [author]) if doc.get("author_name") else [author]),
            isbn_10=next((i for i in isbns if len(i) == 10), ""),
            isbn_13=next((i for i in isbns if len(i) == 13), ""),
        )
    except Exception as e:
        logger.debug("meta_from_search failed: %s", e)
        return None


_dead_mirrors: set[str] = set()


def _reset_dead_mirrors() -> None:
    _dead_mirrors.clear()


RKN_BLOCK_DETECTED = False


def _check_block(html_or_text: str) -> bool:
    return "megafonpro" in html_or_text or "rkn" in html_or_text.lower()


def _search_libgen_mirror(
    session: requests.Session,
    query: str,
    column: str = "def",
    mirrors: list[str] = LIBGEN_MIRRORS,
    timeout: int = 15,
) -> Optional[list[FoundBook]]:
    global RKN_BLOCK_DETECTED
    if RKN_BLOCK_DETECTED:
        return None
    for mirror in mirrors:
        if mirror in _dead_mirrors:
            continue
        try:
            params = {"req": query, "res": 25}
            url = f"{mirror}/index.php"
            r = session.get(url, params=params, timeout=timeout)
            body = r.text[:500].lower()
            if _check_block(body):
                logger.warning("⚠️  LibGen заблокирован провайдером (РКН)")
                RKN_BLOCK_DETECTED = True
                return None
            if r.status_code != 200:
                _dead_mirrors.add(mirror)
                continue
            results = _parse_libgen_results(r.text, mirror)
            if results:
                return results
            _dead_mirrors.add(mirror)
            continue
        except Exception as e:
            logger.debug("mirror %s failed: %s", mirror, e)
            _dead_mirrors.add(mirror)
            continue
    return None


def _parse_libgen_results(html: str, mirror: str) -> list[FoundBook]:
    soup = BeautifulSoup(html, "lxml")
    for i_tag in soup.find_all("i"):
        i_tag.decompose()
    tables = soup.find_all("table")

    info_table = None
    col_count = 0
    for t in tables:
        rows = t.find_all("tr")
        if len(rows) <= 1:
            continue
        sample = rows[1].find_all("td")
        if len(sample) in (9, 15):
            info_table = t
            col_count = len(sample)
            break

    if info_table is None:
        return []

    rows = info_table.find_all("tr")[1:]
    results = []
    for row in rows:
        cells = row.find_all("td")
        if len(cells) != col_count:
            continue
        if col_count == 9:
            title_td = cells[0]
            title_a = title_td.find("a")
            title = "".join(title_td.stripped_strings)
            author = "".join(cells[1].stripped_strings)
            size_td = cells[6]
            size_a = size_td.find("a")
            file_id = ""
            if size_a and size_a.get("href"):
                m = re.search(r"id=(\d+)", size_a["href"])
                if m:
                    file_id = m.group(1)
            extension = "".join(cells[7].stripped_strings).lower()
            results.append(
                FoundBook(
                    title=title,
                    author=author,
                    extension=extension,
                    size="".join(cells[6].stripped_strings),
                    mirror_links=[f"{mirror}/file.php?id={file_id}"] if file_id else [],
                    source=mirror,
                )
            )
        elif col_count == 15:
            col_names = [
                "id", "author", "title", "publisher", "year",
                "pages", "language", "size", "extension",
                "mirror1", "mirror2", "mirror3", "mirror4", "mirror5", "edit",
            ]
            raw = []
            for td in cells:
                a = td.find("a")
                if a and a.get("title"):
                    raw.append(a["href"])
                else:
                    raw.append("".join(td.stripped_strings))
            item = dict(zip(col_names, raw))
            results.append(
                FoundBook(
                    title=item.get("title", ""),
                    author=item.get("author", ""),
                    extension=item.get("extension", ""),
                    size=item.get("size", ""),
                    mirror_links=[
                        item.get("mirror1", ""),
                        item.get("mirror2", ""),
                        item.get("mirror3", ""),
                        item.get("mirror4", ""),
                        item.get("mirror5", ""),
                    ],
                    source=mirror,
                )
            )
    return results


def _resolve_libgen_download(
    session: requests.Session, mirror_url: str
) -> Optional[str]:
    try:
        r = session.get(mirror_url, timeout=20)
        if r.status_code != 200:
            return None
        soup = BeautifulSoup(r.text, "html.parser")
        links = soup.find_all("a", href=True)
        ipfs_cid = None
        for a in links:
            href = a["href"]
            m = re.search(r"ipfs/([a-z2-9]{46,})", href)
            if m:
                ipfs_cid = m.group(1)
                break
        if ipfs_cid:
            return f"https://ipfs.io/ipfs/{ipfs_cid}"
        # old-style mirrors
        for link_text in ("GET", "CloudFlare", "Libgen", "library.lol"):
            a = soup.find("a", string=re.compile(re.escape(link_text), re.IGNORECASE))
            if a and a.get("href"):
                href = a["href"]
                if not href.startswith("http"):
                    parsed = urlparse(mirror_url)
                    href = f"{parsed.scheme}://{parsed.netloc}{href}"
                return href
        return None
    except Exception as e:
        logger.debug("resolve_download failed for %s: %s", mirror_url, e)
        return None


def _search_annas_archive(
    session: requests.Session, query: str
) -> Optional[list[FoundBook]]:
    try:
        r = session.get(
            "https://annas-archive.org/search",
            params={"q": query, "lang": "en", "ext": "pdf,epub"},
            timeout=15,
        )
        if r.status_code != 200:
            return None
        soup = BeautifulSoup(r.text, "lxml")
        results = []
        for item in soup.select("div[class*='h-[125px]']"):
            title_el = item.select_one("h3 a")
            if not title_el:
                continue
            title = title_el.get_text(strip=True)
            href = title_el.get("href", "")
            if href and not href.startswith("http"):
                href = f"https://annas-archive.org{href}"
            meta_els = item.select("div[class*='text-sm']")
            author = ""
            extension = ""
            for el in meta_els:
                text = el.get_text(strip=True)
                if text.lower().startswith("by "):
                    author = text[3:]
                elif text.lower() in ("pdf", "epub", "mobi", "djvu"):
                    extension = text.lower()
            results.append(
                FoundBook(
                    title=title,
                    author=author,
                    extension=extension,
                    mirror_links=[href] if href else [],
                    source="annas-archive",
                )
            )
        return results if results else None
    except Exception as e:
        logger.debug("annas_archive failed: %s", e)
        return None


def search_libgen(
    session: requests.Session,
    meta: BookMeta,
    preferred_extensions: list[str],
) -> Optional[FoundBook]:
    queries = []
    if meta.isbn_13:
        queries.append(meta.isbn_13)
    if meta.isbn_10:
        queries.append(meta.isbn_10)
    if meta.asin:
        queries.append(meta.asin)
    queries.append(f"{meta.title} {meta.author}".strip())
    queries.append(meta.title)

    seen = set()
    for query in queries:
        if not query or query in seen:
            continue
        seen.add(query)
        logger.info("поиск LibGen: %s", query)
        results = _search_libgen_mirror(session, query)
        if not results:
            continue
        best = _pick_best(results, meta, preferred_extensions)
        if best:
            return best
    return None


def _pick_best(
    results: list[FoundBook],
    meta: BookMeta,
    preferred_extensions: list[str],
) -> Optional[FoundBook]:
    scored = []
    for book in results:
        score = 0
        ext = book.extension.lower()
        if ext in preferred_extensions:
            score += 10
        if meta.author and meta.author.lower() in book.author.lower():
            score += 5
        if meta.title and (
            meta.title.lower() in book.title.lower()
            or book.title.lower() in meta.title.lower()
        ):
            score += 3
        scored.append((score, book))
    scored.sort(key=lambda x: -x[0])
    return scored[0][1] if scored else None


def download_book(
    session: requests.Session,
    book: FoundBook,
    dest_dir: str,
) -> Optional[str]:
    for link in book.mirror_links:
        if not link:
            continue
        if "libgen" in book.source or link.startswith("http://library"):
            dl_url = _resolve_libgen_download(session, link)
            if dl_url:
                return _do_download(session, dl_url, dest_dir, book)
        else:
            return _do_download(session, link, dest_dir, book)
    return None


def _do_download(
    session: requests.Session,
    url: str,
    dest_dir: str,
    book: FoundBook,
) -> Optional[str]:
    try:
        os.makedirs(dest_dir, exist_ok=True)
        r = session.get(url, timeout=120, stream=True)
        if r.status_code != 200:
            return None
        ct = r.headers.get("Content-Type", "")
        if "html" in ct:
            logger.warning("скачана HTML-страница вместо книги, пропускаю")
            return None
        filename = None
        content_disposition = r.headers.get("Content-Disposition", "")
        if content_disposition:
            m = re.search(r'filename\*?=(?:UTF-8\'\')?([^;\n]+)', content_disposition)
            if m:
                filename = unquote(m.group(1).strip('"'))
        if not filename:
            parsed = urlparse(url)
            qp = parsed.query
            m = re.search(r"filename=([^&]+)", qp)
            if m:
                filename = unquote(m.group(1))
        if not filename:
            filename = f"{book.title}.{book.extension}"
        filename = re.sub(r'[^\w._-]', "_", filename)
        base, ext = os.path.splitext(filename)
        if not ext and book.extension:
            filename = f"{filename}.{book.extension}"
        dest = os.path.join(dest_dir, filename[:200])
        with open(dest, "wb") as f:
            for chunk in r.iter_content(chunk_size=8192):
                f.write(chunk)
        return dest
    except Exception as e:
        logger.debug("download failed: %s", e)
        return None


def meta_from_asin(session: requests.Session, asin: str) -> Optional[BookMeta]:
    meta = meta_from_isbn(session, asin)
    if meta:
        meta.asin = asin
        return meta
    # try OpenLibrary search with fetchJSON
    try:
        r = session.get(
            f"https://openlibrary.org/isbn/{asin}.json",
            timeout=15,
        )
        if r.status_code == 200:
            data = r.json()
            title = data.get("title", "")
            # get author
            authors = data.get("authors", [])
            author = ""
            if authors:
                author_url = authors[0].get("key", "")
                if author_url:
                    ar = session.get(f"https://openlibrary.org{author_url}.json", timeout=10)
                    if ar.status_code == 200:
                        author = ar.json().get("name", "")
            isbn_10 = data.get("isbn_10", [""])[0] if data.get("isbn_10") else ""
            isbn_13 = data.get("isbn_13", [""])[0] if data.get("isbn_13") else ""
            return BookMeta(title=title, author=author, isbn_10=isbn_10, isbn_13=isbn_13, asin=asin)
    except Exception as e:
        logger.debug("meta_from_asin failed: %s", e)
    return BookMeta(title="", asin=asin)


def _resolve_meta(text: str) -> BookMeta:
    sess, _ = _session(no_auto=True)
    asin = extract_asin(text)
    if asin:
        logger.info("ASIN: %s", asin)
        meta = meta_from_asin(sess, asin)
        if meta and meta.title:
            return meta
    clean = text.strip()
    if re.match(r"^\d{10}$", clean) or re.match(r"^\d{13}(?:X)?$", clean):
        meta = meta_from_isbn(sess, clean)
        if meta and meta.title:
            return meta
    meta = meta_from_search(sess, clean)
    if meta and meta.title:
        return meta
    return BookMeta(title=clean)


def _search_all(
    sess: requests.Session, meta: BookMeta
) -> Optional[FoundBook]:
    book = search_libgen(sess, meta, ["pdf", "epub"])
    if not book:
        logger.info("поиск в Anna's Archive...")
        results = _search_annas_archive(sess, f"{meta.title} {meta.author}".strip())
        if results:
            book = _pick_best(results, meta, ["pdf", "epub"])
    return book


def action_book(
    source_url: str, dest_dir: str = "books", proxy: Optional[str] = None
) -> None:
    logger.info(">>> %s", source_url)

    meta = _resolve_meta(source_url)
    if not meta.title:
        logger.warning("не удалось определить книгу")
        return
    logger.info("📖 %s — %s", meta.title, meta.author or "(автор неизвестен)")

    attempts = [("напрямую", proxy, True)]
    auto_proxy = _detect_proxy()
    if auto_proxy and auto_proxy != proxy:
        attempts.append((f"через {auto_proxy}", auto_proxy, False))

    for label, proxy_cfg, force_direct in attempts:
        _reset_dead_mirrors()
        sess, _ = _session(proxy_cfg, no_auto=force_direct)
        time.sleep(2)
        book = _search_all(sess, meta)
        if book:
            logger.info("найдено: %s (%s, %s)", book.title, book.extension, book.size)
            dest = download_book(sess, book, dest_dir)
            if dest:
                logger.info("скачано: %s", dest)
            else:
                logger.warning("не удалось скачать: %s", book.title)
            return
        if not book:
            logger.info("через %s не найдено%s", label,
                        ", пробую прокси..." if len(attempts) > 1 else "")

    logger.warning("не найдено: %s", meta.title)


def main() -> None:
    logging.basicConfig(
        level=logging.INFO,
        format="%(message)s",
        stream=sys.stderr,
    )

    proxy = None
    dest_dir = "books"
    urls = []
    file_path = None

    i = 1
    while i < len(sys.argv):
        arg = sys.argv[i]
        if arg == "--proxy" and i + 1 < len(sys.argv):
            proxy = sys.argv[i + 1]
            i += 2
        elif arg == "--dir" and i + 1 < len(sys.argv):
            dest_dir = sys.argv[i + 1]
            i += 2
        elif arg == "--file" and i + 1 < len(sys.argv):
            file_path = sys.argv[i + 1]
            i += 2
        elif arg in ("-h", "--help"):
            print("использование: uv run bookfinder.py [--file books.txt] [--proxy socks5://...] [--dir ./books] <url|isbn|book-name>...")
            sys.exit(0)
        else:
            urls.append(arg)
            i += 1

    if file_path:
        with open(file_path) as f:
            for line in f:
                line = line.strip()
                if line and not line.startswith("#"):
                    urls.append(line)

    if not urls:
        print("укажите книгу: ссылку Amazon, ISBN или название")
        print("  uv run bookfinder.py https://www.amazon.com/dp/0735201447")
        print("  uv run bookfinder.py 0735201447")
        print("  uv run bookfinder.py --file books.txt")
        print("  uv run bookfinder.py --proxy socks5://127.0.0.1:9050 0735201447")
        sys.exit(1)

    for url in urls:
        action_book(url, dest_dir, proxy)
        if url != urls[-1]:
            time.sleep(3)


if __name__ == "__main__":
    main()
