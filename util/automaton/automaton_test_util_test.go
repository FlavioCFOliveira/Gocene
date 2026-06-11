// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Port of org.apache.lucene.tests.util.automaton.AutomatonTestUtil from
// Apache Lucene 10.4.0 (releases/lucene/10.4.0, commit 9983b7c). The
// Lucene file lives under lucene/test-framework/src/java/ and provides
// testing utilities for automata: random regular expression generation,
// random automaton construction, slow reference implementations of
// determinize/minimize/sameLanguage/subsetOf, and assertion helpers.
//
// Conventions:
//   - All identifiers keep the same names as the Java originals so
//     cross-referencing stays mechanical.
//   - Java's java.util.Random becomes Go's *rand.Rand (passed explicitly
//     to every function, unlike the Lucene calls that use a thread-local
//     LuceneTestCase.random()).
//   - Java's hppc / java.util collections are rendered with Go built-ins
//     (map[T]struct{} for sets, []T for queues/lists, []bool for bitsets).
//   - The Unicode surrogate constants are defined locally since Gocene's
//     util package does not yet export them.
//   - getSortedTransitions is implemented as a per-call helper that
//     builds Transition slices via InitTransition/GetNextTransition
//     (which already return transitions sorted by min).

package automaton

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// ---- Unicode surrogate constants (Lucene UnicodeUtil) -----------------

const (
	uniSurHighStart = 0xD800
	uniSurHighEnd   = 0xDBFF
	uniSurLowStart  = 0xDC00
	uniSurLowEnd    = 0xDFFF
)

// ---- Public constants (match Lucene's AutomatonTestUtil) ---------------

// DefaultMaxDeterminizedStates is the maximum number of states that
// Determinize should create by default.
const DefaultMaxDeterminizedStates = 1000000

// MaxRecursionLevel is the maximum level of recursion allowed in
// recursive operations (isFinite, getFiniteStrings).
const MaxRecursionLevel = 1000

// ---- Random regexp generation -----------------------------------------

// RandomRegexp returns a random regular expression string, including full
// Unicode range. The result is guaranteed to parse with NewRegExp.
// Port of AutomatonTestUtil.randomRegexp.
func RandomRegexp(r *rand.Rand) string {
	for {
		regexp := randomRegexpString(r)
		// Skip strings that are not valid UTF-16 (surrogate handling)
		if !validUTF16String(regexp) {
			continue
		}
		_, err := NewRegExp(regexp)
		if err == nil {
			return regexp
		}
	}
}

// randomRegexpString generates a raw random regexp string (may be invalid).
// Port of AutomatonTestUtil.randomRegexpString.
func randomRegexpString(r *rand.Rand) string {
	end := r.Intn(20)
	if end == 0 {
		return ""
	}
	var sb strings.Builder
	sb.Grow(end)
	for i := 0; i < end; i++ {
		t := r.Intn(15)
		switch {
		case t == 0 && i < end-1:
			// Make a surrogate pair
			sb.WriteRune(rune(testUtilNextInt(r, 0xd800, 0xdbff))) // High surrogate
			i++
			sb.WriteRune(rune(testUtilNextInt(r, 0xdc00, 0xdfff))) // Low surrogate
		case t <= 1:
			sb.WriteByte(byte(r.Intn(0x80)))
		case t == 2:
			sb.WriteRune(rune(testUtilNextInt(r, 0x80, 0x800)))
		case t == 3:
			sb.WriteRune(rune(testUtilNextInt(r, 0x800, 0xd7ff)))
		case t == 4:
			sb.WriteRune(rune(testUtilNextInt(r, 0xe000, 0xffff)))
		case t == 5:
			sb.WriteByte('.')
		case t == 6:
			sb.WriteByte('?')
		case t == 7:
			sb.WriteByte('*')
		case t == 8:
			sb.WriteByte('+')
		case t == 9:
			sb.WriteByte('(')
		case t == 10:
			sb.WriteByte(')')
		case t == 11:
			sb.WriteByte('-')
		case t == 12:
			sb.WriteByte('[')
		case t == 13:
			sb.WriteByte(']')
		case t == 14:
			sb.WriteByte('|')
		}
	}
	return sb.String()
}

