import asyncio
import os
import tempfile
from collections.abc import AsyncIterator, Callable
from contextlib import asynccontextmanager
from pathlib import Path
from typing import Annotated

from fastapi import FastAPI, File, HTTPException, Query, Request, UploadFile, status
from faster_whisper import WhisperModel
from pydantic import BaseModel


class Settings(BaseModel):
    model: str = os.getenv("WHISPER_MODEL", "small")
    device: str = os.getenv("WHISPER_DEVICE", "cpu")
    compute_type: str = os.getenv("WHISPER_COMPUTE_TYPE", "int8")
    cpu_threads: int = int(os.getenv("WHISPER_CPU_THREADS", "4"))
    max_upload_bytes: int = int(os.getenv("STT_MAX_UPLOAD_BYTES", str(25 * 1024 * 1024)))


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


async def save_upload(upload: UploadFile, max_upload_bytes: int) -> Path:
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
    return path


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
        app.state.model = await asyncio.to_thread(model_loader, service_settings)
        app.state.transcription_lock = asyncio.Lock()
        yield

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

        audio_path = await save_upload(audio, service_settings.max_upload_bytes)
        try:
            async with request.app.state.transcription_lock:
                return await asyncio.to_thread(transcribe, request.app.state.model, audio_path, language)
        finally:
            audio_path.unlink(missing_ok=True)
            await audio.close()

    return app


app = create_app()
