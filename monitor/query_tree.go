// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package monitor

import (
	"fmt"
	"sort"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// AnyTokenField is the index field used to store the any-document sentinel term.
const AnyTokenField = "__anytokenfield"

// AnyToken is the sentinel term value that causes a document to always match.
const AnyToken = "__ANYTOKEN__"

// QueryTree is a node in an abstract query tree.
//
// Queries are analyzed and converted into a tree consisting of conjunction
// and disjunction nodes, with leaf nodes containing terms.  Terms can be
// collected from the most highly-weighted path, and the path can be advanced
// via AdvancePhase.
//
// Port of org.apache.lucene.monitor.QueryTree.
type QueryTree interface {
	// Weight returns the weight of this node.
	Weight() float64

	// CollectTerms calls termCollector with all (field, bytes) pairs on the
	// most highly-weighted path below this node.
	CollectTerms(termCollector func(field string, term *util.BytesRef))

	// AdvancePhase advances to the next-most highly-weighted path.
	// Returns false when no more paths above minWeight remain.
	AdvancePhase(minWeight float64) bool

	// StringAt returns a string representation at a given tree depth.
	StringAt(depth int) string
}

// qtSpace returns a string of width space characters.
func qtSpace(width int) string { return strings.Repeat(" ", width) }

// TermQueryTree is a leaf node wrapping a single term.
type TermQueryTree struct {
	field  string
	term   *util.BytesRef
	weight float64
}

// NewTermQueryTree creates a leaf node for the given (field, term) pair with the given weight.
// weight must be > 0.
func NewTermQueryTree(field string, term *util.BytesRef, weight float64) QueryTree {
	return &TermQueryTree{field: field, term: term, weight: weight}
}

// NewTermQueryTreeFromTerm creates a leaf node using the weightor to compute weight.
func NewTermQueryTreeFromTerm(t *index.Term, weightor TermWeightor) QueryTree {
	return NewTermQueryTree(t.Field, t.Bytes, weightor.ApplyAsDouble(t))
}

func (n *TermQueryTree) Weight() float64 {
	if n.weight <= 0 {
		panic("term weights must be greater than 0")
	}
	return n.weight
}

func (n *TermQueryTree) CollectTerms(collector func(string, *util.BytesRef)) {
	collector(n.field, n.term)
}

func (n *TermQueryTree) AdvancePhase(_ float64) bool { return false }

func (n *TermQueryTree) StringAt(depth int) string {
	var text string
	if n.term != nil {
		text = n.term.String()
	}
	return qtSpace(depth) + n.field + ":" + text + "^" + fmt.Sprintf("%g", n.weight)
}

// AnyTermQueryTree is a leaf node that matches any document.
type AnyTermQueryTree struct {
	reason string
}

// NewAnyTermQueryTree returns a leaf that will cause any document to match.
func NewAnyTermQueryTree(reason string) QueryTree {
	return &AnyTermQueryTree{reason: reason}
}

func (n *AnyTermQueryTree) Weight() float64 { return 0 }

func (n *AnyTermQueryTree) CollectTerms(collector func(string, *util.BytesRef)) {
	collector(AnyTokenField, util.NewBytesRef([]byte(AnyToken)))
}

func (n *AnyTermQueryTree) AdvancePhase(_ float64) bool { return false }

func (n *AnyTermQueryTree) StringAt(depth int) string {
	return qtSpace(depth) + "ANY[" + n.reason + "]"
}

// ConjunctionQueryTree is an internal conjunction node.
// On each phase it exposes the term from the highest-weight child.
type ConjunctionQueryTree struct {
	children []QueryTree
}

// NewConjunctionQueryTree creates a conjunction node from a slice of child factories.
// Panics if children is empty.
func NewConjunctionQueryTree(
	children []func(TermWeightor) QueryTree, weightor TermWeightor,
) QueryTree {
	if len(children) == 0 {
		panic("cannot build a conjunction with no children")
	}
	if len(children) == 1 {
		return children[0](weightor)
	}
	qt := make([]QueryTree, len(children))
	for i, f := range children {
		qt[i] = f(weightor)
	}
	// If all children have weight 0 (ANY), return the first.
	allAny := true
	for _, t := range qt {
		if t.Weight() > 0 {
			allAny = false
			break
		}
	}
	if allAny {
		return qt[0]
	}
	node := &ConjunctionQueryTree{children: qt}
	node.sort()
	return node
}

// newConjunctionFromSlice builds a conjunction directly from a QueryTree slice (internal use).
func newConjunctionFromSlice(children []QueryTree) *ConjunctionQueryTree {
	node := &ConjunctionQueryTree{children: make([]QueryTree, len(children))}
	copy(node.children, children)
	node.sort()
	return node
}

func (n *ConjunctionQueryTree) sort() {
	sort.Slice(n.children, func(i, j int) bool {
		return n.children[i].Weight() > n.children[j].Weight()
	})
}

func (n *ConjunctionQueryTree) Weight() float64 { return n.children[0].Weight() }

func (n *ConjunctionQueryTree) CollectTerms(collector func(string, *util.BytesRef)) {
	n.children[0].CollectTerms(collector)
}

func (n *ConjunctionQueryTree) AdvancePhase(minWeight float64) bool {
	if n.children[0].AdvancePhase(minWeight) {
		n.sort()
		return true
	}
	if len(n.children) == 1 {
		return false
	}
	if n.children[1].Weight() <= minWeight {
		return false
	}
	n.children = n.children[1:]
	return true
}

func (n *ConjunctionQueryTree) StringAt(depth int) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "%sConjunction[%d]^%g\n", qtSpace(depth), len(n.children), n.Weight())
	for _, c := range n.children {
		sb.WriteString(c.StringAt(depth + 2))
		sb.WriteByte('\n')
	}
	return sb.String()
}

