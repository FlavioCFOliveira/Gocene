// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"errors"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// SortedDocValuesTermsEnum is a TermsEnum that exposes the unique values of
// a SortedDocValues field as a sorted, ordinal-addressable term iterator.
// Mirrors org.apache.lucene.index.SortedDocValuesTermsEnum from Apache
// Lucene 10.5.0.
//
// Gocene deviations from the Java original (justification in tracking task
// summary, not in code comments):
//
//   - Lucene's TermsEnum returns a raw BytesRef from seek/next; Gocene's
//     TermsEnum is field-aware and returns *Term. The field name is supplied
//     to the constructor and stamped on every returned Term.
//   - DocFreq, TotalTermFreq, Postings, PostingsWithLiveDocs return
//     ErrUnsupportedSortedDVOp (matching Java's UnsupportedOperationException).
//
// The enum additionally exposes SeekExactOrd(ord) and Ord() — direct
// translations of Java's seekExact(long) and ord(), surfaced on the concrete
// type because Gocene's TermsEnum interface does not declare them.
type SortedDocValuesTermsEnum struct {
	spi.TermsEnumBase
	values     SortedDocValues
	field      string
	currentOrd int
	scratch    *util.BytesRefBuilder
}

// ErrUnsupportedSortedDVOp is returned by operations on
// SortedDocValuesTermsEnum that have no meaningful definition over plain
// SortedDocValues (DocFreq, TotalTermFreq, Postings, PostingsWithLiveDocs).
// Mirrors Java's UnsupportedOperationException.
var ErrUnsupportedSortedDVOp = errors.New("operation not supported on SortedDocValuesTermsEnum")

// errOrdTermStateRequired is returned by SeekExactWithTermState when the
// supplied TermState is not an *OrdTermState. Mirrors Java's assert.
var errOrdTermStateRequired = errors.New("SortedDocValuesTermsEnum: TermState must be *OrdTermState")

// NewSortedDocValuesTermsEnum constructs a TermsEnum over the unique values
// of a SortedDocValues field. field is the name stamped on every returned
// Term; values is the backing SortedDocValues.
func NewSortedDocValuesTermsEnum(field string, values SortedDocValues) *SortedDocValuesTermsEnum {
	return &SortedDocValuesTermsEnum{
		values:     values,
		field:      field,
		currentOrd: -1,
		scratch:    util.NewBytesRefBuilder(),
	}
}

// SeekCeil seeks to term or, if absent, to the smallest term > term.
// The returned *Term carries the field name supplied at construction.
// On end-of-enumeration this returns nil.
func (s *SortedDocValuesTermsEnum) SeekCeil(term *spi.Term) (*spi.Term, error) {
	if term == nil || term.Bytes == nil {
		return nil, nil
	}
	key := term.Bytes.ValidBytes()
	ord, err := LookupTerm(s.values, term.Bytes)
	if err != nil {
		return nil, err
	}
	if ord >= 0 {
		s.currentOrd = ord
		s.scratch.CopyBytes(key, 0, len(key))
		return s.currentTerm(), nil
	}
	s.currentOrd = -ord - 1
	if s.currentOrd == s.values.GetValueCount() {
		return nil, nil
	}
	bytesRef, err := s.values.LookupOrd(s.currentOrd)
	if err != nil {
		return nil, fmt.Errorf("SortedDocValuesTermsEnum.SeekCeil: LookupOrd(%d): %w", s.currentOrd, err)
	}
	s.scratch.CopyBytes(bytesRef, 0, len(bytesRef))
	return s.currentTerm(), nil
}

// SeekExact seeks to term and returns whether it exists.
func (s *SortedDocValuesTermsEnum) SeekExact(term *spi.Term) (bool, error) {
	if term == nil || term.Bytes == nil {
		return false, nil
	}
	ord, err := LookupTerm(s.values, term.Bytes)
	if err != nil {
		return false, err
	}
	if ord < 0 {
		return false, nil
	}
	s.currentOrd = ord
	s.scratch.CopyBytes(term.Bytes.ValidBytes(), 0, term.Bytes.Length)
	return true, nil
}

