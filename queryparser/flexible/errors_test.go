// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package flexible

import (
	"errors"
	"testing"
)

func TestMessageImpl(t *testing.T) {
	m := NewMessageImpl("hello %s", "world")
	if m.GetKey() != "hello %s" {
		t.Errorf("GetKey() = %q", m.GetKey())
	}
	if m.GetLocalizedMessage() != "hello world" {
		t.Errorf("GetLocalizedMessage() = %q, want hello world", m.GetLocalizedMessage())
	}

	m2 := NewMessageImpl("no inserts")
	if m2.GetLocalizedMessage() != "no inserts" {
		t.Errorf("GetLocalizedMessage() = %q, want no inserts", m2.GetLocalizedMessage())
	}
}

func TestNLSException(t *testing.T) {
	cause := errors.New("root cause")
	msg := NewMessageImpl("parse failed: %s", "bad token")
	e := NewNLSException(msg, cause)

	if e.Error() != "parse failed: bad token" {
		t.Errorf("Error() = %q", e.Error())
	}
	if !errors.Is(e, cause) {
		t.Error("Unwrap should expose cause")
	}
}

func TestQueryNodeError(t *testing.T) {
	msg := NewMessageImpl(MsgQueryNodeError)
	e := NewQueryNodeError(msg)
	if e.Error() != MsgQueryNodeError {
		t.Errorf("Error() = %q, want %q", e.Error(), MsgQueryNodeError)
	}
	if e.GetMessage() != msg {
		t.Error("GetMessage() returned different instance")
	}

	cause := errors.New("inner")
	e2 := NewQueryNodeErrorWithCause(msg, cause)
	if !errors.Is(e2, cause) {
		t.Error("Unwrap should expose cause")
	}
}

func TestQueryNodeException(t *testing.T) {
	msg := NewMessageImpl("invalid query")
	e := NewQueryNodeException(msg)
	if e.Error() != "invalid query" {
		t.Errorf("Error() = %q", e.Error())
	}
}

func TestQueryNodeParseException(t *testing.T) {
	msg := NewMessageImpl("parse error")
	e := NewQueryNodeParseExceptionWithLocation(msg, "bad query", 1, 5)
	if e.GetQuery() != "bad query" {
		t.Errorf("GetQuery() = %q", e.GetQuery())
	}
	if e.GetBeginLine() != 1 {
		t.Errorf("GetBeginLine() = %d, want 1", e.GetBeginLine())
	}
	if e.GetBeginColumn() != 5 {
		t.Errorf("GetBeginColumn() = %d, want 5", e.GetBeginColumn())
	}
}

func TestNLS(t *testing.T) {
	var nls NLS
	msg := NewMessageImpl("key")
	if nls.GetLocalizedMessage(msg) != "key" {
		t.Errorf("NLS.GetLocalizedMessage() = %q, want key", nls.GetLocalizedMessage(msg))
	}
	if nls.GetLocalizedMessage(nil) != "" {
		t.Error("NLS.GetLocalizedMessage(nil) should return empty string")
	}
}