// DisjunctionQueryTree is an internal disjunction node.
// It exposes terms from all children (each must be covered for a disjunction to match).
type DisjunctionQueryTree struct {
	children []QueryTree
}

// NewDisjunctionQueryTree creates a disjunction node from a slice of child factories.
// Panics if children is empty.
func NewDisjunctionQueryTree(
	children []func(TermWeightor) QueryTree, weightor TermWeightor,
) QueryTree {
	if len(children) == 0 {
		panic("cannot build a disjunction with no children")
	}
	if len(children) == 1 {
		return children[0](weightor)
	}
	qt := make([]QueryTree, len(children))
	for i, f := range children {
		qt[i] = f(weightor)
	}
	// If any child is an ANY node, return it.
	for _, t := range qt {
		if t.Weight() == 0 {
			return t
		}
	}
	node := &DisjunctionQueryTree{children: qt}
	node.sort()
	return node
}

// newDisjunctionFromSlice builds a disjunction directly from a QueryTree slice (internal use).
func newDisjunctionFromSlice(children []QueryTree) *DisjunctionQueryTree {
	node := &DisjunctionQueryTree{children: make([]QueryTree, len(children))}
	copy(node.children, children)
	node.sort()
	return node
}

func (n *DisjunctionQueryTree) sort() {
	sort.Slice(n.children, func(i, j int) bool {
		return n.children[i].Weight() < n.children[j].Weight()
	})
}

func (n *DisjunctionQueryTree) Weight() float64 { return n.children[0].Weight() }

func (n *DisjunctionQueryTree) CollectTerms(collector func(string, *util.BytesRef)) {
	for _, c := range n.children {
		c.CollectTerms(collector)
	}
}

func (n *DisjunctionQueryTree) AdvancePhase(minWeight float64) bool {
	changed := false
	for _, c := range n.children {
		if c.AdvancePhase(minWeight) {
			changed = true
		}
	}
	if !changed {
		return false
	}
	n.sort()
	return true
}

func (n *DisjunctionQueryTree) StringAt(depth int) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "%sDisjunction[%d]^%g\n", qtSpace(depth), len(n.children), n.Weight())
	for _, c := range n.children {
		sb.WriteString(c.StringAt(depth + 2))
		sb.WriteByte('\n')
	}
	return sb.String()
}
