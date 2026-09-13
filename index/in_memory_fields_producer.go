// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"sort"

	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// InMemoryFieldsProducer implements FieldsProducer using merged in-memory
// postings from a set of DocumentsWriterPerThread instances.
//
// DWPT internal docIDs are 0-based (lastDocID starts at -1 and is incremented
// before use, so the first document gets docID=0).  The global docID for a
// document in pool[i] is docBase(i) + localDoc, where docBase(i) is the total
// number of documents across all DWPTs at indices 0..i-1.
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
	text      string
	docIDs    []int   // sorted ascending
	freqs     []int   // parallel to docIDs
	positions [][]int // positions[i] is the sorted positions for docIDs[i]; may be nil
}

// MergeInMemoryPostings builds an InMemoryFieldsProducer by materialising the
// buffered postings of every DWPT in the pool.
//
// The source of truth is the same in-RAM structure the flush path consumes:
// each DWPT's indexing chain holds one FreqProxTermsWriterPerField per
// inverted field, and FreqProxFields is the Fields view over them (the Go port
// of org.apache.lucene.index.FreqProxFields). Walking that view term by term
// yields exactly the postings a codec would write.
//
// DWPT internal docIDs are 0-based, so the global docID for a document in
// pool[i] is docBase(i) + localDoc, where docBase(i) is the cumulative
// GetNumDocsInRAM of all previous DWPTs. When the pool holds a single DWPT —
// the common case for one flush unit — docBase stays 0 throughout.
func MergeInMemoryPostings(dwptPool []*DocumentsWriterPerThread) (*InMemoryFieldsProducer, error) {
	p := &InMemoryFieldsProducer{
		fields: make(map[string]*inMemField),
	}

	docBase := 0
	for _, dwpt := range dwptPool {
		if dwpt == nil {
			continue
		}
		perFields, err := bufferedPostingsFields(dwpt)
		if err != nil {
			return nil, err
		}
		for _, fp := range perFields {
			if err := p.absorbField(fp, docBase); err != nil {
				return nil, err
			}
		}
		docBase += dwpt.GetNumDocsInRAM()
	}

	return p, nil
}

// freqProxWriterLookup is the registry lookup FreqProxTermsWriter keeps so a
// TermsHashPerField can be resolved back to its FreqProx wrapper. Lucene does
// this with a downcast from TermsHashPerField to
// FreqProxTermsWriterPerField; Go needs the explicit registry, and the
// interface keeps this file independent of which concrete TermsHash the
// indexing chain happens to be wired with.
type freqProxWriterLookup interface {
	lookupFreqProxByBase(base *TermsHashPerField) (*FreqProxTermsWriterPerField, bool)
}

// bufferedPostingsFields returns the per-field postings writers dwpt has
// buffered, ordered by field name.
//
// It mirrors the hand-off IndexingChain.Flush performs: the chain's field hash
// is walked for every field that was actually inverted, and each field's
// TermsHashPerField is resolved back to its FreqProx wrapper through the
// writer's registry (Lucene does this with a downcast). The field-name
// ordering matches the invariant FreqProxTermsWriter establishes before
// building FreqProxFields ("NOTE: fields are already sorted by field name").
func bufferedPostingsFields(dwpt *DocumentsWriterPerThread) ([]*FreqProxTermsWriterPerField, error) {
	chain := dwpt.indexingChain
	if chain == nil {
		return nil, nil
	}
	if chain.termsHash == nil {
		// The inversion chain was never wired, so no field can have been
		// inverted and there is nothing to materialise.
		return nil, nil
	}
	writer, ok := chain.termsHash.(freqProxWriterLookup)
	if !ok {
		return nil, fmt.Errorf("MergeInMemoryPostings: indexing chain terms hash is %T, which exposes no FreqProx per-field registry", chain.termsHash)
	}

	byName := make(map[string]*FreqProxTermsWriterPerField)
	names := make([]string, 0, len(chain.fieldHash))
	for _, bucket := range chain.fieldHash {
		for pf := bucket; pf != nil; pf = pf.next {
			if pf.invertState == nil || pf.termsHashPerField == nil || pf.fieldInfo == nil {
				continue
			}
			fp, ok := writer.lookupFreqProxByBase(pf.termsHashPerField)
			if !ok {
				continue
			}
			name := pf.fieldInfo.Name()
			if _, dup := byName[name]; dup {
				continue
			}
			byName[name] = fp
			names = append(names, name)
		}
	}
	sort.Strings(names)

	ordered := make([]*FreqProxTermsWriterPerField, 0, len(names))
	for _, name := range names {
		ordered = append(ordered, byName[name])
	}
	return ordered, nil
}

// absorbField materialises every term of one buffered field into the
// producer's in-memory postings, shifting each local docID by docBase.
func (p *InMemoryFieldsProducer) absorbField(fp *FreqProxTermsWriterPerField, docBase int) error {
	if fp == nil {
		return nil
	}
	fieldName := fp.GetFieldName()

	// Request the richest postings the field actually carries: FreqProxTermsEnum
	// refuses positions or freqs that were never indexed, exactly as Lucene's
	// FreqProxFields.FreqProxTermsEnum.postings does.
	flags := PostingsFlagNone
	switch {
	case fp.hasProx:
		flags = PostingsFlagPositions
	case fp.hasFreq:
		flags = PostingsFlagFreqs
	}

	enum, err := newFreqProxTerms(fp).GetIterator()
	if err != nil {
		return fmt.Errorf("MergeInMemoryPostings: field %q: iterator: %w", fieldName, err)
	}
	for {
		term, err := enum.Next()
		if err != nil {
			return fmt.Errorf("MergeInMemoryPostings: field %q: next term: %w", fieldName, err)
		}
		if term == nil {
			return nil
		}
		postings, err := enum.Postings(flags)
		if err != nil {
			return fmt.Errorf("MergeInMemoryPostings: field %q term %q: postings: %w", fieldName, term.Text(), err)
		}
		if err := p.absorbTerm(fieldName, term.Text(), postings, flags, docBase); err != nil {
			return err
		}
	}
}

