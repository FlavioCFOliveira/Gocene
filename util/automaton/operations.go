// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Port of org.apache.lucene.util.automaton.Operations from Apache Lucene
// 10.4.0 (Apache License 2.0, derived from dk.brics.automaton).

package automaton

import (
	"errors"
	"fmt"
	"sort"
)

// DefaultDeterminizeWorkLimit matches Lucene's default work limit for
// powerset construction in Operations.determinize.
const DefaultDeterminizeWorkLimit = 10000

// ErrTooComplexToDeterminize signals that determinization exceeded the
// configured effort budget.
var ErrTooComplexToDeterminize = errors.New("automaton: too complex to determinize")

// Determinize converts a possibly non-deterministic automaton into an
// equivalent deterministic one using powerset construction. workLimit caps
// the cumulative effort spent; pass DefaultDeterminizeWorkLimit when in doubt.
func Determinize(a *Automaton, workLimit int) (*Automaton, error) {
	if a.IsDeterministic() {
		return a, nil
	}
	if a.NumStates() <= 1 {
		return a, nil
	}

	b := NewBuilder()
	b.CreateState()
	b.SetAccept(0, a.IsAccept(0))

	type frozenSet struct {
		values   []int
		hash     uint64
		newState int
	}

	keyFor := func(values []int) string {
		// values is already sorted unique
		var sb [4096]byte
		buf := sb[:0]
		for i, v := range values {
			if i > 0 {
				buf = append(buf, ',')
			}
			buf = append(buf, []byte(fmt.Sprintf("%d", v))...)
		}
		return string(buf)
	}

	initial := []int{0}
	initialKey := keyFor(initial)
	newstate := map[string]int{initialKey: 0}

	worklist := []frozenSet{{values: initial, newState: 0}}

	points := newPointTransitionSet()
	statesSet := newRefCountSet()

	t := NewTransition()
	var effortSpent int64
	effortLimit := int64(workLimit) * 10

	for len(worklist) > 0 {
		s := worklist[0]
		worklist = worklist[1:]

		effortSpent += int64(len(s.values))
		if effortSpent >= effortLimit {
			return nil, fmt.Errorf("%w: states=%d transitions=%d limit=%d", ErrTooComplexToDeterminize,
				a.NumStates(), a.NumTransitions(), workLimit)
		}

		for _, s0 := range s.values {
			count := a.InitTransition(s0, t)
			for i := 0; i < count; i++ {
				a.GetNextTransition(t)
				points.add(t)
			}
		}
		if points.count == 0 {
			continue
		}

		points.sort()

		lastPoint := -1
		accCount := 0
		r := s.newState

		for i := 0; i < points.count; i++ {
			point := points.points[i].point

			if statesSet.size() > 0 {
				if lastPoint == -1 {
					panic("automaton: determinize invariant violated (lastPoint == -1)")
				}
				values := statesSet.values()
				key := keyFor(values)
				q, ok := newstate[key]
				if !ok {
					q = b.CreateState()
					worklist = append(worklist, frozenSet{values: append([]int(nil), values...), newState: q})
					b.SetAccept(q, accCount > 0)
					newstate[key] = q
				}
				b.AddTransition(r, q, lastPoint, point-1)
			}

			// Process transitions ending at this point.
			ends := points.points[i].ends
			for j := 0; j < ends.next; j += 3 {
				dest := ends.data[j]
				statesSet.decr(dest)
				if a.IsAccept(dest) {
					accCount--
				}
			}
			ends.next = 0

			// Process transitions starting at this point.
			starts := points.points[i].starts
			for j := 0; j < starts.next; j += 3 {
				dest := starts.data[j]
				statesSet.incr(dest)
				if a.IsAccept(dest) {
					accCount++
				}
			}
			lastPoint = point
			starts.next = 0
		}
		points.reset()
		if statesSet.size() != 0 {
			panic("automaton: determinize invariant violated (statesSet not empty)")
		}
	}

	result := b.Finish()
	return result, nil
}

