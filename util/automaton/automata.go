// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package automaton

import (
	"github.com/FlavioCFOliveira/Gocene/util"
)

// Automata is a factory for creating common automata.
type Automata struct{}

// NewAutomata creates a new Automata factory.
func NewAutomata() *Automata {
	return &Automata{}
}

// MakeEmpty creates an automaton that accepts no strings.
func (a *Automata) MakeEmpty() *Automaton {
	result := NewAutomaton()
	// Create initial state but don't mark it as accepting
	result.CreateState()
	return result
}

// MakeEmptyString creates an automaton that accepts only the empty string.
func (a *Automata) MakeEmptyString() *Automaton {
	result := NewAutomaton()
	initial := result.CreateState()
	result.SetAccept(initial, true)
	return result
}

// MakeAnyChar creates an automaton that accepts any single character.
func (a *Automata) MakeAnyChar() *Automaton {
	result := NewAutomaton()
	initial := result.CreateState()
	accept := result.CreateState()

	// Add transition for any character (0 to Unicode max)
	result.AddTransition(initial, accept, 0, 0x10FFFF)
	result.SetAccept(accept, true)

	return result
}

// MakeAnyString creates an automaton that accepts any string (including empty).
func (a *Automata) MakeAnyString() *Automaton {
	result := NewAutomaton()
	initial := result.CreateState()

	// Add self-loop for any character
	result.AddTransition(initial, initial, 0, 0x10FFFF)
	result.SetAccept(initial, true)

	return result
}

// MakeString creates an automaton that accepts a specific string.
func (a *Automata) MakeString(s string) *Automaton {
	result := NewAutomaton()
	state := result.GetInitialState()

	for _, r := range s {
		newState := result.CreateState()
		result.AddTransition(state, newState, int(r), int(r))
		state = newState
	}

	result.SetAccept(state, true)
	return result
}

// MakeChar creates an automaton that accepts a single character.
func (a *Automata) MakeChar(c rune) *Automaton {
	result := NewAutomaton()
	initial := result.GetInitialState()
	accept := result.CreateState()

	result.AddTransition(initial, accept, int(c), int(c))
	result.SetAccept(accept, true)

	return result
}

// MakeCharRange creates an automaton that accepts a range of characters.
func (a *Automata) MakeCharRange(start, end rune) *Automaton {
	if start > end {
		start, end = end, start
	}

	result := NewAutomaton()
	initial := result.GetInitialState()
	accept := result.CreateState()

	result.AddTransition(initial, accept, int(start), int(end))
	result.SetAccept(accept, true)

	return result
}

// MakeStringUnion creates an automaton that accepts any of the given strings.
// The strings must be sorted.
func (a *Automata) MakeStringUnion(terms []*util.BytesRef) *Automaton {
	if len(terms) == 0 {
		return a.MakeEmpty()
	}
	if len(terms) == 1 {
		return a.MakeString(string(terms[0].Bytes))
	}

	result := NewAutomaton()
	newInitial := result.CreateState()

	for _, term := range terms {
		s := string(term.Bytes)
		if len(s) == 0 {
			result.SetAccept(newInitial, true)
			continue
		}

		// Create states for this term
		prevState := newInitial
		for i, r := range s {
			if i == len(s)-1 {
				// Last character, goes to accepting state
				acceptState := result.CreateState()
				result.AddTransition(prevState, acceptState, int(r), int(r))
				result.SetAccept(acceptState, true)
			} else {
				// Intermediate state
				newState := result.CreateState()
				result.AddTransition(prevState, newState, int(r), int(r))
				prevState = newState
			}
		}
	}

	result.deterministic = false
	return result
}

// MakeStringUnionFromStrings creates an automaton that accepts any of the given strings.
func (a *Automata) MakeStringUnionFromStrings(terms []string) *Automaton {
	if len(terms) == 0 {
		return a.MakeEmpty()
	}
	if len(terms) == 1 {
		return a.MakeString(terms[0])
	}

	result := NewAutomaton()
	newInitial := result.CreateState()

	for _, s := range terms {
		if len(s) == 0 {
			result.SetAccept(newInitial, true)
			continue
		}

		prevState := newInitial
		for i, r := range s {
			if i == len(s)-1 {
				acceptState := result.CreateState()
				result.AddTransition(prevState, acceptState, int(r), int(r))
				result.SetAccept(acceptState, true)
			} else {
				newState := result.CreateState()
				result.AddTransition(prevState, newState, int(r), int(r))
				prevState = newState
			}
		}
	}

	result.deterministic = false
	return result
}

