import asyncio
import contextlib
import json
import logging
import tempfile
import time
from pathlib import Path
from typing import Any

import nats
from nats.aio.msg import Msg
from nats.errors import TimeoutError
from nats.js.api import ObjectStoreConfig, StorageType, StreamConfig
from nats.js.errors import BucketNotFoundError, NotFoundError
from pydantic import BaseModel

from app.main import Transcription, transcribe

logger = logging.getLogger("voice_recognizer")

VOICE_BUCKET = "VOICE"
JOBS_SUBJECT = "stt.jobs"
RESULTS_SUBJECT = "stt.results"


class VoiceJob(BaseModel):
    job_id: str
    object_name: str
    chat_id: int
    message_id: int
    language: str | None = None


class TranscriptionResult(BaseModel):
    job_id: str
    chat_id: int
    message_id: int
    text: str
    language: str
    language_probability: float
    transcription_seconds: float
    audio_duration_seconds: float


async def start_worker(nats_url: str, model: Any, transcription_lock: asyncio.Lock) -> tuple[Any, asyncio.Task[None]]:
    async def on_disconnected() -> None:
        logger.warning("JetStream connection lost; retrying in 30 seconds")

    async def on_reconnected() -> None:
        logger.info("JetStream connection restored")

    async def on_error(error: Exception) -> None:
        logger.warning("JetStream reconnect attempt failed error=%s", error)

    connection = await nats.connect(
        nats_url,
        reconnect_time_wait=30,
        max_reconnect_attempts=-1,
        error_cb=on_error,
        disconnected_cb=on_disconnected,
        reconnected_cb=on_reconnected,
    )
    jetstream = connection.jetstream()
    try:
        await jetstream.stream_info("STT")
        logger.info("JetStream stream ready stream=STT")
    except NotFoundError:
        try:
            await jetstream.add_stream(
                StreamConfig(
                    name="STT",
                    subjects=[JOBS_SUBJECT, RESULTS_SUBJECT],
                    max_age=24 * 60 * 60,
                    storage=StorageType.FILE,
                )
            )
            logger.info("JetStream stream created stream=STT")
        except Exception:
            # Another service can create the same shared stream during startup.
            await jetstream.stream_info("STT")

    try:
        object_store = await jetstream.object_store(VOICE_BUCKET)
        logger.info("JetStream object store ready bucket=%s", VOICE_BUCKET)
    except BucketNotFoundError:
        try:
            object_store = await jetstream.create_object_store(
                VOICE_BUCKET,
                config=ObjectStoreConfig(
                    bucket=VOICE_BUCKET,
                    ttl=24 * 60 * 60,
                    storage=StorageType.FILE,
                )
            )
            logger.info("JetStream object store created bucket=%s", VOICE_BUCKET)
        except Exception:
            # Another service can create the bucket during startup.
            object_store = await jetstream.object_store(VOICE_BUCKET)

    async def process(message: Msg) -> None:
        job: VoiceJob | None = None
        started_at = time.perf_counter()
        try:
            job = VoiceJob.model_validate_json(message.data)
            logger.info(
                "job received job_id=%s chat_id=%d message_id=%d object=%s language=%s",
                job.job_id,
                job.chat_id,
                job.message_id,
                job.object_name,
                job.language,
            )

            object_started_at = time.perf_counter()
            object_result = await object_store.get(job.object_name)
            audio_bytes = object_result.data
            logger.info(
                "job audio loaded job_id=%s size_bytes=%d object_store_seconds=%.3f",
                job.job_id,
                len(audio_bytes),
                time.perf_counter() - object_started_at,
            )
            with tempfile.NamedTemporaryFile(suffix=".ogg", delete=False) as audio_file:
                audio_path = Path(audio_file.name)
                audio_file.write(audio_bytes)
            try:
                queued_at = time.perf_counter()
                logger.info("job waiting for model job_id=%s", job.job_id)
                async with transcription_lock:
                    transcription_started_at = time.perf_counter()
                    logger.info(
                        "job transcription started job_id=%s queue_seconds=%.3f",
                        job.job_id,
                        transcription_started_at - queued_at,
                    )
                    result = await asyncio.to_thread(transcribe, model, audio_path, job.language)
            finally:
                audio_path.unlink(missing_ok=True)
            transcription_seconds = time.perf_counter() - transcription_started_at
            logger.info(
                "job transcription completed job_id=%s transcription_seconds=%.3f audio_duration_seconds=%.3f "
                "language=%s language_probability=%.4f segment_count=%d text_length=%d",
                job.job_id,
                transcription_seconds,
                result.duration,
                result.language,
                result.language_probability,
                len(result.segments),
                len(result.text),
            )

            publish_started_at = time.perf_counter()
            await jetstream.publish(
                RESULTS_SUBJECT,
                TranscriptionResult(
                    job_id=job.job_id,
                    chat_id=job.chat_id,
                    message_id=job.message_id,
                    text=result.text,
                    language=result.language,
                    language_probability=result.language_probability,
                    transcription_seconds=transcription_seconds,
                    audio_duration_seconds=result.duration,
                ).model_dump_json().encode(),
            )
            logger.info(
                "job result published job_id=%s publish_seconds=%.3f",
                job.job_id,
                time.perf_counter() - publish_started_at,
            )
            await message.ack()
            logger.info("job acknowledged job_id=%s total_seconds=%.3f", job.job_id, time.perf_counter() - started_at)
        except Exception:
            logger.exception("job failed job_id=%s", job.job_id if job else "unknown")
            await asyncio.sleep(1)
            await message.nak()

    subscription = await jetstream.pull_subscribe(
        JOBS_SUBJECT,
        durable="voice-recognizer-workers",
        stream="STT",
    )

    async def consume() -> None:
        while True:
            try:
                messages = await subscription.fetch(1, timeout=1)
            except TimeoutError:
                continue
            for message in messages:
                await process(message)

    logger.info("JetStream worker started nats_url=%s subject=%s durable=voice-recognizer-workers", nats_url, JOBS_SUBJECT)
    return connection, asyncio.create_task(consume())


async def stop_worker(connection: Any, task: asyncio.Task[None]) -> None:
    logger.info("stopping JetStream worker")
    task.cancel()
    with contextlib.suppress(asyncio.CancelledError):
        await task
    await connection.drain()
    logger.info("JetStream worker stopped")
