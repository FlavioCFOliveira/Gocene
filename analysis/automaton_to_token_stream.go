// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"errors"

	"github.com/FlavioCFOliveira/Gocene/util/automaton"
)

// AutomatonToTokenStream converts a finite DAG Automaton into a TokenStream.
//
// This is the Go port of Lucene's
// org.apache.lucene.analysis.AutomatonToTokenStream.toAutomaton.
//
// The conversion topologically sorts the automaton's states, grouping nodes
// at the same distance from the start into a single position node. Each
// transition between position nodes is emitted as a token whose
// CharTermAttribute holds the single transition label (the value of the
// transition's [min, max] range expanded one value at a time, matching the
// Java reference). PositionIncrementAttribute, PositionLengthAttribute and
// OffsetAttribute are set accordingly.
//
// The input automaton must be a finite DAG with no cycles and no incoming
// edges to its start state. ToTokenStream returns an error if those
// invariants are violated.
func AutomatonToTokenStreamConvert(a *automaton.Automaton) (TokenStream, error) {
	numStates := a.NumStates()
	if numStates == 0 {
		return newAutomatonTokenStream(nil), nil
	}

	// Compute the in-degree of every state.
	indegree := make([]int, numStates)
	for s := 0; s < numStates; s++ {
		for _, tr := range a.GetTransitions(s) {
			indegree[tr.Dest]++
		}
	}
	if indegree[0] != 0 {
		return nil, errors.New("automaton-to-token-stream: start node has incoming edges, creating cycle")
	}

	// BFS-topological sort, grouping nodes by graph distance from state 0.
	type remapNode struct {
		id  int
		pos int
	}
	positionNodes := make([][]int, 0)
	idToPos := make(map[int]int, numStates)
	queue := []remapNode{{id: 0, pos: 0}}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, tr := range a.GetTransitions(cur.id) {
			indegree[tr.Dest]--
			if indegree[tr.Dest] == 0 {
				queue = append(queue, remapNode{id: tr.Dest, pos: cur.pos + 1})
			}
		}
		if len(positionNodes) == cur.pos {
			positionNodes = append(positionNodes, []int{cur.id})
		} else {
			positionNodes[cur.pos] = append(positionNodes[cur.pos], cur.id)
		}
		idToPos[cur.id] = cur.pos
	}

	for _, d := range indegree {
		if d != 0 {
			return nil, errors.New("automaton-to-token-stream: cycle found in automaton")
		}
	}

	// Build the per-position edge list. Each transition is exploded into one
	// token per code-point in its [min, max] range, exactly as Lucene does.
	edgesByLayer := make([][]autEdgeToken, len(positionNodes))
	for layerIdx, layer := range positionNodes {
		edges := make([]autEdgeToken, 0, len(layer))
		for _, state := range layer {
			for _, tr := range a.GetTransitions(state) {
				for v := tr.Min; v <= tr.Max; v++ {
					destLayer := idToPos[tr.Dest]
					edges = append(edges, autEdgeToken{destination: destLayer, value: v})
					if a.IsAccept(tr.Dest) && destLayer != len(positionNodes)-1 {
						edges = append(edges, autEdgeToken{destination: len(positionNodes) - 1, value: v})
					}
				}
			}
		}
		edgesByLayer[layerIdx] = edges
	}

	return newAutomatonTokenStream(edgesByLayer), nil
}

// autEdgeToken is the in-memory representation of a single token emitted by
// the automaton-derived TokenStream.
type autEdgeToken struct {
	destination int
	value       int
}

// automatonTokenStream is the topo-sorted TokenStream produced by
// AutomatonToTokenStreamConvert.
type automatonTokenStream struct {
	*BaseTokenStream

	edgesByPos       [][]autEdgeToken
	currentPos       int
	currentEdgeIndex int

	termAttr   CharTermAttribute
	posIncAttr PositionIncrementAttribute
	posLenAttr PositionLengthAttribute
	offsetAttr OffsetAttribute
}

func newAutomatonTokenStream(edges [][]autEdgeToken) *automatonTokenStream {
	ts := &automatonTokenStream{
		BaseTokenStream: NewBaseTokenStream(),
		edgesByPos:      edges,
	}
	// Register the attributes the Java reference adds eagerly.
	ts.AddAttribute(NewCharTermAttribute())
	ts.AddAttribute(NewOffsetAttribute())
	ts.AddAttribute(NewPositionIncrementAttribute())
	ts.AddAttribute(NewPositionLengthAttribute())

	src := ts.GetAttributeSource()
	if a := src.GetAttribute(CharTermAttributeType); a != nil {
		ts.termAttr, _ = a.(CharTermAttribute)
	}
	if a := src.GetAttribute(PositionIncrementAttributeType); a != nil {
		ts.posIncAttr, _ = a.(PositionIncrementAttribute)
	}
	if a := src.GetAttribute(PositionLengthAttributeType); a != nil {
		ts.posLenAttr, _ = a.(PositionLengthAttribute)
	}
	if a := src.GetAttribute(OffsetAttributeType); a != nil {
		ts.offsetAttr, _ = a.(OffsetAttribute)
	}
	return ts
}

// IncrementToken advances to the next edge in the topologically-sorted
// graph. Returns false when all edges have been emitted.
func (ts *automatonTokenStream) IncrementToken() (bool, error) {
	ts.ClearAttributes()
	for ts.currentPos < len(ts.edgesByPos) &&
		ts.currentEdgeIndex == len(ts.edgesByPos[ts.currentPos]) {
		ts.currentEdgeIndex = 0
		ts.currentPos++
	}
	if ts.currentPos == len(ts.edgesByPos) {
		return false, nil
	}
	edge := ts.edgesByPos[ts.currentPos][ts.currentEdgeIndex]

	if ts.termAttr != nil {
		ts.termAttr.SetEmpty()
		ts.termAttr.AppendString(string(rune(edge.value)))
	}
	if ts.posIncAttr != nil {
		if ts.currentEdgeIndex == 0 {
			ts.posIncAttr.SetPositionIncrement(1)
		} else {
			ts.posIncAttr.SetPositionIncrement(0)
		}
	}
	if ts.posLenAttr != nil {
		ts.posLenAttr.SetPositionLength(edge.destination - ts.currentPos)
	}
	if ts.offsetAttr != nil {
		ts.offsetAttr.SetStartOffset(ts.currentPos)
		ts.offsetAttr.SetEndOffset(edge.destination)
	}

	ts.currentEdgeIndex++
	return true, nil
}

// Reset re-initialises the cursor so the stream can be replayed.
func (ts *automatonTokenStream) Reset() error {
	ts.ClearAttributes()
	ts.currentPos = 0
	ts.currentEdgeIndex = 0
	return nil
}

// End sets the final offset to the terminal layer index (mirrors Lucene's
// behaviour: the terminal state itself is not counted as a position).
func (ts *automatonTokenStream) End() error {
	ts.ClearAttributes()
	if ts.posIncAttr != nil {
		ts.posIncAttr.SetPositionIncrement(0)
	}
	if ts.offsetAttr != nil {
		end := len(ts.edgesByPos) - 1
		if end < 0 {
			end = 0
		}
		ts.offsetAttr.SetStartOffset(end)
		ts.offsetAttr.SetEndOffset(end)
	}
	return nil
}

// Close releases resources held by the stream (a no-op for this purely
// in-memory implementation).
func (ts *automatonTokenStream) Close() error { return nil }

// Ensure automatonTokenStream implements TokenStream.
var _ TokenStream = (*automatonTokenStream)(nil)
