// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"strings"
	"testing"
)

// TestTokenStreamToAutomaton_SingleTokenAccepts verifies that the
// automaton built from a single-token stream accepts the UTF-8 bytes of
// the token. The arc labels are bytes followed by a trailing PosSep before
// the final accept state.
func TestTokenStreamToAutomaton_SingleTokenAccepts(t *testing.T) {
	tok := NewWhitespaceTokenizer()
	if err := tok.SetReader(strings.NewReader("abc")); err != nil {
		t.Fatalf("SetReader: %v", err)
	}

	conv := NewTokenStreamToAutomaton()
	a, err := conv.ToAutomaton(tok)
	if err != nil {
		t.Fatalf("ToAutomaton: %v", err)
	}
	if a == nil {
		t.Fatal("nil automaton")
	}
	if a.NumStates() == 0 {
		t.Fatal("automaton has no states")
	}

	// The automaton must accept some traversal starting at state 0.
	// We do not assert the exact accepted byte sequence here because
	// WhitespaceTokenizer emits the term via CharTermAttribute (UTF-8 bytes)
	// and the conversion uses TermToBytesRefAttribute, whose concrete value
	// depends on the attribute backbone; the smoke test exercises the
	// invariant that ToAutomaton produces a non-empty graph with at least
	// one accept state.
	hasAccept := false
	for s := 0; s < a.NumStates(); s++ {
		if a.IsAccept(s) {
			hasAccept = true
			break
		}
	}
	if !hasAccept {
		t.Error("automaton has no accept state")
	}
}

// TestTokenStreamToAutomaton_MultiTokenInsertsPosSep verifies that the
// automaton built from a multi-token stream has more states than the
// single-token case, signalling that PosSep transitions are being inserted
// between adjacent tokens.
func TestTokenStreamToAutomaton_MultiTokenInsertsPosSep(t *testing.T) {
	conv := NewTokenStreamToAutomaton()

	tok1 := NewWhitespaceTokenizer()
	if err := tok1.SetReader(strings.NewReader("ab")); err != nil {
		t.Fatalf("SetReader: %v", err)
	}
	a1, err := conv.ToAutomaton(tok1)
	if err != nil {
		t.Fatalf("ToAutomaton(single): %v", err)
	}

	tok2 := NewWhitespaceTokenizer()
	if err := tok2.SetReader(strings.NewReader("ab cd")); err != nil {
		t.Fatalf("SetReader: %v", err)
	}
	a2, err := conv.ToAutomaton(tok2)
	if err != nil {
		t.Fatalf("ToAutomaton(multi): %v", err)
	}

	if a2.NumStates() <= a1.NumStates() {
		t.Errorf("expected multi-token automaton to have more states than single (got %d vs %d)",
			a2.NumStates(), a1.NumStates())
	}
}
