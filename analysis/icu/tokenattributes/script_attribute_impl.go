// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package tokenattributes

import (
	"reflect"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// ScriptAttributeImpl is the concrete implementation of ScriptAttribute
// that stores the script code as an integer.
//
// Go port of
// org.apache.lucene.analysis.icu.tokenattributes.ScriptAttributeImpl
// (Apache Lucene 10.4.0).
type ScriptAttributeImpl struct {
	code int
}

// Compile-time interface assertions.
var (
	_ ScriptAttribute             = (*ScriptAttributeImpl)(nil)
	_ util.AttributeImpl          = (*ScriptAttributeImpl)(nil)
	_ util.AttributeInterfaceProvider = (*ScriptAttributeImpl)(nil)
)

// NewScriptAttributeImpl initialises this attribute with UScript.COMMON.
func NewScriptAttributeImpl() *ScriptAttributeImpl {
	return &ScriptAttributeImpl{code: UScriptCommon}
}

// AttributeInterfaces satisfies [util.AttributeInterfaceProvider].
func (a *ScriptAttributeImpl) AttributeInterfaces() []reflect.Type {
	return []reflect.Type{ScriptAttributeType}
}

// GetCode returns the UScript numeric code.
func (a *ScriptAttributeImpl) GetCode() int { return a.code }

// SetCode sets the UScript numeric code.
func (a *ScriptAttributeImpl) SetCode(code int) { a.code = code }

// GetName returns the full script name for the current code.
func (a *ScriptAttributeImpl) GetName() string { return ScriptGetName(a.code) }

// GetShortName returns the abbreviated ISO 15924 identifier.
func (a *ScriptAttributeImpl) GetShortName() string { return ScriptGetShortName(a.code) }

// Clear resets the attribute to UScript.COMMON.
func (a *ScriptAttributeImpl) Clear() { a.code = UScriptCommon }

// End implements [util.AttributeImpl].
func (a *ScriptAttributeImpl) End() {}

// CloneAttribute returns a deep copy as [util.AttributeImpl].
func (a *ScriptAttributeImpl) CloneAttribute() util.AttributeImpl {
	return &ScriptAttributeImpl{code: a.code}
}

// CopyTo copies this attribute's state onto target.
func (a *ScriptAttributeImpl) CopyTo(target util.AttributeImpl) {
	t, ok := target.(ScriptAttribute)
	if !ok {
		panic("ScriptAttributeImpl.CopyTo: target must implement ScriptAttribute")
	}
	t.SetCode(a.code)
}

// ReflectWith emits the script name using the attribute reflector,
// matching the Java implementation that emits a human-readable name.
func (a *ScriptAttributeImpl) ReflectWith(reflector util.AttributeReflector) {
	// When wordbreaking CJK, UScript.JAPANESE marks Chinese/Japanese runs.
	name := a.GetName()
	if a.code == 105 { // UScript.JAPANESE
		name = "Chinese/Japanese"
	}
	reflector(ScriptAttributeType, "script", name)
}