// IsEmpty returns true when the automaton accepts no strings.
func IsEmpty(a *Automaton) bool {
	if a.NumStates() == 0 {
		return true
	}
	if !a.IsAccept(0) && a.GetNumTransitions(0) == 0 {
		return true
	}
	if a.IsAccept(0) {
		return false
	}
	// BFS from initial state.
	worklist := []int{0}
	seen := make([]bool, a.NumStates())
	seen[0] = true
	t := NewTransition()
	for len(worklist) > 0 {
		s := worklist[0]
		worklist = worklist[1:]
		if a.IsAccept(s) {
			return false
		}
		n := a.InitTransition(s, t)
		for i := 0; i < n; i++ {
			a.GetNextTransition(t)
			if !seen[t.Dest] {
				seen[t.Dest] = true
				worklist = append(worklist, t.Dest)
			}
		}
	}
	return true
}

// IsTotal reports whether the deterministic automaton accepts every string
// composed of code points in [MinCodePoint, MaxCodePoint].
func IsTotal(a *Automaton) bool {
	return IsTotalRange(a, MinCodePoint, MaxCodePoint)
}

// IsTotalRange reports whether the deterministic automaton accepts every
// string composed of code points in [minAlphabet, maxAlphabet].
func IsTotalRange(a *Automaton, minAlphabet, maxAlphabet int) bool {
	live := getLiveStates(a)
	t := NewTransition()
	seenStates := 0
	for state := 0; state < len(live); state++ {
		if !live[state] {
			continue
		}
		if !a.IsAccept(state) {
			return false
		}
		previousLabel := minAlphabet - 1
		count := a.GetNumTransitions(state)
		for i := 0; i < count; i++ {
			a.GetTransition(state, i, t)
			if t.Min > previousLabel+1 {
				return false
			}
			previousLabel = t.Max
		}
		if previousLabel < maxAlphabet {
			return false
		}
		seenStates++
	}
	return seenStates > 0
}

// Run returns true when the deterministic automaton accepts the Unicode
// string s. For full performance, use a RunAutomaton.
func Run(a *Automaton, s string) bool {
	state := 0
	for _, r := range s {
		next := a.Step(state, int(r))
		if next == -1 {
			return false
		}
		state = next
	}
	return a.IsAccept(state)
}

// RemoveDeadStates removes states that are unreachable from the initial
// state or that cannot reach any accept state.
func RemoveDeadStates(a *Automaton) *Automaton {
	numStates := a.NumStates()
	live := getLiveStates(a)
	count := 0
	for _, v := range live {
		if v {
			count++
		}
	}
	if count == numStates {
		return a
	}
	stateMap := make([]int, numStates)
	for i := range stateMap {
		stateMap[i] = -1
	}
	result := NewAutomaton()
	for i := 0; i < numStates; i++ {
		if live[i] {
			stateMap[i] = result.CreateState()
			result.SetAccept(stateMap[i], a.IsAccept(i))
		}
	}
	t := NewTransition()
	for i := 0; i < numStates; i++ {
		if !live[i] {
			continue
		}
		n := a.InitTransition(i, t)
		for k := 0; k < n; k++ {
			a.GetNextTransition(t)
			if live[t.Dest] {
				result.AddTransition(stateMap[i], stateMap[t.Dest], t.Min, t.Max)
			}
		}
	}
	result.FinishState()
	return result
}

// Reverse returns an automaton accepting the reverse language.
func Reverse(a *Automaton) *Automaton {
	if IsEmpty(a) {
		return NewAutomaton()
	}
	numStates := a.NumStates()
	b := NewBuilder()
	// Reserve state 0 as the new initial; offset old states by +1.
	b.CreateState()
	for s := 0; s < numStates; s++ {
		b.CreateState()
	}
	b.SetAccept(1, true)
	t := NewTransition()
	for s := 0; s < numStates; s++ {
		n := a.InitTransition(s, t)
		for i := 0; i < n; i++ {
			a.GetNextTransition(t)
			b.AddTransition(t.Dest+1, s+1, t.Min, t.Max)
		}
	}
	result := b.Finish()
	for s := 0; s < numStates; s++ {
		if a.IsAccept(s) {
			result.AddEpsilon(0, s+1)
		}
	}
	result.FinishState()
	return RemoveDeadStates(result)
}

// Union returns an automaton accepting the union of the input languages.
func Union(list []*Automaton) *Automaton {
	result := NewAutomaton()
	result.CreateState()
	for _, a := range list {
		result.Copy(a)
	}
	stateOffset := 1
	for _, a := range list {
		if a.NumStates() == 0 {
			continue
		}
		result.AddEpsilon(0, stateOffset)
		stateOffset += a.NumStates()
	}
	result.FinishState()
	return MergeAcceptStatesWithNoTransition(RemoveDeadStates(result))
}

