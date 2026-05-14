// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"context"
	"runtime/pprof"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// TestNamedThreadFactory_PrefixDefault verifies the Java
// checkPrefix behaviour: blank → "Lucene".
func TestNamedThreadFactory_PrefixDefault(t *testing.T) {
	f := NewNamedThreadFactory("")
	if !strings.HasPrefix(f.Prefix(), "Lucene-") {
		t.Fatalf("Prefix=%q, want Lucene-...", f.Prefix())
	}
	if !strings.HasSuffix(f.Prefix(), "-thread") {
		t.Fatalf("Prefix=%q, want suffix -thread", f.Prefix())
	}
}

// TestNamedThreadFactory_PrefixCustom verifies that the supplied
// prefix flows into the formatted thread name.
func TestNamedThreadFactory_PrefixCustom(t *testing.T) {
	f := NewNamedThreadFactory("merger")
	if !strings.HasPrefix(f.Prefix(), "merger-") {
		t.Fatalf("Prefix=%q, want merger-...", f.Prefix())
	}
}

// TestNamedThreadFactory_DistinctPoolIDs covers the Java contract
// that successive factories receive monotonically increasing pool
// identifiers via the global threadPoolNumber.
func TestNamedThreadFactory_DistinctPoolIDs(t *testing.T) {
	a := NewNamedThreadFactory("p")
	b := NewNamedThreadFactory("p")
	if a.Prefix() == b.Prefix() {
		t.Fatalf("expected distinct pool ids, both got %q", a.Prefix())
	}
}

// TestNamedThreadFactory_NextIncrements covers per-factory thread
// counter advancement, mirroring Java's threadNumber AtomicInteger.
func TestNamedThreadFactory_NextIncrements(t *testing.T) {
	f := NewNamedThreadFactory("t")
	n1 := f.Next()
	n2 := f.Next()
	if n1 == n2 {
		t.Fatalf("Next produced same name twice: %q", n1)
	}
	if !strings.HasSuffix(n1, "-1") {
		t.Fatalf("first Next=%q, want suffix -1", n1)
	}
	if !strings.HasSuffix(n2, "-2") {
		t.Fatalf("second Next=%q, want suffix -2", n2)
	}
}

// TestNamedThreadFactory_PeekDoesNotIncrement asserts Peek is
// side-effect-free.
func TestNamedThreadFactory_PeekDoesNotIncrement(t *testing.T) {
	f := NewNamedThreadFactory("t")
	if f.Peek() != f.Peek() {
		t.Fatalf("Peek must be idempotent")
	}
}

// TestNamedThreadFactory_RunAppliesLabel verifies the pprof label is
// applied for the duration of the goroutine. We retrieve the label
// inside fn via pprof.Label and assert it matches.
func TestNamedThreadFactory_RunAppliesLabel(t *testing.T) {
	f := NewNamedThreadFactory("test")
	var got atomic.Value
	var wg sync.WaitGroup
	wg.Add(1)
	f.Run(context.Background(), func(ctx context.Context) {
		defer wg.Done()
		if v, ok := pprof.Label(ctx, "lucene-thread"); ok {
			got.Store(v)
		}
	})
	wg.Wait()
	v, _ := got.Load().(string)
	if !strings.HasPrefix(v, "test-") {
		t.Fatalf("lucene-thread label=%q, want test-...", v)
	}
}

// TestNamedThreadFactory_RunNilContext checks the nil-context
// convenience that mirrors Java's lack of explicit context.
func TestNamedThreadFactory_RunNilContext(t *testing.T) {
	f := NewNamedThreadFactory("nil")
	var ran atomic.Bool
	var wg sync.WaitGroup
	wg.Add(1)
	f.Run(nil, func(ctx context.Context) {
		defer wg.Done()
		if ctx == nil {
			t.Errorf("expected non-nil context inside fn")
		}
		ran.Store(true)
	})
	wg.Wait()
	if !ran.Load() {
		t.Fatalf("fn did not run")
	}
}
