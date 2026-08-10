// Package voicequeue provides the JetStream-backed voice transcription workflow.
package voicequeue

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nuid"
)

const (
	voiceBucket    = "VOICE"
	streamName     = "STT"
	jobsSubject    = "stt.jobs"
	resultsSubject = "stt.results"
)

// Job describes a voice recording waiting for transcription.
type Job struct {
	JobID      string `json:"job_id"`
	ObjectName string `json:"object_name"`
	ChatID     int64  `json:"chat_id"`
	MessageID  int64  `json:"message_id"`
	Language   string `json:"language,omitempty"`
}

// Result is the transcription emitted by the voice recognizer.
type Result struct {
	JobID                string  `json:"job_id"`
	ChatID               int64   `json:"chat_id"`
	MessageID            int64   `json:"message_id"`
	Text                 string  `json:"text"`
	Language             string  `json:"language"`
	LanguageProbability  float64 `json:"language_probability"`
	TranscriptionSeconds float64 `json:"transcription_seconds"`
	AudioDurationSeconds float64 `json:"audio_duration_seconds"`
}

// Client stores voice files and dispatches transcription jobs through JetStream.
type Client struct {
	nc      *nats.Conn
	js      nats.JetStreamContext
	objects nats.ObjectStore
}

// NewClient connects to JetStream and creates its voice workflow resources.
func NewClient(url string) (*Client, error) {
	nc, err := nats.Connect(url, nats.Name("heliobot-tg-bot"))
	if err != nil {
		return nil, fmt.Errorf("connect to NATS: %w", err)
	}

	js, err := nc.JetStream()
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("create JetStream context: %w", err)
	}
	if _, err := js.StreamInfo(streamName); err != nil {
		if _, err := js.AddStream(&nats.StreamConfig{
			Name:     streamName,
			Subjects: []string{jobsSubject, resultsSubject},
			MaxAge:   24 * time.Hour,
			Storage:  nats.FileStorage,
		}); err != nil {
			nc.Close()
			return nil, fmt.Errorf("create STT stream: %w", err)
		}
	}

	objects, err := js.ObjectStore(voiceBucket)
	if err != nil {
		objects, err = js.CreateObjectStore(&nats.ObjectStoreConfig{
			Bucket:  voiceBucket,
			TTL:     24 * time.Hour,
			Storage: nats.FileStorage,
		})
		if err != nil {
			nc.Close()
			return nil, fmt.Errorf("create voice object store: %w", err)
		}
	}
	return &Client{nc: nc, js: js, objects: objects}, nil
}

// Enqueue stores audio in Object Store and publishes a durable transcription job.
func (c *Client) Enqueue(ctx context.Context, audio io.Reader, chatID, messageID int64) (string, error) {
	jobID := nuid.Next()
	objectName := jobID + ".ogg"
	if _, err := c.objects.Put(&nats.ObjectMeta{Name: objectName}, audio); err != nil {
		return "", fmt.Errorf("store voice object: %w", err)
	}

	data, err := json.Marshal(Job{JobID: jobID, ObjectName: objectName, ChatID: chatID, MessageID: messageID})
	if err != nil {
		return "", fmt.Errorf("encode transcription job: %w", err)
	}
	if _, err := c.js.Publish(jobsSubject, data, nats.Context(ctx)); err != nil {
		return "", fmt.Errorf("publish transcription job: %w", err)
	}
	return jobID, nil
}

// SubscribeResults invokes handler for each durable transcription result.
func (c *Client) SubscribeResults(handler func(Result) error) (*nats.Subscription, error) {
	return c.js.Subscribe(resultsSubject, func(message *nats.Msg) {
		var result Result
		if err := json.Unmarshal(message.Data, &result); err != nil {
			return
		}
		if err := handler(result); err == nil {
			_ = message.Ack()
		}
	}, nats.Durable("telegram-bot-results"), nats.ManualAck())
}

// Close drains outstanding messages and closes the NATS connection.
func (c *Client) Close() {
	_ = c.nc.Drain()
	c.nc.Close()
}