// getRandomCodePoint picks a random int code point between min and max,
// avoiding surrogates. Panics if the range contains only surrogates.
// Port of AutomatonTestUtil.getRandomCodePoint.
func getRandomCodePoint(r *rand.Rand, min, max int) int {
	var code int
	if max < uniSurHighStart || min > uniSurHighEnd {
		// Easy: entire range is before or after surrogates
		code = min + r.Intn(max-min+1)
	} else if min >= uniSurHighStart {
		if max > uniSurLowEnd {
			// After surrogates
			code = 1 + uniSurLowEnd + r.Intn(max-uniSurLowEnd)
		} else {
			panic(fmt.Sprintf("transition accepts only surrogates: min=%d max=%d", min, max))
		}
	} else if max <= uniSurLowEnd {
		if min < uniSurHighStart {
			// Before surrogates
			code = min + r.Intn(uniSurHighStart-min)
		} else {
			panic(fmt.Sprintf("transition accepts only surrogates: min=%d max=%d", min, max))
		}
	} else {
		// Range includes all surrogates
		gap1 := uniSurHighStart - min
		gap2 := max - uniSurLowEnd
		c := r.Intn(gap1 + gap2)
		if c < gap1 {
			code = min + c
		} else {
			code = uniSurLowEnd + c - gap1 + 1
		}
	}

	if code < min || code > max ||
		(code >= uniSurHighStart && code <= uniSurLowEnd) {
		panic(fmt.Sprintf("code=%d min=%d max=%d", code, min, max))
	}
	return code
}

// validUTF16String returns true if s contains no unpaired surrogates.
// Simplified port of UnicodeUtil.validUTF16String.
func validUTF16String(s string) bool {
	for _, ch := range s {
		if ch >= uniSurHighStart && ch <= uniSurLowEnd {
			// A surrogate outside a valid pair is invalid for our regexp gen.
			// For simplicity we accept high surrogates only when followed by low.
			return false
		}
	}
	return true
}

// testUtilNextInt returns a random int in [min, max]. Port of
// Lucene's TestUtil.nextInt(Random, min, max).
func testUtilNextInt(r *rand.Rand, min, max int) int {
	if min > max {
		panic(fmt.Sprintf("testUtilNextInt: min=%d > max=%d", min, max))
	}
	return min + r.Intn(max-min+1)
}

// ---- RandomAcceptedStrings ---------------------------------------------

// arrivingTransition records a source state and a transition arriving at
// some destination. Port of Lucene's record ArrivingTransition.
type arrivingTransition struct {
	from int
	t    *Transition
}

// RandomAcceptedStrings lets you retrieve random strings accepted by an
// Automaton. Once created, call GetRandomAcceptedString to get a new
// string (in UTF-32 code points).
// Port of AutomatonTestUtil.RandomAcceptedStrings.
type RandomAcceptedStrings struct {
	a            *Automaton
	transitions  [][]*Transition
	leadsToAccept map[*Transition]bool
}

// NewRandomAcceptedStrings builds the reverse-transition map and
// pre-computes which transitions lead to an accept state. Panics if
// the automaton accepts nothing.
// Port of AutomatonTestUtil.RandomAcceptedStrings constructor.
func NewRandomAcceptedStrings(a *Automaton) *RandomAcceptedStrings {
	if a.NumStates() == 0 {
		panic("this automaton accepts nothing")
	}
	numStates := a.NumStates()

	// Build sorted transitions per state
	sorted := make([][]*Transition, numStates)
	t := NewTransition()
	for s := 0; s < numStates; s++ {
		count := a.InitTransition(s, t)
		sorted[s] = make([]*Transition, count)
		for i := 0; i < count; i++ {
			a.GetNextTransition(t)
			cpy := *t
			sorted[s][i] = &cpy
		}
	}

	ras := &RandomAcceptedStrings{
		a:           a,
		transitions: sorted,
		leadsToAccept: make(map[*Transition]bool),
	}

	// Reverse map the transitions, so we can quickly look up all
	// arriving transitions to a given state
	allArriving := make(map[int][]arrivingTransition)
	for s := 0; s < numStates; s++ {
		for _, tr := range sorted[s] {
			allArriving[tr.Dest] = append(allArriving[tr.Dest], arrivingTransition{from: s, t: tr})
		}
	}

	// Breadth-first search, from accept states, backwards
	q := make([]int, 0)
	seen := make(map[int]bool)
	for s := 0; s < numStates; s++ {
		if a.IsAccept(s) {
			q = append(q, s)
			seen[s] = true
		}
	}
	for len(q) > 0 {
		s := q[0]
		q = q[1:]
		arriving, ok := allArriving[s]
		if !ok {
			continue
		}
		for _, at := range arriving {
			if !seen[at.from] {
				q = append(q, at.from)
				seen[at.from] = true
				ras.leadsToAccept[at.t] = true
			}
		}
	}

	return ras
}

