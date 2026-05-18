// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package join

// BlockJoinSelectorType picks which child value bubbles up to the parent
// when the join query reduces several children into a single parent score.
// Mirrors org.apache.lucene.search.join.BlockJoinSelector.Type.
type BlockJoinSelectorType int

const (
	// BlockJoinMin picks the smallest child value.
	BlockJoinMin BlockJoinSelectorType = iota
	// BlockJoinMax picks the largest child value.
	BlockJoinMax
	// BlockJoinAvg returns the arithmetic mean of the child values.
	BlockJoinAvg
	// BlockJoinSum returns the sum of the child values.
	BlockJoinSum
)

// ReduceLongs reduces an int64 slice under the given selector. The semantics
// mirror Lucene's BlockJoinSelector long-value reduction.
func ReduceLongs(t BlockJoinSelectorType, values []int64) int64 {
	if len(values) == 0 {
		return 0
	}
	switch t {
	case BlockJoinMin:
		m := values[0]
		for _, v := range values[1:] {
			if v < m {
				m = v
			}
		}
		return m
	case BlockJoinMax:
		m := values[0]
		for _, v := range values[1:] {
			if v > m {
				m = v
			}
		}
		return m
	case BlockJoinSum:
		var s int64
		for _, v := range values {
			s += v
		}
		return s
	case BlockJoinAvg:
		var s int64
		for _, v := range values {
			s += v
		}
		return s / int64(len(values))
	}
	return 0
}

// ReduceDoubles reduces a float64 slice under the given selector.
func ReduceDoubles(t BlockJoinSelectorType, values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	switch t {
	case BlockJoinMin:
		m := values[0]
		for _, v := range values[1:] {
			if v < m {
				m = v
			}
		}
		return m
	case BlockJoinMax:
		m := values[0]
		for _, v := range values[1:] {
			if v > m {
				m = v
			}
		}
		return m
	case BlockJoinSum:
		var s float64
		for _, v := range values {
			s += v
		}
		return s
	case BlockJoinAvg:
		var s float64
		for _, v := range values {
			s += v
		}
		return s / float64(len(values))
	}
	return 0
}
