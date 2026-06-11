// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package backward_index

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// ---------------------------------------------------------------------------
// bwcTestBase: shared helpers for backwards-compatibility tests.
// Port of org.apache.lucene.backward_index.BackwardsCompatibilityTestBase
// (Lucene 10.4.0).
// ---------------------------------------------------------------------------

type bwcTestBase struct {
	t *testing.T
}

func newBwcTestBase(t *testing.T) *bwcTestBase {
	return &bwcTestBase{t: t}
}

func (b *bwcTestBase) createDir() store.Directory {
	return store.NewByteBuffersDirectory()
}

func (b *bwcTestBase) createAnalyzer() analysis.Analyzer {
	return analysis.NewWhitespaceAnalyzer()
}

func (b *bwcTestBase) createConfig(codec index.Codec) *index.IndexWriterConfig {
	config := index.NewIndexWriterConfig(b.createAnalyzer())
	if codec != nil {
		config.SetCodec(codec)
	}
	return config
}

// registerBackwardCodec registers a FilterCodec wrapping Lucene104Codec with
// the given name, allowing the reader path to resolve it. Returns the codec.
func (b *bwcTestBase) registerBackwardCodec(name string) *codecs.FilterCodec {
	bc := codecs.NewFilterCodec(name, codecs.NewLucene104Codec())
	index.RegisterNamedCodec(name, bc)
	return bc
}

func (b *bwcTestBase) mustOpenReader(dir store.Directory) *index.DirectoryReader {
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		b.t.Fatalf("OpenDirectoryReader: %v", err)
	}
	return reader
}

func (b *bwcTestBase) mustClose(r *index.DirectoryReader) {
	if err := r.Close(); err != nil {
		b.t.Errorf("Close: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Pure helper functions
// ---------------------------------------------------------------------------

// intToStr converts an int to its decimal string representation.
func intToStr(i int) string {
	if i == 0 {
		return "0"
	}
	s := ""
	neg := i < 0
	if neg {
		i = -i
	}
	for i > 0 {
		s = string(rune('0'+i%10)) + s
		i /= 10
	}
	if neg {
		s = "-" + s
	}
	return s
}

// int32ToBytes encodes an int as a 4-byte big-endian slice.
func int32ToBytes(i int) []byte {
	v := int32(i)
	return []byte{
		byte(v >> 24),
		byte(v >> 16),
		byte(v >> 8),
		byte(v),
	}
}

// ---------------------------------------------------------------------------
// Smoke tests
// ---------------------------------------------------------------------------

func TestBwcTestBaseSmoke(t *testing.T) {
	base := newBwcTestBase(t)
	dir := base.createDir()
	dir.Close()
}

func TestBackwardCodecRegistration(t *testing.T) {
	base := newBwcTestBase(t)

	if found := index.LookupCodecByName("LuceneTestBackward"); found != nil {
		t.Fatal("expected nil before registration")
	}

	bc := base.registerBackwardCodec("LuceneTestBackward")

	found := index.LookupCodecByName("LuceneTestBackward")
	if found == nil {
		t.Fatal("expected non-nil after registration")
	}
	if found.Name() != "LuceneTestBackward" {
		t.Fatalf("expected LuceneTestBackward, got %s", found.Name())
	}
	if found.PostingsFormat() == nil {
		t.Fatal("PostingsFormat should not be nil")
	}
	if found.StoredFieldsFormat() == nil {
		t.Fatal("StoredFieldsFormat should not be nil")
	}
	if found.DocValuesFormat() == nil {
		t.Fatal("DocValuesFormat should not be nil")
	}
	_ = bc
}
