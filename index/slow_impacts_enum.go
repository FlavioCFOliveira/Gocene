// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import "math"

// SlowImpactsEnum wraps a PostingsEnum and returns trivial Impacts (constant
// freq=MaxInt32, norm=1) for every doc range. Mirrors
// org.apache.lucene.index.SlowImpactsEnum from Apache Lucene 10.4.0.
//
// This is used for short postings that do not need skipping: it provides a
// legal implementation of the ImpactsEnum API without any skip index.
type SlowImpactsEnum struct {
	delegate PostingsEnum
}

// NewSlowImpactsEnum wraps the given PostingsEnum. The delegate must not be nil.
func NewSlowImpactsEnum(in PostingsEnum) *SlowImpactsEnum {
	return &SlowImpactsEnum{delegate: in}
}

// NextDoc advances to the next document. Delegates to the underlying PostingsEnum.
func (s *SlowImpactsEnum) NextDoc() (int, error) { return s.delegate.NextDoc() }

// DocID returns the current document ID. Delegates to the underlying PostingsEnum.
func (s *SlowImpactsEnum) DocID() int { return s.delegate.DocID() }

// Cost returns the cost of iterating over this enum. Delegates to the underlying PostingsEnum.
func (s *SlowImpactsEnum) Cost() int64 { return s.delegate.Cost() }

// Advance advances to the first document at or beyond target.
// Delegates to the underlying PostingsEnum.
func (s *SlowImpactsEnum) Advance(target int) (int, error) { return s.delegate.Advance(target) }

// StartOffset returns the start character offset of the current occurrence.
// Delegates to the underlying PostingsEnum.
func (s *SlowImpactsEnum) StartOffset() (int, error) { return s.delegate.StartOffset() }

// NextPosition advances to the next position in the current document.
// Delegates to the underlying PostingsEnum.
func (s *SlowImpactsEnum) NextPosition() (int, error) { return s.delegate.NextPosition() }

// GetPayload returns the payload for the current position.
// Delegates to the underlying PostingsEnum.
func (s *SlowImpactsEnum) GetPayload() ([]byte, error) { return s.delegate.GetPayload() }

// Freq returns the term frequency in the current document.
// Delegates to the underlying PostingsEnum.
func (s *SlowImpactsEnum) Freq() (int, error) { return s.delegate.Freq() }

// EndOffset returns the end character offset of the current occurrence.
// Delegates to the underlying PostingsEnum.
func (s *SlowImpactsEnum) EndOffset() (int, error) { return s.delegate.EndOffset() }

// AdvanceShallow is a no-op: short postings do not benefit from shallow
// advances. Mirrors the empty implementation in Lucene.
func (s *SlowImpactsEnum) AdvanceShallow(_ int) error { return nil }

// GetImpacts returns a single-level Impacts with freq=MaxInt32 and norm=1
// spanning the entire remaining doc range. Mirrors
// SlowImpactsEnum.getImpacts() in Apache Lucene 10.4.0 which reports
// Integer.MAX_VALUE / 1L so that no impact-based early termination fires.
func (s *SlowImpactsEnum) GetImpacts() (Impacts, error) {
	return &slowImpacts{}, nil
}

// slowImpacts is the trivial single-level Impacts returned by SlowImpactsEnum.
// It pre-allocates the buffer once (mirroring the initializer block in the
// anonymous Lucene class) and reuses it across calls.
type slowImpacts struct {
	buf *FreqAndNormBuffer
}

// NumLevels returns 1: there is exactly one skip level.
func (si *slowImpacts) NumLevels() int { return 1 }

// GetDocIDUpTo returns NO_MORE_DOCS for the single level, indicating the
// impacts span the entire remaining doc-ID range.
func (si *slowImpacts) GetDocIDUpTo(_ int) int { return NO_MORE_DOCS }

// GetImpacts returns the buffer with (freq=MaxInt32, norm=1). The buffer is
// allocated lazily on first call and reused thereafter (zero-alloc hot path).
func (si *slowImpacts) GetImpacts(_ int) *FreqAndNormBuffer {
	if si.buf == nil {
		si.buf = NewFreqAndNormBuffer()
		si.buf.GrowNoCopy(1)
		si.buf.Freqs[0] = math.MaxInt32
		si.buf.Norms[0] = 1
		si.buf.Size = 1
	}
	// Reset to the canonical single-entry state on every call, matching
	// Lucene's behaviour where the same buffer is returned with freqs[0] and
	// norms[0] always set before returning.
	si.buf.Freqs[0] = math.MaxInt32
	si.buf.Norms[0] = 1
	si.buf.Size = 1
	return si.buf
}

// Compile-time assertion: SlowImpactsEnum satisfies the ImpactsEnum surface.
var _ ImpactsEnum = (*SlowImpactsEnum)(nil)
