// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
// lucene/core/src/java/org/apache/lucene/analysis/tokenattributes/PositionLengthAttributeImpl.java

package analysis

import (
	"fmt"
	"reflect"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// PositionLengthAttributeImpl is the Go port of Lucene's
// org.apache.lucene.analysis.tokenattributes.PositionLengthAttributeImpl.
//
// It is the exported concrete implementation of [PositionLengthAttribute].
// The default position length is 1, matching the Lucene default.
//
// Lucene 10.4.0 reference:
//
//	lucene/core/src/java/org/apache/lucene/analysis/tokenattributes/PositionLengthAttributeImpl.java
type PositionLengthAttributeImpl struct {
	positionLength int
}

// Compile-time assertions.
var (
	_ util.AttributeImpl              = (*PositionLengthAttributeImpl)(nil)
	_ PositionLengthAttribute         = (*PositionLengthAttributeImpl)(nil)
	_ util.AttributeInterfaceProvider = (*PositionLengthAttributeImpl)(nil)
)

// AttributeInterfaces satisfies [util.AttributeInterfaceProvider].
func (p *PositionLengthAttributeImpl) AttributeInterfaces() []reflect.Type {
	return []reflect.Type{PositionLengthAttributeType}
}

// NewPositionLengthAttributeImpl initialises this attribute with the
// default position length of 1, matching the Lucene no-arg constructor.
func NewPositionLengthAttributeImpl() *PositionLengthAttributeImpl {
	return &PositionLengthAttributeImpl{positionLength: 1}
}

// GetPositionLength returns the position length.
func (p *PositionLengthAttributeImpl) GetPositionLength() int { return p.positionLength }

// SetPositionLength replaces the position length without validation.
func (p *PositionLengthAttributeImpl) SetPositionLength(length int) {
	p.positionLength = length
}

// SetPositionLengthValidated panics when length < 1, mirroring Lucene's
// {@code PositionLengthAttributeImpl#setPositionLength(int)}.
func (p *PositionLengthAttributeImpl) SetPositionLengthValidated(length int) {
	if length < 1 {
		panic(fmt.Sprintf(
			"PositionLengthAttributeImpl.SetPositionLengthValidated: position length must be 1 or greater; got %d",
			length))
	}
	p.positionLength = length
}

// Clear resets the position length to 1, matching
// {@code PositionLengthAttributeImpl#clear()}.
func (p *PositionLengthAttributeImpl) Clear() { p.positionLength = 1 }

// End implements util.AttributeImpl.End. The Lucene base calls
// clear() from end().
func (p *PositionLengthAttributeImpl) End() { p.Clear() }

// CloneAttribute returns a deep copy of this impl as [util.AttributeImpl].
func (p *PositionLengthAttributeImpl) CloneAttribute() util.AttributeImpl {
	return &PositionLengthAttributeImpl{positionLength: p.positionLength}
}

// CopyTo copies this attribute onto target, which must implement
// [PositionLengthAttribute]; a panic is raised otherwise.
func (p *PositionLengthAttributeImpl) CopyTo(target util.AttributeImpl) {
	t, ok := target.(PositionLengthAttribute)
	if !ok {
		panic("PositionLengthAttributeImpl.CopyTo: target must implement PositionLengthAttribute")
	}
	t.SetPositionLength(p.positionLength)
}

// ReflectWith pushes the (PositionLengthAttribute, "positionLength",
// value) triple through reflector, matching the Lucene reference.
func (p *PositionLengthAttributeImpl) ReflectWith(reflector util.AttributeReflector) {
	reflector(PositionLengthAttributeType, "positionLength", p.positionLength)
}

// Equals returns true if other is a [PositionLengthAttributeImpl] with
// the same value, matching Lucene's {@code equals(Object)}.
func (p *PositionLengthAttributeImpl) Equals(other any) bool {
	if p == other {
		return true
	}
	o, ok := other.(*PositionLengthAttributeImpl)
	if !ok {
		return false
	}
	return p.positionLength == o.positionLength
}

// HashCode returns positionLength itself, matching
// {@code PositionLengthAttributeImpl#hashCode()}.
func (p *PositionLengthAttributeImpl) HashCode() int { return p.positionLength }
