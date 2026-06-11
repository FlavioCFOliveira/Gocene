// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package compressing

import (
	"testing"
)

// TestLegacyFieldsIndex_CompileTimeAssertion verifies that *LegacyFieldsIndexReader
// satisfies the LegacyFieldsIndex interface at compile time.
func TestLegacyFieldsIndex_CompileTimeAssertion(t *testing.T) {
	var _ LegacyFieldsIndex = (*LegacyFieldsIndexReader)(nil)
}

// TestLegacyFieldsIndexReader_Defaults verifies that a zero-value
// LegacyFieldsIndexReader has a non-empty String and that the no-op methods
// work without panicking.
func TestLegacyFieldsIndexReader_Defaults(t *testing.T) {
	r := &LegacyFieldsIndexReader{}
	if r.String() == "" {
		t.Error("String(): expected non-empty")
	}
	if err := r.CheckIntegrity(); err != nil {
		t.Errorf("CheckIntegrity: unexpected error: %v", err)
	}
	clone := r.Clone()
	if clone == nil {
		t.Error("Clone(): returned nil")
	}
	if err := r.Close(); err != nil {
		t.Errorf("Close: unexpected error: %v", err)
	}
}
