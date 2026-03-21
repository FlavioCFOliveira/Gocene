// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package automaton

// RunAutomaton is a finite-state automaton with fast run operation.
// It provides efficient matching of input against the automaton.
type RunAutomaton struct {
	compiled *CompiledAutomaton
}

// NewRunAutomaton creates a new RunAutomaton from a compiled automaton.
func NewRunAutomaton(compiled *CompiledAutomaton) *RunAutomaton {
	return &RunAutomaton{compiled: compiled}
}

// Run runs the automaton on input bytes starting from the initial state.
// Returns true if the automaton accepts the input.
func (r *RunAutomaton) Run(input []byte) bool {
	return r.compiled.Run(input)
}

// RunString runs the automaton on a string input.
func (r *RunAutomaton) RunString(input string) bool {
	return r.compiled.RunString(input)
}

// RunFromState runs the automaton from a specific state.
func (r *RunAutomaton) RunFromState(input []byte, state int) bool {
	if state == -1 {
		return false
	}

	for _, b := range input {
		if state == -1 {
			return false
		}
		state = r.compiled.GetNextState(state, int(b))
	}

	return r.compiled.IsAccept(state)
}

// Step performs a single step in the automaton.
// Returns the new state or -1 if no transition.
func (r *RunAutomaton) Step(state int, input int) int {
	return r.compiled.GetNextState(state, input)
}

// IsAccept returns true if the state is accepting.
func (r *RunAutomaton) IsAccept(state int) bool {
	return r.compiled.IsAccept(state)
}

// GetInitialState returns the initial state.
func (r *RunAutomaton) GetInitialState() int {
	return r.compiled.GetStartState()
}

// CharacterRunAutomaton is a RunAutomaton for character arrays.
type CharacterRunAutomaton struct {
	*RunAutomaton
}

// NewCharacterRunAutomaton creates a new CharacterRunAutomaton.
func NewCharacterRunAutomaton(compiled *CompiledAutomaton) *CharacterRunAutomaton {
	return &CharacterRunAutomaton{NewRunAutomaton(compiled)}
}

// RunChars runs the automaton on a rune slice.
func (c *CharacterRunAutomaton) RunChars(input []rune) bool {
	state := c.GetInitialState()
	for _, r := range input {
		if state == -1 {
			return false
		}
		state = c.Step(state, int(r))
	}
	return c.IsAccept(state)
}

// RunString runs the automaton on a string (as runes).
func (c *CharacterRunAutomaton) RunString(input string) bool {
	return c.RunChars([]rune(input))
}

// MatchResult represents the result of a match operation.
type MatchResult struct {
	Matched     bool
	Start       int
	End         int
	MatchLength int
}

// Find finds the first match in the input.
func (r *RunAutomaton) Find(input []byte) *MatchResult {
	for start := 0; start < len(input); start++ {
		state := r.GetInitialState()
		for end := start; end < len(input); end++ {
			if state == -1 {
				break
			}
			if r.IsAccept(state) {
				return &MatchResult{
					Matched:     true,
					Start:       start,
					End:         end,
					MatchLength: end - start,
				}
			}
			state = r.Step(state, int(input[end]))
		}
		// Check final state
		if state != -1 && r.IsAccept(state) {
			return &MatchResult{
				Matched:     true,
				Start:       start,
				End:         len(input),
				MatchLength: len(input) - start,
			}
		}
	}
	return &MatchResult{Matched: false}
}

// FindString finds the first match in a string.
func (r *RunAutomaton) FindString(input string) *MatchResult {
	return r.Find([]byte(input))
}

// FindAll finds all non-overlapping matches in the input.
func (r *RunAutomaton) FindAll(input []byte) []*MatchResult {
	results := make([]*MatchResult, 0)
	pos := 0

	for pos < len(input) {
		result := r.Find(input[pos:])
		if !result.Matched {
			break
		}
		result.Start += pos
		result.End += pos
		results = append(results, result)
		pos += result.End
	}

	return results
}

// FindAllString finds all matches in a string.
func (r *RunAutomaton) FindAllString(input string) []*MatchResult {
	return r.FindAll([]byte(input))
}

// LongestMatch finds the longest match starting at a position.
func (r *RunAutomaton) LongestMatch(input []byte, start int) *MatchResult {
	if start >= len(input) {
		return &MatchResult{Matched: false}
	}

	state := r.GetInitialState()
	lastAccept := -1

	for i := start; i < len(input) && state != -1; i++ {
		if r.IsAccept(state) {
			lastAccept = i
		}
		state = r.Step(state, int(input[i]))
	}

	// Check final state
	if state != -1 && r.IsAccept(state) {
		lastAccept = len(input)
	}

	if lastAccept == -1 {
		return &MatchResult{Matched: false}
	}

	return &MatchResult{
		Matched:     true,
		Start:       start,
		End:         lastAccept,
		MatchLength: lastAccept - start,
	}
}

// LongestMatchString finds the longest match in a string.
func (r *RunAutomaton) LongestMatchString(input string, start int) *MatchResult {
	return r.LongestMatch([]byte(input), start)
}
