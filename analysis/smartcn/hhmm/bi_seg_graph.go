// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package hhmm

import (
	"math"

	"github.com/FlavioCFOliveira/Gocene/analysis/smartcn"
)

// BiSegGraph is a graph of possible token pairs (bigrams) at each "to"
// index in a sentence. It is built from a SegGraph and used by the Viterbi
// algorithm to find the shortest (optimal) segmentation path.
//
// Go port of org.apache.lucene.analysis.cn.smart.hhmm.BiSegGraph.
//
// Deviation: Java uses HPPC IntObjectHashMap; Go uses map[int][]*SegTokenPair.
type BiSegGraph struct {
	tokenPairListTable map[int][]*SegTokenPair
	segTokenList       []*SegToken
}

// NewBiSegGraph constructs a BiSegGraph from the given SegGraph.
// It requires the BigramDictionary to be loaded.
func NewBiSegGraph(segGraph *SegGraph) (*BiSegGraph, error) {
	bd, err := GetBigramDictionary()
	if err != nil {
		return nil, err
	}

	g := &BiSegGraph{
		tokenPairListTable: make(map[int][]*SegTokenPair),
	}
	g.segTokenList = segGraph.MakeIndex()
	g.generateBiSegGraph(segGraph, bd)
	return g, nil
}

// generateBiSegGraph populates the token-pair table using bigram weights.
func (g *BiSegGraph) generateBiSegGraph(segGraph *SegGraph, bd *BigramDictionary) {
	const smooth = 0.1
	tinyDouble := 1.0 / float64(smartcn.MaxFrequence)

	maxStart := segGraph.GetMaxStart()

	key := -1
	for key < maxStart {
		if segGraph.IsStartExist(key) {
			tokenList := segGraph.GetStartList(key)
			for _, t1 := range tokenList {
				oneWordFreq := float64(t1.Weight)
				next := t1.EndOffset

				// Find the next start position with tokens.
				var nextTokens []*SegToken
				for next <= maxStart {
					if segGraph.IsStartExist(next) {
						nextTokens = segGraph.GetStartList(next)
						break
					}
					next++
				}
				if nextTokens == nil {
					break
				}

				for _, t2 := range nextTokens {
					// Build the bigram key: t1.charArray + '@' + t2.charArray.
					idBuffer := make([]rune, len(t1.CharArray)+1+len(t2.CharArray))
					copy(idBuffer, t1.CharArray)
					idBuffer[len(t1.CharArray)] = WordSegmentChar
					copy(idBuffer[len(t1.CharArray)+1:], t2.CharArray)

					wordPairFreq := float64(bd.GetFrequency(idBuffer))

					// Smoothed negative-log-probability weight.
					weight := -math.Log(
						smooth*(1.0+oneWordFreq)/float64(smartcn.MaxFrequence) +
							(1.0-smooth)*((1.0-tinyDouble)*wordPairFreq/(1.0+oneWordFreq)+tinyDouble),
					)

					tokenPair := NewSegTokenPair(idBuffer, t1.Index, t2.Index, weight)
					g.addSegTokenPair(tokenPair)
				}
			}
		}
		key++
	}
}

// addSegTokenPair adds a SegTokenPair keyed by its "to" index.
func (g *BiSegGraph) addSegTokenPair(tokenPair *SegTokenPair) {
	to := tokenPair.To
	g.tokenPairListTable[to] = append(g.tokenPairListTable[to], tokenPair)
}

// IsToExist returns true if there are token pairs ending at the given index.
func (g *BiSegGraph) IsToExist(to int) bool {
	list := g.tokenPairListTable[to]
	return len(list) > 0
}

// GetToList returns the list of token pairs ending at the given index.
func (g *BiSegGraph) GetToList(to int) []*SegTokenPair {
	return g.tokenPairListTable[to]
}

// GetToCount returns the number of distinct "to" positions in the graph.
func (g *BiSegGraph) GetToCount() int {
	return len(g.tokenPairListTable)
}

// GetShortPath finds the minimum-cost segmentation via the Viterbi algorithm.
func (g *BiSegGraph) GetShortPath() []*SegToken {
	nodeCount := g.GetToCount()
	path := make([]*PathNode, 0, nodeCount+1)

	zeroPath := &PathNode{Weight: 0, PreNode: 0}
	path = append(path, zeroPath)

	for current := 1; current <= nodeCount; current++ {
		edges := g.GetToList(current)
		minWeight := math.MaxFloat64
		var minEdge *SegTokenPair
		for _, edge := range edges {
			preNode := path[edge.From]
			if preNode.Weight+edge.Weight < minWeight {
				minWeight = preNode.Weight + edge.Weight
				minEdge = edge
			}
		}
		newNode := &PathNode{Weight: minWeight, PreNode: minEdge.From}
		path = append(path, newNode)
	}

	// Trace back the optimal path.
	lastNode := len(path) - 1
	current := lastNode
	rpath := make([]int, 0, nodeCount)
	for current != 0 {
		rpath = append(rpath, current)
		current = path[current].PreNode
	}
	rpath = append(rpath, 0)

	// Reverse and collect tokens.
	result := make([]*SegToken, 0, len(rpath))
	for j := len(rpath) - 1; j >= 0; j-- {
		result = append(result, g.segTokenList[rpath[j]])
	}
	return result
}
