// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util/automaton"
)

// TestAutomatonToTokenStreamConvert_LinearString verifies that a simple
// linear automaton (a single accepted string) produces one token per
// transition, with the expected term values.
func TestAutomatonToTokenStreamConvert_LinearString(t *testing.T) {
	// Build an automaton matching the single string "abc".
	a := automaton.NewAutomaton()
	s0 := a.CreateState()
	s1 := a.CreateState()
	s2 := a.CreateState()
	s3 := a.CreateState()
	a.AddTransitionSingle(s0, s1, 'a')
	a.AddTransitionSingle(s1, s2, 'b')
	a.AddTransitionSingle(s2, s3, 'c')
	a.SetAccept(s3, true)
	a.FinishState()

	ts, err := AutomatonToTokenStreamConvert(a)
	if err != nil {
		t.Fatalf("AutomatonToTokenStreamConvert: %v", err)
	}
	if r, ok := ts.(interface{ Reset() error }); ok {
		if err := r.Reset(); err != nil {
			t.Fatalf("Reset: %v", err)
		}
	}

	got := collectTerms(t, ts)
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("expected %d tokens, got %d (%v)", len(want), len(got), got)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("token %d: expected %q, got %q", i, w, got[i])
		}
	}
}

// TestAutomatonToTokenStreamConvert_CycleRejected verifies that an
// automaton with a cycle is rejected with an error rather than producing a
// stream that would loop forever.
func TestAutomatonToTokenStreamConvert_CycleRejected(t *testing.T) {
	a := automaton.NewAutomaton()
	s0 := a.CreateState()
	s1 := a.CreateState()
	a.AddTransitionSingle(s0, s1, 'a')
	a.AddTransitionSingle(s1, s0, 'b') // creates a cycle
	a.SetAccept(s1, true)
	a.FinishState()

	if _, err := AutomatonToTokenStreamConvert(a); err == nil {
		t.Error("expected error for cyclic automaton, got nil")
	}
}
