// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package hhmm

// SegGraph is a graph representing possible tokens at each start offset in a
// sentence.
//
// For each start offset a list of possible tokens is stored, keyed by the
// offset value.
//
// Go port of org.apache.lucene.analysis.cn.smart.hhmm.SegGraph.
//
// Deviation: Java uses IntObjectHashMap from the internal HPPC library; Go
// uses a plain map[int][]*SegToken, which has equivalent algorithmic
// characteristics.
type SegGraph struct {
	tokenListTable map[int][]*SegToken
	maxStart       int
}

// NewSegGraph constructs an empty SegGraph.
func NewSegGraph() *SegGraph {
	return &SegGraph{
		tokenListTable: make(map[int][]*SegToken),
		maxStart:       -1,
	}
}

// IsStartExist returns true if there are tokens at the given start offset.
func (g *SegGraph) IsStartExist(s int) bool {
	_, ok := g.tokenListTable[s]
	return ok
}

// GetStartList returns the token list at the given start offset, or nil.
func (g *SegGraph) GetStartList(s int) []*SegToken {
	return g.tokenListTable[s]
}

// GetMaxStart returns the highest start offset in the graph, or -1 if empty.
func (g *SegGraph) GetMaxStart() int {
	return g.maxStart
}

// MakeIndex assigns sequential Index values to tokens ordered by startOffset
// and returns the ordered list.
func (g *SegGraph) MakeIndex() []*SegToken {
	result := make([]*SegToken, 0)
	s := -1
	count := 0
	size := len(g.tokenListTable)
	idx := 0
	for count < size {
		if list, ok := g.tokenListTable[s]; ok {
			for _, st := range list {
				st.Index = idx
				result = append(result, st)
				idx++
			}
			count++
		}
		s++
	}
	return result
}

// AddToken adds a SegToken to the graph.
func (g *SegGraph) AddToken(token *SegToken) {
	s := token.StartOffset
	g.tokenListTable[s] = append(g.tokenListTable[s], token)
	if s > g.maxStart {
		g.maxStart = s
	}
}

// ToTokenList returns all tokens ordered by startOffset.
func (g *SegGraph) ToTokenList() []*SegToken {
	result := make([]*SegToken, 0)
	s := -1
	count := 0
	size := len(g.tokenListTable)
	for count < size {
		if list, ok := g.tokenListTable[s]; ok {
			result = append(result, list...)
			count++
		}
		s++
	}
	return result
}