// Concatenate returns an automaton that accepts the concatenation of the
// input languages, in order.
func Concatenate(list []*Automaton) *Automaton {
	result := NewAutomaton()
	for _, a := range list {
		if a.NumStates() == 0 {
			return MakeEmpty()
		}
		for s := 0; s < a.NumStates(); s++ {
			result.CreateState()
		}
	}
	stateOffset := 0
	t := NewTransition()
	for i, a := range list {
		numStates := a.NumStates()
		var nextA *Automaton
		if i != len(list)-1 {
			nextA = list[i+1]
		}
		for s := 0; s < numStates; s++ {
			n := a.InitTransition(s, t)
			for j := 0; j < n; j++ {
				a.GetNextTransition(t)
				result.AddTransition(stateOffset+s, stateOffset+t.Dest, t.Min, t.Max)
			}
			if a.IsAccept(s) {
				followA := nextA
				followOffset := stateOffset
				upto := i + 1
				for {
					if followA != nil {
						n2 := followA.InitTransition(0, t)
						for j := 0; j < n2; j++ {
							followA.GetNextTransition(t)
							result.AddTransition(stateOffset+s, followOffset+numStates+t.Dest, t.Min, t.Max)
						}
						if followA.IsAccept(0) {
							followOffset += followA.NumStates()
							if upto == len(list)-1 {
								followA = nil
							} else {
								followA = list[upto+1]
								upto++
							}
						} else {
							break
						}
					} else {
						result.SetAccept(stateOffset+s, true)
						break
					}
				}
			}
		}
		stateOffset += numStates
	}
	if result.NumStates() == 0 {
		result.CreateState()
	}
	result.FinishState()
	return RemoveDeadStates(result)
}

// Optional returns an automaton accepting the empty string plus the input language.
func Optional(a *Automaton) *Automaton {
	if a.IsAccept(0) {
		return a
	}
	hasIncomingToInitial := false
	t := NewTransition()
	for state := 0; state < a.NumStates() && !hasIncomingToInitial; state++ {
		n := a.InitTransition(state, t)
		for i := 0; i < n; i++ {
			a.GetNextTransition(t)
			if t.Dest == 0 {
				hasIncomingToInitial = true
				break
			}
		}
	}
	if !hasIncomingToInitial {
		result := NewAutomaton()
		result.Copy(a)
		if result.NumStates() == 0 {
			result.CreateState()
		}
		result.SetAccept(0, true)
		return result
	}
	result := NewAutomaton()
	result.CreateState()
	result.SetAccept(0, true)
	if a.NumStates() > 0 {
		result.Copy(a)
		result.AddEpsilon(0, 1)
	}
	result.FinishState()
	return result
}

// Repeat returns the Kleene star (zero or more) of a.
func Repeat(a *Automaton) *Automaton {
	if a.NumStates() == 0 {
		return a
	}
	if a.IsAccept(0) && a.AcceptCardinality() == 1 {
		return a
	}
	b := NewBuilder()
	b.CreateState()
	b.SetAccept(0, true)
	stateMap := make([]int, a.NumStates())
	for s := 0; s < a.NumStates(); s++ {
		if !a.IsAccept(s) {
			stateMap[s] = b.CreateState()
		} else if a.GetNumTransitions(s) == 0 {
			stateMap[s] = 0
		} else {
			ns := b.CreateState()
			stateMap[s] = ns
			b.SetAccept(ns, true)
		}
	}
	t := NewTransition()
	for s := 0; s < a.NumStates(); s++ {
		src := stateMap[s]
		n := a.InitTransition(s, t)
		for i := 0; i < n; i++ {
			a.GetNextTransition(t)
			b.AddTransition(src, stateMap[t.Dest], t.Min, t.Max)
		}
	}
	// Copy transitions of the initial state to our new initial state.
	n := a.InitTransition(0, t)
	for i := 0; i < n; i++ {
		a.GetNextTransition(t)
		b.AddTransition(0, stateMap[t.Dest], t.Min, t.Max)
	}
	// Loop accept-state transitions back via the start state.
	for s := 0; s < a.NumStates(); s++ {
		if a.IsAccept(s) && stateMap[s] != 0 {
			nn := a.InitTransition(0, t)
			for i := 0; i < nn; i++ {
				a.GetNextTransition(t)
				b.AddTransition(stateMap[s], stateMap[t.Dest], t.Min, t.Max)
			}
		}
	}
	return RemoveDeadStates(b.Finish())
}

