// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package suggest

// InputIterator is the foundational iterator the suggest sub-packages
// consume. Mirrors org.apache.lucene.search.suggest.InputIterator.
//
// Each Next call advances to the next term and returns its weight, optional
// payload, and optional contexts. ok=false signals end-of-stream.
//
// The interface corresponds to BytesRefIterator enriched with weight,
// payload, and context metadata for suggester consumption. Currently only
// AnalyzingSuggester, FuzzySuggester, and AnalyzingInfixSuggester support
// payloads.
type InputIterator interface {
	// Next advances the iterator and returns the next term together with its
	// associated weight, payload, contexts, ok, and error. ok=false when the
	// stream is exhausted.
	Next() (term []byte, weight int64, payload []byte, contexts [][]byte, ok bool, err error)

	// HasPayloads reports whether the iterator produces payloads.
	HasPayloads() bool

	// HasContexts reports whether the iterator produces contexts.
	HasContexts() bool
}

// EmptyInputIterator is the singleton InputIterator that iterates over zero
// entries. Mirrors InputIterator.EMPTY.
var EmptyInputIterator InputIterator = &inputIteratorWrapper{wrapped: emptyBytesIter{}}

// emptyBytesIter is a trivial zero-entry byte-slice iterator.
type emptyBytesIter struct{}

func (emptyBytesIter) Next() ([]byte, bool, error) { return nil, false, nil }

// bytesIter is a minimal interface over a raw byte-slice stream.
type bytesIter interface {
	Next() ([]byte, bool, error)
}

// inputIteratorWrapper wraps a bytesIter as an InputIterator, assigning
// weight=1 to every term and carrying no payload or contexts. Mirrors
// org.apache.lucene.search.suggest.InputIterator.InputIteratorWrapper.
type inputIteratorWrapper struct {
	wrapped bytesIter
}

// NewInputIteratorWrapper wraps wrapped as an InputIterator. All weights are
// set to 1; payload and contexts are always nil.
func NewInputIteratorWrapper(wrapped bytesIter) InputIterator {
	return &inputIteratorWrapper{wrapped: wrapped}
}

func (w *inputIteratorWrapper) Next() (term []byte, weight int64, payload []byte, contexts [][]byte, ok bool, err error) {
	t, ok2, e := w.wrapped.Next()
	if e != nil || !ok2 {
		return nil, 0, nil, nil, false, e
	}
	return t, 1, nil, nil, true, nil
}

func (w *inputIteratorWrapper) HasPayloads() bool  { return false }
func (w *inputIteratorWrapper) HasContexts() bool  { return false }

var _ InputIterator = (*inputIteratorWrapper)(nil)
