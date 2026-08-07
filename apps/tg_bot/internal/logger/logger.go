// Package logger provides a colorized slog handler for human-readable output.
package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"
)

const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorYellow = "\033[33m"
	colorCyan   = "\033[36m"
	colorGray   = "\033[90m"
)

var levelColors = map[slog.Level]string{
	slog.LevelDebug: colorGray,
	slog.LevelInfo:  colorCyan,
	slog.LevelWarn:  colorYellow,
	slog.LevelError: colorRed,
}

// Handler is a slog.Handler that writes colored, human-readable log lines.
type Handler struct {
	mu     *sync.Mutex
	w      io.Writer
	level  slog.Leveler
	attrs  []slog.Attr
	groups []string
}

// New creates a slog.Logger using the colorized Handler.
func New(w io.Writer, level slog.Leveler) *slog.Logger {
	return slog.New(NewHandler(w, level))
}

// NewHandler creates the colorized Handler writing to w at the given level.
func NewHandler(w io.Writer, level slog.Leveler) *Handler {
	return &Handler{mu: &sync.Mutex{}, w: w, level: level}
}

func (h *Handler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level.Level()
}

func (h *Handler) Handle(_ context.Context, r slog.Record) error {
	var sb strings.Builder

	sb.WriteString(r.Time.Format(time.DateTime))
	sb.WriteByte(' ')

	color := levelColors[r.Level]
	if color == "" {
		color = colorReset
	}
	fmt.Fprintf(&sb, "%s%-5s%s ", color, r.Level.String(), colorReset)

	sb.WriteString(r.Message)

	writeAttr := func(a slog.Attr) bool {
		if a.Equal(slog.Attr{}) {
			return true
		}
		sb.WriteByte(' ')
		sb.WriteString(colorGray)
		if len(h.groups) > 0 {
			sb.WriteString(strings.Join(h.groups, "."))
			sb.WriteByte('.')
		}
		sb.WriteString(a.Key)
		sb.WriteByte('=')
		sb.WriteString(colorReset)
		sb.WriteString(a.Value.String())
		return true
	}

	for _, a := range h.attrs {
		writeAttr(a)
	}
	r.Attrs(writeAttr)

	sb.WriteByte('\n')

	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := io.WriteString(h.w, sb.String())
	return err
}

func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	clone := *h
	clone.attrs = append(append([]slog.Attr{}, h.attrs...), attrs...)
	return &clone
}

func (h *Handler) WithGroup(name string) slog.Handler {
	clone := *h
	clone.groups = append(append([]string{}, h.groups...), name)
	return &clone
}