// GetRandomAcceptedString returns a random string (in UTF-32 code points)
// accepted by the wrapped automaton.
func (ras *RandomAcceptedStrings) GetRandomAcceptedString(r *rand.Rand) []int {
	var codePoints []int
	s := 0

	for {
		if ras.a.IsAccept(s) {
			if ras.a.GetNumTransitions(s) == 0 {
				break
			}
			if r.Intn(2) == 0 { // r.nextBoolean()
				break
			}
		}

		if ras.a.GetNumTransitions(s) == 0 {
			panic("this automaton has dead states")
		}

		cheat := r.Intn(2) == 0
		var t *Transition
		if cheat {
			// Pick a transition that we know is the fastest path to an accept state
			var toAccept []*Transition
			for _, t0 := range ras.transitions[s] {
				if ras.leadsToAccept[t0] {
					toAccept = append(toAccept, t0)
				}
			}
			if len(toAccept) == 0 {
				// We jumped into a cycle — pick random
				t = ras.transitions[s][r.Intn(len(ras.transitions[s]))]
			} else {
				t = toAccept[r.Intn(len(toAccept))]
			}
		} else {
			t = ras.transitions[s][r.Intn(len(ras.transitions[s]))]
		}
		codePoints = append(codePoints, getRandomCodePoint(r, t.Min, t.Max))
		s = t.Dest
	}
	return codePoints
}

// ---- Random automaton construction ------------------------------------

// randomSingleAutomaton builds a random automaton from a random regexp,
// optionally complemented.
// Port of AutomatonTestUtil.randomSingleAutomaton.
func randomSingleAutomaton(r *rand.Rand) *Automaton {
	for {
		regexp := RandomRegexp(r)
		re, err := NewRegExp(regexp)
		if err != nil {
			continue
		}
		a1, err := re.ToAutomaton()
		if err != nil {
			continue
		}
		if r.Intn(2) == 0 { // r.nextBoolean()
			a1, err = Complement(a1, DefaultMaxDeterminizedStates)
			if err != nil {
				// TooComplexToDeterminizeException — try again
				continue
			}
		}
		return a1
	}
}

// RandomAutomaton returns a random NFA/DFA for testing. It builds two
// random automata from regexps and combines them with a random boolean
// operation (concatenate, union, intersection, or minus).
// Port of AutomatonTestUtil.randomAutomaton.
func RandomAutomaton(r *rand.Rand) *Automaton {
	a1 := randomSingleAutomaton(r)
	a2 := randomSingleAutomaton(r)

	switch r.Intn(4) {
	case 0:
		return Concatenate([]*Automaton{a1, a2})
	case 1:
		return Union([]*Automaton{a1, a2})
	case 2:
		return Intersection(a1, a2)
	default:
		result, err := Minus(a1, a2, DefaultMaxDeterminizedStates)
		if err != nil {
			// Fallback to union on failure
			return Union([]*Automaton{a1, a2})
		}
		return result
	}
}

// ---- Brzozowski minimization (simple, slow reference) ------------------

