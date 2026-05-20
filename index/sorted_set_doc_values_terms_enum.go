// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// SortedSetDocValuesTermsEnum is a TermsEnum that exposes the unique values of
// a SortedSetDocValues field as a sorted, ordinal-addressable term iterator.
// Mirrors org.apache.lucene.index.SortedSetDocValuesTermsEnum from Apache
// Lucene 10.4.0.
//
// It is the sibling of SortedDocValuesTermsEnum; the sole behavioural
// difference in the Java original is that SortedSetDocValues addresses
// ordinals with a 64-bit value, so currentOrd here is an int64.
//
// Gocene deviations from the Java original (justification in tracking task
// summary, not in code comments):
//
//   - Lucene's TermsEnum returns a raw BytesRef from seek/next; Gocene's
//     TermsEnum is field-aware and returns *Term. The field name is supplied
//     to the constructor and stamped on every returned Term.
//   - Lucene's SortedSetDocValues.lookupTerm performs the binary search
//     internally; Gocene's SortedSetDocValues interface does not expose
//     lookupTerm, so the binary search is implemented here against LookupOrd +
//     GetValueCount.
//   - DocFreq, TotalTermFreq, Postings, PostingsWithLiveDocs return
//     ErrUnsupportedSortedSetDVOp (matching Java's UnsupportedOperationException).
//
// The enum additionally exposes SeekExactOrd(ord) and Ord() — direct
// translations of Java's seekExact(long) and ord(), surfaced on the concrete
// type because Gocene's TermsEnum interface does not declare them.
type SortedSetDocValuesTermsEnum struct {
	values     SortedSetDocValues
	field      string
	currentOrd int64
	scratch    *util.BytesRefBuilder
}

// ErrUnsupportedSortedSetDVOp is returned by operations on
// SortedSetDocValuesTermsEnum that have no meaningful definition over plain
// SortedSetDocValues (DocFreq, TotalTermFreq, Postings, PostingsWithLiveDocs).
// Mirrors Java's UnsupportedOperationException.
var ErrUnsupportedSortedSetDVOp = errors.New("operation not supported on SortedSetDocValuesTermsEnum")

// errSetOrdTermStateRequired is returned by SeekExactWithTermState when the
// supplied TermState is not an *OrdTermState. Mirrors Java's assert.
var errSetOrdTermStateRequired = errors.New("SortedSetDocValuesTermsEnum: TermState must be *OrdTermState")

// NewSortedSetDocValuesTermsEnum constructs a TermsEnum over the unique values
// of a SortedSetDocValues field. field is the name stamped on every returned
// Term; values is the backing SortedSetDocValues.
func NewSortedSetDocValuesTermsEnum(field string, values SortedSetDocValues) *SortedSetDocValuesTermsEnum {
	return &SortedSetDocValuesTermsEnum{
		values:     values,
		field:      field,
		currentOrd: -1,
		scratch:    util.NewBytesRefBuilder(),
	}
}

// lookupTerm performs a binary search for key over the sorted values.
// Mirrors SortedSetDocValues.lookupTerm from Apache Lucene 10.4.0: returns the
// matching ordinal on hit, else -(insertionPoint + 1).
func (s *SortedSetDocValuesTermsEnum) lookupTerm(key []byte) (int64, error) {
	low := int64(0)
	high := int64(s.values.GetValueCount()) - 1
	for low <= high {
		mid := int64(uint64(low+high) >> 1)
		term, err := s.values.LookupOrd(int(mid))
		if err != nil {
			return 0, fmt.Errorf("SortedSetDocValuesTermsEnum.lookupTerm: LookupOrd(%d): %w", mid, err)
		}
		switch cmp := bytes.Compare(term, key); {
		case cmp < 0:
			low = mid + 1
		case cmp > 0:
			high = mid - 1
		default:
			return mid, nil
		}
	}
	return -(low + 1), nil
}

// SeekCeil seeks to term or, if absent, to the smallest term > term.
// The returned *Term carries the field name supplied at construction.
// On end-of-enumeration this returns nil.
func (s *SortedSetDocValuesTermsEnum) SeekCeil(term *Term) (*Term, error) {
	if term == nil || term.Bytes == nil {
		return nil, nil
	}
	key := term.Bytes.ValidBytes()
	ord, err := s.lookupTerm(key)
	if err != nil {
		return nil, err
	}
	if ord >= 0 {
		s.currentOrd = ord
		s.scratch.CopyBytes(key, 0, len(key))
		return s.currentTerm(), nil
	}
	s.currentOrd = -ord - 1
	if s.currentOrd == int64(s.values.GetValueCount()) {
		return nil, nil
	}
	bytesRef, err := s.values.LookupOrd(int(s.currentOrd))
	if err != nil {
		return nil, fmt.Errorf("SortedSetDocValuesTermsEnum.SeekCeil: LookupOrd(%d): %w", s.currentOrd, err)
	}
	s.scratch.CopyBytes(bytesRef, 0, len(bytesRef))
	return s.currentTerm(), nil
}

