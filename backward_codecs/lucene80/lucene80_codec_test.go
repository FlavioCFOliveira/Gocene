// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene80

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
)

// TestLucene80Codec_Name verifies the codec name matches Lucene.
func TestLucene80Codec_Name(t *testing.T) {
	c := NewLucene80Codec()
	if got := c.Name(); got != "Lucene80" {
		t.Errorf("Name(): got %q, want %q", got, "Lucene80")
	}
}

// TestLucene80Codec_DocValuesFormat verifies that DocValuesFormat returns a
// non-nil Lucene80DocValuesFormat with the correct name.
func TestLucene80Codec_DocValuesFormat(t *testing.T) {
	c := NewLucene80Codec()
	dvf := c.DocValuesFormat()
	if dvf == nil {
		t.Fatal("DocValuesFormat(): got nil")
	}
	if got := dvf.Name(); got != "Lucene80" {
		t.Errorf("DocValuesFormat().Name(): got %q, want %q", got, "Lucene80")
	}
}

// TestLucene80Codec_NormsFormat verifies that NormsFormat returns a non-nil
// Lucene80NormsFormat with the correct name.
func TestLucene80Codec_NormsFormat(t *testing.T) {
	c := NewLucene80Codec()
	nf := c.NormsFormat()
	if nf == nil {
		t.Fatal("NormsFormat(): got nil")
	}
	if got := nf.Name(); got != "Lucene80" {
		t.Errorf("NormsFormat().Name(): got %q, want %q", got, "Lucene80")
	}
}

// TestLucene80Codec_ImplementsInterface is a compile-time assertion surfaced as
// a runtime no-op.
func TestLucene80Codec_ImplementsInterface(t *testing.T) {
	var _ codecs.Codec = (*Lucene80Codec)(nil)
}
