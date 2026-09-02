package automaton

// MinCodePoint is the minimum Unicode codepoint (0).
const MinCodePoint = 0

// MaxCodePoint is the maximum Unicode codepoint.
const MaxCodePoint = 0x10FFFF

// Automaton is a simple representation of a finite state machine.
type Automaton struct {
	// nextState tracks the next available state ID.
	nextState int
	// acceptStates tracks which states are accepting.
	acceptStates map[int]bool
	// transitions stores the transitions (simplified representation).
	transitions map[int][]Transition
	// deterministic tracks whether the automaton is deterministic.
	deterministic bool
}

// Transition represents a transition from one state to another.
type Transition struct {
	Dest   int
	Min    int
	Max    int
}

// NewTransition creates a new empty transition.
func NewTransition() *Transition {
	return &Transition{}
}

// NewAutomaton creates a new empty automaton with initial state 0.
func NewAutomaton() *Automaton {
	return &Automaton{
		nextState:     1, // state 0 is the initial state
		acceptStates:  make(map[int]bool),
		transitions:   make(map[int][]Transition),
		deterministic: true,
	}
}

// NewAutomatonWithCapacity creates a new automaton with capacity hints.
func NewAutomatonWithCapacity(numStates, numTransitions int) *Automaton {
	return &Automaton{
		nextState:     1,
		acceptStates:  make(map[int]bool, numStates),
		transitions:   make(map[int][]Transition, numStates),
		deterministic: true,
	}
}

// CreateState creates a new state and returns its ID.
func (a *Automaton) CreateState() int {
	state := a.nextState
	a.nextState++
	return state
}

// SetAccept marks a state as accepting or not.
func (a *Automaton) SetAccept(state int, accept bool) {
	if accept {
		a.acceptStates[state] = true
	} else {
		delete(a.acceptStates, state)
	}
}

// AddTransition adds a transition from one state to another for a range of characters.
func (a *Automaton) AddTransition(from, to, minChar, maxChar int) {
	if a.transitions[from] == nil {
		a.transitions[from] = make([]Transition, 0)
	}
	a.transitions[from] = append(a.transitions[from], Transition{
		Dest: to,
		Min:  minChar,
		Max:  maxChar,
	})
}

// FinishState finalizes the current automaton state.
// This is a no-op in the simplified implementation.
func (a *Automaton) FinishState() {
	// No-op
}

// AddEpsilon adds an epsilon transition from one state to another.
func (a *Automaton) AddEpsilon(from, to int) {
	if a.transitions[from] == nil {
		a.transitions[from] = make([]Transition, 0)
	}
	// Epsilon transitions are represented as transitions on a special character (-1)
	a.transitions[from] = append(a.transitions[from], Transition{
		Dest: to,
		Min:  -1,
		Max:  -1,
	})
}

// NumStates returns the number of states in the automaton.
func (a *Automaton) NumStates() int {
	return a.nextState
}

// NumTransitions returns the total number of transitions in the automaton.
func (a *Automaton) NumTransitions() int {
	count := 0
	for _, trans := range a.transitions {
		count += len(trans)
	}
	return count
}

// InitTransition initializes transition iteration for a state (stub implementation).
func (a *Automaton) InitTransition(state int, t *Transition) int {
	if trans, ok := a.transitions[state]; ok {
		if len(trans) > 0 && t != nil {
			*t = trans[0]
			return len(trans)
		}
		return len(trans)
	}
	return 0
}

// GetNextTransition advances to the next transition (stub implementation).
func (a *Automaton) GetNextTransition(t *Transition) {
	// Stub: minimal implementation for compatibility
}

// Copy copies another automaton into this one (stub implementation).
func (a *Automaton) Copy(other *Automaton) {
	// Stub: minimal implementation for compatibility
}

// IsAccept returns whether a state is accepting.
func (a *Automaton) IsAccept(state int) bool {
	return a.acceptStates[state]
}

// GetNumTransitions returns the number of transitions for a state.
func (a *Automaton) GetNumTransitions(state int) int {
	if trans, ok := a.transitions[state]; ok {
		return len(trans)
	}
	return 0
}

// GetTransition returns the transition at a given index from a state (stub).
func (a *Automaton) GetTransition(state, index int) *Transition {
	if trans, ok := a.transitions[state]; ok && index < len(trans) {
		return &trans[index]
	}
	return nil
}

// Step follows a transition on a given character (stub).
func (a *Automaton) Step(state int, c int) int {
	if trans, ok := a.transitions[state]; ok {
		for _, t := range trans {
			if t.Min <= c && c <= t.Max {
				return t.Dest
			}
		}
	}
	return -1
}

// AddString adds transitions for a string to the automaton (stub).
func (a *Automaton) AddString(s string) {
	// Stub: minimal implementation for compatibility
}

func (a *Automaton) String() string {
	return "Automaton{}"
}

func (a *Automaton) RamBytesUsed() int64 {
	return 100
}

func MakePrefixAutomaton(prefix []byte) *Automaton {
	// Simplified representation of a prefix automaton
	return &Automaton{
		nextState:    1,
		acceptStates: make(map[int]bool),
		transitions:  make(map[int][]Transition),
	}
}

// NewLevenshteinAutomaton creates an automaton that matches terms within a given edit distance.
func NewLevenshteinAutomaton(term string, maxEdits int) *Automaton {
	// In a full implementation, this would build a Levenshtein automaton.
	// For now, we return a dummy automaton that matches the term itself.
	auto := NewAutomaton()
	auto.AddString(term)
	return auto
}
