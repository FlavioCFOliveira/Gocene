// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"fmt"
	"reflect"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// PositionIncrementAttributeType is the reflect.Type of the
// PositionIncrementAttribute interface, used as the lookup key for
// AttributeSource. Phase 4 (consumer migration) converts all
// string-keyed GetAttribute calls to use these vars.
var PositionIncrementAttributeType = reflect.TypeOf((*PositionIncrementAttribute)(nil)).Elem()

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

	// Copy returns a deep copy of this attribute. Retained as part of the
	// PositionIncrementAttribute interface contract for Sprint 54 Phase 2:
	// when [AttributeImpl] became an alias for [util.AttributeImpl], its
	// CloneAttribute method replaced the legacy Copy on the underlying
	// interface; preserving Copy here keeps existing consumer code
	// compiling while migration to CloneAttribute is rolled out.
	Copy() AttributeImpl

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

// Compile-time assertions to lock in the contracts this impl
// participates in. Sprint 12 adds opt-in End/ReflectWith hooks via the
// AttributeEnder and AttributeReflectable interfaces.
var (
	_ AttributeImpl                   = (*positionIncrementAttribute)(nil)
	_ PositionIncrementAttribute      = (*positionIncrementAttribute)(nil)
	_ AttributeEnder                  = (*positionIncrementAttribute)(nil)
	_ AttributeReflectable            = (*positionIncrementAttribute)(nil)
	_ util.AttributeInterfaceProvider = (*positionIncrementAttribute)(nil)
)

// AttributeInterfaces satisfies [util.AttributeInterfaceProvider] (see
// charTermAttribute.AttributeInterfaces for the rationale).
func (a *positionIncrementAttribute) AttributeInterfaces() []reflect.Type {
	return []reflect.Type{PositionIncrementAttributeType}
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

// CloneAttribute implements util.AttributeImpl.CloneAttribute. Returns
// a deep copy as util.AttributeImpl. Delegates to the existing Copy().
func (a *positionIncrementAttribute) CloneAttribute() util.AttributeImpl { return a.Copy() }

// GetPositionIncrement returns the position increment.
func (a *positionIncrementAttribute) GetPositionIncrement() int {
	return a.positionIncrement
}

// SetPositionIncrement sets the position increment. It panics with an
// explanatory message when the value is negative, matching the
// IllegalArgumentException thrown by
// org.apache.lucene.analysis.tokenattributes.PositionIncrementAttributeImpl#setPositionIncrement.
func (a *positionIncrementAttribute) SetPositionIncrement(positionIncrement int) {
	if positionIncrement < 0 {
		panic(fmt.Sprintf(
			"PositionIncrementAttribute.SetPositionIncrement: position increment must be zero or greater; got %d",
			positionIncrement))
	}
	a.positionIncrement = positionIncrement
}

// End is the opt-in [AttributeEnder] hook. The Lucene reference
// overrides {@code end()} to set positionIncrement = 0 (distinct from
// Clear, which resets to 1).
func (a *positionIncrementAttribute) End() {
	a.positionIncrement = 0
}

// ReflectWith is the opt-in [AttributeReflectable] hook. It emits a
// single (PositionIncrementAttribute, "positionIncrement", value)
// triple, matching the Lucene reference exactly.
func (a *positionIncrementAttribute) ReflectWith(reflector AttributeReflector) {
	reflector(reflect.TypeOf((*PositionIncrementAttribute)(nil)).Elem(),
		"positionIncrement", a.positionIncrement)
}

// Equals returns true if other is a [positionIncrementAttribute] whose
// positionIncrement compares equal, matching Lucene's instance-of
// guard.
func (a *positionIncrementAttribute) Equals(other any) bool {
	if a == other {
		return true
	}
	o, ok := other.(*positionIncrementAttribute)
	if !ok {
		return false
	}
	return a.positionIncrement == o.positionIncrement
}

// HashCode returns the position increment itself, matching Lucene's
// {@code hashCode()}.
func (a *positionIncrementAttribute) HashCode() int {
	return a.positionIncrement
}
