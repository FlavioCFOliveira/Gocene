// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"github.com/FlavioCFOliveira/Gocene/util"
)

// MockAttribute is a minimal [util.AttributeImpl] used by analysis-
// package tests that need to assert behaviour against an "unrelated"
// AttributeImpl (e.g. Equals returning false, CopyTo panicking on a
// mismatched target type). It is intentionally package-private to the
// analysis test binary.
type MockAttribute struct {
	value string
}

// Clear resets the mock to its empty value.
func (m *MockAttribute) Clear() { m.value = "" }

// End delegates to Clear, matching the Lucene default
// {@code AttributeImpl#end() -> clear()}.
func (m *MockAttribute) End() { m.Clear() }

// CopyTo copies the mock value when target is also a *MockAttribute.
// Mismatched targets are silently ignored to keep the helper benign in
// negative-test scenarios.
func (m *MockAttribute) CopyTo(target util.AttributeImpl) {
	if other, ok := target.(*MockAttribute); ok {
		other.value = m.value
	}
}

// ReflectWith is a no-op: the mock advertises no triples.
func (m *MockAttribute) ReflectWith(reflector util.AttributeReflector) {}

// CloneAttribute returns a deep clone of the mock as a
// [util.AttributeImpl].
func (m *MockAttribute) CloneAttribute() util.AttributeImpl {
	return &MockAttribute{value: m.value}
}