// ReverseOriginal is the original brics implementation of reverse(). It
// tries to satisfy multiple use-cases by populating a set of initial
// states too.
// Port of AutomatonTestUtil.reverseOriginal.
func ReverseOriginal(a *Automaton, initialStates map[int]struct{}) *Automaton {
	if IsEmpty(a) {
		return NewAutomaton()
	}

	numStates := a.NumStates()

	// Build a new automaton with all edges reversed
	builder := NewBuilder()

	// Initial node; we'll add epsilon transitions at the end
	builder.CreateState()

	for s := 0; s < numStates; s++ {
		builder.CreateState()
	}

	// Old initial state becomes new accept state
	builder.SetAccept(1, true)

	t := NewTransition()
	for s := 0; s < numStates; s++ {
		numTransitions := a.InitTransition(s, t)
		for i := 0; i < numTransitions; i++ {
			a.GetNextTransition(t)
			builder.AddTransition(t.Dest+1, s+1, t.Min, t.Max)
		}
	}

	result := builder.Finish()

	// Add epsilon transitions from new initial state to old accept states
	for s := 0; s < numStates; s++ {
		if a.IsAccept(s) {
			result.AddEpsilon(0, s+1)
			if initialStates != nil {
				initialStates[s+1] = struct{}{}
			}
		}
	}

	result.FinishState()
	return result
}

// MinimizeSimple is a simple, original brics implementation of Brzozowski
// minimize(). It is slower than Hopcroft but serves as an independent
// reference for testing.
// Port of AutomatonTestUtil.minimizeSimple.
func MinimizeSimple(a *Automaton) *Automaton {
	initialSet := make(map[int]struct{})
	a = DeterminizeSimpleReverse(a, initialSet)
	initialSet = make(map[int]struct{})
	a = DeterminizeSimpleReverse(a, initialSet)
	return a
}

// DeterminizeSimpleReverse applies the simple determinization using
// reverseOriginal as the first step, populating initial states.
func DeterminizeSimpleReverse(a *Automaton, initialSet map[int]struct{}) *Automaton {
	return DeterminizeSimpleWithInitial(ReverseOriginal(a, initialSet), initialSet)
}

// ---- Assertion helpers -------------------------------------------------

// AssertMinimalDFA asserts that an automaton is a minimal DFA.
// Port of AutomatonTestUtil.assertMinimalDFA.
func AssertMinimalDFA(t *testing.T, automaton *Automaton) {
	t.Helper()
	AssertCleanDFA(t, automaton)
	minimized := MinimizeSimple(automaton)
	if minimized.NumStates() != automaton.NumStates() {
		t.Errorf("not minimal: original=%d states, brzozowski-minimized=%d states",
			automaton.NumStates(), minimized.NumStates())
	}
}

// AssertCleanDFA asserts that an automaton is a DFA with no dead states.
// Port of AutomatonTestUtil.assertCleanDFA.
func AssertCleanDFA(t *testing.T, automaton *Automaton) {
	t.Helper()
	AssertCleanNFA(t, automaton)
	if !automaton.IsDeterministic() {
		t.Error("must be deterministic")
	}
}

// AssertCleanNFA asserts that an automaton has no dead states.
// Port of AutomatonTestUtil.assertCleanNFA.
func AssertCleanNFA(t *testing.T, automaton *Automaton) {
	t.Helper()
	if HasDeadStatesFromInitial(automaton) {
		t.Error("has dead states reachable from initial")
	}
	if hasDeadStatesToAccept(automaton) {
		t.Error("has dead states leading to accept")
	}
	if HasDeadStates(automaton) {
		t.Error("has unreachable dead states (ghost states)")
	}
}

// hasDeadStatesToAccept reports whether some state can reach an accept
// state but is not reachable from the initial state. Port of
// Operations.hasDeadStatesToAccept.
func hasDeadStatesToAccept(a *Automaton) bool {
	reachableFromInitial := liveStatesFromInitial(a)
	reachableFromAccept := liveStatesToAccept(a)
	for i := range reachableFromAccept {
		if reachableFromAccept[i] && !reachableFromInitial[i] {
			return true
		}
	}
	return false
}

// ---- Simple determinization (slow, original brics) ---------------------

