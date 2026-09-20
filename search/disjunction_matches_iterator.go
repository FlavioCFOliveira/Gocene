// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// Ported from Apache Lucene 10.5.0:
//   lucene/core/src/java/org/apache/lucene/search/DisjunctionMatchesIterator.java

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// disjunctionMatchesIterator is a MatchesIterator that merges the matches of
// several sub-iterators, ordering them by position (or, when positions are
// unavailable, by offset).
//
// The Java original is package-private (final class
// DisjunctionMatchesIterator); the Go port follows the same visibility model.
//
// Ported from org.apache.lucene.search.DisjunctionMatchesIterator.
type disjunctionMatchesIterator struct {
	queue   *util.PriorityQueue[MatchesIterator]
	started bool
}

// disjunctionFromTerms builds a disjunction over the postings of terms, all of
// which must belong to field.
//
// Mirrors DisjunctionMatchesIterator.fromTerms(LeafReaderContext, int, Query,
// String, List<Term>).
func disjunctionFromTerms(context *index.LeafReaderContext, doc int, query Query, field string, terms []*index.Term) (MatchesIterator, error) {
	if field == "" {
		return nil, fmt.Errorf("field must not be null")
	}
	for _, term := range terms {
		if term.Field != field {
			return nil, fmt.Errorf(
				"Tried to generate iterator from terms in multiple fields: expected [%s] but got [%s]",
				field, term.Field)
		}
	}
	return disjunctionFromTermsEnum(context, doc, query, field, asBytesRefIterator(terms))
}

// asBytesRefIterator adapts a term list to a BytesRefIterator over the term
// bytes.
//
// Mirrors DisjunctionMatchesIterator.asBytesRefIterator(List<Term>).
func asBytesRefIterator(terms []*index.Term) util.BytesRefIterator {
	return &termBytesRefIterator{terms: terms}
}

type termBytesRefIterator struct {
	terms []*index.Term
	i     int
}

func (t *termBytesRefIterator) Next() (*util.BytesRef, error) {
	if t.i >= len(t.terms) {
		return nil, nil
	}
	b := t.terms[t.i].BytesValue()
	t.i++
	return b, nil
}

// disjunctionFromTermsEnum builds a disjunction over the postings of every term
// produced by terms that is present in field and that matches doc.
//
// Mirrors DisjunctionMatchesIterator.fromTermsEnum(LeafReaderContext, int,
// Query, String, BytesRefIterator).
//
// Java threads a PostingsEnum "reuse" instance through the loop; Gocene's
// TermsEnum.Postings takes no reuse argument, so the reuse variable has no Go
// counterpart. It is a pure allocation optimisation with no observable effect.
func disjunctionFromTermsEnum(context *index.LeafReaderContext, doc int, query Query, field string, terms util.BytesRefIterator) (MatchesIterator, error) {
	if field == "" {
		return nil, fmt.Errorf("field must not be null")
	}
	t, err := index.GetTerms(context.LeafReader(), field)
	if err != nil {
		return nil, err
	}
	te, err := t.Iterator()
	if err != nil {
		return nil, err
	}
	for {
		term, err := terms.Next()
		if err != nil {
			return nil, err
		}
		if term == nil {
			break
		}
		found, err := te.SeekExact(spi.NewTermFromBytesRef(field, term))
		if err != nil {
			return nil, err
		}
		if !found {
			continue
		}
		pe, err := te.Postings(spi.PostingsFlagOffsets)
		if err != nil {
			return nil, err
		}
		advanced, err := pe.Advance(doc)
		if err != nil {
			return nil, err
		}
		if advanced == doc {
			first, err := newTermMatchesIterator(query, pe)
			if err != nil {
				return nil, err
			}
			return &termsEnumDisjunctionMatchesIterator{
				first: first,
				terms: terms,
				te:    te,
				doc:   doc,
				query: query,
				field: field,
			}, nil
		}
	}
	return nil, nil
}

// termsEnumDisjunctionMatchesIterator is a MatchesIterator over a set of terms
// that only loads the first matching term at construction, waiting until the
// iterator is actually used before it loads all other matching terms.
//
// Mirrors DisjunctionMatchesIterator.TermsEnumDisjunctionMatchesIterator.
type termsEnumDisjunctionMatchesIterator struct {
	first MatchesIterator
	terms util.BytesRefIterator
	te    index.TermsEnum
	doc   int
	query Query
	it    MatchesIterator
	// field has no Java counterpart: Java calls TermsEnum.seekExact(BytesRef),
	// while Gocene's TermsEnum.SeekExact takes a *Term, which carries the field
	// name. The field is therefore carried alongside the enum so the same seek
	// can be expressed.
	field string
}

func (t *termsEnumDisjunctionMatchesIterator) init() error {
	mis := []MatchesIterator{t.first}
	for {
		term, err := t.terms.Next()
		if err != nil {
			return err
		}
		if term == nil {
			break
		}
		found, err := t.te.SeekExact(spi.NewTermFromBytesRef(t.field, term))
		if err != nil {
			return err
		}
		if !found {
			continue
		}
		pe, err := t.te.Postings(spi.PostingsFlagOffsets)
		if err != nil {
			return err
		}
		advanced, err := pe.Advance(t.doc)
		if err != nil {
			return err
		}
		if advanced == t.doc {
			tmi, err := newTermMatchesIterator(t.query, pe)
			if err != nil {
				return err
			}
			mis = append(mis, tmi)
		}
	}
	it, err := disjunctionFromSubIterators(mis)
	if err != nil {
		return err
	}
	t.it = it
	return nil
}

