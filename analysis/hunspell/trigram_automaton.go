// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package hunspell

// TrigramAutomaton computes the same scores as non-weighted trigram ngram
// scoring but in O(s2.length) time, using a precomputed deterministic automaton
// over the character 3-grams of s1.
//
// This is the Go port of
// org.apache.lucene.analysis.hunspell.TrigramAutomaton from Apache Lucene 10.4.0.
//
// Deviation: Lucene uses CharacterRunAutomaton from the util.automaton package
// which is not yet ported.  This implementation uses a plain map-based DFA that
// is semantically equivalent for the limited character set of a single word.
type TrigramAutomaton struct {
	// state → map[rune-offset]nextState
	transitions []map[int]int
	state2Score []int
	minChar     rune
	maxChar     rune
	n           int
}

const trigramN = 3

// NewTrigramAutomaton builds a TrigramAutomaton from s1.
func NewTrigramAutomaton(s1 string) *TrigramAutomaton {
	runes := []rune(s1)
	if len(runes) == 0 {
		return &TrigramAutomaton{}
	}

	// Find min/max char.
	minC, maxC := runes[0], runes[0]
	for _, r := range runes[1:] {
		if r < minC {
			minC = r
		}
		if r > maxC {
			maxC = r
		}
	}

	// Collect substring counts.
	substringCounts := make(map[string]int)
	for start := 0; start < len(runes); start++ {
		limit := start + trigramN
		if limit > len(runes) {
			limit = len(runes)
		}
		for end := start + 1; end <= limit; end++ {
			key := string(runes[start:end])
			substringCounts[key]++
		}
	}

	// Build a plain trie-DFA over the substrings (state = node in the trie).
	// State 0 = initial state.
	transitions := []map[int]int{make(map[int]int)}
	state2Score := []int{0}
	stateCount := 1

	newState := func() int {
		transitions = append(transitions, make(map[int]int))
		state2Score = append(state2Score, 0)
		stateCount++
		return stateCount - 1
	}

	// Insert each substring into the trie.
	for substr, count := range substringCounts {
		state := 0
		for _, c := range []rune(substr) {
			label := int(c - minC)
			next, ok := transitions[state][label]
			if !ok {
				next = newState()
				transitions[state][label] = next
			}
			state = next
		}
		state2Score[state] = count
	}

	return &TrigramAutomaton{
		transitions: transitions,
		state2Score: state2Score,
		minChar:     minC,
		maxChar:     maxC,
		n:           trigramN,
	}
}

// NgramScore computes the trigram score for s2 against this automaton.
func (ta *TrigramAutomaton) NgramScore(s2 string) int {
	if len(ta.transitions) == 0 {
		return 0
	}
	runes := []rune(s2)
	counted := make(map[int]struct{})
	score := 0
	state1, state2Var := -1, -1

	for _, c := range runes {
		if c < ta.minChar || c > ta.maxChar {
			state1, state2Var = -1, -1
			continue
		}
		label := int(c - ta.minChar)

		// 3-gram ending at current position.
		state3 := -1
		if state2Var > 0 {
			if next, ok := ta.transitions[state2Var][label]; ok {
				state3 = next
			}
		} else if state2Var == 0 {
			if next, ok := ta.transitions[0][label]; ok {
				state3 = next
			}
		}
		if state3 > 0 {
			score += ta.substringScore(state3, counted)
		}

		// 2-gram ending at current position.
		s2next := -1
		if state1 > 0 {
			if next, ok := ta.transitions[state1][label]; ok {
				s2next = next
			}
		} else if state1 == 0 {
			if next, ok := ta.transitions[0][label]; ok {
				s2next = next
			}
		}
		if s2next > 0 {
			score += ta.substringScore(s2next, counted)
		}
		state2Var = s2next

		// 1-gram ending at current position.
		s1next := -1
		if next, ok := ta.transitions[0][label]; ok {
			s1next = next
		}
		if s1next > 0 {
			score += ta.substringScore(s1next, counted)
		}
		state1 = s1next
	}
	return score
}

func (ta *TrigramAutomaton) substringScore(state int, counted map[int]struct{}) int {
	if _, ok := counted[state]; ok {
		return 0
	}
	counted[state] = struct{}{}
	return ta.state2Score[state]
}