// SeekExactOrd positions the enumerator at the term whose ordinal is ord.
// Mirrors Java's seekExact(long). Returns an error if ord is out of range.
func (s *SortedDocValuesTermsEnum) SeekExactOrd(ord int) error {
	if ord < 0 || ord >= s.values.GetValueCount() {
		return fmt.Errorf("SortedDocValuesTermsEnum.SeekExactOrd: ord %d out of range [0, %d)", ord, s.values.GetValueCount())
	}
	s.currentOrd = ord
	bytesRef, err := s.values.LookupOrd(s.currentOrd)
	if err != nil {
		return fmt.Errorf("SortedDocValuesTermsEnum.SeekExactOrd: LookupOrd(%d): %w", s.currentOrd, err)
	}
	s.scratch.CopyBytes(bytesRef, 0, len(bytesRef))
	return nil
}

// SeekExactWithTermState mirrors Java's seekExact(BytesRef, TermState):
// trusts the supplied OrdTermState and seeks directly by ordinal.
func (s *SortedDocValuesTermsEnum) SeekExactWithTermState(_ *spi.Term, state TermState) error {
	if state == nil {
		return errOrdTermStateRequired
	}
	ots, ok := state.(*OrdTermState)
	if !ok {
		return errOrdTermStateRequired
	}
	return s.SeekExactOrd(int(ots.Ord))
}

// Next advances to the next term in the enumeration. Returns nil at the end.
func (s *SortedDocValuesTermsEnum) Next() (*spi.Term, error) {
	s.currentOrd++
	if s.currentOrd >= s.values.GetValueCount() {
		return nil, nil
	}
	bytesRef, err := s.values.LookupOrd(s.currentOrd)
	if err != nil {
		return nil, fmt.Errorf("SortedDocValuesTermsEnum.Next: LookupOrd(%d): %w", s.currentOrd, err)
	}
	s.scratch.CopyBytes(bytesRef, 0, len(bytesRef))
	return s.currentTerm(), nil
}

// Term returns the current term (or nil if not positioned).
func (s *SortedDocValuesTermsEnum) Term() *spi.Term {
	if s.currentOrd < 0 || s.currentOrd >= s.values.GetValueCount() {
		return nil
	}
	return s.currentTerm()
}

// Ord returns the ordinal of the current term. Mirrors Java's ord().
func (s *SortedDocValuesTermsEnum) Ord() int {
	return s.currentOrd
}

// TermState snapshots the current ordinal into a fresh OrdTermState.
func (s *SortedDocValuesTermsEnum) TermState() (TermState, error) {
	return &OrdTermState{Ord: int64(s.currentOrd)}, nil
}

// DocFreq is unsupported on SortedDocValuesTermsEnum.
func (s *SortedDocValuesTermsEnum) DocFreq() (int, error) {
	return 0, ErrUnsupportedSortedDVOp
}

// TotalTermFreq is unsupported on SortedDocValuesTermsEnum.
func (s *SortedDocValuesTermsEnum) TotalTermFreq() (int64, error) {
	return 0, ErrUnsupportedSortedDVOp
}

// Postings is unsupported on SortedDocValuesTermsEnum.
func (s *SortedDocValuesTermsEnum) Postings(_ int) (PostingsEnum, error) {
	return nil, ErrUnsupportedSortedDVOp
}

// PostingsWithLiveDocs is unsupported on SortedDocValuesTermsEnum.
func (s *SortedDocValuesTermsEnum) PostingsWithLiveDocs(_ util.Bits, _ int) (PostingsEnum, error) {
	return nil, ErrUnsupportedSortedDVOp
}

// currentTerm builds a fresh *Term carrying the configured field and the
// current scratch bytes. A fresh Term is allocated each call to match the
// "callers may retain the result" contract used elsewhere in Gocene.
func (s *SortedDocValuesTermsEnum) currentTerm() *spi.Term {
	bytes := s.scratch.Get()
	buf := make([]byte, bytes.Length)
	copy(buf, bytes.ValidBytes())
	return &spi.Term{
		Field: s.field,
		Bytes: util.NewBytesRef(buf),
	}
}
