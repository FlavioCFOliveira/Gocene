// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package bloom_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs/bloom"
)

// TestBloomPostingsFormat validates the BloomFilteringPostingsFormat,
// FuzzySet, and MurmurHash64 implementations.
// Port of org.apache.lucene.codecs.bloom.TestBloomPostingsFormat.
func TestBloomPostingsFormat(t *testing.T) {
	t.Run("MurmurHash64 deterministic", func(t *testing.T) {
		h := bloom.NewMurmurHash64(42)
		v1 := h.Hash([]byte("test"))
		v2 := h.Hash([]byte("test"))
		if v1 != v2 {
			t.Errorf("MurmurHash64 not deterministic: %d != %d", v1, v2)
		}
		v3 := h.Hash([]byte("different"))
		if v1 == v3 && len("different") != len("test") {
			t.Logf("hash collision between 'test' and 'different': %d (rare but possible)", v1)
		}
	})

	t.Run("MurmurHash64 different seeds", func(t *testing.T) {
		h1 := bloom.NewMurmurHash64(0)
		h2 := bloom.NewMurmurHash64(12345)
		d := []byte("hello")
		if h1.Hash(d) == h2.Hash(d) {
			t.Log("hash collision with different seeds (rare but possible)")
		}
	})

	t.Run("FuzzySet add and mayContain", func(t *testing.T) {
		fs := bloom.NewFuzzySet(1024, bloom.NewMurmurHash64(0))
		fs.Add([]byte("lucene"))
		fs.Add([]byte("gocene"))
		if !fs.MayContain([]byte("lucene")) {
			t.Error("FuzzySet.MayContain('lucene') = false after Add")
		}
		if !fs.MayContain([]byte("gocene")) {
			t.Error("FuzzySet.MayContain('gocene') = false after Add")
		}
		// MayContain must never return false for an added item
	})

	t.Run("FuzzySet false positives are possible", func(t *testing.T) {
		// With a small filter, false positives are expected — verify the
		// API contract: MayContain must never crash.
		fs := bloom.NewFuzzySet(64, bloom.NewMurmurHash64(0))
		for i := 0; i < 100; i++ {
			fs.Add([]byte{byte(i)})
		}
		// Just checking the API doesn't panic
		_ = fs.MayContain([]byte("never-added"))
	})

	t.Run("BloomFilteringPostingsFormat", func(t *testing.T) {
		factory := bloom.DefaultBloomFilterFactory{}
		format := bloom.NewBloomFilteringPostingsFormat(nil, factory)
		if format == nil {
			t.Fatal("NewBloomFilteringPostingsFormat returned nil")
		}
		if format.Factory == nil {
			t.Error("Factory not set")
		}
	})

	t.Run("DefaultBloomFilterFactory sizes correctly", func(t *testing.T) {
		factory := bloom.DefaultBloomFilterFactory{}
		fs := factory.NewFilter(100)
		if fs == nil {
			t.Fatal("NewFilter returned nil")
		}
		// 100 docs * 10 bits = 1000 bits → ceil(1000/64) = 16 words
		expectedWords := (100*10 + 63) / 64
		if len(fs.Bits) != expectedWords {
			t.Errorf("FuzzySet.Bits length = %d, want %d for 100 docs", len(fs.Bits), expectedWords)
		}
	})

	t.Run("default factory for zero docs", func(t *testing.T) {
		factory := bloom.DefaultBloomFilterFactory{}
		fs := factory.NewFilter(0)
		if fs == nil {
			t.Fatal("NewFilter(0) returned nil")
		}
		// Should clamp to at least 1 doc
		if len(fs.Bits) == 0 {
			t.Error("FuzzySet created with zero bits")
		}
	})
}
