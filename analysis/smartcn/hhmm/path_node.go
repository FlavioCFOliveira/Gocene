// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package hhmm

import "math"

// PathNode is the internal node representation used by BiSegGraph to
// find the shortest path with the Viterbi algorithm.
//
// Go port of org.apache.lucene.analysis.cn.smart.hhmm.PathNode.
type PathNode struct {
	// Weight is the accumulated cost up to this node.
	Weight float64

	// PreNode is the index of the predecessor node.
	PreNode int
}

// Compare returns -1, 0, or 1 depending on whether n.Weight is less than,
// equal to, or greater than other.Weight.
func (n *PathNode) Compare(other *PathNode) int {
	if n.Weight < other.Weight {
		return -1
	}
	if n.Weight == other.Weight {
		return 0
	}
	return 1
}

// Equal reports whether n equals other by value (mirrors Java equals).
func (n *PathNode) Equal(other *PathNode) bool {
	if n == other {
		return true
	}
	if other == nil {
		return false
	}
	return n.PreNode == other.PreNode &&
		math.Float64bits(n.Weight) == math.Float64bits(other.Weight)
}