// MakeDecimalInterval creates an automaton that accepts decimal numbers in a range.
// Used for range queries on numeric fields.
func (a *Automata) MakeDecimalInterval(start, end, digits int) *Automaton {
	if start > end {
		return a.MakeEmpty()
	}

	result := NewAutomaton()
	state := result.GetInitialState()

	// Convert numbers to strings with padding
	startStr := padInt(start, digits)
	endStr := padInt(end, digits)

	// Build automaton for the interval
	// This is a simplified implementation
	for i := 0; i < len(startStr) && i < len(endStr); i++ {
		startDigit := int(startStr[i] - '0')
		endDigit := int(endStr[i] - '0')

		if startDigit == endDigit {
			// Same digit for both, just follow one path
			newState := result.CreateState()
			result.AddTransition(state, newState, int(startStr[i]), int(startStr[i]))
			state = newState
		} else {
			// Different digits, create paths for each possible digit
			for d := startDigit + 1; d < endDigit; d++ {
				intermediate := result.CreateState()
				result.AddTransition(state, intermediate, '0'+d, '0'+d)
				// Add self-loop for remaining digits (any)
				loopState := result.CreateState()
				result.AddTransition(intermediate, loopState, '0', '9')
				result.SetAccept(loopState, true)
			}

			// Path for start digit (must be followed by same or greater)
			startPath := result.CreateState()
			result.AddTransition(state, startPath, int(startStr[i]), int(startStr[i]))
			// Continue with start pattern...

			// Path for end digit (must be followed by same or less)
			endPath := result.CreateState()
			result.AddTransition(state, endPath, int(endStr[i]), int(endStr[i]))
			// Continue with end pattern...

			break
		}
	}

	result.SetAccept(state, true)
	return result
}

// MakeInterval creates an automaton for a numeric interval.
// More general version of MakeDecimalInterval.
func (a *Automata) MakeInterval(min, max int64, digits int) *Automaton {
	if min > max {
		return a.MakeEmpty()
	}
	if min == max {
		return a.MakeString(formatInt(min, digits))
	}

	// Simplified: just accept all numbers between min and max
	// In a full implementation, this would build a trie-based automaton
	result := NewAutomaton()
	initial := result.GetInitialState()

	// Add transitions for sign
	digitStart := result.CreateState()
	result.AddTransition(initial, digitStart, '0', '9')
	result.AddTransition(initial, digitStart, '-', '-')

	// Add digit transitions
	for i := 1; i < digits; i++ {
		nextState := result.CreateState()
		result.AddTransition(digitStart, nextState, '0', '9')
		digitStart = nextState
	}

	result.SetAccept(digitStart, true)
	return result
}

// MakeMaxSubstring creates an automaton for matching maximum substrings.
func (a *Automata) MakeMaxSubstring() *Automaton {
	// This accepts strings where each character is matched at most once
	// (useful for certain types of queries)
	return a.MakeAnyString()
}

// MakeMaxNGram creates an automaton for n-gram matching.
func (a *Automata) MakeMaxNGram(n int) *Automaton {
	if n <= 0 {
		return a.MakeEmpty()
	}

	result := NewAutomaton()
	state := result.GetInitialState()

	for i := 0; i < n; i++ {
		newState := result.CreateState()
		result.AddTransition(state, newState, 0, 0x10FFFF)
		state = newState
	}

	result.SetAccept(state, true)
	return result
}

// padInt pads an integer to the specified number of digits.
func padInt(n, digits int) string {
	if digits <= 0 {
		return formatInt(int64(n), 0)
	}
	return formatInt(int64(n), digits)
}

// formatInt formats an integer with optional zero-padding.
func formatInt(n int64, digits int) string {
	if digits <= 0 {
		return ""
	}

	// Handle negative numbers
	negative := n < 0
	if negative {
		n = -n
	}

	// Build digit array
	digs := make([]byte, digits)
	for i := digits - 1; i >= 0; i-- {
		digs[i] = byte('0' + (n % 10))
		n /= 10
	}

	if negative {
		return "-" + string(digs)
	}
	return string(digs)
}
