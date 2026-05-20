// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
// lucene/core/src/java/org/apache/lucene/analysis/tokenattributes/FlagsAttributeImpl.java

package analysis

import (
	"reflect"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// FlagsAttributeImpl is the Go port of Lucene's
// org.apache.lucene.analysis.tokenattributes.FlagsAttributeImpl.
//
// It is the exported concrete implementation of [FlagsAttribute].
// The flags are stored as a single int bitmask, defaulting to 0.
//
// Lucene 10.4.0 reference:
//
//	lucene/core/src/java/org/apache/lucene/analysis/tokenattributes/FlagsAttributeImpl.java
type FlagsAttributeImpl struct {
	flags int
}

// Compile-time assertions.
var (
	_ util.AttributeImpl              = (*FlagsAttributeImpl)(nil)
	_ FlagsAttribute                  = (*FlagsAttributeImpl)(nil)
	_ util.AttributeInterfaceProvider = (*FlagsAttributeImpl)(nil)
)

// AttributeInterfaces satisfies [util.AttributeInterfaceProvider].
func (f *FlagsAttributeImpl) AttributeInterfaces() []reflect.Type {
	return []reflect.Type{FlagsAttributeType}
}

// NewFlagsAttributeImpl initialises this attribute with no bits set,
// matching the Lucene no-arg constructor.
func NewFlagsAttributeImpl() *FlagsAttributeImpl {
	return &FlagsAttributeImpl{}
}

// GetFlags returns the current flag bitmask.
func (f *FlagsAttributeImpl) GetFlags() int { return f.flags }

// SetFlags replaces the flag bitmask.
func (f *FlagsAttributeImpl) SetFlags(flags int) { f.flags = flags }

// IsFlagSet returns true when the given flag bit is present in the
// current bitmask.
func (f *FlagsAttributeImpl) IsFlagSet(flag int) bool { return f.flags&flag != 0 }

// SetFlag sets or clears the given flag bit.
func (f *FlagsAttributeImpl) SetFlag(flag int, set bool) {
	if set {
		f.flags |= flag
	} else {
		f.flags &= ^flag
	}
}

// Clear resets the flags to 0, matching
// {@code FlagsAttributeImpl#clear()}.
func (f *FlagsAttributeImpl) Clear() { f.flags = 0 }

// End implements util.AttributeImpl.End. The Lucene base calls
// clear() from end().
func (f *FlagsAttributeImpl) End() { f.Clear() }

// CloneAttribute returns a deep copy of this impl as [util.AttributeImpl].
func (f *FlagsAttributeImpl) CloneAttribute() util.AttributeImpl {
	return &FlagsAttributeImpl{flags: f.flags}
}

// CopyTo copies the flags onto target, which must implement
// [FlagsAttribute]; a panic is raised otherwise.
func (f *FlagsAttributeImpl) CopyTo(target util.AttributeImpl) {
	t, ok := target.(FlagsAttribute)
	if !ok {
		panic("FlagsAttributeImpl.CopyTo: target must implement FlagsAttribute")
	}
	t.SetFlags(f.flags)
}

// ReflectWith pushes the (FlagsAttribute, "flags", value) triple
// through reflector, matching the Lucene reference.
func (f *FlagsAttributeImpl) ReflectWith(reflector util.AttributeReflector) {
	reflector(FlagsAttributeType, "flags", f.flags)
}

// Equals returns true if other is a [FlagsAttributeImpl] with the same
// bitmask, matching Lucene's {@code equals(Object)}.
func (f *FlagsAttributeImpl) Equals(other any) bool {
	if f == other {
		return true
	}
	o, ok := other.(*FlagsAttributeImpl)
	if !ok {
		return false
	}
	return f.flags == o.flags
}

// HashCode returns the flag bitmask itself, matching
// {@code FlagsAttributeImpl#hashCode()}.
func (f *FlagsAttributeImpl) HashCode() int { return f.flags }
