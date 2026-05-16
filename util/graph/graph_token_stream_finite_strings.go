// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Port of org.apache.lucene.util.graph.GraphTokenStreamFiniteStrings from
// Apache Lucene 10.4.0 (Apache License 2.0).

// Package graph provides utilities for working with graph token streams.
//
// The single exported type, GraphTokenStreamFiniteStrings, consumes a
// TokenStream that exposes PositionIncrementAttribute and
// PositionLengthAttribute and builds an Automaton whose transition labels are
// token ids. Helpers then expose:
//
//   - GetFiniteStrings: iterator over every accepted path, each rendered as a
//     linear TokenStream.
//   - GetTerms: the [index.Term] values that leave a given automaton state.
//   - HasSidePath: whether a state branches into paths of differing lengths.
//   - ArticulationPoints: the cut vertices of the underlying graph.
//
// This is the Go port of Apache Lucene 10.4.0's
// org.apache.lucene.util.graph package.
package graph

import (
	"errors"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util/automaton"
)

// maxRecursionLevel mirrors Lucene's GraphTokenStreamFiniteStrings
// MAX_RECURSION_LEVEL (1000): the maximum DFS depth allowed by
// articulationPoints before the helper aborts with an error.
const maxRecursionLevel = 1000

// ErrMalformedStartToken is returned when the very first token of the stream
// has a position increment lower than 1. Mirrors Lucene's
// IllegalStateException("Malformed TokenStream, ...").
var ErrMalformedStartToken = errors.New(
	"graph: malformed TokenStream, start token can't have increment less than 1")

// ErrRecursionLimitExceeded is returned by ArticulationPoints when the DFS
// would exceed maxRecursionLevel. Mirrors Lucene's IllegalArgumentException.
var ErrRecursionLimitExceeded = errors.New(
	"graph: exceeded maximum recursion level during graph analysis")

// tokenSnapshot freezes the per-token attribute values we need to replay a
// path later. We carry only the three attributes exercised by Lucene's
// reference tests (term, position increment, position length); a full
// AttributeSource clone would require the broader analysis.tokenattributes
// surface, which belongs to a later sprint.
type tokenSnapshot struct {
	term    []byte
	posIncr int
	posLen  int
}

// GraphTokenStreamFiniteStrings builds an Automaton from a graph TokenStream
// and exposes helpers to walk the resulting paths.
//
// Construction consumes the input stream eagerly (reset, increment*, end).
// After NewGraphTokenStreamFiniteStrings returns the wrapped stream is no
// longer used; callers retain ownership of its lifecycle (e.g. Close()).
type GraphTokenStreamFiniteStrings struct {
	// tokens is the per-id token snapshot, parallel to Java's
	// AttributeSource[] tokens field.
	tokens []*tokenSnapshot

	// det is the determinized + dead-state-removed automaton built from the
	// position graph; its transition labels are token ids.
	det *automaton.Automaton

	// transition is a reusable Transition cursor shared by the read helpers.
	transition *automaton.Transition
}

// NewGraphTokenStreamFiniteStrings consumes in and returns the helper. The
// stream must produce its tokens through PositionIncrementAttribute and
// PositionLengthAttribute as in Apache Lucene 10.4.0.
//
// Returns ErrMalformedStartToken if the first token's position increment is
// less than 1. Any other error from the stream is wrapped.
func NewGraphTokenStreamFiniteStrings(in analysis.TokenStream) (*GraphTokenStreamFiniteStrings, error) {
	g := &GraphTokenStreamFiniteStrings{
		transition: automaton.NewTransition(),
	}
	aut, err := g.build(in)
	if err != nil {
		return nil, err
	}
	det, err := automaton.Determinize(aut, automaton.DefaultDeterminizeWorkLimit)
	if err != nil {
		return nil, fmt.Errorf("graph: determinizing token graph: %w", err)
	}
	g.det = automaton.RemoveDeadStates(det)
	return g, nil
}

