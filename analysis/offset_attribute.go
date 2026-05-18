// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"fmt"
	"reflect"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// OffsetAttributeType is the reflect.Type of the OffsetAttribute
// interface, used as the lookup key for AttributeSource. Phase 4
// (consumer migration) converts all string-keyed GetAttribute calls to
// use these vars.
var OffsetAttributeType = reflect.TypeOf((*OffsetAttribute)(nil)).Elem()

// OffsetAttribute stores the character offsets of a token in the original text.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.tokenattributes.OffsetAttribute.
//
// Offsets are used for highlighting and to store the position of the token
// in the original input text. StartOffset is inclusive, EndOffset is exclusive.
//
// Sprint 12 adds the combined SetOffset(start, end) form to bring the
// interface in line with the Lucene reference; the legacy per-field
// setters are retained for back-compat with existing consumers.
type OffsetAttribute interface {
	AttributeImpl

	// Copy returns a deep copy of this attribute. Retained as part of the
	// OffsetAttribute interface contract for Sprint 54 Phase 2: when
	// [AttributeImpl] became an alias for [util.AttributeImpl], its
	// CloneAttribute method replaced the legacy Copy on the underlying
	// interface; preserving Copy here keeps existing consumer code
	// compiling while migration to CloneAttribute is rolled out.
	Copy() AttributeImpl

	// StartOffset returns the inclusive start offset of the token.
	StartOffset() int

	// SetStartOffset sets the start offset of the token.
	SetStartOffset(offset int)

	// EndOffset returns the exclusive end offset of the token.
	EndOffset() int

	// SetEndOffset sets the end offset of the token.
	SetEndOffset(offset int)

	// SetOffset is the Lucene-faithful combined setter. It panics with
	// an explanatory message when startOffset is negative or
	// endOffset < startOffset, matching the IllegalArgumentException
	// thrown by org.apache.lucene.analysis.tokenattributes.OffsetAttributeImpl.
	SetOffset(startOffset, endOffset int)
}

// offsetAttribute is the default implementation of OffsetAttribute.
type offsetAttribute struct {
	startOffset int
	endOffset   int
}

// Compile-time assertions to lock in the contracts this impl
// participates in.
var (
	_ AttributeImpl                   = (*offsetAttribute)(nil)
	_ OffsetAttribute                 = (*offsetAttribute)(nil)
	_ AttributeReflectable            = (*offsetAttribute)(nil)
	_ util.AttributeInterfaceProvider = (*offsetAttribute)(nil)
)

// AttributeInterfaces satisfies [util.AttributeInterfaceProvider] (see
// charTermAttribute.AttributeInterfaces for the rationale).
func (a *offsetAttribute) AttributeInterfaces() []reflect.Type {
	return []reflect.Type{OffsetAttributeType}
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

// End implements util.AttributeImpl.End. Lucene default behavior is to
// call clear(); concrete impls override when end-of-field state differs.
func (a *offsetAttribute) End() { a.Clear() }

// CloneAttribute implements util.AttributeImpl.CloneAttribute. Returns
// a deep copy as util.AttributeImpl. Delegates to the existing Copy().
func (a *offsetAttribute) CloneAttribute() util.AttributeImpl { return a.Copy() }

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

// SetOffset is the Lucene-faithful combined setter. It validates the
// arguments against the same invariants as
// {@code OffsetAttributeImpl#setOffset(int, int)} and panics with an
// explanatory message when they are violated.
func (a *offsetAttribute) SetOffset(startOffset, endOffset int) {
	if startOffset < 0 || endOffset < startOffset {
		panic(fmt.Sprintf(
			"OffsetAttribute.SetOffset: startOffset must be non-negative and endOffset must be >= startOffset; got startOffset=%d, endOffset=%d",
			startOffset, endOffset))
	}
	a.startOffset = startOffset
	a.endOffset = endOffset
}

// ReflectWith is the opt-in [AttributeReflectable] hook. It emits the
// two parity triples expected by the Lucene reference: startOffset and
// endOffset, both under the OffsetAttribute key.
func (a *offsetAttribute) ReflectWith(reflector AttributeReflector) {
	attType := reflect.TypeOf((*OffsetAttribute)(nil)).Elem()
	reflector(attType, "startOffset", a.startOffset)
	reflector(attType, "endOffset", a.endOffset)
}

// Equals returns true if other is an [offsetAttribute] whose start and
// end offsets compare equal, matching Lucene's instance-of guard.
func (a *offsetAttribute) Equals(other any) bool {
	if a == other {
		return true
	}
	o, ok := other.(*offsetAttribute)
	if !ok {
		return false
	}
	return a.startOffset == o.startOffset && a.endOffset == o.endOffset
}

// HashCode returns the Lucene-parity hash: 31 * startOffset + endOffset.
func (a *offsetAttribute) HashCode() int {
	return a.startOffset*31 + a.endOffset
}
