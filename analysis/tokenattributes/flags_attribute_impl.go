// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package tokenattributes

import (
	"reflect"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// FlagsAttributeType is the reflect.Type of the
// FlagsAttribute interface, used as the lookup key for
// AttributeSource.
var FlagsAttributeType = reflect.TypeOf((*FlagsAttribute)(nil)).Elem()

// FlagsAttribute provides a way to set and get flags for a token.
//
// This is the Go port of
// org.apache.lucene.analysis.tokenattributes.FlagsAttribute.
type FlagsAttribute interface {
	util.AttributeImpl

	// GetFlags returns the flags of this token.
	GetFlags() int

	// SetFlags sets the flags.
	SetFlags(flags int)
}

// flagsAttribute is the default implementation.
type flagsAttribute struct {
	util.BaseAttributeImpl
	flags int
}

// NewFlagsAttribute creates a new FlagsAttribute with no bits set.
func NewFlagsAttribute() FlagsAttribute {
	return &flagsAttribute{
		flags: 0,
	}
}

// GetFlags returns the flags.
func (a *flagsAttribute) GetFlags() int {
	return a.flags
}

// SetFlags sets the flags.
func (a *flagsAttribute) SetFlags(flags int) {
	a.flags = flags
}

// Clear resets the flags to 0.
func (a *flagsAttribute) Clear() {
	a.flags = 0
}

// CopyTo copies this attribute's state onto target.
func (a *flagsAttribute) CopyTo(target util.AttributeImpl) {
	if t, ok := target.(FlagsAttribute); ok {
		t.SetFlags(a.flags)
	}
}

// CloneAttribute returns a deep clone of this attribute.
func (a *flagsAttribute) CloneAttribute() util.AttributeImpl {
	return &flagsAttribute{
		flags: a.flags,
	}
}

// ReflectWith emits the single (FlagsAttribute, "flags", value) triple.
func (a *flagsAttribute) ReflectWith(reflector util.AttributeReflector) {
	reflector(FlagsAttributeType, "flags", a.flags)
}

// Equals returns true if other is a FlagsAttribute with the same value.
func (a *flagsAttribute) Equals(other any) bool {
	if a == other {
		return true
	}
	o, ok := other.(FlagsAttribute)
	if !ok {
		return false
	}
	return a.flags == o.GetFlags()
}

// HashCode returns the flags themselves.
func (a *flagsAttribute) HashCode() int {
	return a.flags
}
