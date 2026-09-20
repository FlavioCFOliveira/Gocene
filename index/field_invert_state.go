// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"github.com/FlavioCFOliveira/Gocene/analysis/api"
)

// FieldInvertState tracks the number and position / offset parameters of terms being added to the index.
// The information collected in this class is also used to calculate the normalization factor for a field.
//
// This is the Go port of Lucene's org.apache.lucene.index.FieldInvertState.
type FieldInvertState struct {
	indexCreatedVersionMajor int
	name                     string
	indexOptions             IndexOptions
	position                 int
	length                   int
	numOverlap               int
	offset                   int
	maxTermFrequency         int
	uniqueTermCount          int
	lastStartOffset          int
	lastPosition             int
	attributeSource          api.AttributeSource

	offsetAttribute   api.OffsetAttribute
	posIncrAttribute  api.PositionIncrementAttribute
	payloadAttribute  api.PayloadAttribute
	termAttribute     api.TermToBytesRefAttribute
	termFreqAttribute api.TermFrequencyAttribute
}

// NewFieldInvertState creates a FieldInvertState for the specified field name.
func NewFieldInvertState(indexCreatedVersionMajor int, name string, indexOptions IndexOptions) *FieldInvertState {
	return &FieldInvertState{
		indexCreatedVersionMajor: indexCreatedVersionMajor,
		name:                     name,
		indexOptions:             indexOptions,
	}
}

// NewFieldInvertStateFull creates a FieldInvertState for the specified field name and values for all fields.
func NewFieldInvertStateFull(
	indexCreatedVersionMajor int,
	name string,
	indexOptions IndexOptions,
	position, length, numOverlap, offset, maxTermFrequency, uniqueTermCount int,
) *FieldInvertState {
	fis := NewFieldInvertState(indexCreatedVersionMajor, name, indexOptions)
	fis.position = position
	fis.length = length
	fis.numOverlap = numOverlap
	fis.offset = offset
	fis.maxTermFrequency = maxTermFrequency
	fis.uniqueTermCount = uniqueTermCount
	return fis
}

// reset re-initializes the state.
func (fis *FieldInvertState) reset() {
	fis.position = -1
	fis.length = 0
	fis.numOverlap = 0
	fis.offset = 0
	fis.maxTermFrequency = 0
	fis.uniqueTermCount = 0
	fis.lastStartOffset = 0
	fis.lastPosition = 0
}

// setAttributeSource sets attributeSource to a new instance.
func (fis *FieldInvertState) setAttributeSource(attributeSource api.AttributeSource) {
	if fis.attributeSource != attributeSource {
		fis.attributeSource = attributeSource
		if attributeSource == nil {
			fis.termAttribute = nil
			fis.termFreqAttribute = nil
			fis.posIncrAttribute = nil
			fis.offsetAttribute = nil
			fis.payloadAttribute = nil
		} else {
			fis.termAttribute = attributeSource.GetAttributeTermToBytesRef()
			fis.termFreqAttribute = attributeSource.AddAttributeTermFrequency()
			fis.posIncrAttribute = attributeSource.AddAttributePositionIncrement()
			fis.offsetAttribute = attributeSource.AddAttributeOffset()
			fis.payloadAttribute = attributeSource.GetAttributePayload()
		}
	}
}

// Position gets the last processed term position.
func (fis *FieldInvertState) Position() int {
	return fis.position
}

// Length gets total number of terms in this field.
func (fis *FieldInvertState) Length() int {
	return fis.length
}

// SetLength sets the length value.
func (fis *FieldInvertState) SetLength(length int) {
	fis.length = length
}

// NumOverlap gets the number of terms with positionIncrement == 0.
func (fis *FieldInvertState) NumOverlap() int {
	return fis.numOverlap
}

// SetNumOverlap sets the number of terms with positionIncrement == 0.
func (fis *FieldInvertState) SetNumOverlap(numOverlap int) {
	fis.numOverlap = numOverlap
}

// Offset gets the end offset of the last processed term.
func (fis *FieldInvertState) Offset() int {
	return fis.offset
}

// MaxTermFrequency gets the maximum term-frequency encountered for any term in the field.
func (fis *FieldInvertState) MaxTermFrequency() int {
	return fis.maxTermFrequency
}

// UniqueTermCount returns the number of unique terms encountered in this field.
func (fis *FieldInvertState) UniqueTermCount() int {
	return fis.uniqueTermCount
}

// AttributeSource returns the AttributeSource from the TokenStream that provided the indexed tokens for this field.
func (fis *FieldInvertState) AttributeSource() api.AttributeSource {
	return fis.attributeSource
}

// Name returns the field's name.
func (fis *FieldInvertState) Name() string {
	return fis.name
}

// IndexCreatedVersionMajor returns the version that was used to create the index, or 6 if it was created before 7.0.
func (fis *FieldInvertState) IndexCreatedVersionMajor() int {
	return fis.indexCreatedVersionMajor
}

// IndexOptions gets the index options for this field.
func (fis *FieldInvertState) IndexOptions() IndexOptions {
	return fis.indexOptions
}
