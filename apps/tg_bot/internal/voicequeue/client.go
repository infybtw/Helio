// Package voicequeue provides the JetStream-backed voice transcription workflow.
package voicequeue

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"sync"
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
	FileSuffix string `json:"file_suffix,omitempty"`
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
	url     string
	log     *slog.Logger
	mu      sync.Mutex
	nc      *nats.Conn
	js      nats.JetStreamContext
	objects nats.ObjectStore
	handler func(Result) error
	sub     *nats.Subscription
}

// NewClient connects to JetStream and creates its voice workflow resources.
func NewClient(url string, log *slog.Logger) (*Client, error) {
	client := &Client{url: url, log: log}
	client.mu.Lock()
	defer client.mu.Unlock()
	if err := client.connectLocked(false); err != nil {
		return nil, err
	}
	return client, nil
}

func (c *Client) connectLocked(reconnecting bool) error {
	if c.nc != nil {
		c.nc.Close()
	}
	nc, err := nats.Connect(c.url,
		nats.Name("heliobot-tg-bot"),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(30*time.Second),
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			c.log.Warn("NATS connection lost", "error", err, "retry_interval", "30s")
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			c.log.Info("NATS connection restored", "server", nc.ConnectedUrl())
		}),
	)
	if err != nil {
		if reconnecting {
			c.log.Warn("NATS reconnect attempt failed", "error", err)
		}
		return fmt.Errorf("connect to NATS: %w", err)
	}

	js, err := nc.JetStream()
	if err != nil {
		nc.Close()
		return fmt.Errorf("create JetStream context: %w", err)
	}
	if _, err := js.StreamInfo(streamName); err != nil {
		if _, err := js.AddStream(&nats.StreamConfig{
			Name:     streamName,
			Subjects: []string{jobsSubject, resultsSubject},
			MaxAge:   24 * time.Hour,
			Storage:  nats.FileStorage,
		}); err != nil {
			nc.Close()
			return fmt.Errorf("create STT stream: %w", err)
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
			return fmt.Errorf("create voice object store: %w", err)
		}
	}
	c.nc, c.js, c.objects = nc, js, objects
	if c.handler != nil {
		sub, err := c.subscribeResultsLocked(c.handler)
		if err != nil {
			nc.Close()
			return err
		}
		c.sub = sub
	}
	if reconnecting {
		c.log.Info("NATS reconnect attempt succeeded", "server", nc.ConnectedUrl())
	}
	return nil
}

// Enqueue stores media in Object Store and publishes a durable transcription job.
func (c *Client) Enqueue(ctx context.Context, media io.Reader, chatID, messageID int64, fileSuffix string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.nc == nil || !c.nc.IsConnected() {
		if err := c.connectLocked(true); err != nil {
			return "", fmt.Errorf("reconnect to NATS: %w", err)
		}
	}
	jobID := nuid.Next()
	if fileSuffix == "" {
		fileSuffix = ".ogg"
	}
	objectName := jobID + fileSuffix
	if _, err := c.objects.Put(&nats.ObjectMeta{Name: objectName}, media); err != nil {
		return "", fmt.Errorf("store media object: %w", err)
	}

	data, err := json.Marshal(Job{JobID: jobID, ObjectName: objectName, ChatID: chatID, MessageID: messageID, FileSuffix: fileSuffix})
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
	c.mu.Lock()
	defer c.mu.Unlock()
	c.handler = handler
	if c.nc == nil || !c.nc.IsConnected() {
		if err := c.connectLocked(true); err != nil {
			return nil, fmt.Errorf("reconnect to NATS: %w", err)
		}
		return c.sub, nil
	}
	sub, err := c.subscribeResultsLocked(handler)
	if err == nil {
		c.sub = sub
	}
	return sub, err
}

func (c *Client) subscribeResultsLocked(handler func(Result) error) (*nats.Subscription, error) {
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
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.nc == nil {
		return
	}
	_ = c.nc.Drain()
	c.nc.Close()
}