// DeterminizeSimple determinizes the given automaton using the simple
// subset-construction algorithm. This is much slower than Operations.
// Determinize but serves as an independent reference.
// Port of AutomatonTestUtil.determinizeSimple.
func DeterminizeSimple(a *Automaton) *Automaton {
	initialSet := make(map[int]struct{})
	initialSet[0] = struct{}{}
	return DeterminizeSimpleWithInitial(a, initialSet)
}

// DeterminizeSimpleWithInitial determinizes the given automaton using the
// given set of initial states.
// Port of AutomatonTestUtil.determinizeSimple(automaton, set).
func DeterminizeSimpleWithInitial(a *Automaton, initialSet map[int]struct{}) *Automaton {
	if a.NumStates() == 0 {
		return a
	}

	points := a.GetStartPoints()

	// subset construction
	sets := make(map[string]map[int]struct{})
	worklist := make([]map[int]struct{}, 0)
	newstate := make(map[string]int)

	setToKey := func(s map[int]struct{}) string {
		keys := make([]int, 0, len(s))
		for k := range s {
			keys = append(keys, k)
		}
		// Sort for deterministic key (subset construction needs stable keys)
		sortInts(keys)
		var sb strings.Builder
		for i, k := range keys {
			if i > 0 {
				sb.WriteByte(',')
			}
			fmt.Fprintf(&sb, "%d", k)
		}
		return sb.String()
	}

	initialKey := setToKey(initialSet)
	sets[initialKey] = initialSet
	worklist = append(worklist, initialSet)

	result := NewBuilder()
	result.CreateState()
	newstate[initialKey] = 0

	tScratch := NewTransition()

	for len(worklist) > 0 {
		s := worklist[0]
		worklist = worklist[1:]
		r := newstate[setToKey(s)]

		for q := range s {
			if a.IsAccept(q) {
				result.SetAccept(r, true)
				break
			}
		}

		for n := 0; n < len(points); n++ {
			p := make(map[int]struct{})
			for q := range s {
				count := a.InitTransition(q, tScratch)
				for i := 0; i < count; i++ {
					a.GetNextTransition(tScratch)
					if tScratch.Min <= points[n] && points[n] <= tScratch.Max {
						p[tScratch.Dest] = struct{}{}
					}
				}
			}

			pKey := setToKey(p)
			if _, exists := sets[pKey]; !exists {
				sets[pKey] = p
				worklist = append(worklist, p)
				newstate[pKey] = result.CreateState()
			}
			qDest := newstate[pKey]
			min := points[n]
			var max int
			if n+1 < len(points) {
				max = points[n+1] - 1
			} else {
				max = MaxCodePoint
			}
			result.AddTransition(r, qDest, min, max)
		}
	}

	return RemoveDeadStates(result.Finish())
}

// sortInts is a simple insertion sort for small integer slices used in
// set-to-key conversion within DeterminizeSimpleWithInitial.
func sortInts(a []int) {
	for i := 1; i < len(a); i++ {
		for j := i; j > 0 && a[j] < a[j-1]; j-- {
			a[j], a[j-1] = a[j-1], a[j]
		}
	}
}

// ---- Finite strings ----------------------------------------------------

// GetFiniteStringsRecursive returns the set of accepted strings, assuming
// that at most limit strings are accepted. If more than limit strings are
// accepted, the first limit strings found are returned. If limit < 0, the
// limit is infinite.
//
// This implementation is recursive: it uses one stack frame for each digit
// in the returned strings (so the max stack depth is the max length of a
// returned string).
// Port of AutomatonTestUtil.getFiniteStringsRecursive.
func GetFiniteStringsRecursive(a *Automaton, limit int) []*util.IntsRef {
	strings := make(map[string]*util.IntsRef)
	if !getFiniteStringsRec(a, 0, make(map[int]bool), strings, util.NewIntsRefBuilder(), limit) {
		result := make([]*util.IntsRef, 0, len(strings))
		for _, v := range strings {
			result = append(result, v)
		}
		return result
	}
	result := make([]*util.IntsRef, 0, len(strings))
	for _, v := range strings {
		result = append(result, v)
	}
	return result
}

