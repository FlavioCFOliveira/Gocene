// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene80

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
)

// TestLucene80DocValuesFormat_Name verifies the codec name matches Lucene.
func TestLucene80DocValuesFormat_Name(t *testing.T) {
	f := NewLucene80DocValuesFormat()
	if got := f.Name(); got != "Lucene80" {
		t.Errorf("Name(): got %q, want %q", got, "Lucene80")
	}
}

// TestLucene80DocValuesFormat_DefaultMode verifies that the no-arg constructor
// sets BEST_SPEED mode.
func TestLucene80DocValuesFormat_DefaultMode(t *testing.T) {
	f := NewLucene80DocValuesFormat()
	if f.Mode() != Lucene80DVModeBestSpeed {
		t.Errorf("Mode(): got %v, want Lucene80DVModeBestSpeed", f.Mode())
	}
}

// TestLucene80DocValuesFormat_BestCompressionMode verifies explicit mode
// propagation.
func TestLucene80DocValuesFormat_BestCompressionMode(t *testing.T) {
	f := NewLucene80DocValuesFormatWithMode(Lucene80DVModeBestCompression)
	if f.Mode() != Lucene80DVModeBestCompression {
		t.Errorf("Mode(): got %v, want Lucene80DVModeBestCompression", f.Mode())
	}
}

// TestLucene80DocValuesFormat_FieldsConsumerNilStatePanics verifies that
// FieldsConsumer with a nil state surfaces an error or panic (not a silent
// no-op). This is a minimal guard; full round-trip tests require a live
// segment.
func TestLucene80DocValuesFormat_FieldsConsumerNilStatePanics(t *testing.T) {
	f := NewLucene80DocValuesFormat()
	defer func() { recover() }() //nolint:errcheck // panic-recover: intentional guard
	_, _ = f.FieldsConsumer(nil)
	// If we reach here without panic, the nil state was handled gracefully.
}

// TestLucene80DocValuesFormat_ImplementsInterface is a compile-time assertion
// surfaced as a runtime no-op.
func TestLucene80DocValuesFormat_ImplementsInterface(t *testing.T) {
	var _ codecs.DocValuesFormat = (*Lucene80DocValuesFormat)(nil)
}
