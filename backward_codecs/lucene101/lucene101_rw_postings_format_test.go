// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene101

import (
	"testing"
)

// TestLucene101RWPostingsFormat_BlockSize verifies the BlockSize constant.
func TestLucene101RWPostingsFormat_BlockSize(t *testing.T) {
	if BlockSize != 128 {
		t.Errorf("BlockSize = %d, want 128", BlockSize)
	}
}

// TestLucene101RWPostingsFormat_PostingsFormatConstuctor verifies the
// Lucene101PostingsFormat constructor and fields.
func TestLucene101RWPostingsFormat_PostingsFormatConstuctor(t *testing.T) {
	pf := NewLucene101PostingsFormat("10.1")
	if pf.Name != "Lucene101PostingsFormat" {
		t.Errorf("Name = %q, want %q", pf.Name, "Lucene101PostingsFormat")
	}
	if pf.Version != "10.1" {
		t.Errorf("Version = %q, want %q", pf.Version, "10.1")
	}
}

// TestLucene101RWPostingsFormat_CodecConstructor verifies the Lucene101Codec constructor.
func TestLucene101RWPostingsFormat_CodecConstructor(t *testing.T) {
	c := NewLucene101Codec("10.1.0")
	if c.Name != "Lucene101Codec" {
		t.Errorf("Codec.Name = %q, want %q", c.Name, "Lucene101Codec")
	}
	if c.Version != "10.1.0" {
		t.Errorf("Codec.Version = %q, want %q", c.Version, "10.1.0")
	}
}
