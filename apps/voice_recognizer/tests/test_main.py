from fastapi.testclient import TestClient

from apps.voice_recognizer.app.main import Settings, create_app
from apps.voice_recognizer.app import worker
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


def create_test_client(max_upload_bytes: int = 1024) -> TestClient:
    app = create_app(
        Settings(max_upload_bytes=max_upload_bytes, nats_url=""),
        model_loader=lambda _settings: FakeModel(),
    )
    return TestClient(app)


def test_health_reports_ready() -> None:
    with create_test_client() as client:
        response = client.get("/health")

    assert response.status_code == 200
    assert response.json() == {"status": "ok"}


def test_stt_returns_transcription() -> None:
    with create_test_client() as client:
        response = client.post(
            "/stt?language=ru",
            files={"audio": ("voice.ogg", b"audio", "audio/ogg")},
        )

    assert response.status_code == 200
    assert response.json() == {
        "text": "Привет, мир!",
        "language": "ru",
        "language_probability": 0.99,
        "duration": 1.5,
        "segments": [{"start": 0.0, "end": 1.5, "text": "Привет, мир!"}],
    }


def test_stt_rejects_large_upload() -> None:
    with create_test_client(max_upload_bytes=3) as client:
        response = client.post("/stt", files={"audio": ("voice.ogg", b"audio")})

    assert response.status_code == 413


def test_voice_job_preserves_media_file_suffix() -> None:
    legacy_job = VoiceJob(job_id="job", object_name="job.ogg", chat_id=1, message_id=2)
    job = VoiceJob(job_id="job", object_name="job.mp4", chat_id=1, message_id=2, file_suffix=".mp4")

    assert legacy_job.file_suffix == ".ogg"
    assert job.file_suffix == ".mp4"


def test_worker_consumer_extends_long_running_jobs() -> None:
    config = worker.worker_consumer_config()

    assert config.durable_name == worker.WORKER_DURABLE
    assert config.filter_subject == worker.JOBS_SUBJECT
    assert config.ack_wait == worker.ACK_WAIT_SECONDS
    assert config.backoff[0] == worker.ACK_WAIT_SECONDS
    assert config.max_deliver == 5
