// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
// lucene/core/src/java/org/apache/lucene/analysis/tokenattributes/PositionIncrementAttributeImpl.java

package analysis

import (
	"fmt"
	"reflect"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// PositionIncrementAttributeImpl is the Go port of Lucene's
// org.apache.lucene.analysis.tokenattributes.PositionIncrementAttributeImpl.
//
// It is the exported concrete implementation of
// [PositionIncrementAttribute]. The default position increment is 1.
// End() resets to 0 (distinct from Clear which resets to 1), mirroring
// the Lucene override.
//
// Lucene 10.4.0 reference:
//
//	lucene/core/src/java/org/apache/lucene/analysis/tokenattributes/PositionIncrementAttributeImpl.java
type PositionIncrementAttributeImpl struct {
	positionIncrement int
}

// Compile-time assertions.
var (
	_ util.AttributeImpl              = (*PositionIncrementAttributeImpl)(nil)
	_ PositionIncrementAttribute      = (*PositionIncrementAttributeImpl)(nil)
	_ util.AttributeInterfaceProvider = (*PositionIncrementAttributeImpl)(nil)
)

// AttributeInterfaces satisfies [util.AttributeInterfaceProvider].
func (p *PositionIncrementAttributeImpl) AttributeInterfaces() []reflect.Type {
	return []reflect.Type{PositionIncrementAttributeType}
}

// NewPositionIncrementAttributeImpl initialises this attribute with the
// default position increment of 1, matching the Lucene no-arg
// constructor.
func NewPositionIncrementAttributeImpl() *PositionIncrementAttributeImpl {
	return &PositionIncrementAttributeImpl{positionIncrement: 1}
}

// GetPositionIncrement returns the position increment.
func (p *PositionIncrementAttributeImpl) GetPositionIncrement() int { return p.positionIncrement }

// SetPositionIncrement replaces the position increment. Panics when
// the value is negative, matching Lucene's
// {@code PositionIncrementAttributeImpl#setPositionIncrement(int)}.
func (p *PositionIncrementAttributeImpl) SetPositionIncrement(inc int) {
	if inc < 0 {
		panic(fmt.Sprintf(
			"PositionIncrementAttributeImpl.SetPositionIncrement: position increment must be zero or greater; got %d",
			inc))
	}
	p.positionIncrement = inc
}

// Clear resets the position increment to 1, matching
// {@code PositionIncrementAttributeImpl#clear()}.
func (p *PositionIncrementAttributeImpl) Clear() { p.positionIncrement = 1 }

// End resets the position increment to 0, matching
// {@code PositionIncrementAttributeImpl#end()}.
func (p *PositionIncrementAttributeImpl) End() { p.positionIncrement = 0 }

// Copy returns a deep clone of this impl as [util.AttributeImpl].
func (p *PositionIncrementAttributeImpl) Copy() util.AttributeImpl {
	return &PositionIncrementAttributeImpl{positionIncrement: p.positionIncrement}
}

// CloneAttribute returns a deep copy of this impl as [util.AttributeImpl].
func (p *PositionIncrementAttributeImpl) CloneAttribute() util.AttributeImpl {
	return p.Copy()
}

// CopyTo copies the position increment onto target, which must
// implement [PositionIncrementAttribute]; a panic is raised otherwise.
func (p *PositionIncrementAttributeImpl) CopyTo(target util.AttributeImpl) {
	t, ok := target.(PositionIncrementAttribute)
	if !ok {
		panic("PositionIncrementAttributeImpl.CopyTo: target must implement PositionIncrementAttribute")
	}
	t.SetPositionIncrement(p.positionIncrement)
}

// ReflectWith pushes the (PositionIncrementAttribute,
// "positionIncrement", value) triple through reflector, matching the
// Lucene reference.
func (p *PositionIncrementAttributeImpl) ReflectWith(reflector util.AttributeReflector) {
	reflector(PositionIncrementAttributeType, "positionIncrement", p.positionIncrement)
}

// Equals returns true if other is a [PositionIncrementAttributeImpl]
// with the same value, matching Lucene's {@code equals(Object)}.
func (p *PositionIncrementAttributeImpl) Equals(other any) bool {
	if p == other {
		return true
	}
	o, ok := other.(*PositionIncrementAttributeImpl)
	if !ok {
		return false
	}
	return p.positionIncrement == o.positionIncrement
}

// HashCode returns positionIncrement itself, matching
// {@code PositionIncrementAttributeImpl#hashCode()}.
func (p *PositionIncrementAttributeImpl) HashCode() int { return p.positionIncrement }
