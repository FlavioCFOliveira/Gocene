// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package valuesource

import (
	"github.com/FlavioCFOliveira/Gocene/queries/function"
)

// FieldCacheSource is a base struct for ValueSource implementations that
// retrieve values for a single field from DocValues.
type FieldCacheSource struct {
	function.BaseValueSource
	Field string
}

// GetField returns the field name.
func (f FieldCacheSource) GetField() string {
	return f.Field
}

// Description returns the field name.
func (f FieldCacheSource) Description() string {
	return f.Field
}

// Equals reports value-equality based on the field name.
//
// Mirrors FieldCacheSource.equals(Object), whose `o instanceof FieldCacheSource`
// test becomes an assertion to the accessor the abstract base declares: a Go
// type that embeds FieldCacheSource is never assertable to the embedded struct
// itself.
func (f FieldCacheSource) Equals(other function.ValueSource) bool {
	if other == nil {
		return false
	}
	otherFS, ok := other.(interface{ GetField() string })
	if !ok {
		return false
	}
	return f.Field == otherFS.GetField()
}

// HashCode returns a stable hash of the field name.
func (f FieldCacheSource) HashCode() int32 {
	return hashString(f.Field)
}
