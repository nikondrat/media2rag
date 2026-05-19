import json
import subprocess
import tempfile
from pathlib import Path

from config import WhisperConfig
from domain.document import ExtractedContent, DocumentMetadata
from extractors.base import BaseExtractor


class VideoExtractor(BaseExtractor):
    SUPPORTED_EXTENSIONS = {".mp4", ".mkv", ".avi", ".mov", ".webm"}
    VIDEO_URL_PATTERNS = ["youtube.com", "youtu.be", "vimeo.com"]

    def __init__(self, cfg: WhisperConfig):
        self._cfg = cfg

    def extract(self, source: Path | str) -> ExtractedContent:
        is_url = isinstance(source, str) and self._is_url(source)
        tmpdir_ctx = tempfile.TemporaryDirectory() if is_url else None

        try:
            if is_url:
                video_path, video_title = self._download_video(source, tmpdir_ctx.name)
            else:
                video_path = Path(source) if isinstance(source, str) else source
                video_title = video_path.stem

            if not video_path.exists():
                raise FileNotFoundError(f"File not found: {video_path}")

            audio_path = self._extract_audio(video_path)
            try:
                raw_text = self._transcribe(audio_path)
            finally:
                if audio_path.exists():
                    audio_path.unlink()
                txt_file = audio_path.with_suffix(".txt")
                if txt_file.exists():
                    txt_file.unlink()

            duration = self._get_duration(video_path)

            return ExtractedContent(
                raw_text=raw_text,
                metadata=DocumentMetadata(
                    title=video_title,
                    source=str(source),
                    doc_type="video",
                    word_count=len(raw_text.split()),
                ),
                duration_seconds=duration,
            )
        finally:
            if tmpdir_ctx:
                tmpdir_ctx.cleanup()

    def supports(self, source: Path | str) -> bool:
        if isinstance(source, str) and self._is_url(source):
            return True
        path = Path(source) if isinstance(source, str) else source
        return path.suffix.lower() in self.SUPPORTED_EXTENSIONS

    def _is_url(self, source: str) -> bool:
        return source.startswith("http") and any(p in source for p in self.VIDEO_URL_PATTERNS)

    def _download_video(self, url: str, tmpdir: str) -> tuple[Path, str]:
        result = subprocess.run(
            ["yt-dlp", "--dump-json", "-f", "bestvideo[ext=mp4]+bestaudio[ext=m4a]/best", url],
            capture_output=True, text=True, timeout=600, check=False,
        )
        if result.returncode != 0:
            raise RuntimeError(f"yt-dlp failed: {result.stderr[:500]}")

        info = json.loads(result.stdout)
        title = info.get("title", "unknown")
        video_path = Path(tmpdir) / "video.mp4"

        subprocess.run(
            ["yt-dlp", "-f", "bestvideo[ext=mp4]+bestaudio[ext=m4a]/best",
             "-o", str(video_path), url],
            capture_output=True, text=True, timeout=600, check=True,
        )

        if not video_path.exists():
            files = list(Path(tmpdir).glob("video.*"))
            if not files:
                raise RuntimeError("yt-dlp: no video downloaded")
            video_path = files[0]

        return video_path, title

    def _extract_audio(self, video_path: Path) -> Path:
        audio_path = video_path.with_suffix(".wav")
        subprocess.run(
            ["ffmpeg", "-i", str(video_path), "-vn", "-acodec", "pcm_s16le",
             "-ar", "16000", "-ac", "1", str(audio_path), "-y"],
            capture_output=True, text=True, timeout=300, check=True,
        )
        return audio_path

    def _transcribe(self, audio_path: Path) -> str:
        lang_args = ["--language", self._cfg.language] if self._cfg.language else []
        result = subprocess.run(
            [
                "whisper",
                str(audio_path),
                "--model", self._cfg.model,
                "--device", self._cfg.device,
                "--output_format", "txt",
                "--output_dir", str(audio_path.parent),
            ] + lang_args,
            capture_output=True,
            text=True,
            timeout=3600,
        )

        if result.returncode != 0:
            raise RuntimeError(f"Whisper failed: {result.stderr[:500]}")

        txt_file = audio_path.with_suffix(".txt")
        return txt_file.read_text(encoding="utf-8") if txt_file.exists() else ""

    def _get_duration(self, path: Path) -> float:
        try:
            result = subprocess.run(
                ["ffprobe", "-v", "error", "-show_entries", "format=duration",
                 "-of", "default=noprint_wrappers=1:nokey=1", str(path)],
                capture_output=True, text=True, timeout=10,
            )
            return float(result.stdout.strip())
        except Exception:
            return 0.0
