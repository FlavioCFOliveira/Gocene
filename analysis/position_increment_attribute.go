// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

// PositionIncrementAttribute controls the position increment between tokens.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.tokenattributes.PositionIncrementAttribute.
//
// The position increment determines the position of a token relative to the
// previous token. A value of 1 means the tokens are adjacent, 0 means they
// occupy the same position (synonyms), and values > 1 indicate gaps (removed tokens).
//
// This attribute is crucial for phrase queries - terms with position increment
// 0 will be matched by phrase queries as if they were at the same position.
type PositionIncrementAttribute interface {
	AttributeImpl

	// GetPositionIncrement returns the position increment.
	// Default is 1.
	GetPositionIncrement() int

	// SetPositionIncrement sets the position increment.
	SetPositionIncrement(positionIncrement int)
}

// positionIncrementAttribute is the default implementation.
type positionIncrementAttribute struct {
	positionIncrement int
}

// NewPositionIncrementAttribute creates a new PositionIncrementAttribute
// with the default increment of 1.
func NewPositionIncrementAttribute() PositionIncrementAttribute {
	return &positionIncrementAttribute{
		positionIncrement: 1,
	}
}

// Clear resets the position increment to 1.
func (a *positionIncrementAttribute) Clear() {
	a.positionIncrement = 1
}

// CopyTo copies this attribute to another implementation.
func (a *positionIncrementAttribute) CopyTo(target AttributeImpl) {
	if t, ok := target.(PositionIncrementAttribute); ok {
		t.SetPositionIncrement(a.positionIncrement)
	}
}

// Copy creates a deep copy of this attribute.
func (a *positionIncrementAttribute) Copy() AttributeImpl {
	copy := NewPositionIncrementAttribute()
	copy.SetPositionIncrement(a.positionIncrement)
	return copy
}

// GetPositionIncrement returns the position increment.
func (a *positionIncrementAttribute) GetPositionIncrement() int {
	return a.positionIncrement
}

// SetPositionIncrement sets the position increment.
func (a *positionIncrementAttribute) SetPositionIncrement(positionIncrement int) {
	a.positionIncrement = positionIncrement
}
