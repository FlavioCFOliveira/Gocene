// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"sort"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// InMemoryFieldsProducer implements FieldsProducer using merged in-memory
// postings from a set of DocumentsWriterPerThread instances.
//
// Each DWPT in the pool processed exactly one document whose internal docID
// is 1.  The global docID for pool[i] is i (0-based).  This producer
// remaps local docID 1 → global docID i for each entry.
//
// This is used as a lightweight alternative to a full codec round-trip when
// the caller never set a codec (e.g. in unit tests that create an
// IndexWriter without a codec via IndexWriterConfig).
type InMemoryFieldsProducer struct {
	// fields maps field name → sorted term slice
	fields map[string]*inMemField
}

// inMemField holds all postings for a single field.
type inMemField struct {
	fieldName string
	terms     map[string]*inMemTerm // keyed by term text
}

// inMemTerm holds the posting list for a single (field, term) pair.
type inMemTerm struct {
	text   string
	docIDs []int // sorted ascending
	freqs  []int // parallel to docIDs
}

// MergeInMemoryPostings builds an InMemoryFieldsProducer by merging
// postings from all DWPTs in the pool.  dwptPool[i] processed global
// document i; its local docID inside the DWPT is always 1.
func MergeInMemoryPostings(dwptPool []*DocumentsWriterPerThread) *InMemoryFieldsProducer {
	p := &InMemoryFieldsProducer{
		fields: make(map[string]*inMemField),
	}

	for globalDocID, dwpt := range dwptPool {
		dwpt.invertedIndex.mu.RLock()
		for fieldName, fp := range dwpt.invertedIndex.fields {
			fp.mu.RLock()
			for termText, posting := range fp.terms {
				// Each DWPT has at most one document (local docID=1).
				// Map local docID 1 → globalDocID.
				for i, localDoc := range posting.docIDs {
					_ = localDoc // always 1, ignored
					freq := 1
					if i < len(posting.freqs) {
						freq = posting.freqs[i]
					}

					imf, ok := p.fields[fieldName]
					if !ok {
						imf = &inMemField{
							fieldName: fieldName,
							terms:     make(map[string]*inMemTerm),
						}
						p.fields[fieldName] = imf
					}
					imt, ok := imf.terms[termText]
					if !ok {
						imt = &inMemTerm{text: termText}
						imf.terms[termText] = imt
					}
					imt.docIDs = append(imt.docIDs, globalDocID)
					imt.freqs = append(imt.freqs, freq)
				}
			}
			fp.mu.RUnlock()
		}
		dwpt.invertedIndex.mu.RUnlock()
	}

	return p
}

// Terms returns an in-memory Terms implementation for the given field.
// Returns nil when the field has no indexed terms.
func (p *InMemoryFieldsProducer) Terms(field string) (Terms, error) {
	imf, ok := p.fields[field]
	if !ok {
		return nil, nil
	}
	return newInMemTerms(imf), nil
}

// Close is a no-op for in-memory data.
func (p *InMemoryFieldsProducer) Close() error { return nil }

// ─── inMemTerms ──────────────────────────────────────────────────────────────

type inMemTerms struct {
	TermsBase
	field *inMemField
}

func newInMemTerms(f *inMemField) *inMemTerms {
	return &inMemTerms{field: f}
}

func (t *inMemTerms) GetIterator() (TermsEnum, error) {
	return newInMemTermsEnum(t.field, ""), nil
}

func (t *inMemTerms) GetIteratorWithSeek(seekTerm *Term) (TermsEnum, error) {
	text := ""
	if seekTerm != nil {
		text = seekTerm.Text()
	}
	return newInMemTermsEnum(t.field, text), nil
}

func (t *inMemTerms) GetPostingsReader(termText string, _ int) (PostingsEnum, error) {
	imt, ok := t.field.terms[termText]
	if !ok {
		return nil, nil
	}
	return newInMemPostingsEnum(imt), nil
}

func (t *inMemTerms) Size() int64 { return int64(len(t.field.terms)) }

func (t *inMemTerms) GetDocCount() (int, error) {
	seen := make(map[int]struct{})
	for _, imt := range t.field.terms {
		for _, d := range imt.docIDs {
			seen[d] = struct{}{}
		}
	}
	return len(seen), nil
}

func (t *inMemTerms) GetSumDocFreq() (int64, error) {
	var sum int64
	for _, imt := range t.field.terms {
		sum += int64(len(imt.docIDs))
	}
	return sum, nil
}

func (t *inMemTerms) GetSumTotalTermFreq() (int64, error) {
	var sum int64
	for _, imt := range t.field.terms {
		for _, f := range imt.freqs {
			sum += int64(f)
		}
	}
	return sum, nil
}

func (t *inMemTerms) HasFreqs() bool     { return true }
func (t *inMemTerms) HasOffsets() bool   { return false }
func (t *inMemTerms) HasPositions() bool { return false }
func (t *inMemTerms) HasPayloads() bool  { return false }

func (t *inMemTerms) GetMin() (*Term, error) {
	terms := t.sortedTermTexts()
	if len(terms) == 0 {
		return nil, nil
	}
	return NewTerm(t.field.fieldName, terms[0]), nil
}

func (t *inMemTerms) GetMax() (*Term, error) {
	terms := t.sortedTermTexts()
	if len(terms) == 0 {
		return nil, nil
	}
	return NewTerm(t.field.fieldName, terms[len(terms)-1]), nil
}

