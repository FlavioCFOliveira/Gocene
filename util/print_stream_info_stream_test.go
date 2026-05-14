// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"bytes"
	"context"
	"os"
	"regexp"
	"runtime/pprof"
	"strings"
	"sync"
	"testing"
)

// TestPrintStreamInfoStream_MessageFormat verifies the Java
// "<component> <messageID> [<timestamp>; <thread>]: <message>" layout.
func TestPrintStreamInfoStream_MessageFormat(t *testing.T) {
	var buf bytes.Buffer
	is := NewPrintStreamInfoStreamWithID(&buf, 42)
	is.Message("IW", "hello world")
	line := buf.String()

	re := regexp.MustCompile(`^IW 42 \[[^;]+; [^\]]+\]: hello world\n$`)
	if !re.MatchString(line) {
		t.Fatalf("line=%q does not match expected format", line)
	}
}

// TestPrintStreamInfoStream_IsEnabled mirrors the always-true Java
// contract.
func TestPrintStreamInfoStream_IsEnabled(t *testing.T) {
	is := NewPrintStreamInfoStream(&bytes.Buffer{})
	if !is.IsEnabled("any") {
		t.Fatalf("expected IsEnabled to be true")
	}
}

// TestPrintStreamInfoStream_DistinctMessageIDs covers the process-wide
// AtomicInteger-backed id allocator.
func TestPrintStreamInfoStream_DistinctMessageIDs(t *testing.T) {
	a := NewPrintStreamInfoStream(&bytes.Buffer{})
	b := NewPrintStreamInfoStream(&bytes.Buffer{})
	if a.MessageID() == b.MessageID() {
		t.Fatalf("expected distinct ids, both got %d", a.MessageID())
	}
}

// TestPrintStreamInfoStream_CloseGuardsSystemStream verifies the
// io.Stdout/Stderr guard from the Java close() method.
func TestPrintStreamInfoStream_CloseGuardsSystemStream(t *testing.T) {
	is := NewPrintStreamInfoStream(os.Stdout)
	if !is.IsSystemStream() {
		t.Fatalf("expected stdout to be a system stream")
	}
	// Should be a no-op (won't actually close stdout).
	if err := is.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// closableBuffer is a bytes.Buffer with a Close method that flips a
// flag — used to test the non-system-stream close path.
type closableBuffer struct {
	bytes.Buffer
	closed bool
}

func (c *closableBuffer) Close() error {
	c.closed = true
	return nil
}

// TestPrintStreamInfoStream_CloseNonSystemStream verifies Close
// propagates to the underlying io.Closer when the stream is not stdout
// or stderr.
func TestPrintStreamInfoStream_CloseNonSystemStream(t *testing.T) {
	w := &closableBuffer{}
	is := NewPrintStreamInfoStream(w)
	if err := is.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if !w.closed {
		t.Fatalf("expected underlying writer to be closed")
	}
}

// TestPrintStreamInfoStream_LabelInheritance verifies the goroutine
// name resolution: MessageCtx with a pprof-labelled context surfaces
// the lucene-thread label.
func TestPrintStreamInfoStream_LabelInheritance(t *testing.T) {
	var buf bytes.Buffer
	is := NewPrintStreamInfoStreamWithID(&buf, 7)
	labels := pprof.Labels("lucene-thread", "merger-1-thread-3")
	var wg sync.WaitGroup
	wg.Add(1)
	go pprof.Do(context.Background(), labels, func(ctx context.Context) {
		defer wg.Done()
		is.MessageCtx(ctx, "MERGE", "starting")
	})
	wg.Wait()
	got := buf.String()
	if !strings.Contains(got, "merger-1-thread-3") {
		t.Fatalf("expected thread name in line, got %q", got)
	}
}

// TestPrintStreamInfoStream_ConcurrentMessages stresses the mutex
// serialisation and verifies that line boundaries remain intact.
func TestPrintStreamInfoStream_ConcurrentMessages(t *testing.T) {
	var buf bytes.Buffer
	is := NewPrintStreamInfoStreamWithID(&buf, 0)
	const N = 64
	var wg sync.WaitGroup
	wg.Add(N)
	for i := 0; i < N; i++ {
		go func(i int) {
			defer wg.Done()
			is.Message("C", "line")
		}(i)
	}
	wg.Wait()
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != N {
		t.Fatalf("got %d lines, want %d", len(lines), N)
	}
}

// TestPrintStreamInfoStream_ImplementsInterface verifies the type
// satisfies the InfoStream contract at compile time.
func TestPrintStreamInfoStream_ImplementsInterface(t *testing.T) {
	var _ InfoStream = (*PrintStreamInfoStream)(nil)
}