// HasSidePath reports whether state begins multiple paths of different length
// (eg "new york" vs "ny").
func (g *GraphTokenStreamFiniteStrings) HasSidePath(state int) bool {
	numT := g.det.InitTransition(state, g.transition)
	if numT <= 1 {
		return false
	}
	g.det.GetNextTransition(g.transition)
	dest := g.transition.Dest
	for i := 1; i < numT; i++ {
		g.det.GetNextTransition(g.transition)
		if dest != g.transition.Dest {
			return true
		}
	}
	return false
}

// GetTokens returns the tokens whose transitions leave state.
// Each token is represented by its snapshot (term + position metadata).
func (g *GraphTokenStreamFiniteStrings) GetTokens(state int) []*tokenSnapshot {
	numT := g.det.InitTransition(state, g.transition)
	out := make([]*tokenSnapshot, 0, numT)
	for i := 0; i < numT; i++ {
		g.det.GetNextTransition(g.transition)
		for id := g.transition.Min; id <= g.transition.Max; id++ {
			out = append(out, g.tokens[id])
		}
	}
	return out
}

// GetTerms returns the *index.Term values leaving state for the given field.
// The order matches the transitions order (ascending token id).
func (g *GraphTokenStreamFiniteStrings) GetTerms(field string, state int) []*index.Term {
	toks := g.GetTokens(state)
	out := make([]*index.Term, 0, len(toks))
	for _, t := range toks {
		out = append(out, index.NewTermFromBytes(field, t.term))
	}
	return out
}

// GetFiniteStrings returns an iterator over all paths from the initial state
// to any accept state.
func (g *GraphTokenStreamFiniteStrings) GetFiniteStrings() *FiniteStringsTokenStreamIterator {
	return g.GetFiniteStringsRange(0, -1)
}

// GetFiniteStringsRange returns an iterator over all paths from startState to
// endState (or to any accept state when endState == -1).
func (g *GraphTokenStreamFiniteStrings) GetFiniteStringsRange(startState, endState int) *FiniteStringsTokenStreamIterator {
	return &FiniteStringsTokenStreamIterator{
		parent: g,
		it:     automaton.NewFiniteStringsIteratorRange(g.det, startState, endState),
	}
}

// ArticulationPoints returns the cut vertices of the underlying graph (a state
// whose removal increases the number of connected components).
// Reference: https://en.wikipedia.org/wiki/Biconnected_component
func (g *GraphTokenStreamFiniteStrings) ArticulationPoints() ([]int, error) {
	numStates := g.det.NumStates()
	if numStates == 0 {
		return []int{}, nil
	}
	// Build an undirected copy of g.det.
	undirect := automaton.NewBuilder()
	undirect.Copy(g.det)
	for i := 0; i < numStates; i++ {
		numT := g.det.InitTransition(i, g.transition)
		for j := 0; j < numT; j++ {
			g.det.GetNextTransition(g.transition)
			undirect.AddTransitionSingle(g.transition.Dest, i, g.transition.Min)
		}
	}
	undirected := undirect.Finish()

	visited := make([]bool, numStates)
	depth := make([]int, numStates)
	low := make([]int, numStates)
	parent := make([]int, numStates)
	for i := range parent {
		parent[i] = -1
	}
	var points []int
	if err := articulationPointsRecurse(undirected, 0, 0, depth, low, parent, visited, &points); err != nil {
		return nil, err
	}
	// Lucene returns points in DFS-postorder reversed; reverse here too.
	for i, j := 0, len(points)-1; i < j; i, j = i+1, j-1 {
		points[i], points[j] = points[j], points[i]
	}
	return points, nil
}

