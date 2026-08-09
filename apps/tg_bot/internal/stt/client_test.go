package stt

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTranscribeUploadsAudio(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if err := r.ParseMultipartForm(1024); err != nil {
			t.Fatalf("parse multipart form: %v", err)
		}
		file, header, err := r.FormFile("audio")
		if err != nil {
			t.Fatalf("read audio form file: %v", err)
		}
		defer file.Close()
		body, err := io.ReadAll(file)
		if err != nil {
			t.Fatalf("read audio: %v", err)
		}
		if header.Filename != "voice.ogg" || string(body) != "audio" {
			t.Fatalf("uploaded file = %q %q", header.Filename, body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"text":"Привет"}`))
	}))
	defer server.Close()

	text, err := NewClient(server.URL).Transcribe(context.Background(), strings.NewReader("audio"), "voice.ogg")
	if err != nil {
		t.Fatalf("Transcribe() error = %v", err)
	}
	if text != "Привет" {
		t.Fatalf("Transcribe() text = %q, want %q", text, "Привет")
	}
}

func TestTranscribeRejectsServiceError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	_, err := NewClient(server.URL).Transcribe(context.Background(), strings.NewReader("audio"), "voice.ogg")
	if err == nil || !strings.Contains(err.Error(), "503") {
		t.Fatalf("Transcribe() error = %v, want status error", err)
	}
}
