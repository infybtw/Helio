import asyncio
import logging
import os
import time
from collections.abc import Callable
from dataclasses import dataclass
from pathlib import Path

from faster_whisper import WhisperModel

if not logging.getLogger().handlers:
    logging.basicConfig(
        level=os.getenv("LOG_LEVEL", "INFO").upper(),
        format="%(asctime)s %(levelname)s %(name)s: %(message)s",
    )

logging.getLogger("httpx").setLevel(logging.WARNING)
logging.getLogger("huggingface_hub").setLevel(logging.WARNING)
logging.getLogger("nats").setLevel(logging.CRITICAL)
logger = logging.getLogger("voice_recognizer")


@dataclass
class Settings:
    model: str = os.getenv("WHISPER_MODEL", "small")
    device: str = os.getenv("WHISPER_DEVICE", "cpu")
    compute_type: str = os.getenv("WHISPER_COMPUTE_TYPE", "int8")
    cpu_threads: int = int(os.getenv("WHISPER_CPU_THREADS", "4"))
    nats_url: str = os.getenv("NATS_URL", "nats://nats:4222")


@dataclass
class Segment:
    start: float
    end: float
    text: str


@dataclass
class Transcription:
    text: str
    language: str
    language_probability: float
    duration: float
    segments: list[Segment]


ModelLoader = Callable[[Settings], WhisperModel]


def load_model(settings: Settings) -> WhisperModel:
    return WhisperModel(
        settings.model,
        device=settings.device,
        compute_type=settings.compute_type,
        cpu_threads=settings.cpu_threads,
    )


def transcribe(model: WhisperModel, audio_path: Path, language: str | None) -> Transcription:
    segments, info = model.transcribe(str(audio_path), language=language, vad_filter=True)
    result_segments = [
        Segment(start=segment.start, end=segment.end, text=segment.text.strip())
        for segment in segments
    ]
    return Transcription(
        text=" ".join(segment.text for segment in result_segments).strip(),
        language=info.language,
        language_probability=info.language_probability,
        duration=info.duration,
        segments=result_segments,
    )


def main(model_loader: ModelLoader = load_model) -> None:
    settings = Settings()
    logger.info(
        "loading whisper model model=%s device=%s compute_type=%s cpu_threads=%d",
        settings.model,
        settings.device,
        settings.compute_type,
        settings.cpu_threads,
    )
    started_at = time.perf_counter()
    try:
        model = model_loader(settings)
    except Exception:
        logger.exception("failed to load whisper model")
        raise
    logger.info("whisper model loaded elapsed_seconds=%.3f", time.perf_counter() - started_at)

    from app.worker import run_worker

    asyncio.run(run_worker(settings.nats_url, model))


if __name__ == "__main__":
    main()
