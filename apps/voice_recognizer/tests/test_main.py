from fastapi.testclient import TestClient

from apps.voice_recognizer.app.main import Settings, create_app


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
        Settings(max_upload_bytes=max_upload_bytes),
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