// getFiniteStringsRec returns the strings that can be produced from the
// given state, or false if more than limit strings are found.
// limit < 0 means "infinite".
// Port of AutomatonTestUtil.getFiniteStrings (private).
func getFiniteStringsRec(
	a *Automaton,
	s int,
	pathstates map[int]bool,
	strings map[string]*util.IntsRef,
	path *util.IntsRefBuilder,
	limit int,
) bool {
	pathstates[s] = true
	t := NewTransition()
	count := a.InitTransition(s, t)
	for i := 0; i < count; i++ {
		a.GetNextTransition(t)
		if pathstates[t.Dest] {
			return false
		}
		for n := t.Min; n <= t.Max; n++ {
			path.Append(n)
			if a.IsAccept(t.Dest) {
				ref := &util.IntsRef{
					Ints:   make([]int, path.Length()),
					Offset: 0,
					Length: path.Length(),
				}
				copy(ref.Ints, path.Ints()[:path.Length()])
				key := intsRefKey(ref)
				strings[key] = ref
				if limit >= 0 && len(strings) > limit {
					return false
				}
			}
			if !getFiniteStringsRec(a, t.Dest, pathstates, strings, path, limit) {
				return false
			}
			path.SetLength(path.Length() - 1)
		}
	}
	delete(pathstates, s)
	return true
}

// intsRefKey creates a stable string key for an IntsRef for use in maps.
func intsRefKey(ref *util.IntsRef) string {
	var sb strings.Builder
	for i := 0; i < ref.Length; i++ {
		if i > 0 {
			sb.WriteByte(',')
		}
		fmt.Fprintf(&sb, "%d", ref.Ints[ref.Offset+i])
	}
	return sb.String()
}

// ---- Finiteness check --------------------------------------------------

// IsFiniteAutomaton returns true if the language of this automaton is
// finite. The automaton must not have any dead states.
// Port of AutomatonTestUtil.isFinite.
func IsFiniteAutomaton(a *Automaton) bool {
	if a.NumStates() == 0 {
		return true
	}
	return isFiniteRecursive(
		NewTransition(), a, 0,
		make([]bool, a.NumStates()),
		make([]bool, a.NumStates()),
		0,
	)
}

// isFiniteRecursive checks whether there is a loop containing state.
// (This is sufficient since there are never transitions to dead states.)
// Port of AutomatonTestUtil.isFinite (private).
func isFiniteRecursive(
	scratch *Transition,
	a *Automaton,
	state int,
	path []bool,
	visited []bool,
	level int,
) bool {
	if level > MaxRecursionLevel {
		panic(fmt.Sprintf("input automaton is too large: %d", level))
	}
	path[state] = true
	numTransitions := a.InitTransition(state, scratch)
	for t := 0; t < numTransitions; t++ {
		a.GetTransition(state, t, scratch)
		if path[scratch.Dest] ||
			(!visited[scratch.Dest] &&
				!isFiniteRecursive(scratch, a, scratch.Dest, path, visited, level+1)) {
			return false
		}
	}
	path[state] = false
	visited[state] = true
	return true
}

// ---- Detached states check ---------------------------------------------

// AssertNoDetachedStates checks that an automaton has no detached states
// that are unreachable from the initial state.
// Port of AutomatonTestUtil.assertNoDetachedStates.
func AssertNoDetachedStates(t *testing.T, a *Automaton) {
	t.Helper()
	a2 := RemoveDeadStates(a)
	if a.NumStates() != a2.NumStates() {
		t.Errorf("automaton has %d detached states", a.NumStates()-a2.NumStates())
	}
}

// ---- Slow deterministic check ------------------------------------------

