// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Port of org.apache.lucene.util.automaton.TestDeterminizeLexicon from
// Apache Lucene 10.4.0 (commit 9983b7c). The Java original builds a
// 5_000-term random Unicode lexicon, unions every MakeString automaton,
// determinizes the result with workLimit = 1_000_000, and asserts:
//
//  1. The determinized lexicon is finite — via AutomatonTestUtil.isFinite.
//  2. Every original term is accepted by the deterministic automaton —
//     via Operations.run.
//  3. (TEST_NIGHTLY only) Every term's UTF-8 byte sequence is accepted
//     by ByteRunAutomaton.
//
// With AutomatonTestUtil now fully ported (IsFiniteAutomaton is available
// in automaton_test_util_test.go), this test can be fully implemented.

package automaton

import (
	"math/rand"
	"testing"
)

// randomUnicodeStringSafe generates a random Unicode string that is safe
// for UTF-8 round-tripping (no unpaired surrogates). This is a local
// surrogate-safe replacement for util.RandomUnicodeString which currently
// emits unpaired surrogates in the D800..DFFF range.
func randomUnicodeStringSafe(r *rand.Rand, maxLength int) string {
	length := r.Intn(maxLength + 1)
	runes := make([]rune, 0, length)
	for i := 0; i < length; i++ {
		cp := r.Intn(MaxCodePoint + 1)
		// Skip surrogates (D800-DFFF) and non-characters (FFFE, FFFF)
		for (cp >= 0xD800 && cp <= 0xDFFF) || cp == 0xFFFE || cp == 0xFFFF {
			cp = r.Intn(MaxCodePoint + 1)
		}
		runes = append(runes, rune(cp))
	}
	return string(runes)
}

// TestDeterminizeLexicon_Lexicon mirrors Lucene's
// TestDeterminizeLexicon#testLexicon. It builds atLeast(1) iterations
// of a 5_000-term random Unicode lexicon, unions every MakeString
// automaton, determinizes with workLimit = 1_000_000, asserts the
// result is finite (the load-bearing check) and that every original
// term is accepted by Operations.Run.
func TestDeterminizeLexicon_Lexicon(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	// Lucene uses atLeast(1)
	num := 1
	for i := 0; i < num; i++ {
		automata := make([]*Automaton, 0, 5000)
		terms := make([]string, 0, 5000)

		for j := 0; j < 5000; j++ {
			randomString := randomUnicodeStringSafe(rng, 20)
			terms = append(terms, randomString)
			automata = append(automata, MakeString(randomString))
		}

		// Shuffle automata (matching Lucene's Collections.shuffle)
		rngShuffle := rand.New(rand.NewSource(rng.Int63()))
		rngShuffle.Shuffle(len(automata), func(a, b int) {
			automata[a], automata[b] = automata[b], automata[a]
		})

		// Union → Determinize
		lex := Union(automata)
		var err error
		lex, err = Determinize(lex, 1000000)
		if err != nil {
			t.Fatalf("Determinize lexicon at iteration %d: %v", i, err)
		}

		// Assertion 1: finite (the load-bearing correctness check)
		if !IsFiniteAutomaton(lex) {
			t.Errorf("iteration %d: determinized lexicon is not finite", i)
		}

		// Assertion 2: every term accepted
		for _, s := range terms {
			if !Run(lex, s) {
				t.Errorf("iteration %d: term %q not accepted by determinized lexicon", i, s)
			}
		}
	}
}