// build is the port of Lucene's private build(TokenStream) -> Automaton.
func (g *GraphTokenStreamFiniteStrings) build(in analysis.TokenStream) (*automaton.Automaton, error) {
	src := in.(interface {
		GetAttribute(string) analysis.AttributeImpl
	})

	posIncRaw := src.GetAttribute("PositionIncrementAttribute")
	if posIncRaw == nil {
		return nil, errors.New("graph: input TokenStream lacks PositionIncrementAttribute")
	}
	posIncAtt, ok := posIncRaw.(analysis.PositionIncrementAttribute)
	if !ok {
		return nil, errors.New("graph: PositionIncrementAttribute has unexpected type")
	}
	posLenRaw := src.GetAttribute("PositionLengthAttribute")
	if posLenRaw == nil {
		return nil, errors.New("graph: input TokenStream lacks PositionLengthAttribute")
	}
	posLenAtt, ok := posLenRaw.(*analysis.PositionLengthAttribute)
	if !ok {
		return nil, errors.New("graph: PositionLengthAttribute has unexpected type")
	}
	termRaw := src.GetAttribute("CharTermAttribute")
	if termRaw == nil {
		return nil, errors.New("graph: input TokenStream lacks CharTermAttribute")
	}
	termAtt, ok := termRaw.(analysis.CharTermAttribute)
	if !ok {
		return nil, errors.New("graph: CharTermAttribute has unexpected type")
	}

	builder := automaton.NewBuilder()

	pos := -1
	prevIncr := 1
	state := -1
	id := -1
	gap := 0
	for {
		hasNext, err := in.IncrementToken()
		if err != nil {
			return nil, fmt.Errorf("graph: incrementing input stream: %w", err)
		}
		if !hasNext {
			break
		}
		currentIncr := posIncAtt.GetPositionIncrement()
		if pos == -1 && currentIncr < 1 {
			return nil, ErrMalformedStartToken
		}
		if currentIncr == 0 {
			if gap > 0 {
				pos -= gap
			}
		} else {
			pos++
			gap = currentIncr - 1
		}
		endPos := pos + posLenAtt.GetPositionLength() + gap
		for state < endPos {
			state = builder.CreateState()
		}

		id++
		// Capture the per-token snapshot (defensive copy of term bytes).
		snap := &tokenSnapshot{
			posIncr: currentIncr,
			posLen:  1, // linear output: always 1, see below
		}
		bytes := termAtt.Bytes()
		snap.term = make([]byte, len(bytes))
		copy(snap.term, bytes)
		// stacked-token rule (currentIncr == 0): emit previous increment
		if currentIncr == 0 {
			snap.posIncr = prevIncr
		}
		// Grow tokens slice if needed.
		for len(g.tokens) <= id {
			g.tokens = append(g.tokens, nil)
		}
		g.tokens[id] = snap

		builder.AddTransition(pos, endPos, id, id)
		pos += gap

		if currentIncr > 0 {
			prevIncr = currentIncr
		}
	}

	if err := in.End(); err != nil {
		return nil, fmt.Errorf("graph: ending input stream: %w", err)
	}
	if state != -1 {
		builder.SetAccept(state, true)
	}
	return builder.Finish(), nil
}

// articulationPointsRecurse mirrors Lucene's helper of the same name.
func articulationPointsRecurse(
	a *automaton.Automaton,
	state, d int,
	depth, low, parent []int,
	visited []bool,
	points *[]int,
) error {
	visited[state] = true
	depth[state] = d
	low[state] = d
	childCount := 0
	isArticulation := false
	t := automaton.NewTransition()
	numT := a.InitTransition(state, t)
	for i := 0; i < numT; i++ {
		a.GetNextTransition(t)
		dest := t.Dest
		if !visited[dest] {
			parent[dest] = state
			if d >= maxRecursionLevel {
				return ErrRecursionLimitExceeded
			}
			if err := articulationPointsRecurse(a, dest, d+1, depth, low, parent, visited, points); err != nil {
				return err
			}
			childCount++
			if low[dest] >= depth[state] {
				isArticulation = true
			}
			if low[dest] < low[state] {
				low[state] = low[dest]
			}
		} else if dest != parent[state] {
			if depth[dest] < low[state] {
				low[state] = depth[dest]
			}
		}
	}
	if (parent[state] != -1 && isArticulation) || (parent[state] == -1 && childCount > 1) {
		*points = append(*points, state)
	}
	return nil
}

