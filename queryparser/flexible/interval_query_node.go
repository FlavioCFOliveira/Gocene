// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package flexible

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/queries/intervals"
	"github.com/FlavioCFOliveira/Gocene/queryparser/flexible/standard/nodes/intervalfn"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// IntervalQueryNode represents an interval function.
//
// Mirrors org.apache.lucene.queryparser.flexible.standard.nodes.IntervalQueryNode.
type IntervalQueryNode struct {
	*QueryNodeImpl
	source   intervalfn.IntervalFunction
	field    string
	analyzer analysis.Analyzer
}

// NewIntervalQueryNode is the sole constructor.
//
// Mirrors IntervalQueryNode(String field, IntervalFunction source).
func NewIntervalQueryNode(field string, source intervalfn.IntervalFunction) *IntervalQueryNode {
	if source == nil {
		panic("source must not be null")
	}
	n := &IntervalQueryNode{QueryNodeImpl: NewQueryNodeImpl(nil)}
	// IntervalQueryNode extends QueryNodeImpl without calling setLeaf(false).
	n.SetLeaf(true)
	n.field = field
	n.source = source
	return n
}

// GetSource returns the interval function this node wraps.
func (n *IntervalQueryNode) GetSource() intervalfn.IntervalFunction { return n.source }

// GetQuery builds the IntervalQuery this node represents.
//
// Mirrors IntervalQueryNode#getQuery().
func (n *IntervalQueryNode) GetQuery() search.Query {
	if n.field == "" {
		panic("Field must not be null for interval queries.")
	}
	if n.analyzer == nil {
		panic("Analyzer must not be null for interval queries.")
	}
	return intervals.NewIntervalQuery(n.field, n.source.ToIntervalSource(n.field, n.analyzer))
}

// ToQueryString renders "<field>:<source>".
//
// Mirrors IntervalQueryNode#toQueryString(EscapeQuerySyntax), which is
// String.format(Locale.ROOT, "%s:%s", field, source).
func (n *IntervalQueryNode) ToQueryString(escapeSyntaxParser EscapeQuerySyntax) string {
	return fmt.Sprintf("%s:%s", n.field, n.source)
}

// String renders the node through ToQueryString with the standard escaper.
//
// Mirrors IntervalQueryNode#toString().
func (n *IntervalQueryNode) String() string {
	return n.ToQueryString(NewEscapeQuerySyntaxImpl())
}

// GetField returns the field name.
//
// Mirrors IntervalQueryNode#getField().
func (n *IntervalQueryNode) GetField() string { return n.field }

// SetField sets the field name.
//
// Mirrors IntervalQueryNode#setField(CharSequence).
func (n *IntervalQueryNode) SetField(fieldName string) { n.field = fieldName }

// CloneTree returns a fresh IntervalQueryNode over the same field and source.
//
// Mirrors IntervalQueryNode#cloneTree().
func (n *IntervalQueryNode) CloneTree() QueryNode {
	return NewIntervalQueryNode(n.field, n.source)
}

// SetAnalyzer sets the analyzer used to build the interval source.
//
// Mirrors IntervalQueryNode#setAnalyzer(Analyzer).
func (n *IntervalQueryNode) SetAnalyzer(analyzer analysis.Analyzer) {
	if analyzer == nil {
		panic("Analyzer must not be null for interval queries.")
	}
	n.analyzer = analyzer
}

// GetAnalyzer returns the analyzer set with SetAnalyzer.
func (n *IntervalQueryNode) GetAnalyzer() analysis.Analyzer { return n.analyzer }
