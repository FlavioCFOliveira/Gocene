// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package automaton

// CompiledAutomaton is an immutable compiled automaton for efficient matching.
// It provides fast execution by using a pre-computed transition table.
type CompiledAutomaton struct {
	automaton    *Automaton
	start        int
	accept       map[int]bool
	transitions  [][]int // state x input -> next state
	classCount   int     // number of character classes
	classMap     []int   // maps code points to class indices
	minCodePoint int
	maxCodePoint int
}

// Compile compiles an automaton for efficient execution.
func Compile(a *Automaton) *CompiledAutomaton {
	if a.IsEmpty() {
		return &CompiledAutomaton{
			automaton:    NewAutomaton(),
			start:        -1,
			accept:       make(map[int]bool),
			transitions:  make([][]int, 0),
			classCount:   0,
			minCodePoint: 0,
			maxCodePoint: 0,
		}
	}

	// Determinize and minimize first
	det := a.Determinize(DefaultDeterminizeWorkLimit)
	if det == nil {
		det = a
	}
	min := det.Minimize()

	// Compute character classes
	classMap, classCount, minCP, maxCP := computeCharClasses(min)

	// Build transition table
	numStates := min.NumStates()
	transitions := make([][]int, numStates)
	for i := 0; i < numStates; i++ {
		transitions[i] = make([]int, classCount)
		for j := 0; j < classCount; j++ {
			transitions[i][j] = -1 // Default: no transition
		}
	}

	// Fill transition table
	for i := 0; i < numStates; i++ {
		for _, trans := range min.GetTransitions(i) {
			for c := trans.min; c <= trans.max; c++ {
				if c >= minCP && c <= maxCP {
					class := classMap[c-minCP]
					transitions[i][class] = trans.to
				}
			}
		}
	}

	// Build accept map
	accept := make(map[int]bool)
	for i := 0; i < numStates; i++ {
		if min.IsAccept(i) {
			accept[i] = true
		}
	}

	return &CompiledAutomaton{
		automaton:    min,
		start:        min.GetInitialState(),
		accept:       accept,
		transitions:  transitions,
		classCount:   classCount,
		classMap:     classMap,
		minCodePoint: minCP,
		maxCodePoint: maxCP,
	}
}

// Run runs the compiled automaton on input bytes.
func (c *CompiledAutomaton) Run(input []byte) bool {
	if c.start == -1 {
		return false
	}

	state := c.start
	for _, b := range input {
		if state == -1 {
			return false
		}

		// Get character class
		cp := int(b)
		if cp < c.minCodePoint || cp > c.maxCodePoint {
			return false
		}
		class := c.classMap[cp-c.minCodePoint]

		// Follow transition
		if class >= c.classCount {
			return false
		}
		state = c.transitions[state][class]
	}

	return c.accept[state]
}

// RunString runs the compiled automaton on a string input.
func (c *CompiledAutomaton) RunString(input string) bool {
	return c.Run([]byte(input))
}

// GetAutomaton returns the underlying automaton.
func (c *CompiledAutomaton) GetAutomaton() *Automaton {
	return c.automaton
}

// GetStartState returns the start state.
func (c *CompiledAutomaton) GetStartState() int {
	return c.start
}

// IsAccept returns true if the state is accepting.
func (c *CompiledAutomaton) IsAccept(state int) bool {
	return c.accept[state]
}

// GetNextState returns the next state given current state and input character.
func (c *CompiledAutomaton) GetNextState(state int, input int) int {
	if state < 0 || state >= len(c.transitions) {
		return -1
	}

	if input < c.minCodePoint || input > c.maxCodePoint {
		return -1
	}

	class := c.classMap[input-c.minCodePoint]
	if class >= c.classCount {
		return -1
	}

	return c.transitions[state][class]
}

// Type returns the type of this automaton.
// Returns: NONE, ALL, SINGLE, or NORMAL.
func (c *CompiledAutomaton) Type() string {
	if c.start == -1 {
		return "NONE"
	}

	numStates := len(c.transitions)
	if numStates == 0 {
		return "NONE"
	}

	// Check if it's a single string
	if c.isSingleString() {
		return "SINGLE"
	}

	// Check if it's "all" (accepts everything)
	if c.acceptsAll() {
		return "ALL"
	}

	return "NORMAL"
}

