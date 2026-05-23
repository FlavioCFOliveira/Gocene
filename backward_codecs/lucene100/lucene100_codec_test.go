// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene100

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
)

// TestLucene100Codec_Name verifies the codec name matches Lucene.
func TestLucene100Codec_Name(t *testing.T) {
	c := NewLucene100Codec()
	if got := c.Name(); got != "Lucene100" {
		t.Errorf("Name(): got %q, want %q", got, "Lucene100")
	}
}

// TestLucene100Codec_DefaultMode verifies that the no-arg constructor uses
// BEST_SPEED mode.
func TestLucene100Codec_DefaultMode(t *testing.T) {
	c := NewLucene100Codec()
	if c.Mode() != Lucene100ModeBestSpeed {
		t.Errorf("Mode(): got %v, want Lucene100ModeBestSpeed", c.Mode())
	}
}

// TestLucene100Codec_BestCompressionMode verifies explicit mode propagation.
func TestLucene100Codec_BestCompressionMode(t *testing.T) {
	c := NewLucene100CodecWithMode(Lucene100ModeBestCompression)
	if c.Mode() != Lucene100ModeBestCompression {
		t.Errorf("Mode(): got %v, want Lucene100ModeBestCompression", c.Mode())
	}
}

// TestLucene100Codec_PostingsFormat verifies that PostingsFormat returns a
// non-nil format with name "Lucene912".
func TestLucene100Codec_PostingsFormat(t *testing.T) {
	c := NewLucene100Codec()
	pf := c.PostingsFormat()
	if pf == nil {
		t.Fatal("PostingsFormat(): got nil")
	}
	if got := pf.Name(); got != "Lucene912" {
		t.Errorf("PostingsFormat().Name(): got %q, want %q", got, "Lucene912")
	}
}

// TestLucene100Codec_ImplementsCodec is a compile-time assertion surfaced as
// a runtime no-op.
func TestLucene100Codec_ImplementsCodec(t *testing.T) {
	var _ codecs.Codec = (*Lucene100Codec)(nil)
}
