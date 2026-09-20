// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Port of org.apache.lucene.util.automaton.CompiledAutomaton from Apache
// Lucene 10.5.0 (Apache License 2.0).

package automaton

import (
	"math"

	"github.com/FlavioCFOliveira/Gocene/internal/hppc"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// compiledAutomatonBaseRAMBytes mirrors the static BASE_RAM_BYTES, which Java
// computes as RamUsageEstimator.shallowSizeOfInstance(CompiledAutomaton.class).
var compiledAutomatonBaseRAMBytes = util.ShallowSizeOf(CompiledAutomaton{})

// AutomatonType enumerates the internal forms an Automaton is compiled into,
// chosen for the most efficient execution of the language it accepts.
//
// Mirrors the nested enum CompiledAutomaton.AUTOMATON_TYPE.
type AutomatonType int

const (
	// AutomatonTypeNone is an automaton that accepts no strings.
	AutomatonTypeNone AutomatonType = iota
	// AutomatonTypeAll is an automaton that accepts all possible strings.
	AutomatonTypeAll
	// AutomatonTypeSingle is an automaton that accepts only a single fixed string.
	AutomatonTypeSingle
	// AutomatonTypeNormal is the catch-all for any other automata.
	AutomatonTypeNormal
)

// String returns the Java enum constant name, so that a value formatted with
// %v or %s reads exactly as AUTOMATON_TYPE.name() does.
func (t AutomatonType) String() string {
	switch t {
	case AutomatonTypeNone:
		return "NONE"
	case AutomatonTypeAll:
		return "ALL"
	case AutomatonTypeSingle:
		return "SINGLE"
	case AutomatonTypeNormal:
		return "NORMAL"
	default:
		return "UNKNOWN"
	}
}

// CompiledAutomaton is an immutable value holding compiled details for a given
// Automaton. The Automaton can be either deterministic or non-deterministic.
// A deterministic automaton must not have dead states, though it need not be
// minimal, and is executed using ByteRunAutomaton; a non-deterministic one is
// executed using NFARunAutomaton.
//
// Mirrors org.apache.lucene.util.automaton.CompiledAutomaton, which implements
// Accountable.
//
// PORT NOTE. Two members of the Java class cannot be declared here, because
// their signatures name types from packages that already depend on this one
// and Go forbids the resulting import cycle:
//
//   - getTermsEnum(org.apache.lucene.index.Terms) is rendered as the
//     package-level function index.CompiledAutomatonTermsEnum, in the package
//     that owns Terms, TermsEnum and SingleTermsEnum;
//   - visit(QueryVisitor, Query, String) is rendered as the package-level
//     function search.CompiledAutomatonVisit, in the package that owns
//     QueryVisitor and Query.
//
// This is the same split the port already applies to SortField, whose state
// lives in spi and whose search-typed members (rewrite, Provider) are
// package-level functions in search; see search/sort_field.go.
type CompiledAutomaton struct {
	// Type is the "simplified" type when the constructor was asked to
	// simplify, else NORMAL.
	//
	// Mirrors the public final field type.
	Type AutomatonType

	// Term is the singleton term, for AutomatonTypeSingle.
	//
	// Mirrors the public final field term.
	Term *util.BytesRef

	// RunAutomaton is the matcher for quickly determining whether a byte
	// slice is accepted. Only valid for AutomatonTypeNormal.
	//
	// Mirrors the public final field runAutomaton.
	RunAutomaton *ByteRunAutomaton

	// Automaton carries the transitions indexed by state number for
	// traversal. The state numbering is consistent with RunAutomaton. Only
	// valid for AutomatonTypeNormal.
	//
	// Mirrors the public final field automaton.
	Automaton *Automaton

	// nfaRunAutomaton is a matcher run directly on an NFA; it determinizes
	// states on demand and caches them. This field and RunAutomaton are never
	// both non-nil.
	//
	// Mirrors the package-private final field nfaRunAutomaton.
	//
	// TODO: merge this with RunAutomaton (carried over from the Java source).
	nfaRunAutomaton *NFARunAutomaton

	// CommonSuffixRef is the suffix shared by every string the automaton
	// accepts. Only valid for AutomatonTypeNormal, and only when the automaton
	// accepts an infinite language. It is nil when the common suffix has
	// length 0.
	//
	// Mirrors the public final field commonSuffixRef.
	CommonSuffixRef *util.BytesRef

	// Finite indicates whether the automaton accepts a finite set of strings.
	// Only valid for AutomatonTypeNormal.
	//
	// Mirrors the public final field finite.
	Finite bool

	// SinkState is the state that accepts all suffixes, if any, else -1.
	//
	// Mirrors the public final field sinkState.
	SinkState int

	// transition is the scratch Transition reused by addTail and Floor.
	//
	// Mirrors the private field transition.
	transition *Transition
}

// NewCompiledAutomatonSimplified compiles a, passing simplify=true so that the
// constructor tries to simplify the automaton.
//
// Mirrors CompiledAutomaton(Automaton), whose body is
// this(automaton, false, true).
func NewCompiledAutomatonSimplified(a *Automaton) *CompiledAutomaton {
	return NewCompiledAutomaton(a, false, true, false)
}

// NewCompiledAutomatonFinite compiles a. When simplify is true the constructor
// runs possibly expensive operations to determine whether the automaton is one
// of the cases in AutomatonType. Set finite to true when the automaton is
// finite, false when it is infinite or unknown.
//
// Mirrors CompiledAutomaton(Automaton, boolean, boolean), whose body is
// this(automaton, finite, simplify, false).
func NewCompiledAutomatonFinite(a *Automaton, finite, simplify bool) *CompiledAutomaton {
	return NewCompiledAutomaton(a, finite, simplify, false)
}

// findSinkState returns the sink state, if present, else -1.
//
// Mirrors the private static CompiledAutomaton.findSinkState(Automaton).
func findSinkState(a *Automaton) int {
	numStates := a.NumStates()
	t := NewTransition()
	foundState := -1
	for s := 0; s < numStates; s++ {
		if a.IsAccept(s) {
			count := a.InitTransition(s, t)
			isSinkState := false
			for i := 0; i < count; i++ {
				a.GetNextTransition(t)
				if t.Dest == s && t.Min == 0 && t.Max == 0xff {
					isSinkState = true
					break
				}
			}
			if isSinkState {
				foundState = s
				break
			}
		}
	}

	return foundState
}

// NewCompiledAutomaton compiles a. When simplify is true the constructor runs
// possibly expensive operations to determine whether the automaton is one of
// the cases in AutomatonType. Set finite to true when the automaton is finite,
// false when it is infinite or unknown. Set isBinary to true when the caller
// has already built a binary automaton, as PrefixQuery does because it can be
// given a binary (not necessarily UTF-8) term.
//
// Mirrors CompiledAutomaton(Automaton, boolean, boolean, boolean).
//
// PORT NOTE. Java signals a determinization blow-up with the unchecked
// TooComplexToDeterminizeException, so no Go error is returned; the failure
// panics, exactly as the sibling constructor NewByteRunAutomatonBinary in this
// package already does.
func NewCompiledAutomaton(a *Automaton, finite, simplify, isBinary bool) *CompiledAutomaton {
	ca := &CompiledAutomaton{transition: NewTransition()}

	if a.NumStates() == 0 {
		a = NewAutomaton()
		a.CreateState()
	}

	// simplify requires a DFA
	if simplify && a.IsDeterministic() {

		// Test whether the automaton is a "simple" form and
		// if so, don't create a runAutomaton.  Note that on a
		// large automaton these tests could be costly:

		if IsEmpty(a) {
			// matches nothing
			ca.Type = AutomatonTypeNone
			ca.Term = nil
			ca.CommonSuffixRef = nil
			ca.RunAutomaton = nil
			ca.Automaton = nil
			ca.Finite = true
			ca.SinkState = -1
			ca.nfaRunAutomaton = nil
			return ca
		}

		var isTotal bool

		// NOTE: only approximate, because automaton may not be minimal:
		if isBinary {
			isTotal = IsTotalRange(a, 0, 0xff)
		} else {
			isTotal = IsTotal(a)
		}

		if isTotal {
			// matches all possible strings
			ca.Type = AutomatonTypeAll
			ca.Term = nil
			ca.CommonSuffixRef = nil
			ca.RunAutomaton = nil
			ca.Automaton = nil
			ca.Finite = false
			ca.SinkState = -1
			ca.nfaRunAutomaton = nil
			return ca
		}

		singleton := GetSingleton(a)

		if singleton != nil {
			// matches a fixed string
			ca.Type = AutomatonTypeSingle
			ca.CommonSuffixRef = nil
			ca.RunAutomaton = nil
			ca.Automaton = nil
			ca.Finite = true

			if isBinary {
				term, err := util.IntsRefToBytesRef(&util.IntsRef{
					Ints:   singleton,
					Offset: 0,
					Length: len(singleton),
				})
				if err != nil {
					panic(err)
				}
				ca.Term = term
			} else {
				ca.Term = util.NewBytesRef([]byte(util.NewStringFromCodePoints(singleton, 0, len(singleton))))
			}
			ca.SinkState = -1
			ca.nfaRunAutomaton = nil
			return ca
		}
	}

	ca.Type = AutomatonTypeNormal
	ca.Term = nil

	ca.Finite = finite

	var binary *Automaton
	if isBinary {
		// Caller already built binary automaton themselves, e.g. PrefixQuery
		// does this since it can be provided with a binary (not necessarily
		// UTF8!) term:
		binary = a
	} else {
		// Incoming automaton is unicode, and we must convert to UTF8 to match what's in the index:
		binary = NewUTF32ToUTF8().Convert(a)
	}

	// compute a common suffix for infinite DFAs, this is an optimization for "leading wildcard"
	// so don't burn cycles on it if the DFA is finite, or largeish
	if ca.Finite || a.NumStates()+a.NumTransitions() > 1000 {
		ca.CommonSuffixRef = nil
	} else {
		suffix, err := GetCommonSuffixBytesRef(binary)
		if err != nil {
			panic(err)
		}
		if suffix.Length == 0 {
			ca.CommonSuffixRef = nil
		} else {
			ca.CommonSuffixRef = suffix
		}
	}

	if !a.IsDeterministic() && !binary.IsDeterministic() {
		ca.Automaton = nil
		ca.RunAutomaton = nil
		ca.SinkState = -1
		ca.nfaRunAutomaton = NewNFARunAutomatonAlphabet(binary, 0xff)
	} else {
		// We already had a DFA (or threw exception), according to mike UTF32toUTF8 won't "blow up"
		det, err := Determinize(binary, math.MaxInt32)
		if err != nil {
			panic(err)
		}
		binary = det
		ca.RunAutomaton = NewByteRunAutomatonBinary(binary, true)

		ca.Automaton = ca.RunAutomaton.GetAutomaton()

		// TODO: this is a bit fragile because if the automaton is not minimized there could be more
		// than 1 sink state but auto-prefix will fail
		// to run for those:
		ca.SinkState = findSinkState(ca.Automaton)
		ca.nfaRunAutomaton = nil
	}
	return ca
}

// addTail appends the largest tail below leadLabel to term and returns it.
//
// Mirrors the private CompiledAutomaton.addTail(int, BytesRefBuilder, int, int).
func (ca *CompiledAutomaton) addTail(state int, term *util.BytesRefBuilder, idx, leadLabel int) *util.BytesRef {
	// Find biggest transition that's < label
	// TODO: use binary search here
	maxIndex := -1
	numTransitions := ca.Automaton.InitTransition(state, ca.transition)
	for i := 0; i < numTransitions; i++ {
		ca.Automaton.GetNextTransition(ca.transition)
		if ca.transition.Min < leadLabel {
			maxIndex = i
		} else {
			// Transitions are always sorted
			break
		}
	}

	// assert maxIndex != -1
	ca.Automaton.GetTransition(state, maxIndex, ca.transition)

	// Append floorLabel
	var floorLabel int
	if ca.transition.Max > leadLabel-1 {
		floorLabel = leadLabel - 1
	} else {
		floorLabel = ca.transition.Max
	}
	term.Grow(1 + idx)
	term.SetByteAt(idx, byte(floorLabel))

	state = ca.transition.Dest
	idx++

	// Push down to last accept state
	for {
		numTransitions = ca.Automaton.GetNumTransitions(state)
		if numTransitions == 0 {
			// assert ca.RunAutomaton.IsAccept(state)
			term.SetLength(idx)
			return term.Get()
		}
		// We are pushing "top" -- so get last label of
		// last transition:
		ca.Automaton.GetTransition(state, numTransitions-1, ca.transition)
		term.Grow(1 + idx)
		term.SetByteAt(idx, byte(ca.transition.Max))
		state = ca.transition.Dest
		idx++
	}
}

// Floor finds the largest term accepted by this automaton that is less than or
// equal to the provided input term. The result is placed in output; it is fine
// for output and input to point at the same bytes. The returned result is
// either the provided output, or nil when there is no floor term, that is when
// the provided input term sorts before the first term this automaton accepts.
//
// Mirrors CompiledAutomaton.floor(BytesRef, BytesRefBuilder).
func (ca *CompiledAutomaton) Floor(input *util.BytesRef, output *util.BytesRefBuilder) *util.BytesRef {

	state := 0

	// Special case empty string:
	if input.Length == 0 {
		if ca.RunAutomaton.IsAccept(state) {
			output.Clear()
			return output.Get()
		}
		return nil
	}

	stack := hppc.NewIntArrayList()

	idx := 0
	for {
		label := int(input.Bytes[input.Offset+idx]) & 0xff
		nextState := ca.RunAutomaton.Step(state, label)

		if idx == input.Length-1 {
			if nextState != -1 && ca.RunAutomaton.IsAccept(nextState) {
				// Input string is accepted
				output.Grow(1 + idx)
				output.SetByteAt(idx, byte(label))
				output.SetLength(input.Length)
				return output.Get()
			}
			nextState = -1
		}

		if nextState == -1 {

			// Pop back to a state that has a transition
			// <= our label:
			for {
				numTransitions := ca.Automaton.GetNumTransitions(state)
				if numTransitions == 0 {
					// assert ca.RunAutomaton.IsAccept(state)
					output.SetLength(idx)
					return output.Get()
				}
				ca.Automaton.GetTransition(state, 0, ca.transition)

				if label-1 < ca.transition.Min {

					if ca.RunAutomaton.IsAccept(state) {
						output.SetLength(idx)
						return output.Get()
					}
					// pop
					if stack.Size() == 0 {
						return nil
					}
					state = stack.RemoveLast()
					idx--
					label = int(input.Bytes[input.Offset+idx]) & 0xff
				} else {
					break
				}
			}

			return ca.addTail(state, output, idx, label)
		}

		output.Grow(1 + idx)
		output.SetByteAt(idx, byte(label))
		stack.Add(state)
		state = nextState
		idx++
	}
}

// GetByteRunnable returns a ByteRunnable instance. Which one it is depends on
// whether an NFA or a DFA was passed in, and the result is not guaranteed to
// be non-nil.
//
// Mirrors CompiledAutomaton.getByteRunnable().
func (ca *CompiledAutomaton) GetByteRunnable() ByteRunnable {
	// they can be both null but not both non-null
	// assert ca.nfaRunAutomaton == nil || ca.RunAutomaton == nil
	if ca.nfaRunAutomaton == nil {
		if ca.RunAutomaton == nil {
			return nil
		}
		return ca.RunAutomaton
	}
	return ca.nfaRunAutomaton
}

// GetTransitionAccessor returns a TransitionAccessor instance. Which one it is
// depends on whether an NFA or a DFA was passed in, and the result is not
// guaranteed to be non-nil.
//
// Mirrors CompiledAutomaton.getTransitionAccessor().
func (ca *CompiledAutomaton) GetTransitionAccessor() TransitionAccessor {
	// they can be both null but not both non-null
	// assert ca.nfaRunAutomaton == nil || ca.Automaton == nil
	if ca.nfaRunAutomaton == nil {
		if ca.Automaton == nil {
			return nil
		}
		return ca.Automaton
	}
	return ca.nfaRunAutomaton
}

// GetAutomaton returns the compiled transition table, which is nil for every
// type but AutomatonTypeNormal.
//
// PORT NOTE. Java has no such accessor because the field is public; this is a
// Gocene-only reader kept for the call sites that predate the faithful field
// set, and it returns exactly the Automaton field.
func (ca *CompiledAutomaton) GetAutomaton() *Automaton {
	return ca.Automaton
}

// Run reports whether the automaton accepts term.
//
// PORT NOTE. Java's CompiledAutomaton has no run method; matching goes through
// getByteRunnable(). This is a Gocene-only convenience retained for the call
// sites that already depend on it, expressed in terms of the Java members: the
// classification in Type decides NONE / ALL / SINGLE without a matcher, and
// NORMAL defers to the ByteRunnable.
func (ca *CompiledAutomaton) Run(term *util.BytesRef) bool {
	if term == nil {
		return false
	}
	switch ca.Type {
	case AutomatonTypeNone:
		return false
	case AutomatonTypeAll:
		return true
	case AutomatonTypeSingle:
		return util.BytesRefEquals(ca.Term, term)
	default:
		runnable := ca.GetByteRunnable()
		if runnable == nil {
			return false
		}
		return runnable.Run(term.Bytes, term.Offset, term.Length)
	}
}

// HashCode returns a hash consistent with Equals.
//
// Mirrors CompiledAutomaton.hashCode().
//
// PORT NOTE. Two of the four Java components have no reproducible Go
// counterpart: AUTOMATON_TYPE.hashCode() is the JVM identity hash of an enum
// constant, and NFARunAutomaton inherits Object.hashCode(). The Go rendering
// substitutes the enum ordinal and the structural NFARunAutomaton.HashCode()
// this package already defines, so that the value is deterministic within a
// run and across runs. hashCode carries no binary contract.
func (ca *CompiledAutomaton) HashCode() int {
	const prime = 31
	result := 1
	if ca.RunAutomaton == nil {
		result = prime * result
	} else {
		result = prime*result + ca.RunAutomaton.HashCode()
	}
	if ca.nfaRunAutomaton == nil {
		result = prime * result
	} else {
		result = prime*result + ca.nfaRunAutomaton.HashCode()
	}
	if ca.Term == nil {
		result = prime * result
	} else {
		result = prime*result + ca.Term.HashCode()
	}
	result = prime*result + int(ca.Type)
	return result
}

// Equals reports whether ca and other compile to the same language in the same
// internal form.
//
// Mirrors CompiledAutomaton.equals(Object). The NFA leg compares identity,
// which is what Objects.equals does for NFARunAutomaton because that class does
// not override Object.equals.
func (ca *CompiledAutomaton) Equals(other *CompiledAutomaton) bool {
	if ca == other {
		return true
	}
	if ca == nil || other == nil {
		return false
	}
	if ca.Type != other.Type {
		return false
	}
	if ca.Type == AutomatonTypeSingle {
		if !util.BytesRefEquals(ca.Term, other.Term) {
			return false
		}
	} else if ca.Type == AutomatonTypeNormal {
		return byteRunAutomatonEquals(ca.RunAutomaton, other.RunAutomaton) &&
			ca.nfaRunAutomaton == other.nfaRunAutomaton
	}

	return true
}

// byteRunAutomatonEquals renders Objects.equals(ByteRunAutomaton,
// ByteRunAutomaton): equal when both are absent, else RunAutomaton.equals.
func byteRunAutomatonEquals(a, b *ByteRunAutomaton) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.RunAutomaton.Equals(b.RunAutomaton)
}

// RamBytesUsed returns the approximate memory footprint of this instance.
//
// Mirrors CompiledAutomaton.ramBytesUsed(); this.Automaton is accounted for
// via RunAutomaton.
func (ca *CompiledAutomaton) RamBytesUsed() int64 {
	total := compiledAutomatonBaseRAMBytes
	if ca.CommonSuffixRef != nil {
		total += util.ShallowSizeOf(ca.CommonSuffixRef)
	}
	if ca.RunAutomaton != nil {
		total += util.ShallowSizeOf(ca.RunAutomaton)
	}
	if ca.nfaRunAutomaton != nil {
		total += util.ShallowSizeOf(ca.nfaRunAutomaton)
	}
	if ca.Term != nil {
		total += util.ShallowSizeOf(ca.Term)
	}
	if ca.transition != nil {
		total += util.ShallowSizeOf(ca.transition)
	}
	return total
}
