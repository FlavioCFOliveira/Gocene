// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

// FieldInvertState aggregates per-document inversion statistics for a field.
// Mirrors org.apache.lucene.index.FieldInvertState from Apache Lucene 10.4.0.
//
// It is mutable: the inversion pipeline updates the fields directly as it
// walks each token. The Attribute hook stays out-of-scope until the search/
// analysis attribute consolidation lands (#2690).
type FieldInvertState struct {
	indexCreatedVersionMajor int
	name                     string
	indexOptions             IndexOptions

	position         int
	length           int
	numOverlap       int
	offset           int
	maxTermFrequency int
	uniqueTermCount  int
}

// NewFieldInvertState constructs a zero-valued FieldInvertState for the given
// name and index options (mirrors the small Java constructor).
func NewFieldInvertState(indexCreatedVersionMajor int, name string, opts IndexOptions) *FieldInvertState {
	return &FieldInvertState{
		indexCreatedVersionMajor: indexCreatedVersionMajor,
		name:                     name,
		indexOptions:             opts,
	}
}

// NewFieldInvertStateFull constructs a pre-populated FieldInvertState
// (mirrors the wide Java constructor).
func NewFieldInvertStateFull(indexCreatedVersionMajor int, name string, opts IndexOptions,
	position, length, numOverlap, offset, maxTermFrequency, uniqueTermCount int) *FieldInvertState {
	return &FieldInvertState{
		indexCreatedVersionMajor: indexCreatedVersionMajor,
		name:                     name,
		indexOptions:             opts,
		position:                 position,
		length:                   length,
		numOverlap:               numOverlap,
		offset:                   offset,
		maxTermFrequency:         maxTermFrequency,
		uniqueTermCount:          uniqueTermCount,
	}
}

// Name returns the field name.
func (s *FieldInvertState) Name() string { return s.name }

// IndexOptions returns the field's index options.
func (s *FieldInvertState) IndexOptions() IndexOptions { return s.indexOptions }

// IndexCreatedVersionMajor returns the version major recorded with the index.
func (s *FieldInvertState) IndexCreatedVersionMajor() int { return s.indexCreatedVersionMajor }

// Position returns the running token position.
func (s *FieldInvertState) Position() int { return s.position }

// SetPosition updates the running token position.
func (s *FieldInvertState) SetPosition(p int) { s.position = p }

// Length returns the running field length.
func (s *FieldInvertState) Length() int { return s.length }

// SetLength updates the running field length.
func (s *FieldInvertState) SetLength(l int) { s.length = l }

// NumOverlap returns the number of overlapping tokens.
func (s *FieldInvertState) NumOverlap() int { return s.numOverlap }

// SetNumOverlap updates the number of overlapping tokens.
func (s *FieldInvertState) SetNumOverlap(n int) { s.numOverlap = n }

// Offset returns the running token end offset.
func (s *FieldInvertState) Offset() int { return s.offset }

// SetOffset updates the running token end offset.
func (s *FieldInvertState) SetOffset(o int) { s.offset = o }

// MaxTermFrequency returns the maximum term frequency seen.
func (s *FieldInvertState) MaxTermFrequency() int { return s.maxTermFrequency }

// SetMaxTermFrequency updates the maximum term frequency seen.
func (s *FieldInvertState) SetMaxTermFrequency(f int) { s.maxTermFrequency = f }

// UniqueTermCount returns the number of unique terms seen.
func (s *FieldInvertState) UniqueTermCount() int { return s.uniqueTermCount }

// SetUniqueTermCount updates the number of unique terms seen.
func (s *FieldInvertState) SetUniqueTermCount(c int) { s.uniqueTermCount = c }