// SeekExact seeks to term and returns whether it exists.
func (s *SortedSetDocValuesTermsEnum) SeekExact(term *Term) (bool, error) {
	if term == nil || term.Bytes == nil {
		return false, nil
	}
	key := term.Bytes.ValidBytes()
	ord, err := s.lookupTerm(key)
	if err != nil {
		return false, err
	}
	if ord < 0 {
		return false, nil
	}
	s.currentOrd = ord
	s.scratch.CopyBytes(key, 0, len(key))
	return true, nil
}

// SeekExactOrd positions the enumerator at the term whose ordinal is ord.
// Mirrors Java's seekExact(long). Returns an error if ord is out of range.
func (s *SortedSetDocValuesTermsEnum) SeekExactOrd(ord int64) error {
	if ord < 0 || ord >= int64(s.values.GetValueCount()) {
		return fmt.Errorf("SortedSetDocValuesTermsEnum.SeekExactOrd: ord %d out of range [0, %d)", ord, s.values.GetValueCount())
	}
	s.currentOrd = ord
	bytesRef, err := s.values.LookupOrd(int(s.currentOrd))
	if err != nil {
		return fmt.Errorf("SortedSetDocValuesTermsEnum.SeekExactOrd: LookupOrd(%d): %w", s.currentOrd, err)
	}
	s.scratch.CopyBytes(bytesRef, 0, len(bytesRef))
	return nil
}

// SeekExactWithTermState mirrors Java's seekExact(BytesRef, TermState):
// trusts the supplied OrdTermState and seeks directly by ordinal.
func (s *SortedSetDocValuesTermsEnum) SeekExactWithTermState(_ *Term, state TermState) error {
	if state == nil {
		return errSetOrdTermStateRequired
	}
	ots, ok := state.(*OrdTermState)
	if !ok {
		return errSetOrdTermStateRequired
	}
	return s.SeekExactOrd(ots.Ord)
}

// Next advances to the next term in the enumeration. Returns nil at the end.
func (s *SortedSetDocValuesTermsEnum) Next() (*Term, error) {
	s.currentOrd++
	if s.currentOrd >= int64(s.values.GetValueCount()) {
		return nil, nil
	}
	bytesRef, err := s.values.LookupOrd(int(s.currentOrd))
	if err != nil {
		return nil, fmt.Errorf("SortedSetDocValuesTermsEnum.Next: LookupOrd(%d): %w", s.currentOrd, err)
	}
	s.scratch.CopyBytes(bytesRef, 0, len(bytesRef))
	return s.currentTerm(), nil
}

// Term returns the current term (or nil if not positioned).
func (s *SortedSetDocValuesTermsEnum) Term() *Term {
	if s.currentOrd < 0 || s.currentOrd >= int64(s.values.GetValueCount()) {
		return nil
	}
	return s.currentTerm()
}

// Ord returns the ordinal of the current term. Mirrors Java's ord().
func (s *SortedSetDocValuesTermsEnum) Ord() int64 {
	return s.currentOrd
}

// TermState snapshots the current ordinal into a fresh OrdTermState.
func (s *SortedSetDocValuesTermsEnum) TermState() (TermState, error) {
	return &OrdTermState{Ord: s.currentOrd}, nil
}

// DocFreq is unsupported on SortedSetDocValuesTermsEnum.
func (s *SortedSetDocValuesTermsEnum) DocFreq() (int, error) {
	return 0, ErrUnsupportedSortedSetDVOp
}

// TotalTermFreq is unsupported on SortedSetDocValuesTermsEnum.
func (s *SortedSetDocValuesTermsEnum) TotalTermFreq() (int64, error) {
	return 0, ErrUnsupportedSortedSetDVOp
}

// Postings is unsupported on SortedSetDocValuesTermsEnum.
func (s *SortedSetDocValuesTermsEnum) Postings(_ int) (PostingsEnum, error) {
	return nil, ErrUnsupportedSortedSetDVOp
}

// PostingsWithLiveDocs is unsupported on SortedSetDocValuesTermsEnum.
func (s *SortedSetDocValuesTermsEnum) PostingsWithLiveDocs(_ util.Bits, _ int) (PostingsEnum, error) {
	return nil, ErrUnsupportedSortedSetDVOp
}

// currentTerm builds a fresh *Term carrying the configured field and the
// current scratch bytes. A fresh Term is allocated each call to match the
// "callers may retain the result" contract used elsewhere in Gocene.
func (s *SortedSetDocValuesTermsEnum) currentTerm() *Term {
	bytes := s.scratch.Get()
	// Copy bytes so the returned Term does not alias scratch's buffer.
	buf := make([]byte, bytes.Length)
	copy(buf, bytes.ValidBytes())
	return &Term{
		Field: s.field,
		Bytes: util.NewBytesRef(buf),
	}
}
