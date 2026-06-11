// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package monitor

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// safeQueryString safely obtains a string representation of a Query.
// Since String() is not part of Gocene's Query interface, we reach it via
// optional interface assertion, falling back to type name on failure.
func safeQueryString(q search.Query) string {
	if s, ok := q.(interface{ String() string }); ok {
		return s.String()
	}
	return fmt.Sprintf("%T", q)
}

// unknownQueryMapper is a function that maps an unrecognised leaf query to a
// QueryTree, or returns nil to signal that the default any-term fallback
// should be used.
type unknownQueryMapper func(query search.Query, weightor TermWeightor) QueryTree

// QueryAnalyzer analyses a Lucene Query and produces a QueryTree suitable for
// use by a Presearcher to index the query for matching.
//
// This is the Go port of org.apache.lucene.monitor.QueryAnalyzer from
// Apache Lucene 10.4.0.
type QueryAnalyzer struct {
	unknownQueryMapper unknownQueryMapper
}

// NewQueryAnalyzer creates a QueryAnalyzer with a list of CustomQueryHandlers.
// Each handler is tried in order when an unrecognised leaf query is
// encountered; the first non-nil QueryTree returned wins. If no handler
// matches, an any-term node is emitted.
//
// This is the Go equivalent of QueryAnalyzer(List<CustomQueryHandler>).
func NewQueryAnalyzer(handlers []CustomQueryHandler) *QueryAnalyzer {
	return &QueryAnalyzer{
		unknownQueryMapper: buildMapper(handlers),
	}
}

// NewDefaultQueryAnalyzer creates a QueryAnalyzer with no custom handlers.
// Unrecognised leaf queries will produce any-term nodes.
//
// This is the Go equivalent of QueryAnalyzer().
func NewDefaultQueryAnalyzer() *QueryAnalyzer {
	return &QueryAnalyzer{
		unknownQueryMapper: func(_ search.Query, _ TermWeightor) QueryTree { return nil },
	}
}

// buildMapper creates an unknownQueryMapper from a list of CustomQueryHandlers.
func buildMapper(handlers []CustomQueryHandler) unknownQueryMapper {
	if len(handlers) == 0 {
		return func(_ search.Query, _ TermWeightor) QueryTree { return nil }
	}
	return func(q search.Query, w TermWeightor) QueryTree {
		for _, handler := range handlers {
			if qt := handler.HandleQuery(q, w); qt != nil {
				return qt
			}
		}
		return nil
	}
}

// BuildTree analyses a Query and produces a QueryTree.
//
// This is the Go equivalent of QueryAnalyzer.buildTree(Query, TermWeightor).
func (qa *QueryAnalyzer) BuildTree(luceneQuery search.Query, weightor TermWeightor) QueryTree {
	builder := newQueryBuilder(qa.unknownQueryMapper)
	// Visit the query tree using QueryVisitor.
	if v, ok := luceneQuery.(interface{ Visit(search.QueryVisitor) }); ok {
		v.Visit(builder)
	} else {
		// If the query does not implement Visit, treat it as a leaf.
		builder.VisitLeaf(luceneQuery)
	}
	return builder.apply(weightor)
}

// queryBuilder is a QueryVisitor that collects term factories from a query tree
// and builds a QueryTree from them. It mirrors the Java inner class
// QueryAnalyzer.QueryBuilder.
type queryBuilder struct {
	children          []func(TermWeightor) QueryTree
	unknownMapper     unknownQueryMapper
}

// newQueryBuilder creates a new queryBuilder.
func newQueryBuilder(mapper unknownQueryMapper) *queryBuilder {
	return &queryBuilder{
		children:      make([]func(TermWeightor) QueryTree, 0),
		unknownMapper: mapper,
	}
}

// GetSubVisitor returns a child visitor according to the Occur of the clause.
//
// MUST and FILTER clauses produce a new queryBuilder (conjunction child).
// MUST_NOT clauses are ignored unless the parent is a pure negative query.
// SHOULD clauses are ignored if the parent BooleanQuery has any MUST or FILTER
// clauses; otherwise they produce a disjunction child.
func (qb *queryBuilder) GetSubVisitor(occur search.Occur, parent search.Query) search.QueryVisitor {
	switch occur {
	case search.MUST, search.FILTER:
		n := newQueryBuilder(qb.unknownMapper)
		qb.children = append(qb.children, n.apply)
		return n

	case search.MUST_NOT:
		// Check if we're in a pure negative disjunction (no positive clauses).
		if bq, ok := parent.(*search.BooleanQuery); ok {
			positiveCount := 0
			for _, c := range bq.Clauses() {
				if c.Occur != search.MUST_NOT {
					positiveCount++
				}
			}
			if positiveCount == 0 {
				// Pure negative query — emit an ANY term so the presearcher
				// still matches the document.
				reason := fmt.Sprintf("PURE NEGATIVE QUERY[%v]", parent)
				qb.children = append(qb.children, func(_ TermWeightor) QueryTree {
					return NewAnyTermQueryTree(reason)
				})
			}
		}
		return search.EmptyQueryVisitor

	default: // SHOULD
		// If the parent has MUST or FILTER clauses, we can ignore disjunctions.
		if bq, ok := parent.(*search.BooleanQuery); ok {
			requiredCount := 0
			for _, c := range bq.Clauses() {
				if c.Occur == search.MUST || c.Occur == search.FILTER {
					requiredCount++
				}
			}
			if requiredCount > 0 {
				return search.EmptyQueryVisitor
			}
		}
		n := &disjunctionBuilder{unknownMapper: qb.unknownMapper}
		qb.children = append(qb.children, n.apply)
		return n
	}
}