// RepeatN returns an automaton accepting >= n concatenated repetitions of a.
func RepeatN(a *Automaton, n int) *Automaton {
	if n == 0 {
		return Repeat(a)
	}
	list := make([]*Automaton, 0, n+1)
	for ; n > 0; n-- {
		list = append(list, a)
	}
	list = append(list, Repeat(a))
	return Concatenate(list)
}

// RepeatRange returns an automaton accepting between min and max repetitions of a.
func RepeatRange(a *Automaton, min, max int) *Automaton {
	if min > max {
		return MakeEmpty()
	}
	var head *Automaton
	switch {
	case min == 0:
		head = MakeEmptyString()
	case min == 1:
		head = NewAutomaton()
		head.Copy(a)
	default:
		list := make([]*Automaton, 0, min)
		for i := 0; i < min; i++ {
			list = append(list, a)
		}
		head = Concatenate(list)
	}
	prevAccept := acceptSet(head, 0)
	b := NewBuilder()
	b.Copy(head)
	for i := min; i < max; i++ {
		numStates := b.GetNumStates()
		b.Copy(a)
		for _, s := range prevAccept {
			b.AddEpsilon(s, numStates)
		}
		prevAccept = acceptSet(a, numStates)
	}
	return RemoveDeadStates(b.Finish())
}

func acceptSet(a *Automaton, offset int) []int {
	out := []int{}
	for s := 0; s < a.NumStates(); s++ {
		if a.IsAccept(s) {
			out = append(out, offset+s)
		}
	}
	return out
}

// Complement returns an automaton accepting the complement of the input.
func Complement(a *Automaton, workLimit int) (*Automaton, error) {
	det, err := Determinize(a, workLimit)
	if err != nil {
		return nil, err
	}
	tot := Totalize(det)
	for s := 0; s < tot.NumStates(); s++ {
		tot.SetAccept(s, !tot.IsAccept(s))
	}
	return RemoveDeadStates(tot), nil
}

// Minus returns an automaton accepting strings in a1 but not in a2.
func Minus(a1, a2 *Automaton, workLimit int) (*Automaton, error) {
	if IsEmpty(a1) || a1 == a2 {
		return MakeEmpty(), nil
	}
	if IsEmpty(a2) {
		return a1, nil
	}
	comp, err := Complement(a2, workLimit)
	if err != nil {
		return nil, err
	}
	return Intersection(a1, comp), nil
}

// Intersection returns an automaton accepting strings in both a1 and a2.
func Intersection(a1, a2 *Automaton) *Automaton {
	if a1 == a2 {
		return a1
	}
	if a1.NumStates() == 0 {
		return a1
	}
	if a2.NumStates() == 0 {
		return a2
	}
	// Snapshot transitions for both automatons to enable random access.
	type tr struct{ dest, min, max int }
	t := NewTransition()
	snap := func(a *Automaton) [][]tr {
		out := make([][]tr, a.NumStates())
		for s := 0; s < a.NumStates(); s++ {
			n := a.InitTransition(s, t)
			out[s] = make([]tr, n)
			for i := 0; i < n; i++ {
				a.GetNextTransition(t)
				out[s][i] = tr{t.Dest, t.Min, t.Max}
			}
		}
		return out
	}
	t1 := snap(a1)
	t2 := snap(a2)

	c := NewAutomaton()
	c.CreateState()
	type pair struct{ s1, s2 int }
	stateMap := map[pair]int{{0, 0}: 0}
	worklist := []pair{{0, 0}}

	for len(worklist) > 0 {
		p := worklist[0]
		worklist = worklist[1:]
		me := stateMap[p]
		c.SetAccept(me, a1.IsAccept(p.s1) && a2.IsAccept(p.s2))
		row1 := t1[p.s1]
		row2 := t2[p.s2]
		for n1 := 0; n1 < len(row1); n1++ {
			r1 := row1[n1]
			for n2 := 0; n2 < len(row2); n2++ {
				r2 := row2[n2]
				if r2.max < r1.min {
					continue
				}
				if r2.min > r1.max {
					break
				}
				min := r1.min
				if r2.min > min {
					min = r2.min
				}
				max := r1.max
				if r2.max < max {
					max = r2.max
				}
				newPair := pair{r1.dest, r2.dest}
				np, ok := stateMap[newPair]
				if !ok {
					np = c.CreateState()
					stateMap[newPair] = np
					worklist = append(worklist, newPair)
				}
				c.AddTransition(me, np, min, max)
			}
		}
	}
	c.FinishState()
	return RemoveDeadStates(c)
}

