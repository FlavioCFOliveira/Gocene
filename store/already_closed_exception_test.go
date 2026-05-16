// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
	"errors"
	"testing"
)

func TestAlreadyClosedException_Message(t *testing.T) {
	err := NewAlreadyClosedException("reader is closed", nil)
	if got, want := err.Error(), "reader is closed"; got != want {
		t.Fatalf("Error() = %q, want %q", got, want)
	}
	if err.Unwrap() != nil {
		t.Fatalf("Unwrap() = %v, want nil", err.Unwrap())
	}
}

func TestAlreadyClosedException_WithCause(t *testing.T) {
	cause := errors.New("file handle invalid")
	err := NewAlreadyClosedException("reader is closed", cause)
	if got, want := err.Error(), "reader is closed: file handle invalid"; got != want {
		t.Fatalf("Error() = %q, want %q", got, want)
	}
	if !errors.Is(err, cause) {
		t.Fatalf("errors.Is(err, cause) = false, want true")
	}
}

func TestAlreadyClosedException_NilSafety(t *testing.T) {
	var err *AlreadyClosedException
	if got, want := err.Error(), "<nil>"; got != want {
		t.Fatalf("nil Error() = %q, want %q", got, want)
	}
	if err.Unwrap() != nil {
		t.Fatalf("nil Unwrap() = %v, want nil", err.Unwrap())
	}
}

func TestAlreadyClosedException_AsTarget(t *testing.T) {
	original := NewAlreadyClosedException("closed", nil)
	var wrapped error = original
	var target *AlreadyClosedException
	if !errors.As(wrapped, &target) {
		t.Fatalf("errors.As did not match")
	}
	if target != original {
		t.Fatalf("As target = %v, want %v", target, original)
	}
}
