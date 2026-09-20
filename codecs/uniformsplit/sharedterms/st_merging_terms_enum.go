// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package sharedterms

import (
	"errors"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// errSTMergingTermsEnumUnsupported renders the UnsupportedOperationException
// thrown by most STMergingTermsEnum members and by
// MultiSegmentsPostingsEnum.advance (STMergingTermsEnum.java:59-115, 223).
var errSTMergingTermsEnumUnsupported = errors.New("STMergingTermsEnum: unsupported operation")

// stMergingTermsEnum combines spi.PostingsEnum for the same term for a given
// field from multiple segments. It is used during segment merging.
//
// Mirrors the package-private class
// org.apache.lucene.codecs.uniformsplit.sharedterms.STMergingTermsEnum from
// Apache Lucene 10.5.0, which extends org.apache.lucene.index.BaseTermsEnum.
// The Java class is package-private, so its Go spelling is unexported.
type stMergingTermsEnum struct {
	// fieldName mirrors the protected final field
	// STMergingTermsEnum.fieldName (STMergingTermsEnum.java:38).
	fieldName string

	// multiPostingsEnum mirrors the protected final field
	// STMergingTermsEnum.multiPostingsEnum (STMergingTermsEnum.java:39).
	multiPostingsEnum *multiSegmentsPostingsEnum

	// term mirrors the protected field STMergingTermsEnum.term
	// (STMergingTermsEnum.java:40).
	term *util.BytesRef
}

// newSTMergingTermsEnum constructs a stMergingTermsEnum for a given field.
//
// Mirrors the protected STMergingTermsEnum(String, int) constructor
// (STMergingTermsEnum.java:43).
func newSTMergingTermsEnum(fieldName string, numSegments int) *stMergingTermsEnum {
	e := &stMergingTermsEnum{fieldName: fieldName}
	e.multiPostingsEnum = newMultiSegmentsPostingsEnum(e, numSegments)
	return e
}

// reset resets this stMergingTermsEnum with a new term and its list of
// segmentPostings to combine. segmentPostings is sorted by segment index.
//
// Mirrors STMergingTermsEnum.reset(BytesRef, List)
// (STMergingTermsEnum.java:54).
func (e *stMergingTermsEnum) reset(term *util.BytesRef, segmentPostings []*segmentPostings) {
	e.term = term
	e.multiPostingsEnum.reset(segmentPostings)
}

// Attributes mirrors STMergingTermsEnum.attributes
// (STMergingTermsEnum.java:59), whose body is `throw new
// UnsupportedOperationException()`. The Gocene contract returns no error, so
// the refusal is a nil AttributeSource: any use dereferences nil and fails at
// the same point Java throws.
func (e *stMergingTermsEnum) Attributes() *util.AttributeSource {
	return nil
}

// SeekCeil mirrors STMergingTermsEnum.seekCeil(BytesRef)
// (STMergingTermsEnum.java:64), whose body is `throw new
// UnsupportedOperationException()`.
func (e *stMergingTermsEnum) SeekCeil(_ *spi.Term) (*spi.Term, error) {
	return nil, errSTMergingTermsEnumUnsupported
}

// SeekExactOrd mirrors STMergingTermsEnum.seekExact(long)
// (STMergingTermsEnum.java:69), whose body is `throw new
// UnsupportedOperationException()`.
func (e *stMergingTermsEnum) SeekExactOrd(_ int64) error {
	return errSTMergingTermsEnumUnsupported
}

// SeekExactWithState mirrors STMergingTermsEnum.seekExact(BytesRef,
// TermState) (STMergingTermsEnum.java:74), whose body is `throw new
// UnsupportedOperationException()`.
func (e *stMergingTermsEnum) SeekExactWithState(_ *spi.Term, _ index.TermState) error {
	return errSTMergingTermsEnumUnsupported
}

// SeekExact carries the body BaseTermsEnum.seekExact(BytesRef) inherits
// (BaseTermsEnum.java:41): it calls seekCeil, which this class refuses.
func (e *stMergingTermsEnum) SeekExact(term *spi.Term) (bool, error) {
	if _, err := e.SeekCeil(term); err != nil {
		return false, err
	}
	return false, nil
}

// Term mirrors STMergingTermsEnum.term (STMergingTermsEnum.java:79), whose
// body is `return term`.
//
// PORT NOTE: the Java method returns a bare BytesRef while the Gocene
// spi.TermsEnum contract returns a *spi.Term, which carries a field name.
// This enumerator is built for exactly one field, so the Term it returns
// carries that field name; its bytes are the bytes Java returns.
func (e *stMergingTermsEnum) Term() *spi.Term {
	if e.term == nil {
		return nil
	}
	return spi.NewTermFromBytesRef(e.fieldName, e.term)
}

// Ord mirrors STMergingTermsEnum.ord (STMergingTermsEnum.java:84), whose body
// is `throw new UnsupportedOperationException()`. The Gocene contract returns
// no error, so the refusal is the -1 that spi.TermsEnum documents for an
// enumerator without ordinals.
func (e *stMergingTermsEnum) Ord() int64 {
	return -1
}

// DocFreq mirrors STMergingTermsEnum.docFreq (STMergingTermsEnum.java:89),
// whose body is `throw new UnsupportedOperationException()`.
func (e *stMergingTermsEnum) DocFreq() (int, error) {
	return 0, errSTMergingTermsEnumUnsupported
}

// TotalTermFreq mirrors STMergingTermsEnum.totalTermFreq
// (STMergingTermsEnum.java:94), whose body is `throw new
// UnsupportedOperationException()`.
func (e *stMergingTermsEnum) TotalTermFreq() (int64, error) {
	return 0, errSTMergingTermsEnumUnsupported
}

// Postings mirrors STMergingTermsEnum.postings(PostingsEnum, int)
// (STMergingTermsEnum.java:99). Java's reuse parameter is ignored by this
// body — the enumerator always returns its own multiPostingsEnum — and the
// Gocene spi.TermsEnum contract does not carry it.
func (e *stMergingTermsEnum) Postings(flags int) (spi.PostingsEnum, error) {
	e.multiPostingsEnum.setPostingFlags(flags)
	return e.multiPostingsEnum, nil
}

// PostingsWithLiveDocs carries the Gocene spi.TermsEnum member that renders
// the live-docs filtered form of TermsEnum.postings. The postings this
// enumerator combines come from segments whose deletions are already folded
// into the per-segment MergeState.DocMap, exactly as in Java, so the live-docs
// argument adds nothing and the body is Postings.
func (e *stMergingTermsEnum) PostingsWithLiveDocs(_ util.Bits, flags int) (spi.PostingsEnum, error) {
	return e.Postings(flags)
}

// Impacts mirrors STMergingTermsEnum.impacts(int)
// (STMergingTermsEnum.java:105), whose body is `throw new
// UnsupportedOperationException()`.
func (e *stMergingTermsEnum) Impacts(_ int) (spi.ImpactsEnum, error) {
	return nil, errSTMergingTermsEnumUnsupported
}

// TermState mirrors STMergingTermsEnum.termState
// (STMergingTermsEnum.java:110), whose body is `throw new
// UnsupportedOperationException()`.
func (e *stMergingTermsEnum) TermState() (index.TermState, error) {
	return nil, errSTMergingTermsEnumUnsupported
}

// Next mirrors STMergingTermsEnum.next (STMergingTermsEnum.java:115), whose
// body is `throw new UnsupportedOperationException()`.
func (e *stMergingTermsEnum) Next() (*spi.Term, error) {
	return nil, errSTMergingTermsEnumUnsupported
}

// multiSegmentsPostingsEnum combines multiple segments spi.PostingsEnum as a
// single spi.PostingsEnum, for one field and one term.
//
// This spi.PostingsEnum does not extend index.FilterPostingsEnum because it
// updates the delegate for each segment.
//
// Mirrors the protected inner class
// STMergingTermsEnum.MultiSegmentsPostingsEnum
// (STMergingTermsEnum.java:125), which extends
// org.apache.lucene.index.PostingsEnum. Java's inner class reads fieldName
// from its enclosing instance, so the Go rendering keeps that instance in
// outer.
type multiSegmentsPostingsEnum struct {
	// outer renders the implicit STMergingTermsEnum.this of the Java inner
	// class, which getPostings reads for fieldName.
	outer *stMergingTermsEnum

	reusablePostingsEnums []spi.PostingsEnum
	segmentPostingsList   []*segmentPostings
	segmentIndex          int
	postingsEnum          spi.PostingsEnum
	postingsEnumExhausted bool
	docMap                index.DocMap
	docID                 int
	postingsFlags         int
}

// newMultiSegmentsPostingsEnum mirrors the protected
// MultiSegmentsPostingsEnum(int) constructor (STMergingTermsEnum.java:136).
func newMultiSegmentsPostingsEnum(outer *stMergingTermsEnum, numSegments int) *multiSegmentsPostingsEnum {
	return &multiSegmentsPostingsEnum{
		outer:                 outer,
		reusablePostingsEnums: make([]spi.PostingsEnum, numSegments),
	}
}

// reset resets/reuses this spi.PostingsEnum. segmentPostingsList is the list
// of segment postings ordered by segment index.
//
// Mirrors MultiSegmentsPostingsEnum.reset(List)
// (STMergingTermsEnum.java:145).
func (p *multiSegmentsPostingsEnum) reset(segmentPostingsList []*segmentPostings) {
	p.segmentPostingsList = segmentPostingsList
	p.segmentIndex = -1
	p.postingsEnumExhausted = true
	p.docID = -1
}

// setPostingFlags mirrors MultiSegmentsPostingsEnum.setPostingFlags(int)
// (STMergingTermsEnum.java:153).
func (p *multiSegmentsPostingsEnum) setPostingFlags(flags int) {
	p.postingsFlags = flags
}

// Freq mirrors MultiSegmentsPostingsEnum.freq (STMergingTermsEnum.java:158).
func (p *multiSegmentsPostingsEnum) Freq() (int, error) {
	return p.postingsEnum.Freq()
}

// NextPosition mirrors MultiSegmentsPostingsEnum.nextPosition
// (STMergingTermsEnum.java:163).
func (p *multiSegmentsPostingsEnum) NextPosition() (int, error) {
	return p.postingsEnum.NextPosition()
}

// StartOffset mirrors MultiSegmentsPostingsEnum.startOffset
// (STMergingTermsEnum.java:168).
func (p *multiSegmentsPostingsEnum) StartOffset() (int, error) {
	return p.postingsEnum.StartOffset()
}

// EndOffset mirrors MultiSegmentsPostingsEnum.endOffset
// (STMergingTermsEnum.java:173).
func (p *multiSegmentsPostingsEnum) EndOffset() (int, error) {
	return p.postingsEnum.EndOffset()
}

// GetPayload mirrors MultiSegmentsPostingsEnum.getPayload
// (STMergingTermsEnum.java:178).
func (p *multiSegmentsPostingsEnum) GetPayload() ([]byte, error) {
	return p.postingsEnum.GetPayload()
}

// DocID mirrors MultiSegmentsPostingsEnum.docID
// (STMergingTermsEnum.java:183).
func (p *multiSegmentsPostingsEnum) DocID() int {
	return p.docID
}

// NextDoc mirrors MultiSegmentsPostingsEnum.nextDoc
// (STMergingTermsEnum.java:188).
func (p *multiSegmentsPostingsEnum) NextDoc() (int, error) {
	// assert segmentPostingsList != null : "reset not called"
	for {
		if p.postingsEnumExhausted {
			if p.segmentIndex == len(p.segmentPostingsList)-1 {
				p.docID = spi.NO_MORE_DOCS
				return p.docID, nil
			}
			p.segmentIndex++
			sp := p.segmentPostingsList[p.segmentIndex]
			postingsEnum, err := p.getPostings(sp)
			if err != nil {
				return 0, err
			}
			p.postingsEnum = postingsEnum
			p.postingsEnumExhausted = false
			p.docMap = sp.docMap
		}
		docID, err := p.postingsEnum.NextDoc()
		if err != nil {
			return 0, err
		}
		if docID == spi.NO_MORE_DOCS {
			p.postingsEnumExhausted = true
		} else {
			docID = p.docMap.Get(docID)
			if docID != -1 {
				// assert docID > this.docID
				p.docID = docID
				return p.docID, nil
			}
		}
	}
}

// getPostings mirrors MultiSegmentsPostingsEnum.getPostings(SegmentPostings)
// (STMergingTermsEnum.java:212). The field is present in the segment because
// it is one of the segments provided in the reset method.
func (p *multiSegmentsPostingsEnum) getPostings(sp *segmentPostings) (spi.PostingsEnum, error) {
	postingsEnum, err := sp.getPostings(
		p.outer.fieldName, p.reusablePostingsEnums[sp.segmentIndex], p.postingsFlags)
	if err != nil {
		return nil, err
	}
	p.reusablePostingsEnums[sp.segmentIndex] = postingsEnum
	return postingsEnum, nil
}

// Advance mirrors MultiSegmentsPostingsEnum.advance(int)
// (STMergingTermsEnum.java:223), whose body is `throw new
// UnsupportedOperationException()`.
func (p *multiSegmentsPostingsEnum) Advance(_ int) (int, error) {
	return 0, errSTMergingTermsEnumUnsupported
}

// Cost mirrors MultiSegmentsPostingsEnum.cost
// (STMergingTermsEnum.java:228), whose body is `return 0; // Cost is not
// useful here.`
func (p *multiSegmentsPostingsEnum) Cost() int64 {
	return 0
}

// IntoBitSet carries the default body of
// DocIdSetIterator.intoBitSet(int, FixedBitSet, int) in Apache Lucene 10.5.0,
// which MultiSegmentsPostingsEnum does not override.
func (p *multiSegmentsPostingsEnum) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return util.DefaultIntoBitSet(p, upTo, bitSet, offset)
}

// DocIDRunEnd carries the default body of DocIdSetIterator.docIDRunEnd() in
// Apache Lucene 10.5.0, which MultiSegmentsPostingsEnum does not override.
func (p *multiSegmentsPostingsEnum) DocIDRunEnd() (int, error) {
	return util.DefaultDocIDRunEnd(p)
}

var (
	_ spi.TermsEnum    = (*stMergingTermsEnum)(nil)
	_ spi.PostingsEnum = (*multiSegmentsPostingsEnum)(nil)
)