// IsDeterministicSlow returns true if the automaton is deterministic.
// This is a slow but obviously-correct implementation used to cross-check
// the fast implementation.
// Port of AutomatonTestUtil.isDeterministicSlow.
func IsDeterministicSlow(a *Automaton) bool {
	t := NewTransition()
	numStates := a.NumStates()
	for s := 0; s < numStates; s++ {
		count := a.InitTransition(s, t)
		lastMax := -1
		for i := 0; i < count; i++ {
			a.GetNextTransition(t)
			if t.Min <= lastMax {
				// Sanity check: the fast path must agree
				if a.IsDeterministic() {
					panic("isDeterministicSlow disagrees with IsDeterministic: fast says true, slow says false")
				}
				return false
			}
			lastMax = t.Max
		}
	}

	if !a.IsDeterministic() {
		panic("isDeterministicSlow disagrees with IsDeterministic: fast says false, slow says true")
	}
	return true
}

// ---- Language equivalence (independent of Operations.SameLanguage) ------

// SameLanguageReference returns true if these two automata accept exactly
// the same language. This is a costly computation! Both automata must be
// determinized and have no dead states!
// Port of AutomatonTestUtil.sameLanguage.
func SameLanguageReference(a1, a2 *Automaton) bool {
	if a1 == a2 {
		return true
	}
	return SubsetOfReference(a2, a1) && SubsetOfReference(a1, a2)
}

// SubsetOfReference returns true if the language of a1 is a subset of the
// language of a2. Both automata must be determinized and must have no
// dead states. Complexity: quadratic in number of states.
// This is the reference implementation from AutomatonTestUtil (independent
// of Operations.SubsetOf).
// Port of AutomatonTestUtil.subsetOf.
func SubsetOfReference(a1, a2 *Automaton) bool {
	if !a1.IsDeterministic() {
		panic("a1 must be deterministic")
	}
	if !a2.IsDeterministic() {
		panic("a2 must be deterministic")
	}
	if HasDeadStatesFromInitial(a1) {
		panic("a1 has dead states from initial")
	}
	if HasDeadStatesFromInitial(a2) {
		panic("a2 has dead states from initial")
	}
	if a1.NumStates() == 0 {
		// Empty language is always a subset of any other language
		return true
	}
	if a2.NumStates() == 0 {
		return IsEmpty(a1)
	}

	// Build sorted transition slices per state (matching Java's getSortedTransitions())
	t1 := buildSortedTransitions(a1)
	t2 := buildSortedTransitions(a2)

	type statePair struct{ s1, s2 int }
	worklist := make([]statePair, 0)
	visited := make(map[statePair]bool)
	p := statePair{0, 0}
	worklist = append(worklist, p)
	visited[p] = true

	for len(worklist) > 0 {
		p = worklist[0]
		worklist = worklist[1:]
		if a1.IsAccept(p.s1) && !a2.IsAccept(p.s2) {
			return false
		}
		tr1 := t1[p.s1]
		tr2 := t2[p.s2]
		for n1, b2 := 0, 0; n1 < len(tr1); n1++ {
			for b2 < len(tr2) && tr2[b2].Max < tr1[n1].Min {
				b2++
			}
			min1 := tr1[n1].Min
			max1 := tr1[n1].Max

			for n2 := b2; n2 < len(tr2) && tr1[n1].Max >= tr2[n2].Min; n2++ {
				if tr2[n2].Min > min1 {
					return false
				}
				if tr2[n2].Max < MaxCodePoint {
					min1 = tr2[n2].Max + 1
				} else {
					min1 = MaxCodePoint
					max1 = MinCodePoint
				}
				q := statePair{tr1[n1].Dest, tr2[n2].Dest}
				if !visited[q] {
					worklist = append(worklist, q)
					visited[q] = true
				}
			}
			if min1 <= max1 {
				return false
			}
		}
	}
	return true
}

// buildSortedTransitions returns transitions grouped by state, sorted by
// (min, max). Each state's transitions are obtained via InitTransition/
// GetNextTransition which already return them sorted.
func buildSortedTransitions(a *Automaton) [][]*Transition {
	numStates := a.NumStates()
	result := make([][]*Transition, numStates)
	t := NewTransition()
	for s := 0; s < numStates; s++ {
		count := a.InitTransition(s, t)
		result[s] = make([]*Transition, count)
		for i := 0; i < count; i++ {
			a.GetNextTransition(t)
			cpy := *t
			result[s][i] = &cpy
		}
	}
	return result
}