// absorbTerm drains one posting list into the producer's in-memory buffers.
func (p *InMemoryFieldsProducer) absorbTerm(fieldName, termText string, postings PostingsEnum, flags, docBase int) error {
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

	for {
		doc, err := postings.NextDoc()
		if err != nil {
			return fmt.Errorf("MergeInMemoryPostings: field %q term %q: next doc: %w", fieldName, termText, err)
		}
		if doc == NO_MORE_DOCS {
			return nil
		}

		freq := 1
		if flags != PostingsFlagNone {
			freq, err = postings.Freq()
			if err != nil {
				return fmt.Errorf("MergeInMemoryPostings: field %q term %q: freq: %w", fieldName, termText, err)
			}
		}

		var positions []int
		if flags == PostingsFlagPositions {
			positions = make([]int, 0, freq)
			for i := 0; i < freq; i++ {
				pos, err := postings.NextPosition()
				if err != nil {
					return fmt.Errorf("MergeInMemoryPostings: field %q term %q: next position: %w", fieldName, termText, err)
				}
				if pos == NO_MORE_POSITIONS {
					break
				}
				positions = append(positions, pos)
			}
			if len(positions) == 0 {
				positions = nil
			}
		}

		imt.docIDs = append(imt.docIDs, docBase+doc)
		imt.freqs = append(imt.freqs, freq)
		imt.positions = append(imt.positions, positions)
	}
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

func (t *inMemTerms) HasFreqs() bool   { return true }
func (t *inMemTerms) HasOffsets() bool { return false }
func (t *inMemTerms) HasPositions() bool {
	// Returns true if any term in the field has positions stored.
	for _, imt := range t.field.terms {
		if imt.positions != nil {
			return true
		}
	}
	return false
}
func (t *inMemTerms) HasPayloads() bool { return false }

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

// Impacts returns an ImpactsEnum over the current term's postings. Mirrors
// org.apache.lucene.index.BaseTermsEnum#impacts, whose default wraps the
// postings in a SlowImpactsEnum because in-memory postings carry no impact
// index.
func (e *inMemTermsEnum) Impacts(flags int) (spi.ImpactsEnum, error) {
	postings, err := e.Postings(flags)
	if err != nil {
		return nil, err
	}
	return spiImpactsEnum{ImpactsEnum: NewSlowImpactsEnum(postings)}, nil
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
		e.TermsEnumBase.SetCurrentTerm(nil)
		return nil, nil
	}
	text := e.sorted[e.idx]
	e.currentTerm = e.field.terms[text]
	t := NewTerm(e.field.fieldName, text)
	e.TermsEnumBase.SetCurrentTerm(t)
	return t, nil
}

func (e *inMemTermsEnum) SeekExact(term *Term) (bool, error) {
	if term == nil {
		return false, nil
	}
	imt, ok := e.field.terms[term.Text()]
	if !ok {
		e.currentTerm = nil
		e.TermsEnumBase.SetCurrentTerm(nil)
		return false, nil
	}
	e.currentTerm = imt
	e.TermsEnumBase.SetCurrentTerm(term)
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
		e.TermsEnumBase.SetCurrentTerm(nil)
		return nil, nil
	}
	e.idx = pos
	text := e.sorted[pos]
	e.currentTerm = e.field.terms[text]
	t := NewTerm(e.field.fieldName, text)
	e.TermsEnumBase.SetCurrentTerm(t)
	return t, nil
}

func (e *inMemTermsEnum) Term() *Term {
	return e.TermsEnumBase.Term()
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
// When positions were indexed, NextPosition() returns them in order.
type inMemPostingsEnum struct {
	PostingsEnumBase
	term   *inMemTerm
	idx    int // current position in docIDs (-1 = before start)
	docID  int
	posIdx int // index into current doc's positions slice
}

func newInMemPostingsEnum(t *inMemTerm) *inMemPostingsEnum {
	return &inMemPostingsEnum{
		term:   t,
		idx:    -1,
		docID:  -1,
		posIdx: 0,
	}
}

func (e *inMemPostingsEnum) NextDoc() (int, error) {
	e.idx++
	e.posIdx = 0 // reset position cursor for new doc
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

// NextPosition returns the next position for the current document.
// Returns NO_MORE_POSITIONS when all positions have been consumed or when
// positions were not indexed for this term.
func (e *inMemPostingsEnum) NextPosition() (int, error) {
	if e.idx < 0 || e.idx >= len(e.term.positions) {
		return NO_MORE_POSITIONS, nil
	}
	positions := e.term.positions[e.idx]
	if positions == nil || e.posIdx >= len(positions) {
		return NO_MORE_POSITIONS, nil
	}
	pos := positions[e.posIdx]
	e.posIdx++
	return pos, nil
}

func (e *inMemPostingsEnum) StartOffset() (int, error)   { return -1, nil }
func (e *inMemPostingsEnum) EndOffset() (int, error)     { return -1, nil }
func (e *inMemPostingsEnum) GetPayload() ([]byte, error) { return nil, nil }

func (e *inMemPostingsEnum) Cost() int64 {
	return int64(len(e.term.docIDs))
}

// IntoBitSet carries the default body of
// DocIdSetIterator.intoBitSet(int, FixedBitSet, int) in Apache Lucene
// 10.5.0, which every subclass inherits unless it overrides it.
func (i *inMemPostingsEnum) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return util.DefaultIntoBitSet(i, upTo, bitSet, offset)
}
