import asyncio
import contextlib
import json
import logging
import tempfile
import time
from dataclasses import asdict, dataclass
from pathlib import Path
from typing import Any

import nats
from nats.aio.msg import Msg
from nats.errors import TimeoutError
from nats.js.api import AckPolicy, ConsumerConfig, ObjectStoreConfig, StorageType, StreamConfig
from nats.js.errors import BucketNotFoundError, NotFoundError

from app.main import transcribe

logger = logging.getLogger("voice_recognizer")

VOICE_BUCKET = "VOICE"
JOBS_SUBJECT = "stt.jobs"
RESULTS_SUBJECT = "stt.results"
WORKER_DURABLE = "voice-recognizer-workers"
ACK_WAIT_SECONDS = 5 * 60
HEARTBEAT_INTERVAL_SECONDS = 60


@dataclass
class VoiceJob:
    job_id: str
    object_name: str
    chat_id: int
    message_id: int
    language: str | None = None
    file_suffix: str = ".ogg"


@dataclass
class TranscriptionResult:
    job_id: str
    chat_id: int
    message_id: int
    text: str
    language: str
    language_probability: float
    transcription_seconds: float
    audio_duration_seconds: float


def worker_consumer_config() -> ConsumerConfig:
    return ConsumerConfig(
        durable_name=WORKER_DURABLE,
        filter_subject=JOBS_SUBJECT,
        ack_policy=AckPolicy.EXPLICIT,
        ack_wait=ACK_WAIT_SECONDS,
        max_deliver=5,
        backoff=[ACK_WAIT_SECONDS, 15 * 60, 30 * 60, 60 * 60, 2 * 60 * 60],
    )


async def keep_message_alive(message: Msg) -> None:
    """Reset the JetStream ACK deadline while synchronous inference runs."""
    while True:
        await asyncio.sleep(HEARTBEAT_INTERVAL_SECONDS)
        await message.in_progress()


async def run_worker(nats_url: str, model: Any) -> None:
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
            job = VoiceJob(**json.loads(message.data))
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
            with tempfile.NamedTemporaryFile(suffix=job.file_suffix, delete=False) as audio_file:
                audio_path = Path(audio_file.name)
                audio_file.write(audio_bytes)
            heartbeat_task: asyncio.Task[None] | None = None
            try:
                transcription_started_at = time.perf_counter()
                logger.info("job transcription started job_id=%s", job.job_id)
                heartbeat_task = asyncio.create_task(keep_message_alive(message))
                result = await asyncio.to_thread(transcribe, model, audio_path, job.language)
            finally:
                if heartbeat_task is not None:
                    heartbeat_task.cancel()
                    with contextlib.suppress(asyncio.CancelledError):
                        await heartbeat_task
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
            transcription_result = TranscriptionResult(
                job_id=job.job_id,
                chat_id=job.chat_id,
                message_id=job.message_id,
                text=result.text,
                language=result.language,
                language_probability=result.language_probability,
                transcription_seconds=transcription_seconds,
                audio_duration_seconds=result.duration,
            )
            await jetstream.publish(
                RESULTS_SUBJECT,
                json.dumps(asdict(transcription_result), separators=(",", ":")).encode(),
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
            await message.nak()

    consumer_config = worker_consumer_config()
    try:
        await jetstream.consumer_info("STT", WORKER_DURABLE)
        # JetStream uses the durable consumer create endpoint for updates too.
        await jetstream.add_consumer("STT", consumer_config)
    except NotFoundError:
        await jetstream.add_consumer("STT", consumer_config)

    subscription = await jetstream.pull_subscribe(
        JOBS_SUBJECT,
        durable=WORKER_DURABLE,
        stream="STT",
    )

    logger.info("JetStream worker started nats_url=%s subject=%s durable=%s ack_wait_seconds=%d", nats_url, JOBS_SUBJECT, WORKER_DURABLE, ACK_WAIT_SECONDS)
    try:
        while True:
            try:
                messages = await subscription.fetch(1, timeout=1)
            except TimeoutError:
                continue
            for message in messages:
                await process(message)
    finally:
        logger.info("stopping JetStream worker")
        await connection.drain()
        logger.info("JetStream worker stopped")
