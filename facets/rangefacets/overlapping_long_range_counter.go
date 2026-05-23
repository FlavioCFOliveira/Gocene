// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package rangefacets

import (
	"math"
	"sort"
)

// LongRangeNode is one node in the segment tree built by
// OverlappingLongRangeCounter. Mirrors
// OverlappingLongRangeCounter.LongRangeNode.
type LongRangeNode struct {
	Left, Right *LongRangeNode

	// Inclusive range covered by this node.
	start, end int64

	// elementaryIntervalIndex is the index of the elementary interval this
	// leaf covers (−1 for internal nodes).
	elementaryIntervalIndex int

	// outputs is the list of user-requested range indices rooted here.
	outputs []int
}

// addOutputs recursively assigns the range at index i to all nodes fully
// within [range.InclusiveMin(), range.InclusiveMax()]. Mirrors
// LongRangeNode.addOutputs.
func (n *LongRangeNode) addOutputs(index int, r *LongRange) {
	lo, hi := r.InclusiveMin(), r.InclusiveMax()
	if n.start >= lo && n.end <= hi {
		n.outputs = append(n.outputs, index)
	} else if n.Left != nil {
		n.Left.addOutputs(index, r)
		n.Right.addOutputs(index, r)
	}
}

// splitTree builds a balanced binary segment tree over elementary intervals
// [start, end).
func splitTree(start, end int, intervals []InclusiveRange) *LongRangeNode {
	if start == end-1 {
		iv := intervals[start]
		return &LongRangeNode{
			start:                   iv.start,
			end:                     iv.end,
			elementaryIntervalIndex: start,
		}
	}
	mid := (start + end) >> 1
	left := splitTree(start, mid, intervals)
	right := splitTree(mid, end, intervals)
	return &LongRangeNode{
		start: left.start,
		end:   right.end,
		Left:  left, Right: right,
		elementaryIntervalIndex: -1,
	}
}

// buildOverlappingElementaryIntervals creates elementary intervals for a set
// of possibly-overlapping ranges. Mirrors
// OverlappingLongRangeCounter.buildElementaryIntervals.
func buildOverlappingElementaryIntervals(ranges []*LongRange) []InclusiveRange {
	type flag struct {
		v     int64
		flags int // 1 = start, 2 = end
	}
	endsMap := map[int64]int{}
	set := func(v, bit int64) {
		if _, ok := endsMap[v]; !ok {
			endsMap[v] = int(bit)
		} else {
			endsMap[v] |= int(bit)
		}
	}
	set(math.MinInt64, 1)
	set(math.MaxInt64, 2)
	for _, r := range ranges {
		set(r.InclusiveMin(), 1)
		set(r.InclusiveMax(), 2)
	}

	keys := make([]int64, 0, len(endsMap))
	for k := range endsMap {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })

	var intervals []InclusiveRange
	upto := 1
	v := keys[0]
	var prev int64
	if endsMap[v] == 3 {
		intervals = append(intervals, InclusiveRange{v, v})
		prev = v + 1
	} else {
		prev = v
	}
	for upto < len(keys) {
		v = keys[upto]
		fl := endsMap[v]
		switch fl {
		case 3:
			if v > prev {
				intervals = append(intervals, InclusiveRange{prev, v - 1})
			}
			intervals = append(intervals, InclusiveRange{v, v})
			prev = v + 1
		case 1:
			if v > prev {
				intervals = append(intervals, InclusiveRange{prev, v - 1})
			}
			prev = v
		default: // 2
			intervals = append(intervals, InclusiveRange{prev, v})
			prev = v + 1
		}
		upto++
	}
	return intervals
}

// OverlappingLongRangeCounter handles ranges that may overlap by building a
// segment tree over elementary intervals and rolling up counts at the end.
// Mirrors org.apache.lucene.facet.range.OverlappingLongRangeCounter.
type OverlappingLongRangeCounter struct {
	base *longRangeCounter

	root       *LongRangeNode
	boundaries []int64

	hasUnflushedCounts bool

	// Single-valued path: elementary interval counts, aggregated at finish().
	singleEICounts []int

	// Multi-valued path: bitsets per doc to prevent double-counting.
	multiDocEIHits   []bool
	multiDocRangeHits []bool

	elementaryIntervalUpto int
	missingCount           int
}

