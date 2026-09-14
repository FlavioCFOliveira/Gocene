// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package flexible

import (
	"fmt"
	"strconv"
	"strings"
)

// PointRangeQueryNode represents a range query composed by PointQueryNode
// bounds.
//
// Mirrors org.apache.lucene.queryparser.flexible.standard.nodes.PointRangeQueryNode.
type PointRangeQueryNode struct {
	*AbstractRangeQueryNode
	// NumericConfig is the PointsConfig associated with the bounds. Java
	// exposes it as a public field of the same name.
	NumericConfig *PointsConfig
}

// NewPointRangeQueryNode constructs a PointRangeQueryNode using the given
// PointQueryNode bounds and PointsConfig.
//
// Mirrors the Java constructor, whose whole body is
// setBounds(lower, upper, lowerInclusive, upperInclusive, numericConfig).
func NewPointRangeQueryNode(lower, upper *PointQueryNode, lowerInclusive, upperInclusive bool, numericConfig *PointsConfig) *PointRangeQueryNode {
	n := &PointRangeQueryNode{AbstractRangeQueryNode: newAbstractRangeQueryNode()}
	n.SetPointBounds(lower, upper, lowerInclusive, upperInclusive, numericConfig)
	return n
}

// SetPointBounds sets the upper and lower bounds of this range query node and
// the PointsConfig associated with those bounds.
//
// Mirrors PointRangeQueryNode#setBounds(PointQueryNode, PointQueryNode,
// boolean, boolean, PointsConfig).
func (n *PointRangeQueryNode) SetPointBounds(lower, upper *PointQueryNode, lowerInclusive, upperInclusive bool, pointsConfig *PointsConfig) {
	if pointsConfig == nil {
		panic("pointsConfig must not be null!")
	}

	var lowerBound, upperBound FieldValuePairQueryNode
	if lower != nil {
		lowerBound = lower
	}
	if upper != nil {
		upperBound = upper
	}

	n.SetBounds(lowerBound, upperBound, lowerInclusive, upperInclusive)
	n.NumericConfig = pointsConfig
}

// GetPointsConfig returns the PointsConfig associated with the lower and upper
// bounds.
//
// Mirrors PointRangeQueryNode#getPointsConfig().
func (n *PointRangeQueryNode) GetPointsConfig() *PointsConfig { return n.NumericConfig }

// GetLowerPoint returns the lower bound as a PointQueryNode, or nil.
func (n *PointRangeQueryNode) GetLowerPoint() *PointQueryNode {
	bound, _ := n.GetLowerBound().(*PointQueryNode)
	return bound
}

// GetUpperPoint returns the upper bound as a PointQueryNode, or nil.
func (n *PointRangeQueryNode) GetUpperPoint() *PointQueryNode {
	bound, _ := n.GetUpperBound().(*PointQueryNode)
	return bound
}

// CloneTree deep-copies this node.
func (n *PointRangeQueryNode) CloneTree() QueryNode {
	var lower, upper *PointQueryNode
	if b := n.GetLowerPoint(); b != nil {
		lower, _ = b.CloneTree().(*PointQueryNode)
	}
	if b := n.GetUpperPoint(); b != nil {
		upper, _ = b.CloneTree().(*PointQueryNode)
	}
	return NewPointRangeQueryNode(lower, upper, n.IsLowerInclusive(), n.IsUpperInclusive(), n.NumericConfig)
}

// String returns the pseudo-XML debug form.
//
// Mirrors PointRangeQueryNode#toString().
func (n *PointRangeQueryNode) String() string {
	var sb strings.Builder
	sb.WriteString("<pointRange lowerInclusive='")
	sb.WriteString(strconv.FormatBool(n.IsLowerInclusive()))
	sb.WriteString("' upperInclusive='")
	sb.WriteString(strconv.FormatBool(n.IsUpperInclusive()))
	sb.WriteString("' type='")
	sb.WriteString(n.NumericConfig.GetType().String())
	sb.WriteString("'>\n")
	sb.WriteString(fmt.Sprintf("%v", n.GetLowerBound()))
	sb.WriteByte('\n')
	sb.WriteString(fmt.Sprintf("%v", n.GetUpperBound()))
	sb.WriteByte('\n')
	sb.WriteString("</pointRange>")
	return sb.String()
}
