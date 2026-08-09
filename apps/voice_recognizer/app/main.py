import asyncio
import logging
import os
import tempfile
import time
import uuid
from collections.abc import AsyncIterator, Callable
from contextlib import asynccontextmanager
from pathlib import Path
from typing import Annotated

from fastapi import FastAPI, File, HTTPException, Query, Request, UploadFile, status
from faster_whisper import WhisperModel
from pydantic import BaseModel

if not logging.getLogger().handlers:
    logging.basicConfig(
        level=os.getenv("LOG_LEVEL", "INFO").upper(),
        format="%(asctime)s %(levelname)s %(name)s: %(message)s",
    )

logging.getLogger("httpx").setLevel(logging.WARNING)
logging.getLogger("huggingface_hub").setLevel(logging.WARNING)
logger = logging.getLogger("voice_recognizer")


class Settings(BaseModel):
    model: str = os.getenv("WHISPER_MODEL", "small")
    device: str = os.getenv("WHISPER_DEVICE", "cpu")
    compute_type: str = os.getenv("WHISPER_COMPUTE_TYPE", "int8")
    cpu_threads: int = int(os.getenv("WHISPER_CPU_THREADS", "4"))
    max_upload_bytes: int = int(os.getenv("STT_MAX_UPLOAD_BYTES", str(25 * 1024 * 1024)))
    nats_url: str = os.getenv("NATS_URL", "nats://nats:4222")


class Segment(BaseModel):
    start: float
    end: float
    text: str


class Transcription(BaseModel):
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


async def save_upload(upload: UploadFile, max_upload_bytes: int) -> tuple[Path, int]:
    suffix = Path(upload.filename or "audio").suffix
    with tempfile.NamedTemporaryFile(suffix=suffix, delete=False) as temporary_file:
        path = Path(temporary_file.name)
        uploaded_bytes = 0
        while chunk := await upload.read(1024 * 1024):
            uploaded_bytes += len(chunk)
            if uploaded_bytes > max_upload_bytes:
                path.unlink(missing_ok=True)
                raise HTTPException(
                    status_code=status.HTTP_413_REQUEST_ENTITY_TOO_LARGE,
                    detail=f"Audio exceeds the {max_upload_bytes}-byte upload limit",
                )
            temporary_file.write(chunk)
    return path, uploaded_bytes


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


def create_app(
    settings: Settings | None = None,
    model_loader: ModelLoader = load_model,
) -> FastAPI:
    service_settings = settings or Settings()

    @asynccontextmanager
    async def lifespan(app: FastAPI) -> AsyncIterator[None]:
        logger.info(
            "loading whisper model model=%s device=%s compute_type=%s cpu_threads=%d",
            service_settings.model,
            service_settings.device,
            service_settings.compute_type,
            service_settings.cpu_threads,
        )
        started_at = time.perf_counter()
        try:
            app.state.model = await asyncio.to_thread(model_loader, service_settings)
        except Exception:
            logger.exception("failed to load whisper model")
            raise
        app.state.transcription_lock = asyncio.Lock()
        if service_settings.nats_url:
            from app.worker import start_worker

            app.state.nats_connection, app.state.nats_worker = await start_worker(
                service_settings.nats_url,
                app.state.model,
                app.state.transcription_lock,
            )
        logger.info("whisper model loaded elapsed_seconds=%.3f", time.perf_counter() - started_at)
        yield
        if hasattr(app.state, "nats_connection"):
            from app.worker import stop_worker

            await stop_worker(app.state.nats_connection, app.state.nats_worker)
        logger.info("voice recognizer stopped")

    app = FastAPI(title="Helio Voice Recognizer", lifespan=lifespan)

    @app.get("/health")
    async def health(request: Request) -> dict[str, str]:
        if not hasattr(request.app.state, "model"):
            raise HTTPException(status_code=status.HTTP_503_SERVICE_UNAVAILABLE, detail="Model is unavailable")
        return {"status": "ok"}

    @app.post("/stt", response_model=Transcription)
    async def speech_to_text(
        request: Request,
        audio: Annotated[UploadFile, File(description="Audio file to transcribe")],
        language: Annotated[str | None, Query(min_length=2, max_length=10)] = None,
    ) -> Transcription:
        if not hasattr(request.app.state, "model"):
            raise HTTPException(status_code=status.HTTP_503_SERVICE_UNAVAILABLE, detail="Model is unavailable")

        request_id = uuid.uuid4().hex[:12]
        upload_started_at = time.perf_counter()
        try:
            audio_path, uploaded_bytes = await save_upload(audio, service_settings.max_upload_bytes)
        except HTTPException:
            logger.warning("audio upload rejected request_id=%s content_type=%s", request_id, audio.content_type)
            raise
        logger.info(
            "audio upload saved request_id=%s content_type=%s size_bytes=%d upload_seconds=%.3f language=%s",
            request_id,
            audio.content_type,
            uploaded_bytes,
            time.perf_counter() - upload_started_at,
            language,
        )
        try:
            queued_at = time.perf_counter()
            logger.info("transcription queued request_id=%s", request_id)
            async with request.app.state.transcription_lock:
                started_at = time.perf_counter()
                logger.info("transcription started request_id=%s queue_seconds=%.3f", request_id, started_at - queued_at)
                result = await asyncio.to_thread(transcribe, request.app.state.model, audio_path, language)
            logger.info(
                "transcription completed request_id=%s transcription_seconds=%.3f audio_duration_seconds=%.3f "
                "language=%s language_probability=%.4f segment_count=%d text_length=%d",
                request_id,
                time.perf_counter() - started_at,
                result.duration,
                result.language,
                result.language_probability,
                len(result.segments),
                len(result.text),
            )
            return result
        except Exception:
            logger.exception("transcription failed request_id=%s", request_id)
            raise
        finally:
            audio_path.unlink(missing_ok=True)
            await audio.close()

    return app


app = create_app()