func newOverlappingLongRangeCounter(ranges []*LongRange, countBuffer []int) *OverlappingLongRangeCounter {
	c := &OverlappingLongRangeCounter{
		base: newLongRangeCounter(countBuffer),
	}

	intervals := buildOverlappingElementaryIntervals(ranges)
	c.root = splitTree(0, len(intervals), intervals)
	for i, r := range ranges {
		c.root.addOutputs(i, r)
	}
	c.boundaries = make([]int64, len(intervals))
	for i, iv := range intervals {
		c.boundaries[i] = iv.end
	}

	c.base.boundariesFn = func() []int64 { return c.boundaries }
	c.base.processSingleValuedFn = c.processSingleValuedHit
	c.base.processMultiValuedFn = c.processMultiValuedHit
	return c
}

func (c *OverlappingLongRangeCounter) processSingleValuedHit(idx int) {
	if c.singleEICounts == nil {
		c.singleEICounts = make([]int, len(c.boundaries))
	}
	c.singleEICounts[idx]++
	c.hasUnflushedCounts = true
}

func (c *OverlappingLongRangeCounter) processMultiValuedHit(idx int) {
	c.multiDocEIHits[idx] = true
}

// StartMultiValuedDoc implements LongRangeCounterI.
func (c *OverlappingLongRangeCounter) StartMultiValuedDoc() {
	c.base.startMultiValuedDoc()
	if c.multiDocEIHits == nil {
		c.multiDocEIHits = make([]bool, len(c.boundaries))
	} else {
		for i := range c.multiDocEIHits {
			c.multiDocEIHits[i] = false
		}
	}
}

// EndMultiValuedDoc implements LongRangeCounterI.
func (c *OverlappingLongRangeCounter) EndMultiValuedDoc() bool {
	if c.base.rangeCount() == 0 {
		return false
	}
	if c.multiDocRangeHits == nil {
		c.multiDocRangeHits = make([]bool, c.base.rangeCount())
	} else {
		for i := range c.multiDocRangeHits {
			c.multiDocRangeHits[i] = false
		}
	}
	c.elementaryIntervalUpto = 0
	c.rollupMultiValued(c.root)

	matched := false
	for i, hit := range c.multiDocRangeHits {
		if hit {
			c.base.increment(i)
			matched = true
		}
	}
	return matched
}

// AddSingleValued implements LongRangeCounterI.
func (c *OverlappingLongRangeCounter) AddSingleValued(v int64) { c.base.addSingleValued(v) }

// AddMultiValued implements LongRangeCounterI.
func (c *OverlappingLongRangeCounter) AddMultiValued(v int64) { c.base.addMultiValued(v) }

// Finish implements LongRangeCounterI.
func (c *OverlappingLongRangeCounter) Finish() int {
	if !c.hasUnflushedCounts {
		return 0
	}
	c.missingCount = 0
	c.elementaryIntervalUpto = 0
	c.rollupSingleValued(c.root, false)
	return c.missingCount
}

func (c *OverlappingLongRangeCounter) rollupSingleValued(node *LongRangeNode, sawOutputs bool) int {
	sawOutputs = sawOutputs || len(node.outputs) > 0
	var count int
	if node.Left != nil {
		count = c.rollupSingleValued(node.Left, sawOutputs)
		count += c.rollupSingleValued(node.Right, sawOutputs)
	} else {
		count = c.singleEICounts[c.elementaryIntervalUpto]
		c.elementaryIntervalUpto++
		if !sawOutputs {
			c.missingCount += count
		}
	}
	for _, ri := range node.outputs {
		c.base.incrementBy(ri, count)
	}
	return count
}

func (c *OverlappingLongRangeCounter) rollupMultiValued(node *LongRangeNode) bool {
	var containedHit bool
	if node.Left != nil {
		containedHit = c.rollupMultiValued(node.Left)
		containedHit = c.rollupMultiValued(node.Right) || containedHit
	} else {
		containedHit = c.multiDocEIHits[c.elementaryIntervalUpto]
		c.elementaryIntervalUpto++
	}
	if containedHit {
		for _, ri := range node.outputs {
			c.multiDocRangeHits[ri] = true
		}
	}
	return containedHit
}

var _ LongRangeCounterI = (*OverlappingLongRangeCounter)(nil)
