// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of
// org.apache.lucene.sandbox.facet.cutters.ranges.LongRangeNode.
package ranges

import (
	"fmt"
	"strings"
)

// LongRangeNode holds one node of the segment tree used by
// OverlappingLongRangeFacetCutter.
//
// Mirrors org.apache.lucene.sandbox.facet.cutters.ranges.LongRangeNode.
type LongRangeNode struct {
	Left  *LongRangeNode
	Right *LongRangeNode

	// Start is the inclusive lower bound of this node's range.
	Start int64
	// End is the inclusive upper bound of this node's range.
	End int64

	// Outputs lists the range indices to emit when a query document lands
	// within this node's range.
	Outputs []int32
}

// NewLongRangeNode creates a node covering [start, end] with the given
// optional children.
func NewLongRangeNode(start, end int64, left, right *LongRangeNode) *LongRangeNode {
	return &LongRangeNode{
		Start: start,
		End:   end,
		Left:  left,
		Right: right,
	}
}

// AddOutputs recursively assigns range outputs.
//
// If this node's interval is fully contained within the given range, the
// range position is recorded in Outputs. Otherwise the children are
// visited.
//
// Mirrors LongRangeNode.addOutputs(LongRangeFacetCutter.LongRangeAndPos).
func (n *LongRangeNode) AddOutputs(r LongRangeAndPos) {
	if n.Start >= r.Range.Min && n.End <= r.Range.Max {
		n.Outputs = append(n.Outputs, int32(r.Pos))
	} else if n.Left != nil {
		n.Left.AddOutputs(r)
		n.Right.AddOutputs(r)
	}
}

// String returns a human-readable representation of the subtree.
//
// Mirrors LongRangeNode.toString().
func (n *LongRangeNode) String() string {
	var sb strings.Builder
	n.appendTo(&sb, 0)
	return sb.String()
}

func (n *LongRangeNode) appendTo(sb *strings.Builder, depth int) {
	indent := fmt.Sprintf("%s", strings.Repeat("  ", depth))
	if n.Left == nil {
		fmt.Fprintf(sb, "%sleaf: %d to %d", indent, n.Start, n.End)
	} else {
		fmt.Fprintf(sb, "%snode: %d to %d", indent, n.Start, n.End)
	}
	if len(n.Outputs) > 0 {
		fmt.Fprintf(sb, " outputs=%v", n.Outputs)
	}
	sb.WriteByte('\n')
	if n.Left != nil {
		n.Left.appendTo(sb, depth+1)
		n.Right.appendTo(sb, depth+1)
	}
}
