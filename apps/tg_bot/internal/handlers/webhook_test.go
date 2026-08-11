package handlers

import (
	"testing"

	"tg_bot/internal/telegram"
)

func TestTranscriptionMedia(t *testing.T) {
	tests := []struct {
		name       string
		message    *telegram.Message
		fileID     string
		duration   int
		fileSuffix string
		mediaType  string
	}{
		{
			name: "voice message",
			message: &telegram.Message{Voice: &telegram.Voice{
				FileID:   "voice-file",
				Duration: 12,
			}},
			fileID: "voice-file", duration: 12, fileSuffix: ".ogg", mediaType: "voice",
		},
		{
			name: "video note",
			message: &telegram.Message{VideoNote: &telegram.VideoNote{
				FileID:   "video-note-file",
				Duration: 42,
			}},
			fileID: "video-note-file", duration: 42, fileSuffix: ".mp4", mediaType: "video_note",
		},
		{
			name:    "unsupported message",
			message: &telegram.Message{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fileID, duration, fileSuffix, mediaType := transcriptionMedia(test.message)
			if fileID != test.fileID || duration != test.duration || fileSuffix != test.fileSuffix || mediaType != test.mediaType {
				t.Fatalf("transcriptionMedia() = (%q, %d, %q, %q), want (%q, %d, %q, %q)", fileID, duration, fileSuffix, mediaType, test.fileID, test.duration, test.fileSuffix, test.mediaType)
			}
		})
	}
}

func TestSafeRedirect(t *testing.T) {
	authHandler := &Auth{dashURL: "https://dashboard.example.com"}

	for _, target := range []string{"//attacker.example", "https://attacker.example", "\\attacker.example", "dashboard"} {
		if got := authHandler.safeRedirect(target); got != authHandler.dashURL {
			t.Errorf("safeRedirect(%q) = %q, want dashboard URL", target, got)
		}
	}
	if got := authHandler.safeRedirect("/dashboard?view=voice"); got != "/dashboard?view=voice" {
		t.Errorf("safeRedirect() = %q, want local dashboard path", got)
	}
}
