// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

// OffsetAttribute stores the character offsets of a token in the original text.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.tokenattributes.OffsetAttribute.
//
// Offsets are used for highlighting and to store the position of the token
// in the original input text. StartOffset is inclusive, EndOffset is exclusive.
type OffsetAttribute interface {
	AttributeImpl

	// StartOffset returns the inclusive start offset of the token.
	StartOffset() int

	// SetStartOffset sets the start offset of the token.
	SetStartOffset(offset int)

	// EndOffset returns the exclusive end offset of the token.
	EndOffset() int

	// SetEndOffset sets the end offset of the token.
	SetEndOffset(offset int)
}

// offsetAttribute is the default implementation of OffsetAttribute.
type offsetAttribute struct {
	startOffset int
	endOffset   int
}

// NewOffsetAttribute creates a new OffsetAttribute with zero offsets.
func NewOffsetAttribute() OffsetAttribute {
	return &offsetAttribute{
		startOffset: 0,
		endOffset:   0,
	}
}

// Clear resets the offsets to zero.
func (a *offsetAttribute) Clear() {
	a.startOffset = 0
	a.endOffset = 0
}

// CopyTo copies this attribute to another implementation.
func (a *offsetAttribute) CopyTo(target AttributeImpl) {
	if t, ok := target.(OffsetAttribute); ok {
		t.SetStartOffset(a.startOffset)
		t.SetEndOffset(a.endOffset)
	}
}

// Copy creates a deep copy of this attribute.
func (a *offsetAttribute) Copy() AttributeImpl {
	copy := NewOffsetAttribute()
	copy.SetStartOffset(a.startOffset)
	copy.SetEndOffset(a.endOffset)
	return copy
}

// StartOffset returns the start offset.
func (a *offsetAttribute) StartOffset() int {
	return a.startOffset
}

// SetStartOffset sets the start offset.
func (a *offsetAttribute) SetStartOffset(offset int) {
	a.startOffset = offset
}

// EndOffset returns the end offset.
func (a *offsetAttribute) EndOffset() int {
	return a.endOffset
}

// SetEndOffset sets the end offset.
func (a *offsetAttribute) SetEndOffset(offset int) {
	a.endOffset = offset
}
