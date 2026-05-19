import json
import os
import re
import subprocess
import sys

from yt_dlp import YoutubeDL


def sanitize_filename(title):
    s = title.strip().replace("/", " ").replace("\\", " ").replace(":", " -")
    s = re.sub(r'[<>"|?*]', "", s)
    s = re.sub(r"\s+", " ", s)
    return s[:120].rstrip(" .")


DONE_FILE = "_done.json"


def get_video_ids(channel_url):
    ydl_opts = {
        "quiet": True,
        "extract_flat": True,
    }
    with YoutubeDL(ydl_opts) as ydl:
        info = ydl.extract_info(channel_url, download=False)
        if "entries" in info:
            return [
                {"id": entry["id"], "title": entry.get("title", entry["id"])}
                for entry in info["entries"]
            ]
        return []


def load_done():
    if not os.path.isfile(DONE_FILE):
        return {}
    with open(DONE_FILE, encoding="utf-8") as f:
        return json.load(f)


def save_transcript(video, output_dir="transcripts"):
    os.makedirs(output_dir, exist_ok=True)

    title = video["title"]
    safe = sanitize_filename(title)
    filename = f"{safe}.md"
    filepath_md = os.path.join(output_dir, filename)

    url = f"https://youtu.be/{video['id']}"

    try:
        result = subprocess.run(
            ["npx", "rdrr", url],
            capture_output=True,
            text=True,
            check=True,
        )

        output = result.stdout.strip()
        if not output:
            print(f"  ✗ Пустой вывод: {title}", flush=True)
            return None

        with open(filepath_md, "w", encoding="utf-8") as f:
            f.write(output)
        return filename

    except subprocess.CalledProcessError as e:
        print(f"  ✗ Ошибка rdrr: {title} — {e.stderr[:200]}", flush=True)
        return None
    except Exception as e:
        print(f"  ✗ Ошибка: {title} — {e}", flush=True)
        return None


if __name__ == "__main__":
    channel_url = (
        sys.argv[1]
        if len(sys.argv) > 1
        else "https://www.youtube.com/@grebenukm/videos"
    )
    output_dir = "transcripts"

    print(">>> Шаг 1: Получение списка видео с канала...", flush=True)
    videos = get_video_ids(channel_url)
    print(f"    Найдено видео: {len(videos)}", flush=True)

    print(">>> Шаг 2: Проверка уже скачанных...", flush=True)
    done = load_done()
    print(f"    Уже скачано: {len(done)}", flush=True)

    todo = [v for v in videos if v["id"] not in done]
    if not todo:
        print(">>> Всё уже скачано!", flush=True)
        sys.exit(0)
    print(f"    Осталось: {len(todo)}", flush=True)

    print(">>> Шаг 3: Скачивание транскрипций...", flush=True)
    ok = 0
    for i, v in enumerate(todo, 1):
        print(f"  [{i}/{len(todo)}] {v['title']}", flush=True)
        filename = save_transcript(v, output_dir)
        if filename:
            done[v["id"]] = filename
            with open(DONE_FILE, "w", encoding="utf-8") as f:
                json.dump(done, f, ensure_ascii=False, indent=2)
            ok += 1

    print(f">>> Готово! Скачано {ok}/{len(todo)} в папке {output_dir}", flush=True)
