// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene80

import (
	"testing"
)

// TestLucene80NormsFormatMergeInstance_Name verifies the NormsFormat name.
func TestLucene80NormsFormatMergeInstance_Name(t *testing.T) {
	f := NewLucene80NormsFormat()
	if got := f.Name(); got != "Lucene80" {
		t.Errorf("Name(): got %q, want %q", got, "Lucene80")
	}
}

// TestLucene80NormsFormatMergeInstance_NormsConsumer verifies that
// NormsConsumer returns the expected error for this read-only codec.
func TestLucene80NormsFormatMergeInstance_NormsConsumer(t *testing.T) {
	f := NewLucene80NormsFormat()
	_, err := f.NormsConsumer(nil)
	if err == nil {
		t.Error("NormsConsumer: expected error for read-only codec, got nil")
	}
}
