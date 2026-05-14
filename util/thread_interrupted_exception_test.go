// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"context"
	"errors"
	"testing"
)

// TestThreadInterruptedError_Sentinel verifies that the bare sentinel works
// with errors.Is, which is the primary way Go callers detect a
// thread-interrupted state (replacing Java's instanceof check).
func TestThreadInterruptedError_Sentinel(t *testing.T) {
	if !errors.Is(ErrThreadInterrupted, ErrThreadInterrupted) {
		t.Fatalf("sentinel must match itself via errors.Is")
	}
}

// TestThreadInterruptedError_NilCause confirms the no-cause path reuses the
// sentinel's message verbatim.
func TestThreadInterruptedError_NilCause(t *testing.T) {
	err := NewThreadInterruptedError(nil)
	if got, want := err.Error(), "thread interrupted"; got != want {
		t.Fatalf("Error()=%q, want %q", got, want)
	}
	if !errors.Is(err, ErrThreadInterrupted) {
		t.Fatalf("errors.Is(err, ErrThreadInterrupted) should hold")
	}
}

// TestThreadInterruptedError_WithCause verifies that a wrapped cause is
// reachable via errors.Is and the message follows the Java RuntimeException
// "message: cause" pattern.
func TestThreadInterruptedError_WithCause(t *testing.T) {
	cause := context.Canceled
	err := NewThreadInterruptedError(cause)

	if got, want := err.Error(), "thread interrupted: context canceled"; got != want {
		t.Fatalf("Error()=%q, want %q", got, want)
	}
	if !errors.Is(err, ErrThreadInterrupted) {
		t.Fatalf("errors.Is sentinel should hold even with a cause")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("errors.Is cause should hold")
	}
}

// TestThreadInterruptedError_AsTarget shows that errors.As recovers the
// concrete type for inspection (Lucene callers occasionally introspect the
// cause via getCause()).
func TestThreadInterruptedError_AsTarget(t *testing.T) {
	wrapped := NewThreadInterruptedError(context.DeadlineExceeded)
	var target *ThreadInterruptedError
	if !errors.As(wrapped, &target) {
		t.Fatalf("errors.As should recover *ThreadInterruptedError")
	}
	if target.Cause != context.DeadlineExceeded {
		t.Fatalf("recovered cause = %v, want context.DeadlineExceeded", target.Cause)
	}
}

// TestThreadInterruptedError_NilReceiver guards against a nil pointer not
// blowing up when Error/Unwrap are called.
func TestThreadInterruptedError_NilReceiver(t *testing.T) {
	var e *ThreadInterruptedError
	if got, want := e.Error(), "thread interrupted"; got != want {
		t.Fatalf("nil.Error()=%q, want %q", got, want)
	}
	if e.Unwrap() != nil {
		t.Fatalf("nil.Unwrap() must return nil")
	}
}