// isSingleString returns true if the automaton accepts exactly one string.
func (c *CompiledAutomaton) isSingleString() bool {
	// Check that there's exactly one accepting path
	// and no branching
	if !c.accept[c.start] && len(c.transitions[c.start]) == 0 {
		return false
	}

	visited := make(map[int]bool)
	var checkPath func(int) bool
	checkPath = func(state int) bool {
		if visited[state] {
			return true // Cycle found
		}
		visited[state] = true

		// Count outgoing transitions
		count := 0
		for _, next := range c.transitions[state] {
			if next != -1 {
				count++
				if !checkPath(next) {
					return false
				}
			}
		}

		// Should have at most 1 transition (if not accepting)
		// or 0 transitions (if accepting)
		if !c.accept[state] && count > 1 {
			return false
		}

		return true
	}

	return checkPath(c.start)
}

// acceptsAll returns true if the automaton accepts all strings.
func (c *CompiledAutomaton) acceptsAll() bool {
	// All states should be accepting
	// and there should be transitions for all inputs
	for i := 0; i < len(c.transitions); i++ {
		if !c.accept[i] {
			return false
		}
	}

	// Check that initial state has self-loop for all inputs
	for _, next := range c.transitions[c.start] {
		if next != c.start {
			return false
		}
	}

	return true
}

// GetTerm returns the single term this automaton matches (if Type is SINGLE).
func (c *CompiledAutomaton) GetTerm() string {
	if c.Type() != "SINGLE" {
		return ""
	}

	// Follow the single path from start
	var result []rune
	state := c.start

	for !c.accept[state] {
		// Find the single transition
		for class, next := range c.transitions[state] {
			if next != -1 {
				// Find a code point in this class
				for cp := c.minCodePoint; cp <= c.maxCodePoint; cp++ {
					if c.classMap[cp-c.minCodePoint] == class {
						result = append(result, rune(cp))
						state = next
						break
					}
				}
				break
			}
		}
	}

	return string(result)
}

// computeCharClasses computes character classes for the automaton.
// It partitions the code point space into equivalence classes.
func computeCharClasses(a *Automaton) ([]int, int, int, int) {
	if a.IsEmpty() {
		return []int{}, 0, 0, 0
	}

	// Find min and max code points used
	minCP := 0x10FFFF + 1
	maxCP := -1

	for i := 0; i < a.NumStates(); i++ {
		for _, trans := range a.GetTransitions(i) {
			if trans.min < minCP {
				minCP = trans.min
			}
			if trans.max > maxCP {
				maxCP = trans.max
			}
		}
	}

	if maxCP < minCP {
		return []int{}, 0, 0, 0
	}

	// Collect all transition boundaries
	boundaries := make(map[int]bool)
	boundaries[minCP] = true

	for i := 0; i < a.NumStates(); i++ {
		for _, trans := range a.GetTransitions(i) {
			boundaries[trans.min] = true
			if trans.max+1 <= maxCP {
				boundaries[trans.max+1] = true
			}
		}
	}

	// Sort boundaries
	bounds := make([]int, 0, len(boundaries))
	for b := range boundaries {
		bounds = append(bounds, b)
	}
	sortInts(bounds)

	// Create class map
	classCount := len(bounds)
	classMap := make([]int, maxCP-minCP+1)

	currentClass := 0
	for i := minCP; i <= maxCP; i++ {
		if currentClass+1 < classCount && i >= bounds[currentClass+1] {
			currentClass++
		}
		classMap[i-minCP] = currentClass
	}

	return classMap, classCount, minCP, maxCP
}

// sortInts sorts a slice of integers (helper function).
func sortInts(a []int) {
	for i := 0; i < len(a); i++ {
		for j := i + 1; j < len(a); j++ {
			if a[j] < a[i] {
				a[i], a[j] = a[j], a[i]
			}
		}
	}
}