// Totalize returns an automaton accepting the same language with a sink state
// added so that every (state, label) has a defined transition.
func Totalize(a *Automaton) *Automaton {
	result := NewAutomaton()
	numStates := a.NumStates()
	for s := 0; s < numStates; s++ {
		result.CreateState()
		result.SetAccept(s, a.IsAccept(s))
	}
	dead := result.CreateState()
	result.AddTransition(dead, dead, MinCodePoint, MaxCodePoint)
	t := NewTransition()
	for s := 0; s < numStates; s++ {
		maxi := MinCodePoint
		n := a.InitTransition(s, t)
		for i := 0; i < n; i++ {
			a.GetNextTransition(t)
			result.AddTransition(s, t.Dest, t.Min, t.Max)
			if t.Min > maxi {
				result.AddTransition(s, dead, maxi, t.Min-1)
			}
			if t.Max+1 > maxi {
				maxi = t.Max + 1
			}
		}
		if maxi <= MaxCodePoint {
			result.AddTransition(s, dead, maxi, MaxCodePoint)
		}
	}
	result.FinishState()
	return result
}

// SameLanguage returns true when a1 and a2 accept the same language.
func SameLanguage(a1, a2 *Automaton, workLimit int) (bool, error) {
	d1, err := Minus(a1, a2, workLimit)
	if err != nil {
		return false, err
	}
	if !IsEmpty(d1) {
		return false, nil
	}
	d2, err := Minus(a2, a1, workLimit)
	if err != nil {
		return false, err
	}
	return IsEmpty(d2), nil
}

// SubsetOf returns true when L(a1) ⊆ L(a2).
func SubsetOf(a1, a2 *Automaton, workLimit int) (bool, error) {
	m, err := Minus(a1, a2, workLimit)
	if err != nil {
		return false, err
	}
	return IsEmpty(m), nil
}

// HasDeadStates reports whether some reachable state cannot reach an accept state
// or some state is unreachable.
func HasDeadStates(a *Automaton) bool {
	live := getLiveStates(a)
	num := 0
	for _, v := range live {
		if v {
			num++
		}
	}
	return num < a.NumStates()
}

// HasDeadStatesFromInitial reports whether there are reachable-from-initial
// states that cannot reach any accept state.
func HasDeadStatesFromInitial(a *Automaton) bool {
	fromInitial := liveStatesFromInitial(a)
	toAccept := liveStatesToAccept(a)
	for i := range fromInitial {
		if fromInitial[i] && !toAccept[i] {
			return true
		}
	}
	return false
}

// GetCommonPrefix returns the longest prefix shared by all accepted strings.
func GetCommonPrefix(a *Automaton) (string, error) {
	if HasDeadStatesFromInitial(a) {
		return "", fmt.Errorf("automaton: input has dead states")
	}
	if IsEmpty(a) {
		return "", nil
	}
	var sb []rune
	t := NewTransition()
	visited := make([]bool, a.NumStates())
	current := []bool{}
	if len(visited) > 0 {
		current = make([]bool, a.NumStates())
		current[0] = true
	}
	next := make([]bool, len(visited))
	for {
		label := -1
		stop := false
		for state := 0; state < len(current); state++ {
			if !current[state] {
				continue
			}
			visited[state] = true
			if a.IsAccept(state) {
				stop = true
				break
			}
			count := a.GetNumTransitions(state)
			for i := 0; i < count; i++ {
				a.GetTransition(state, i, t)
				if label == -1 {
					label = t.Min
				}
				if t.Min != t.Max || t.Min != label {
					stop = true
					break
				}
				next[t.Dest] = true
			}
			if stop {
				break
			}
		}
		if stop {
			break
		}
		if label == -1 {
			break
		}
		sb = append(sb, rune(label))
		// Swap current and next, clear next.
		current, next = next, current
		for i := range next {
			next[i] = false
		}
	}
	return string(sb), nil
}