// FiniteStringsTokenStreamIterator is the iterator returned by GetFiniteStrings
// and GetFiniteStringsRange. Calls to HasNext drive the underlying
// FiniteStringsIterator one step at a time; Next consumes the cached path.
type FiniteStringsTokenStreamIterator struct {
	parent   *GraphTokenStreamFiniteStrings
	it       *automaton.FiniteStringsIterator
	current  []int
	err      error
	finished bool
}

// HasNext returns true while another path is available. After it returns
// false, Err may carry the iterator error (e.g. cycles).
func (it *FiniteStringsTokenStreamIterator) HasNext() bool {
	if it.finished {
		return false
	}
	if it.current != nil {
		return true
	}
	ref, err := it.it.Next()
	if err != nil {
		it.err = err
		it.finished = true
		return false
	}
	if ref == nil {
		it.finished = true
		return false
	}
	cp := make([]int, ref.Length)
	copy(cp, ref.Ints[ref.Offset:ref.Offset+ref.Length])
	it.current = cp
	return true
}

// Next returns the next path as a TokenStream. Each call invalidates the
// previously returned stream's underlying ids slice. Panics if HasNext is
// false; callers should always probe with HasNext first.
func (it *FiniteStringsTokenStreamIterator) Next() analysis.TokenStream {
	if it.current == nil {
		if !it.HasNext() {
			panic("graph: Next called when no path is available")
		}
	}
	ids := it.current
	it.current = nil
	return newFiniteStringsTokenStream(it.parent.tokens, ids)
}

// Err returns the first error observed while iterating, or nil.
func (it *FiniteStringsTokenStreamIterator) Err() error { return it.err }

// finiteStringsTokenStream replays a single path as a linear TokenStream.
// PositionLength is always 1 because GetFiniteStrings yields linear paths.
type finiteStringsTokenStream struct {
	*analysis.BaseTokenStream
	tokens []*tokenSnapshot
	ids    []int
	offset int

	termAtt    analysis.CharTermAttribute
	posIncrAtt analysis.PositionIncrementAttribute
	posLenAtt  *analysis.PositionLengthAttribute
}

func newFiniteStringsTokenStream(tokens []*tokenSnapshot, ids []int) *finiteStringsTokenStream {
	s := &finiteStringsTokenStream{
		BaseTokenStream: analysis.NewBaseTokenStream(),
		tokens:          tokens,
		ids:             ids,
	}
	s.termAtt = analysis.NewCharTermAttribute()
	s.posIncrAtt = analysis.NewPositionIncrementAttribute()
	s.posLenAtt = analysis.NewPositionLengthAttribute()
	s.AddAttribute(s.termAtt)
	s.AddAttribute(s.posIncrAtt)
	s.AddAttribute(s.posLenAtt)
	return s
}

// IncrementToken advances to the next token on the path.
func (s *finiteStringsTokenStream) IncrementToken() (bool, error) {
	if s.offset >= len(s.ids) {
		return false, nil
	}
	s.ClearAttributes()
	t := s.tokens[s.ids[s.offset]]
	s.termAtt.SetValue(string(t.term))
	s.posIncrAtt.SetPositionIncrement(t.posIncr)
	s.posLenAtt.SetPositionLength(1)
	s.offset++
	return true, nil
}

// End is a no-op for this synthetic stream.
func (s *finiteStringsTokenStream) End() error { return nil }

// Close is a no-op for this synthetic stream.
func (s *finiteStringsTokenStream) Close() error { return nil }
