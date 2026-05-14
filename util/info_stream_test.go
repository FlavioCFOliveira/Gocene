// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestNoOpInfoStream(t *testing.T) {
	t.Run("messages are silently discarded", func(t *testing.T) {
		NoOpInfoStream.Message("indexer", "hello") // must not panic
	})
	t.Run("isenabled is false everywhere", func(t *testing.T) {
		if NoOpInfoStream.IsEnabled("indexer") {
			t.Fatalf("NoOpInfoStream must never be enabled")
		}
	})
	t.Run("close returns nil", func(t *testing.T) {
		if err := NoOpInfoStream.Close(); err != nil {
			t.Fatalf("Close got %v want nil", err)
		}
	})
}

// fakeInfoStream is a counting InfoStream used to verify default-stream
// dispatch and SetDefaultInfoStream behavior.
type fakeInfoStream struct {
	calls   atomic.Int64
	enabled bool
}

func (f *fakeInfoStream) Message(_, _ string) { f.calls.Add(1) }
func (f *fakeInfoStream) IsEnabled(string) bool {
	return f.enabled
}
func (f *fakeInfoStream) Close() error { return nil }

func TestDefaultInfoStream(t *testing.T) {
	t.Cleanup(func() { SetDefaultInfoStream(nil) })

	t.Run("initial default is NoOp", func(t *testing.T) {
		SetDefaultInfoStream(nil) // reset
		if !infoStreamIsNoOp(DefaultInfoStream()) {
			t.Fatalf("initial default must be NoOpInfoStream")
		}
	})

	t.Run("set custom default", func(t *testing.T) {
		fake := &fakeInfoStream{enabled: true}
		SetDefaultInfoStream(fake)
		got := DefaultInfoStream()
		if got != fake {
			t.Fatalf("default got %p want %p", got, fake)
		}
		got.Message("indexer", "hello")
		if fake.calls.Load() != 1 {
			t.Fatalf("Message calls got %d want 1", fake.calls.Load())
		}
		if !got.IsEnabled("indexer") {
			t.Fatalf("IsEnabled got false want true")
		}
	})

	t.Run("setting nil restores NoOp", func(t *testing.T) {
		SetDefaultInfoStream(&fakeInfoStream{})
		SetDefaultInfoStream(nil)
		if !infoStreamIsNoOp(DefaultInfoStream()) {
			t.Fatalf("nil reset must restore NoOpInfoStream")
		}
	})

	t.Run("concurrent reads are race-free", func(t *testing.T) {
		fake := &fakeInfoStream{enabled: true}
		SetDefaultInfoStream(fake)
		var wg sync.WaitGroup
		for i := 0; i < 32; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < 100; j++ {
					DefaultInfoStream().Message("c", "m")
				}
			}()
		}
		wg.Wait()
		if fake.calls.Load() != 32*100 {
			t.Fatalf("concurrent calls got %d want 3200", fake.calls.Load())
		}
	})
}

// infoStreamIsNoOp reports whether s is the NoOpInfoStream singleton,
// matching by interface identity.
func infoStreamIsNoOp(s InfoStream) bool {
	_, ok := s.(noOpInfoStream)
	return ok
}
