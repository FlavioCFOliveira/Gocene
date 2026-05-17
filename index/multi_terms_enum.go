// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"github.com/FlavioCFOliveira/Gocene/util"
)

// MultiTermsEnum is the priority-queue-based merge of several TermsEnum
// instances, exposing each unique term exactly once. Mirrors
// org.apache.lucene.index.MultiTermsEnum (Apache Lucene 10.4.0).
//
// Gocene skeleton: only the constructor and accessor surface are present.
// The priority-queue advance logic, postings union, and impacts merge are
// deferred to backlog #2706.
type MultiTermsEnum struct {
	TermsEnumBase
	slices []ReaderSlice
}

// NewMultiTermsEnum builds a MultiTermsEnum over the supplied slice list.
// The actual sub-enumerators are bound via Reset (deferred).
func NewMultiTermsEnum(slices []ReaderSlice) *MultiTermsEnum {
	return &MultiTermsEnum{slices: slices}
}

// GetMatchCount returns the number of sub-enumerators currently positioned
// on the same term. Returns 0 in the skeleton.
func (m *MultiTermsEnum) GetMatchCount() int { return 0 }

// GetSlices returns the underlying ReaderSlice list.
func (m *MultiTermsEnum) GetSlices() []ReaderSlice { return m.slices }

// Next is not yet implemented.
func (m *MultiTermsEnum) Next() (*Term, error) { return nil, ErrMultiTermsEnumNotImplemented }

// SeekCeil is not yet implemented.
func (m *MultiTermsEnum) SeekCeil(_ *Term) (*Term, error) {
	return nil, ErrMultiTermsEnumNotImplemented
}

// SeekExact is not yet implemented.
func (m *MultiTermsEnum) SeekExact(_ *Term) (bool, error) {
	return false, ErrMultiTermsEnumNotImplemented
}

// DocFreq is not yet implemented.
func (m *MultiTermsEnum) DocFreq() (int, error) { return 0, ErrMultiTermsEnumNotImplemented }

// TotalTermFreq is not yet implemented.
func (m *MultiTermsEnum) TotalTermFreq() (int64, error) { return 0, ErrMultiTermsEnumNotImplemented }

// Postings is not yet implemented.
func (m *MultiTermsEnum) Postings(_ int) (PostingsEnum, error) {
	return nil, ErrMultiTermsEnumNotImplemented
}

// PostingsWithLiveDocs is not yet implemented.
func (m *MultiTermsEnum) PostingsWithLiveDocs(_ util.Bits, _ int) (PostingsEnum, error) {
	return nil, ErrMultiTermsEnumNotImplemented
}
