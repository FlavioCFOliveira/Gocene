// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
	"strings"
	"testing"
)

// TestNewDefaultIOContext_NoHints mirrors `new DefaultIOContext()` in Lucene
// 10.4.0: the resulting IOContext has Context.DEFAULT semantics (ContextRead
// in Gocene), no merge/flush info, and an empty hints set.
func TestNewDefaultIOContext_NoHints(t *testing.T) {
	ctx := NewDefaultIOContext()
	if ctx.Context != ContextRead {
		t.Fatalf("context = %v, want ContextRead", ctx.Context)
	}
	if ctx.MergeInfo != nil {
		t.Fatalf("MergeInfo = %v, want nil", ctx.MergeInfo)
	}
	if ctx.FlushInfo != nil {
		t.Fatalf("FlushInfo = %v, want nil", ctx.FlushInfo)
	}
	if len(ctx.Hints) != 0 {
		t.Fatalf("hints len = %d, want 0", len(ctx.Hints))
	}
}

// TestNewDefaultIOContext_WithDistinctHints replicates the Lucene READONCE
// construction: `new DefaultIOContext(DataAccessHint.SEQUENTIAL, ReadOnceHint.INSTANCE)`.
func TestNewDefaultIOContext_WithDistinctHints(t *testing.T) {
	ctx := NewDefaultIOContext(DataAccessSequential, ReadOnceInstance)
	if ctx.Context != ContextRead {
		t.Fatalf("context = %v, want ContextRead", ctx.Context)
	}
	if got := len(ctx.Hints); got != 2 {
		t.Fatalf("hints len = %d, want 2", got)
	}
	want := map[FileOpenHint]bool{
		DataAccessSequential: true,
		ReadOnceInstance:     true,
	}
	for _, h := range ctx.Hints {
		if !want[h] {
			t.Errorf("unexpected hint %#v", h)
		}
	}
}

// TestNewDefaultIOContext_DuplicateHintTypePanics asserts the canonical
// constructor invariant: "there should only be one hint of each type".
// In Lucene this is IllegalArgumentException; in Gocene this is a panic.
func TestNewDefaultIOContext_DuplicateHintTypePanics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for duplicate hint type, got nil")
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("panic value type = %T, want string", r)
		}
		if !strings.Contains(msg, "multiple hints of type") {
			t.Fatalf("panic message = %q, want substring %q", msg, "multiple hints of type")
		}
	}()
	_ = NewDefaultIOContext(DataAccessSequential, DataAccessRandom)
}

// TestNewDefaultIOContext_NilHintPanics guards against nil entries in the
// variadic slice. Lucene's Set.copyOf would NPE on a null element; the Go
// port surfaces that as an explicit panic with a clear message.
func TestNewDefaultIOContext_NilHintPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for nil hint, got nil")
		}
	}()
	_ = NewDefaultIOContext(nil)
}

// TestNewDefaultIOContext_HintsAreDefensiveCopy ensures the returned slice
// is decoupled from the caller's argument array, matching the immutability
// of Lucene's Set.copyOf.
func TestNewDefaultIOContext_HintsAreDefensiveCopy(t *testing.T) {
	src := []FileOpenHint{DataAccessSequential, ReadOnceInstance}
	ctx := NewDefaultIOContext(src...)
	src[0] = DataAccessRandom // mutate caller's slice
	if ctx.Hints[0] != DataAccessSequential {
		t.Fatalf("hints[0] = %v, want DataAccessSequential (defensive copy violated)", ctx.Hints[0])
	}
}

// TestNewDefaultIOContext_WithHintsChainsToNewInstance verifies that
// IOContext.WithHints on a default-context value returns a context with the
// merged hints, matching the Lucene contract that withHints returns a fresh
// DefaultIOContext for the DEFAULT context.
func TestNewDefaultIOContext_WithHintsChainsToNewInstance(t *testing.T) {
	base := NewDefaultIOContext()
	next := base.WithHints(ReadOnceInstance)
	if next.Context != ContextRead {
		t.Fatalf("next.Context = %v, want ContextRead", next.Context)
	}
	if len(next.Hints) != 1 || next.Hints[0] != ReadOnceInstance {
		t.Fatalf("next.Hints = %v, want [ReadOnceInstance]", next.Hints)
	}
	if len(base.Hints) != 0 {
		t.Fatalf("base.Hints mutated = %v, want []", base.Hints)
	}
}
