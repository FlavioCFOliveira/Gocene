// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package tokenattributes

import (
	"fmt"
	"reflect"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// PositionIncrementAttributeType is the reflect.Type of the
// PositionIncrementAttribute interface, used as the lookup key for
// AttributeSource.
var PositionIncrementAttributeType = reflect.TypeOf((*PositionIncrementAttribute)(nil)).Elem()

// PositionIncrementAttribute determines the position of this token relative to the previous Token in a TokenStream, used in
// phrase searching.
//
// The default value is one.
//
// This is the Go port of
// org.apache.lucene.tokenattributes.PositionIncrementAttribute.
type PositionIncrementAttribute interface {
	util.AttributeImpl

	// GetPositionIncrement returns the position increment of this Token.
	GetPositionIncrement() int

	// SetPositionIncrement sets the position increment. The default value is one.
	// Panics if positionIncrement is negative.
	SetPositionIncrement(positionIncrement int)
}

// positionIncrementAttribute is the default implementation.
type positionIncrementAttribute struct {
	util.BaseAttributeImpl
	positionIncrement int
}

// NewPositionIncrementAttribute creates a new PositionIncrementAttribute with the default increment of 1.
func NewPositionIncrementAttribute() PositionIncrementAttribute {
	return &positionIncrementAttribute{
		positionIncrement: 1,
	}
}

// GetPositionIncrement returns the position increment.
func (a *positionIncrementAttribute) GetPositionIncrement() int {
	return a.positionIncrement
}

// SetPositionIncrement sets the position increment. Panics if value is negative,
// mirroring the IllegalArgumentException thrown by Lucene.
func (a *positionIncrementAttribute) SetPositionIncrement(positionIncrement int) {
	if positionIncrement < 0 {
		panic(fmt.Sprintf("PositionIncrementAttribute.SetPositionIncrement: position increment must be zero or greater; got %d", positionIncrement))
	}
	a.positionIncrement = positionIncrement
}

// Clear resets the position increment to 1.
func (a *positionIncrementAttribute) Clear() {
	a.positionIncrement = 1
}

// End resets the position increment to 0, mirroring the Lucene override.
func (a *positionIncrementAttribute) End() {
	a.positionIncrement = 0
}

// CopyTo copies this attribute's state onto target.
func (a *positionIncrementAttribute) CopyTo(target util.AttributeImpl) {
	if t, ok := target.(PositionIncrementAttribute); ok {
		t.SetPositionIncrement(a.positionIncrement)
	}
}

// CloneAttribute returns a deep clone of this attribute.
func (a *positionIncrementAttribute) CloneAttribute() util.AttributeImpl {
	return &positionIncrementAttribute{
		positionIncrement: a.positionIncrement,
	}
}

// ReflectWith emits the single (PositionIncrementAttribute, "positionIncrement", value) triple.
func (a *positionIncrementAttribute) ReflectWith(reflector util.AttributeReflector) {
	reflector(PositionIncrementAttributeType, "positionIncrement", a.positionIncrement)
}

// Equals returns true if other is a PositionIncrementAttribute with the same value.
func (a *positionIncrementAttribute) Equals(other any) bool {
	if a == other {
		return true
	}
	o, ok := other.(PositionIncrementAttribute)
	if !ok {
		return false
	}
	return a.positionIncrement == o.GetPositionIncrement()
}

// HashCode returns the position increment itself.
func (a *positionIncrementAttribute) HashCode() int {
	return a.positionIncrement
}
