// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene80

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
)

// TestLucene80NormsFormat_Name verifies the codec name matches Lucene.
func TestLucene80NormsFormat_Name(t *testing.T) {
	f := NewLucene80NormsFormat()
	if got := f.Name(); got != "Lucene80" {
		t.Errorf("Name(): got %q, want %q", got, "Lucene80")
	}
}

// TestLucene80NormsFormat_NormsConsumerUnsupported verifies that NormsConsumer
// returns a non-nil error (old codecs are read-only).
func TestLucene80NormsFormat_NormsConsumerUnsupported(t *testing.T) {
	f := NewLucene80NormsFormat()
	_, err := f.NormsConsumer(nil)
	if err == nil {
		t.Error("NormsConsumer: expected error for read-only codec, got nil")
	}
}

// TestLucene80NormsFormat_ImplementsInterface is a compile-time assertion
// surfaced as a runtime no-op.
func TestLucene80NormsFormat_ImplementsInterface(t *testing.T) {
	var _ codecs.NormsFormat = (*Lucene80NormsFormat)(nil)
}