// AcceptField always returns true — we want to visit all fields.
func (qb *queryBuilder) AcceptField(_ string) bool { return true }

// ConsumeTerms stores a term factory for each term reported.
func (qb *queryBuilder) ConsumeTerms(query search.Query, terms ...*index.Term) {
	for _, term := range terms {
		if term == nil {
			continue
		}
		t := term // capture
		qb.children = append(qb.children, func(w TermWeightor) QueryTree {
			return NewTermQueryTreeFromTerm(t, w)
		})
	}
}

// ConsumeTermsMatching handles automaton-based term matching by treating the
// field as an any-term node (the presearcher cannot efficiently index an
// automaton's term set).
func (qb *queryBuilder) ConsumeTermsMatching(query search.Query, field string, automaton func() search.ByteRunAutomaton) {
	qb.children = append(qb.children, func(_ TermWeightor) QueryTree {
		return NewAnyTermQueryTree(fmt.Sprintf("AUTOMATON[%s]", field))
	})
}

// VisitLeaf handles leaf queries that cannot provide individual terms via
// ConsumeTerms. The unknownMapper is tried first; if it returns nil, an
// any-term node is emitted as a fallback.
func (qb *queryBuilder) VisitLeaf(query search.Query) {
	q := query // capture
	qb.children = append(qb.children, func(w TermWeightor) QueryTree {
		if qt := qb.unknownMapper(q, w); qt != nil {
			return qt
		}
		return NewAnyTermQueryTree(safeQueryString(query))
	})
}

// apply builds a conjunction QueryTree from the collected children.
func (qb *queryBuilder) apply(weightor TermWeightor) QueryTree {
	if len(qb.children) == 0 {
		return NewAnyTermQueryTree("empty query")
	}
	return NewConjunctionQueryTree(qb.children, weightor)
}

// disjunctionBuilder is a queryBuilder whose apply method creates a
// disjunction instead of a conjunction. It mirrors the Java inner class
// QueryAnalyzer.Disjunction.
type disjunctionBuilder struct {
	children      []func(TermWeightor) QueryTree
	unknownMapper unknownQueryMapper
}

func (db *disjunctionBuilder) GetSubVisitor(occur search.Occur, parent search.Query) search.QueryVisitor {
	// Disjunctions delegate sub-visitor requests the same way as queryBuilder
	// for MUST/FILTER, MUST_NOT, and SHOULD.
	switch occur {
	case search.MUST, search.FILTER:
		n := newQueryBuilder(db.unknownMapper)
		db.children = append(db.children, n.apply)
		return n
	case search.MUST_NOT:
		// Check if we're in a pure negative disjunction (no positive clauses).
		if bq, ok := parent.(*search.BooleanQuery); ok {
			positiveCount := 0
			for _, c := range bq.Clauses() {
				if c.Occur != search.MUST_NOT {
					positiveCount++
				}
			}
			if positiveCount == 0 {
				reason := fmt.Sprintf("PURE NEGATIVE QUERY[%v]", parent)
				db.children = append(db.children, func(_ TermWeightor) QueryTree {
					return NewAnyTermQueryTree(reason)
				})
			}
		}
		return search.EmptyQueryVisitor
	default: // SHOULD
		// If the parent has MUST/FILTER, ignore disjunction children.
		if bq, ok := parent.(*search.BooleanQuery); ok {
			for _, c := range bq.Clauses() {
				if c.Occur == search.MUST || c.Occur == search.FILTER {
					return search.EmptyQueryVisitor
				}
			}
		}
		n := &disjunctionBuilder{unknownMapper: db.unknownMapper}
		db.children = append(db.children, n.apply)
		return n
	}
}

func (db *disjunctionBuilder) AcceptField(_ string) bool { return true }

func (db *disjunctionBuilder) ConsumeTerms(query search.Query, terms ...*index.Term) {
	for _, term := range terms {
		if term == nil {
			continue
		}
		t := term
		db.children = append(db.children, func(w TermWeightor) QueryTree {
			return NewTermQueryTreeFromTerm(t, w)
		})
	}
}

func (db *disjunctionBuilder) ConsumeTermsMatching(query search.Query, field string, automaton func() search.ByteRunAutomaton) {
	db.children = append(db.children, func(_ TermWeightor) QueryTree {
		return NewAnyTermQueryTree(fmt.Sprintf("AUTOMATON[%s]", field))
	})
}

func (db *disjunctionBuilder) VisitLeaf(query search.Query) {
	q := query
	db.children = append(db.children, func(w TermWeightor) QueryTree {
		if qt := db.unknownMapper(q, w); qt != nil {
			return qt
		}
		return NewAnyTermQueryTree(safeQueryString(query))
	})
}

func (db *disjunctionBuilder) apply(weightor TermWeightor) QueryTree {
	if len(db.children) == 0 {
		return NewAnyTermQueryTree("empty disjunction")
	}
	return NewDisjunctionQueryTree(db.children, weightor)
}
