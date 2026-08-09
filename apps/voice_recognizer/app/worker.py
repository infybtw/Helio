import asyncio
import contextlib
import json
import logging
import tempfile
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


async def start_worker(nats_url: str, model: Any, transcription_lock: asyncio.Lock) -> tuple[Any, asyncio.Task[None]]:
    connection = await nats.connect(nats_url)
    jetstream = connection.jetstream()
    try:
        await jetstream.stream_info("STT")
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
        except Exception:
            # Another service can create the same shared stream during startup.
            await jetstream.stream_info("STT")

    try:
        object_store = await jetstream.object_store(VOICE_BUCKET)
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
        except Exception:
            # Another service can create the bucket during startup.
            object_store = await jetstream.object_store(VOICE_BUCKET)

    async def process(message: Msg) -> None:
        try:
            job = VoiceJob.model_validate_json(message.data)
            logger.info("processing transcription job job_id=%s", job.job_id)
            object_result = await object_store.get(job.object_name)
            with tempfile.NamedTemporaryFile(suffix=".ogg", delete=False) as audio_file:
                audio_path = Path(audio_file.name)
                audio_file.write(object_result.data)
            try:
                async with transcription_lock:
                    result = await asyncio.to_thread(transcribe, model, audio_path, job.language)
            finally:
                audio_path.unlink(missing_ok=True)

            await jetstream.publish(
                RESULTS_SUBJECT,
                TranscriptionResult(
                    job_id=job.job_id,
                    chat_id=job.chat_id,
                    message_id=job.message_id,
                    text=result.text,
                    language=result.language,
                ).model_dump_json().encode(),
            )
            await message.ack()
            logger.info("published transcription result job_id=%s", job.job_id)
        except Exception:
            logger.exception("failed to process transcription job")
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

    logger.info("connected to JetStream nats_url=%s subject=%s", nats_url, JOBS_SUBJECT)
    return connection, asyncio.create_task(consume())


async def stop_worker(connection: Any, task: asyncio.Task[None]) -> None:
    task.cancel()
    with contextlib.suppress(asyncio.CancelledError):
        await task
    await connection.drain()
