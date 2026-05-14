// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"sync"
	"testing"
)

// captureHandler is a minimal slog.Handler that records the records it
// processes, used to assert that JavaLoggingInfoStream forwards
// messages correctly. It supports WithAttrs by carrying the inherited
// attrs into each emitted record, mirroring slog.TextHandler semantics.
type captureHandler struct {
	mu      *sync.Mutex
	records *[]slog.Record
	level   slog.Leveler
	attrs   []slog.Attr
}

func (h *captureHandler) Enabled(_ context.Context, lvl slog.Level) bool {
	if h.level == nil {
		return true
	}
	return lvl >= h.level.Level()
}
func (h *captureHandler) Handle(_ context.Context, r slog.Record) error {
	clone := r.Clone()
	if len(h.attrs) > 0 {
		clone.AddAttrs(h.attrs...)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	*h.records = append(*h.records, clone)
	return nil
}
func (h *captureHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	merged := make([]slog.Attr, 0, len(h.attrs)+len(attrs))
	merged = append(merged, h.attrs...)
	merged = append(merged, attrs...)
	return &captureHandler{mu: h.mu, records: h.records, level: h.level, attrs: merged}
}
func (h *captureHandler) WithGroup(_ string) slog.Handler { return h }
func (h *captureHandler) Records() []slog.Record {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]slog.Record(nil), (*h.records)...)
}

func newCaptureLogger(lvl slog.Leveler) (*slog.Logger, *captureHandler) {
	var records []slog.Record
	h := &captureHandler{mu: &sync.Mutex{}, records: &records, level: lvl}
	return slog.New(h), h
}

func TestJavaLoggingInfoStream_DefaultPrefixUsed(t *testing.T) {
	base, h := newCaptureLogger(slog.LevelDebug)
	is := NewJavaLoggingInfoStream(slog.LevelInfo)
	is.Base = base
	is.Message("MS", "hello")
	records := h.Records()
	if len(records) != 1 {
		t.Fatalf("records=%d want 1", len(records))
	}
	rec := records[0]
	if got := loggerAttr(rec); got != "org.apache.lucene.MS" {
		t.Fatalf("logger=%q want %q", got, "org.apache.lucene.MS")
	}
	if rec.Message != "hello" {
		t.Fatalf("message=%q want %q", rec.Message, "hello")
	}
}

func TestJavaLoggingInfoStream_WithPrefix(t *testing.T) {
	base, h := newCaptureLogger(slog.LevelDebug)
	is := NewJavaLoggingInfoStreamWithPrefix("acme.", slog.LevelInfo)
	is.Base = base
	is.Message("Merge", "abc")
	recs := h.Records()
	if len(recs) != 1 {
		t.Fatalf("records=%d", len(recs))
	}
	if logger := loggerAttr(recs[0]); logger != "acme.Merge" {
		t.Fatalf("logger=%q want acme.Merge", logger)
	}
}

func TestJavaLoggingInfoStream_WithPrefixPanicsOnInvalid(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic on prefix missing trailing dot")
		}
	}()
	NewJavaLoggingInfoStreamWithPrefix("acme", slog.LevelInfo)
}

func TestJavaLoggingInfoStream_WithMapper(t *testing.T) {
	base, h := newCaptureLogger(slog.LevelDebug)
	is := NewJavaLoggingInfoStreamWithMapper(func(c string) string {
		return strings.ToLower(c) + "-suffix"
	}, slog.LevelInfo)
	is.Base = base
	is.Message("FOO", "hi")
	recs := h.Records()
	if len(recs) == 0 {
		t.Fatalf("no records captured")
	}
	if logger := loggerAttr(recs[0]); logger != "foo-suffix" {
		t.Fatalf("logger=%q want foo-suffix", logger)
	}
}

func TestJavaLoggingInfoStream_WithMapperRejectsNil(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic on nil mapper")
		}
	}()
	NewJavaLoggingInfoStreamWithMapper(nil, slog.LevelInfo)
}

func TestJavaLoggingInfoStream_IsEnabledRespectsLevel(t *testing.T) {
	disabled, _ := newCaptureLogger(slog.LevelWarn)
	is := NewJavaLoggingInfoStream(slog.LevelInfo)
	is.Base = disabled
	if is.IsEnabled("X") {
		t.Fatalf("IsEnabled should be false when handler level > stream level")
	}

	enabled, _ := newCaptureLogger(slog.LevelDebug)
	is2 := NewJavaLoggingInfoStream(slog.LevelInfo)
	is2.Base = enabled
	if !is2.IsEnabled("X") {
		t.Fatalf("IsEnabled should be true when handler level <= stream level")
	}
}

func TestJavaLoggingInfoStream_DisabledLevelDropsMessages(t *testing.T) {
	base, h := newCaptureLogger(slog.LevelError)
	is := NewJavaLoggingInfoStream(slog.LevelInfo)
	is.Base = base
	is.Message("Q", "ignored")
	if recs := h.Records(); len(recs) != 0 {
		t.Fatalf("records=%d want 0 (level disabled)", len(recs))
	}
}

func TestJavaLoggingInfoStream_CacheReuse(t *testing.T) {
	base, h := newCaptureLogger(slog.LevelDebug)
	is := NewJavaLoggingInfoStream(slog.LevelInfo)
	is.Base = base
	is.Message("X", "a")
	is.Message("X", "b")
	recs := h.Records()
	if len(recs) != 2 {
		t.Fatalf("records=%d want 2", len(recs))
	}
	for _, r := range recs {
		if loggerAttr(r) != "org.apache.lucene.X" {
			t.Fatalf("unexpected logger %q", loggerAttr(r))
		}
	}
}

func TestJavaLoggingInfoStream_Close(t *testing.T) {
	is := NewJavaLoggingInfoStream(slog.LevelInfo)
	is.Base = slog.Default()
	is.Message("X", "warm cache")
	if err := is.Close(); err != nil {
		t.Fatalf("Close error: %v", err)
	}
	// After Close the cache should be empty; subsequent Message calls
	// must still succeed (lazy re-creation).
	is.Message("X", "again")
}

func TestJavaLoggingInfoStream_ImplementsInfoStream(t *testing.T) {
	var _ InfoStream = NewJavaLoggingInfoStream(slog.LevelInfo)
}

func TestJavaLoggingInfoStream_UsesSlogDefaultWhenBaseNil(t *testing.T) {
	var buf bytes.Buffer
	original := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(original) })

	is := NewJavaLoggingInfoStream(slog.LevelInfo)
	is.Message("X", "default-bridge")
	if !strings.Contains(buf.String(), "default-bridge") {
		t.Fatalf("expected default slog to receive message, got: %s", buf.String())
	}
	if !strings.Contains(buf.String(), "org.apache.lucene.X") {
		t.Fatalf("expected default logger name in output, got: %s", buf.String())
	}
}

// loggerAttr extracts the "logger" attribute value from a slog.Record.
func loggerAttr(r slog.Record) string {
	var name string
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == "logger" {
			name = a.Value.String()
			return false
		}
		return true
	})
	return name
}
