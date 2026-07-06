// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
//go:build gocene_monsters

package automaton

import "testing"

// TestMinimize_Huge mirrors Lucene's TestMinimize#testMinimizeHuge, marked
// @Nightly upstream. It guards against quadratic-space regressions in Hopcroft
// by building a non-trivial regexp automaton and asserting the minimized
// output is deterministic.
func TestMinimize_Huge(t *testing.T) {
	// Lucene's exact regexp from testMinimizeHuge
	a, err := func() (*Automaton, error) {
		r, err := NewRegExp("+-*(A|.....|BC)*]")
		if err != nil {
			return nil, err
		}
		return r.ToAutomaton()
	}()
	if err != nil {
		t.Fatalf("Lucene @Nightly regexp failed: %v", err)
	}

	b, err := minimizeForTest(a, 1000000)
	if err != nil {
		t.Fatalf("minimizeForTest: %v", err)
	}
	if !b.IsDeterministic() {
		t.Error("minimizeForTest result is not deterministic")
	}
}
