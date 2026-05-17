// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import "sort"

// PrefixCodedTerms compactly stores a sorted list of (field, term) pairs
// using shared-prefix encoding. Mirrors
// org.apache.lucene.index.PrefixCodedTerms from Apache Lucene 10.4.0.
//
// Gocene skeleton: this initial port wires the Builder + TermIterator API
// and the in-memory term list, deferring the actual byte-stream encoding to
// a follow-up task; see backlog #2705. The semantic contract (sorted output,
// per-field grouping, delGen tag) is preserved.
type PrefixCodedTerms struct {
	// terms is the canonical sorted list of (field, bytes) pairs.
	terms []prefixCodedEntry
	// delGen is the deletion generation tag returned by TermIterator.DelGen.
	delGen int64
}

type prefixCodedEntry struct {
	field string
	bytes []byte
}

// Size returns the number of (field, term) pairs.
func (p *PrefixCodedTerms) Size() int64 { return int64(len(p.terms)) }

// SetDelGen records the deletion generation that downstream iterators will
// expose via TermIterator.DelGen.
func (p *PrefixCodedTerms) SetDelGen(gen int64) { p.delGen = gen }

// Iterator returns a TermIterator positioned before the first entry.
func (p *PrefixCodedTerms) Iterator() *PrefixCodedTermIterator {
	return &PrefixCodedTermIterator{owner: p, idx: -1}
}

// PrefixCodedTermIterator walks a PrefixCodedTerms in stored order.
type PrefixCodedTermIterator struct {
	owner *PrefixCodedTerms
	idx   int
}

// Next advances and returns the next term bytes, or nil when exhausted.
func (it *PrefixCodedTermIterator) Next() []byte {
	it.idx++
	if it.idx >= len(it.owner.terms) {
		return nil
	}
	return it.owner.terms[it.idx].bytes
}

// Field returns the field name for the term most recently returned by Next.
func (it *PrefixCodedTermIterator) Field() string {
	if it.idx < 0 || it.idx >= len(it.owner.terms) {
		return ""
	}
	return it.owner.terms[it.idx].field
}

// DelGen returns the deletion generation tag set via SetDelGen.
func (it *PrefixCodedTermIterator) DelGen() int64 { return it.owner.delGen }

// PrefixCodedTermsBuilder accumulates (field, term) pairs prior to Finish.
type PrefixCodedTermsBuilder struct {
	entries []prefixCodedEntry
}

// NewPrefixCodedTermsBuilder returns an empty builder.
func NewPrefixCodedTermsBuilder() *PrefixCodedTermsBuilder {
	return &PrefixCodedTermsBuilder{}
}

// Add records a Term. The pair is sorted at Finish time.
func (b *PrefixCodedTermsBuilder) Add(term *Term) {
	b.AddFieldBytes(term.Field, []byte(term.Text()))
}

// AddFieldBytes records a (field, bytes) pair directly.
func (b *PrefixCodedTermsBuilder) AddFieldBytes(field string, bytes []byte) {
	bcopy := make([]byte, len(bytes))
	copy(bcopy, bytes)
	b.entries = append(b.entries, prefixCodedEntry{field: field, bytes: bcopy})
}

// Finish sorts the accumulated entries and returns the PrefixCodedTerms.
func (b *PrefixCodedTermsBuilder) Finish() *PrefixCodedTerms {
	out := &PrefixCodedTerms{terms: b.entries}
	sort.SliceStable(out.terms, func(i, j int) bool {
		if out.terms[i].field != out.terms[j].field {
			return out.terms[i].field < out.terms[j].field
		}
		return bytesLess(out.terms[i].bytes, out.terms[j].bytes)
	})
	return out
}

// bytesLess compares two byte slices unsigned-lexicographically.
func bytesLess(a, b []byte) bool {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return len(a) < len(b)
}
