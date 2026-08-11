from dataclasses import asdict

from apps.voice_recognizer.app import worker
from apps.voice_recognizer.app.main import transcribe
from apps.voice_recognizer.app.worker import VoiceJob


class FakeSegment:
    def __init__(self, start: float, end: float, text: str) -> None:
        self.start = start
        self.end = end
        self.text = text


class FakeInfo:
    language = "ru"
    language_probability = 0.99
    duration = 1.5


class FakeModel:
    def transcribe(self, audio_path: str, language: str | None, vad_filter: bool):
        assert audio_path
        assert language == "ru"
        assert vad_filter is True
        return iter([FakeSegment(0.0, 1.5, " Привет, мир! ")]), FakeInfo()


def test_transcribe_returns_transcription(tmp_path) -> None:
    result = transcribe(FakeModel(), tmp_path / "voice.ogg", "ru")

    assert asdict(result) == {
        "text": "Привет, мир!",
        "language": "ru",
        "language_probability": 0.99,
        "duration": 1.5,
        "segments": [{"start": 0.0, "end": 1.5, "text": "Привет, мир!"}],
    }


def test_voice_job_preserves_media_file_suffix() -> None:
    legacy_job = VoiceJob(job_id="job", object_name="job.ogg", chat_id=1, message_id=2)
    job = VoiceJob(job_id="job", object_name="job.mp4", chat_id=1, message_id=2, file_suffix=".mp4")

    assert legacy_job.file_suffix == ".ogg"
    assert job.file_suffix == ".mp4"


def test_worker_consumer_configuration() -> None:
    config = worker.worker_consumer_config()

    assert config.durable_name == worker.WORKER_DURABLE
    assert config.filter_subject == worker.JOBS_SUBJECT
    assert config.ack_wait == worker.ACK_WAIT_SECONDS
    assert config.backoff[0] == worker.ACK_WAIT_SECONDS
    assert config.max_deliver == 5