func (t *inMemTerms) sortedTermTexts() []string {
	out := make([]string, 0, len(t.field.terms))
	for k := range t.field.terms {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// ─── inMemTermsEnum ──────────────────────────────────────────────────────────

// inMemTermsEnum iterates over sorted terms in a single field.
type inMemTermsEnum struct {
	TermsEnumBase
	field       *inMemField
	sorted      []string // sorted term texts
	idx         int      // current position in sorted (-1 = before start)
	currentTerm *inMemTerm
}

func newInMemTermsEnum(field *inMemField, seekText string) *inMemTermsEnum {
	sorted := make([]string, 0, len(field.terms))
	for k := range field.terms {
		sorted = append(sorted, k)
	}
	sort.Strings(sorted)

	e := &inMemTermsEnum{
		field:  field,
		sorted: sorted,
		idx:    -1,
	}

	if seekText != "" {
		// Position at or after seekText.
		pos := sort.SearchStrings(sorted, seekText)
		e.idx = pos - 1 // Next() will advance to pos.
	}
	return e
}

func (e *inMemTermsEnum) Next() (*Term, error) {
	e.idx++
	if e.idx >= len(e.sorted) {
		e.currentTerm = nil
		e.TermsEnumBase.currentTerm = nil
		return nil, nil
	}
	text := e.sorted[e.idx]
	e.currentTerm = e.field.terms[text]
	t := NewTerm(e.field.fieldName, text)
	e.TermsEnumBase.currentTerm = t
	return t, nil
}

func (e *inMemTermsEnum) SeekExact(term *Term) (bool, error) {
	if term == nil {
		return false, nil
	}
	imt, ok := e.field.terms[term.Text()]
	if !ok {
		e.currentTerm = nil
		e.TermsEnumBase.currentTerm = nil
		return false, nil
	}
	e.currentTerm = imt
	e.TermsEnumBase.currentTerm = term
	// Update idx to the correct position so Term() remains consistent.
	pos := sort.SearchStrings(e.sorted, term.Text())
	if pos < len(e.sorted) && e.sorted[pos] == term.Text() {
		e.idx = pos
	}
	return true, nil
}

func (e *inMemTermsEnum) SeekCeil(term *Term) (*Term, error) {
	if term == nil {
		return e.Next()
	}
	pos := sort.SearchStrings(e.sorted, term.Text())
	if pos >= len(e.sorted) {
		e.currentTerm = nil
		e.TermsEnumBase.currentTerm = nil
		return nil, nil
	}
	e.idx = pos
	text := e.sorted[pos]
	e.currentTerm = e.field.terms[text]
	t := NewTerm(e.field.fieldName, text)
	e.TermsEnumBase.currentTerm = t
	return t, nil
}

func (e *inMemTermsEnum) Term() *Term {
	return e.TermsEnumBase.currentTerm
}

func (e *inMemTermsEnum) DocFreq() (int, error) {
	if e.currentTerm == nil {
		return 0, nil
	}
	return len(e.currentTerm.docIDs), nil
}

func (e *inMemTermsEnum) TotalTermFreq() (int64, error) {
	if e.currentTerm == nil {
		return 0, nil
	}
	var sum int64
	for _, f := range e.currentTerm.freqs {
		sum += int64(f)
	}
	return sum, nil
}

func (e *inMemTermsEnum) Postings(_ int) (PostingsEnum, error) {
	if e.currentTerm == nil {
		return nil, nil
	}
	return newInMemPostingsEnum(e.currentTerm), nil
}

func (e *inMemTermsEnum) PostingsWithLiveDocs(_ util.Bits, flags int) (PostingsEnum, error) {
	return e.Postings(flags)
}

// ─── inMemPostingsEnum ───────────────────────────────────────────────────────

// inMemPostingsEnum iterates over the document list of a single term.
type inMemPostingsEnum struct {
	PostingsEnumBase
	term  *inMemTerm
	idx   int // current position in docIDs (-1 = before start)
	docID int
}

func newInMemPostingsEnum(t *inMemTerm) *inMemPostingsEnum {
	return &inMemPostingsEnum{
		term:  t,
		idx:   -1,
		docID: -1,
	}
}

func (e *inMemPostingsEnum) NextDoc() (int, error) {
	e.idx++
	if e.idx >= len(e.term.docIDs) {
		e.docID = NO_MORE_DOCS
		return NO_MORE_DOCS, nil
	}
	e.docID = e.term.docIDs[e.idx]
	return e.docID, nil
}

func (e *inMemPostingsEnum) Advance(target int) (int, error) {
	for {
		doc, err := e.NextDoc()
		if err != nil || doc == NO_MORE_DOCS || doc >= target {
			return doc, err
		}
	}
}

func (e *inMemPostingsEnum) DocID() int { return e.docID }

func (e *inMemPostingsEnum) Freq() (int, error) {
	if e.idx < 0 || e.idx >= len(e.term.freqs) {
		return 1, nil
	}
	return e.term.freqs[e.idx], nil
}

func (e *inMemPostingsEnum) NextPosition() (int, error)  { return NO_MORE_POSITIONS, nil }
func (e *inMemPostingsEnum) StartOffset() (int, error)   { return -1, nil }
func (e *inMemPostingsEnum) EndOffset() (int, error)     { return -1, nil }
func (e *inMemPostingsEnum) GetPayload() ([]byte, error) { return nil, nil }

func (e *inMemPostingsEnum) Cost() int64 {
	return int64(len(e.term.docIDs))
}
