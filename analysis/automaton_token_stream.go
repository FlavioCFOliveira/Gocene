// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"fmt"
	"github.com/FlavioCFOliveira/Gocene/util/automaton"
)

// AutomatonToTokenStream converts an Automaton into a TokenStream.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.AutomatonToTokenStream.
type AutomatonToTokenStream struct{}

type edgeToken struct {
	destination int
	value       int
}

type remapNode struct {
	id  int
	pos int
}

type topoTokenStream struct {
	edgesByPos       [][]edgeToken
	currentPos       int
	currentEdgeIndex int
}

func NewTopoTokenStream(edgesByPos [][]edgeToken) *topoTokenStream {
	return &topoTokenStream{
		edgesByPos: edgesByPos,
	}
}

func (t *topoTokenStream) Reset() error {
	t.currentPos = 0
	t.currentEdgeIndex = 0
	return nil
}

func (t *topoTokenStream) Next() (Token, bool) {
	for t.currentPos < len(t.edgesByPos) && t.currentEdgeIndex == len(t.edgesByPos[t.currentPos]) {
		t.currentEdgeIndex = 0
		t.currentPos++
	}

	if t.currentPos == len(t.edgesByPos) {
		return Token{}, false
	}

	currentEdge := t.edgesByPos[t.currentPos][t.currentEdgeIndex]

	token := Token{
		Term:        string([]rune{rune(currentEdge.value)}),
		Position:    t.currentPos,
		PositionInc: 1,
	}
	if t.currentEdgeIndex != 0 {
		token.PositionInc = 0
	}

	token.StartOffset = t.currentPos
	token.EndOffset = currentEdge.destination

	t.currentEdgeIndex++
	return token, true
}

func (t *topoTokenStream) Close() error {
	return nil
}

func ToTokenStream(automaton *automaton.Automaton) TokenStream {
	positionNodes := make([][]int, 0)

	transitions := automaton.GetSortedTransitions()
	indegree := make([]int, len(transitions))

	for i := 0; i < len(transitions); i++ {
		for _, t := range transitions[i] {
			indegree[t.Dest]++
		}
	}
	if indegree[0] != 0 {
		panic("Start node has incoming edges, creating cycle")
	}

	noIncomingEdges := make([]remapNode, 0)
	idToPos := make(map[int]int)
	noIncomingEdges = append(noIncomingEdges, remapNode{id: 0, pos: 0})

	for len(noIncomingEdges) > 0 {
		currState := noIncomingEdges[0]
		noIncomingEdges = noIncomingEdges[1:]

		for _, t := range transitions[currState.id] {
			indegree[t.Dest]--
			if indegree[t.Dest] == 0 {
				noIncomingEdges = append(noIncomingEdges, remapNode{id: t.Dest, pos: currState.pos + 1})
			}
		}

		if len(positionNodes) == currState.pos {
			positionNodes = append(positionNodes, []int{currState.id})
		} else {
			positionNodes[currState.pos] = append(positionNodes[currState.pos], currState.id)
		}
		idToPos[currState.id] = currState.pos
	}

	for i := 0; i < len(indegree); i++ {
		if indegree[i] != 0 {
			panic("Cycle found in automaton")
		}
	}

	edgesByLayer := make([][]edgeToken, 0)
	for _, layer := range positionNodes {
		edges := make([]edgeToken, 0)
		for _, state := range layer {
			for _, t := range transitions[state] {
				for val := t.Min; val <= t.Max; val++ {
					destLayer := idToPos[t.Dest]
					edges = append(edges, edgeToken{destination: destLayer, value: val})
					if automaton.IsAccept(t.Dest) && destLayer != len(positionNodes)-1 {
						edges = append(edges, edgeToken{destination: len(positionNodes) - 1, value: val})
					}
				}
			}
		}
		edgesByLayer = append(edgesByLayer, edges)
	}

	return NewTopoTokenStream(edgesByLayer)
}
