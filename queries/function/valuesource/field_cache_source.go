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
func (f FieldCacheSource) Equals(other function.ValueSource) bool {
	if other == nil {
		return false
	}
	if otherFS, ok := other.(FieldCacheSource); ok {
		return f.Field == otherFS.Field
	}
	// Check for potential pointer/interface variations if needed,
	// but for FieldCacheSource, matching the field name is the core identity.
	return false
}

// HashCode returns a stable hash of the field name.
func (f FieldCacheSource) HashCode() int32 {
	return int32(hashString(f.Field))
}

func hashString(s string) uint32 {
	var h uint32 = 2166136261
	for i := 0; i < len(s); i++ {
		h *= 16777619
		h ^= uint32(s[i])
	}
	return h
}
