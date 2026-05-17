// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "github.com/FlavioCFOliveira/Gocene/index"

// Occur classifies how a clause should contribute to a boolean query result.
// It mirrors BooleanClause.Occur in the parent visitor argument; we redeclare
// it here in this file for clarity even though search/boolean_query.go
// provides the canonical definition.
//
// We import the canonical Occur from boolean_query.go to avoid a cycle.

// QueryVisitor walks a Query tree, allowing inspection of leaf queries and the
// terms they match.
//
// Mirrors org.apache.lucene.search.QueryVisitor.
type QueryVisitor interface {
	// ConsumeTerms is invoked when a query exposes one or more index terms.
	ConsumeTerms(query Query, terms ...*index.Term)

	// ConsumeTermsMatching is invoked when a query expands to a (potentially
	// large) set of terms described by an automaton supplier.
	ConsumeTermsMatching(query Query, field string, automaton func() ByteRunAutomaton)

	// VisitLeaf is invoked when a leaf query is visited.
	VisitLeaf(query Query)

	// AcceptField returns true if the visitor wants to descend into the field.
	AcceptField(field string) bool

	// GetSubVisitor returns a visitor for a parent compound query and the
	// Occur clause that produced it.
	GetSubVisitor(occur Occur, parent Query) QueryVisitor
}

// ByteRunAutomaton is the structural placeholder for the
// util/automaton.ByteRunAutomaton used by Lucene visitors. It is intentionally
// minimal here to avoid a hard dependency at this layer; concrete code that
// produces automatons should use the canonical type via interface satisfaction.
type ByteRunAutomaton interface {
	Run(input []byte) bool
}

// EmptyQueryVisitor is the canonical no-op visitor. It accepts any field and
// returns itself for every sub-visitor request, so a recursive traversal of a
// query tree using EmptyQueryVisitor visits every leaf without recording
// anything.
var EmptyQueryVisitor QueryVisitor = emptyQueryVisitor{}

type emptyQueryVisitor struct{}

func (emptyQueryVisitor) ConsumeTerms(query Query, terms ...*index.Term) {}
func (emptyQueryVisitor) ConsumeTermsMatching(query Query, field string, automaton func() ByteRunAutomaton) {
}
func (emptyQueryVisitor) VisitLeaf(query Query)             {}
func (emptyQueryVisitor) AcceptField(field string) bool     { return true }
func (emptyQueryVisitor) GetSubVisitor(o Occur, p Query) QueryVisitor {
	return EmptyQueryVisitor
}

// TermCollectorVisitor collects every term reported via ConsumeTerms into the
// provided set. Mirrors QueryVisitor.termCollector(Set<Term>).
type TermCollectorVisitor struct {
	EmptyQueryVisitorBase
	Terms map[string]*index.Term // keyed by Term.String() for deterministic insertion
}

// NewTermCollectorVisitor creates a TermCollectorVisitor with an empty set.
func NewTermCollectorVisitor() *TermCollectorVisitor {
	return &TermCollectorVisitor{Terms: make(map[string]*index.Term)}
}

// ConsumeTerms adds each provided term to the collector.
func (v *TermCollectorVisitor) ConsumeTerms(query Query, terms ...*index.Term) {
	for _, t := range terms {
		if t == nil {
			continue
		}
		v.Terms[t.String()] = t
	}
}

// EmptyQueryVisitorBase provides default no-op implementations for the optional
// methods, so test/utility visitors only need to override the methods they care
// about.
type EmptyQueryVisitorBase struct{}

func (EmptyQueryVisitorBase) ConsumeTerms(query Query, terms ...*index.Term) {}
func (EmptyQueryVisitorBase) ConsumeTermsMatching(query Query, field string, automaton func() ByteRunAutomaton) {
}
func (EmptyQueryVisitorBase) VisitLeaf(query Query)             {}
func (EmptyQueryVisitorBase) AcceptField(field string) bool     { return true }
func (EmptyQueryVisitorBase) GetSubVisitor(o Occur, p Query) QueryVisitor {
	return EmptyQueryVisitor
}
