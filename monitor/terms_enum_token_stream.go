// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package monitor

import (
	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// BytesRefIterator is an iterator over BytesRef values, analogous to
// org.apache.lucene.util.BytesRefIterator.
type BytesRefIterator interface {
	// Next returns the next BytesRef, or nil when exhausted.
	Next() (*util.BytesRef, error)
}

// TermsEnumTokenStream is a TokenStream backed by a BytesRefIterator.
//
// Port of org.apache.lucene.monitor.TermsEnumTokenStream.
type TermsEnumTokenStream struct {
	analysis.BaseTokenStream
	termsEnum BytesRefIterator
	charTerm  analysis.CharTermAttribute
}

// NewTermsEnumTokenStream creates a TermsEnumTokenStream for the given iterator.
func NewTermsEnumTokenStream(termsEnum BytesRefIterator) *TermsEnumTokenStream {
	ts := &TermsEnumTokenStream{
		termsEnum: termsEnum,
		charTerm:  analysis.NewCharTermAttribute(),
	}
	return ts
}

// IncrementToken advances to the next term.  Returns false when exhausted.
func (t *TermsEnumTokenStream) IncrementToken() (bool, error) {
	bytes, err := t.termsEnum.Next()
	if err != nil {
		return false, err
	}
	if bytes == nil {
		return false, nil
	}
	t.charTerm.SetEmpty()
	t.charTerm.AppendString(bytes.String())
	return true, nil
}

// End performs end-of-stream operations (no-op).
func (t *TermsEnumTokenStream) End() error { return nil }

// Reset resets the stream (no-op; the underlying iterator cannot be rewound).
func (t *TermsEnumTokenStream) Reset() error { return nil }

// Close closes the stream.
func (t *TermsEnumTokenStream) Close() error { return nil }