// GetSingleton returns the single accepted code-point sequence if any, else nil.
// Requires the automaton to be deterministic.
func GetSingleton(a *Automaton) []int {
	if !a.IsDeterministic() {
		return nil
	}
	var out []int
	visited := make(map[int]bool)
	s := 0
	t := NewTransition()
	for {
		visited[s] = true
		accept := a.IsAccept(s)
		count := a.GetNumTransitions(s)
		if !accept {
			if count == 1 {
				a.GetTransition(s, 0, t)
				if t.Min == t.Max && !visited[t.Dest] {
					out = append(out, t.Min)
					s = t.Dest
					continue
				}
			}
			return nil
		}
		if count == 0 {
			return out
		}
		return nil
	}
}

// TopoSortStates returns reachable states in topological order or an error
// if the automaton has a cycle. Equivalent to Lucene's topoSortStates.
func TopoSortStates(a *Automaton) ([]int, error) {
	if a.NumStates() == 0 {
		return nil, nil
	}
	numStates := a.NumStates()
	onStack := make([]bool, numStates)
	visited := make([]bool, numStates)
	stack := []int{0}
	visited[0] = true
	out := make([]int, 0, numStates)
	t := NewTransition()
	for len(stack) > 0 {
		state := stack[len(stack)-1]
		pushed := false
		count := a.InitTransition(state, t)
		for i := 0; i < count; i++ {
			a.GetNextTransition(t)
			if !visited[t.Dest] {
				visited[t.Dest] = true
				stack = append(stack, t.Dest)
				onStack[state] = true
				pushed = true
				break
			}
			if onStack[t.Dest] {
				return nil, errors.New("automaton: input has a cycle")
			}
		}
		if !pushed {
			onStack[state] = false
			stack = stack[:len(stack)-1]
			out = append(out, state)
		}
	}
	// Reverse for post-order topo sort.
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}

// MergeAcceptStatesWithNoTransition merges accept states that have no outgoing
// transitions into a single shared state. Reduces output of operations like
// concatenation that produce many equivalent accept states.
func MergeAcceptStatesWithNoTransition(a *Automaton) *Automaton {
	numStates := a.NumStates()
	mergeable := make([]int, 0)
	for i := 0; i < numStates; i++ {
		if a.IsAccept(i) && a.GetNumTransitions(i) == 0 {
			mergeable = append(mergeable, i)
		}
	}
	if len(mergeable) <= 1 {
		return a
	}
	// mergeable is already in ascending order.
	remap := func(s int) int {
		idx := sort.SearchInts(mergeable, s)
		if idx < len(mergeable) && mergeable[idx] == s {
			return mergeable[0]
		}
		if idx <= 1 {
			return s
		}
		return s - (idx - 1)
	}

	result := NewAutomaton()
	for s := 0; s < numStates; s++ {
		newS := remap(s)
		for result.NumStates() <= newS {
			result.CreateState()
		}
		if a.IsAccept(s) {
			result.SetAccept(newS, true)
		}
	}
	t := NewTransition()
	for s := 0; s < numStates; s++ {
		rs := remap(s)
		n := a.InitTransition(s, t)
		for i := 0; i < n; i++ {
			a.GetNextTransition(t)
			result.AddTransition(rs, remap(t.Dest), t.Min, t.Max)
		}
	}
	result.FinishState()
	return result
}

// --- live state computation ---

func liveStatesFromInitial(a *Automaton) []bool {
	live := make([]bool, a.NumStates())
	if a.NumStates() == 0 {
		return live
	}
	live[0] = true
	worklist := []int{0}
	t := NewTransition()
	for len(worklist) > 0 {
		s := worklist[0]
		worklist = worklist[1:]
		n := a.InitTransition(s, t)
		for i := 0; i < n; i++ {
			a.GetNextTransition(t)
			if !live[t.Dest] {
				live[t.Dest] = true
				worklist = append(worklist, t.Dest)
			}
		}
	}
	return live
}

func liveStatesToAccept(a *Automaton) []bool {
	// Build reversed adjacency once.
	numStates := a.NumStates()
	type edge struct{ src int }
	rev := make([][]edge, numStates)
	t := NewTransition()
	for s := 0; s < numStates; s++ {
		n := a.InitTransition(s, t)
		for i := 0; i < n; i++ {
			a.GetNextTransition(t)
			rev[t.Dest] = append(rev[t.Dest], edge{src: s})
		}
	}
	live := make([]bool, numStates)
	worklist := make([]int, 0)
	for s := 0; s < numStates; s++ {
		if a.IsAccept(s) {
			live[s] = true
			worklist = append(worklist, s)
		}
	}
	for len(worklist) > 0 {
		s := worklist[0]
		worklist = worklist[1:]
		for _, e := range rev[s] {
			if !live[e.src] {
				live[e.src] = true
				worklist = append(worklist, e.src)
			}
		}
	}
	return live
}