func (t *termsEnumDisjunctionMatchesIterator) Next() (bool, error) {
	if t.it == nil {
		if err := t.init(); err != nil {
			return false, err
		}
	}
	return t.it.Next()
}

func (t *termsEnumDisjunctionMatchesIterator) StartPosition() int { return t.it.StartPosition() }
func (t *termsEnumDisjunctionMatchesIterator) EndPosition() int   { return t.it.EndPosition() }

func (t *termsEnumDisjunctionMatchesIterator) StartOffset() (int, error) { return t.it.StartOffset() }
func (t *termsEnumDisjunctionMatchesIterator) EndOffset() (int, error)   { return t.it.EndOffset() }

func (t *termsEnumDisjunctionMatchesIterator) GetSubMatches() (MatchesIterator, error) {
	return t.it.GetSubMatches()
}

func (t *termsEnumDisjunctionMatchesIterator) GetQuery() Query { return t.it.GetQuery() }

// disjunctionFromSubIterators merges the supplied sub-iterators.
//
// Mirrors DisjunctionMatchesIterator.fromSubIterators(List<MatchesIterator>).
func disjunctionFromSubIterators(mis []MatchesIterator) (MatchesIterator, error) {
	if len(mis) == 0 {
		return nil, nil
	}
	if len(mis) == 1 {
		return mis[0], nil
	}
	return newDisjunctionMatchesIterator(mis)
}

// newDisjunctionMatchesIterator mirrors the private
// DisjunctionMatchesIterator(List<MatchesIterator>) constructor.
func newDisjunctionMatchesIterator(matches []MatchesIterator) (MatchesIterator, error) {
	queue, err := util.NewPriorityQueue(len(matches), disjunctionMatchesLessThan)
	if err != nil {
		return nil, err
	}
	for _, mi := range matches {
		ok, err := mi.Next()
		if err != nil {
			return nil, err
		}
		if ok {
			queue.Add(mi)
		}
	}
	return &disjunctionMatchesIterator{queue: queue}, nil
}

// disjunctionMatchesLessThan mirrors the anonymous PriorityQueue.lessThan
// override in the DisjunctionMatchesIterator constructor.
//
// Java declares lessThan without a checked exception, so it wraps the
// IOException that startOffset()/endOffset() may throw in an unchecked
// IllegalArgumentException("Failed to retrieve term offset", e). The Go
// rendering panics with the same message, which is the faithful translation of
// an unchecked exception escaping the comparator.
func disjunctionMatchesLessThan(a, b MatchesIterator) bool {
	if a.StartPosition() == -1 && b.StartPosition() == -1 {
		aStart := mustOffset(a.StartOffset())
		bStart := mustOffset(b.StartOffset())
		aEnd := mustOffset(a.EndOffset())
		bEnd := mustOffset(b.EndOffset())
		return aStart < bStart ||
			(aStart == bStart && aEnd < bEnd) ||
			(aStart == bStart && aEnd == bEnd)
	}
	aStart := a.StartPosition()
	bStart := b.StartPosition()
	aEnd := a.EndPosition()
	bEnd := b.EndPosition()
	return aStart < bStart ||
		(aStart == bStart && aEnd < bEnd) ||
		(aStart == bStart && aEnd == bEnd)
}

// mustOffset mirrors the catch block that rethrows an offset IOException as an
// unchecked IllegalArgumentException.
func mustOffset(off int, err error) int {
	if err != nil {
		panic(fmt.Sprintf("Failed to retrieve term offset: %v", err))
	}
	return off
}

// Next advances to the next match in the merged stream.
func (d *disjunctionMatchesIterator) Next() (bool, error) {
	if !d.started {
		d.started = true
		return d.queue.Size() > 0, nil
	}
	ok, err := d.queue.Top().Next()
	if err != nil {
		return false, err
	}
	if !ok {
		d.queue.Pop()
	}
	if d.queue.Size() > 0 {
		d.queue.UpdateTop()
		return true, nil
	}
	return false, nil
}

// StartPosition returns the start position of the current top match.
func (d *disjunctionMatchesIterator) StartPosition() int { return d.queue.Top().StartPosition() }

// EndPosition returns the end position of the current top match.
func (d *disjunctionMatchesIterator) EndPosition() int { return d.queue.Top().EndPosition() }

// StartOffset returns the start offset of the current top match.
func (d *disjunctionMatchesIterator) StartOffset() (int, error) {
	return d.queue.Top().StartOffset()
}

// EndOffset returns the end offset of the current top match.
func (d *disjunctionMatchesIterator) EndOffset() (int, error) { return d.queue.Top().EndOffset() }

// GetSubMatches returns the sub-matches of the current top match.
func (d *disjunctionMatchesIterator) GetSubMatches() (MatchesIterator, error) {
	return d.queue.Top().GetSubMatches()
}

// GetQuery returns the query of the current top match.
func (d *disjunctionMatchesIterator) GetQuery() Query { return d.queue.Top().GetQuery() }

var (
	_ MatchesIterator = (*disjunctionMatchesIterator)(nil)
	_ MatchesIterator = (*termsEnumDisjunctionMatchesIterator)(nil)
)
