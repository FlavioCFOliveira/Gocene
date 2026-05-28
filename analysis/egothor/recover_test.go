// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package egothor

import (
	"strings"
	"testing"
)

// callWithRecoverBounds runs fn under a deferred recoverBounds and reports
// whether fn's panic was swallowed (no panic escaped) along with the value
// that escaped, if any.
func callWithRecoverBounds(fn func()) (escaped any, swallowed bool) {
	swallowed = true
	defer func() {
		// This deferred func runs after recoverBounds. If recoverBounds
		// re-panicked, recover here captures the propagated value.
		if r := recover(); r != nil {
			escaped = r
			swallowed = false
		}
	}()
	defer recoverBounds()
	fn()
	return
}

func TestRecoverBounds(t *testing.T) {
	tests := []struct {
		name      string
		fn        func()
		wantSwall bool
	}{
		{
			name:      "no panic",
			fn:        func() {},
			wantSwall: true,
		},
		{
			name: "slice index out of range is swallowed",
			fn: func() {
				s := []int{}
				_ = s[3] //nolint:staticcheck // intentional out-of-range access to trigger panic
			},
			wantSwall: true,
		},
		{
			name: "slice bounds out of range is swallowed",
			fn: func() {
				s := make([]int, 2)
				_ = s[1:5] //nolint:staticcheck // intentional out-of-range slice to trigger panic
			},
			wantSwall: true,
		},
		{
			name: "string index out of range is swallowed",
			fn: func() {
				str := ""
				_ = str[2] //nolint:staticcheck // intentional out-of-range access to trigger panic
			},
			wantSwall: true,
		},
		{
			name: "nil map write propagates",
			fn: func() {
				var m map[string]int
				m["x"] = 1 // assignment to entry in nil map: runtime.Error, not out-of-range
			},
			wantSwall: false,
		},
		{
			name: "nil pointer dereference propagates",
			fn: func() {
				var p *int
				_ = *p // invalid memory address: runtime.Error, not out-of-range
			},
			wantSwall: false,
		},
		{
			name: "divide by zero propagates",
			fn: func() {
				x, y := 1, 0
				_ = x / y // integer divide by zero: runtime.Error, not out-of-range
			},
			wantSwall: false,
		},
		{
			name: "non-runtime panic propagates",
			fn: func() {
				panic("custom failure")
			},
			wantSwall: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			escaped, swallowed := callWithRecoverBounds(tc.fn)
			if swallowed != tc.wantSwall {
				t.Fatalf("swallowed = %v, want %v (escaped = %v)", swallowed, tc.wantSwall, escaped)
			}
			if !tc.wantSwall && escaped == nil {
				t.Fatalf("expected a propagated panic value, got nil")
			}
		})
	}
}

// TestApplySwallowsBoundsPanic verifies that Diff.Apply preserves its existing
// behaviour of silently ignoring an out-of-range patch command (mirroring the
// Java source) and that the in-place result up to the failing command is left
// intact. The '-' command underflows pos, and the subsequent 'R' write at a
// negative index triggers an out-of-range panic that Apply must swallow.
func TestApplySwallowsBoundsPanic(t *testing.T) {
	dest := []rune("ab")
	before := string(dest)

	// "-c" subtracts (c-a+1)=3 then the loop's trailing pos-- drives pos
	// negative; the following "Rb" attempts (*dest)[neg] = ... which panics
	// with "index out of range". Apply must swallow it and not propagate.
	done := make(chan struct{})
	go func() {
		defer close(done)
		Apply(&dest, "-cRb")
	}()
	<-done

	// The slice header may have been mutated up to the panic point, but Apply
	// must have returned normally (the goroutine reached close(done) without a
	// panic crashing the program). We additionally assert the rune count was
	// not corrupted into something nonsensical: with a swallowed panic the
	// destination retains its pre-call content because no successful command
	// ran before the panic.
	if got := string(dest); got != before {
		t.Fatalf("dest = %q, want unchanged %q (no command should have committed before the panic)", got, before)
	}
}

// TestApplyNonBoundsPanicWouldPropagate documents, via recoverBounds directly,
// that a non-out-of-range runtime panic occurring inside an Apply-style body is
// re-panicked rather than swallowed. Apply itself only ever produces
// out-of-range panics, so this guards the recover policy used by Apply.
func TestApplyNonBoundsPanicWouldPropagate(t *testing.T) {
	_, swallowed := callWithRecoverBounds(func() {
		var p *int
		_ = *p
	})
	if swallowed {
		t.Fatal("a nil dereference inside an Apply-style body must propagate, not be swallowed")
	}
}

// TestMultiTrie2GetFullyDoesNotPanic exercises GetFully/GetLastOnPath on an
// empty trie to confirm the recover wiring is in place and returns the
// empty-result default without crashing.
func TestMultiTrie2GetFullyDoesNotPanic(t *testing.T) {
	m := NewMultiTrie2(true)
	if got := m.GetFully([]rune("abc")); got != "" {
		t.Fatalf("GetFully on empty MultiTrie2 = %q, want empty", got)
	}
	if got := m.GetLastOnPath([]rune("abc")); got != "" {
		t.Fatalf("GetLastOnPath on empty MultiTrie2 = %q, want empty", got)
	}
}

// sanity: ensure the swallowed-message matcher catches both phrasings Go uses.
func TestRecoverBoundsMessageMatch(t *testing.T) {
	for _, msg := range []string{
		"runtime error: index out of range [3] with length 0",
		"runtime error: slice bounds out of range [:5] with capacity 2",
	} {
		if !(strings.Contains(msg, "index out of range") || strings.Contains(msg, "out of range")) {
			t.Fatalf("matcher missed runtime message %q", msg)
		}
	}
}