func getLiveStates(a *Automaton) []bool {
	from := liveStatesFromInitial(a)
	to := liveStatesToAccept(a)
	for i := range from {
		from[i] = from[i] && to[i]
	}
	return from
}

// --- internal data structures supporting determinization ---

type transitionList struct {
	data []int // packed (dest, min, max) triples
	next int
}

func (tl *transitionList) add(t *Transition) {
	if cap(tl.data) < tl.next+3 {
		ngrow := cap(tl.data)*2 + 3
		if ngrow < tl.next+3 {
			ngrow = tl.next + 3
		}
		gd := make([]int, len(tl.data), ngrow)
		copy(gd, tl.data)
		tl.data = gd
	}
	tl.data = append(tl.data, t.Dest, t.Min, t.Max)
	tl.next += 3
}

type pointTransitions struct {
	point  int
	ends   *transitionList
	starts *transitionList
}

func newPointTransitions(point int) *pointTransitions {
	return &pointTransitions{
		point:  point,
		ends:   &transitionList{},
		starts: &transitionList{},
	}
}

func (pt *pointTransitions) reset(point int) {
	pt.point = point
	pt.ends.next = 0
	pt.ends.data = pt.ends.data[:0]
	pt.starts.next = 0
	pt.starts.data = pt.starts.data[:0]
}

type pointTransitionSet struct {
	count    int
	points   []*pointTransitions
	useHash  bool
	pointMap map[int]*pointTransitions
}

const pointTransitionHashCutover = 30

func newPointTransitionSet() *pointTransitionSet {
	return &pointTransitionSet{
		points:   make([]*pointTransitions, 0, 8),
		pointMap: make(map[int]*pointTransitions),
	}
}

func (s *pointTransitionSet) next(point int) *pointTransitions {
	if s.count == len(s.points) {
		s.points = append(s.points, newPointTransitions(point))
		s.count++
		return s.points[s.count-1]
	}
	pt := s.points[s.count]
	if pt == nil {
		pt = newPointTransitions(point)
		s.points[s.count] = pt
	} else {
		pt.reset(point)
	}
	s.count++
	return pt
}

func (s *pointTransitionSet) find(point int) *pointTransitions {
	if s.useHash {
		if pt, ok := s.pointMap[point]; ok {
			return pt
		}
		pt := s.next(point)
		s.pointMap[point] = pt
		return pt
	}
	for i := 0; i < s.count; i++ {
		if s.points[i].point == point {
			return s.points[i]
		}
	}
	pt := s.next(point)
	if s.count == pointTransitionHashCutover {
		for i := 0; i < s.count; i++ {
			s.pointMap[s.points[i].point] = s.points[i]
		}
		s.useHash = true
	}
	return pt
}

func (s *pointTransitionSet) add(t *Transition) {
	s.find(t.Min).starts.add(t)
	s.find(t.Max + 1).ends.add(t)
}

func (s *pointTransitionSet) sort() {
	sort.Slice(s.points[:s.count], func(i, j int) bool {
		return s.points[i].point < s.points[j].point
	})
}

func (s *pointTransitionSet) reset() {
	if s.useHash {
		for k := range s.pointMap {
			delete(s.pointMap, k)
		}
		s.useHash = false
	}
	s.count = 0
}

// refCountSet is a small reference-counted set of state ids.
type refCountSet struct {
	counts map[int]int
	cache  []int
	dirty  bool
}

func newRefCountSet() *refCountSet {
	return &refCountSet{counts: make(map[int]int)}
}

func (r *refCountSet) incr(state int) {
	r.counts[state]++
	r.dirty = true
}

func (r *refCountSet) decr(state int) {
	r.counts[state]--
	if r.counts[state] == 0 {
		delete(r.counts, state)
		r.dirty = true
	}
}

func (r *refCountSet) size() int { return len(r.counts) }

func (r *refCountSet) values() []int {
	if !r.dirty {
		return r.cache
	}
	out := make([]int, 0, len(r.counts))
	for k := range r.counts {
		out = append(out, k)
	}
	sort.Ints(out)
	r.cache = out
	r.dirty = false
	return out
}

func (r *refCountSet) reset() {
	for k := range r.counts {
		delete(r.counts, k)
	}
	r.cache = nil
	r.dirty = false
}
