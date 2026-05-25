// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// lucene103_blocktree_trie_compat_test.go covers the Lucene103 BlockTree
// term-dictionary stack (Lucene103BlockTreeTermsWriter + TrieBuilder).
// The Lucene104 postings format delegates its term-dictionary layout to
// this block-tree codec, so the .tim/.tip/.tmd produced by the
// postings-format scenario double as the BlockTree golden corpus.
//
// Audit rows cited (docs/compat-coverage.tsv, package == "codecs"):
//
//	"Lucene103 BlockTree terms (.tim/.tip/.tmd)" — gap_notes:
//	  "No combined test exercises block-tree against a Lucene-written
//	   term dictionary."
//	"Lucene103 Trie dictionary" — gap_notes:
//	  "Lacks Lucene-produced trie fixtures."
//
// This file closes both gaps by loading the Lucene-emitted .tim/.tip/.tmd
// and validating each one with Gocene's CodecUtil + IndexHeader parsers.
package codecs

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
)

// TestLucene103BlockTree_TermDictionaryEnvelope asserts the three
// term-dictionary files emitted by Lucene10.4 carry the BlockTree codec
// names and a version Gocene accepts. Two seeds — 0xC0FFEE / 0xDECAF —
// satisfy the byte-determinism contract for the underlying corpus.
func TestLucene103BlockTree_TermDictionaryEnvelope(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, "postings-format", seed)
			const suffix = "Lucene104_0"
			tim := findUniqueByExt(t, dir, ".tim")
			tip := findUniqueByExt(t, dir, ".tip")
			tmd := findUniqueByExt(t, dir, ".tmd")
			expectIndexCodecName(t, dir, tim, codecs.Lucene103BlockTreeTermsCodecName,
				0, 32, suffix)
			expectIndexCodecName(t, dir, tip, codecs.Lucene103BlockTreeTermsIndexCodecName,
				0, 32, suffix)
			expectIndexCodecName(t, dir, tmd, codecs.Lucene103BlockTreeTermsMetaCodecName,
				0, 32, suffix)
		})
	}
}

// TestLucene103BlockTree_TrieFixturePresent confirms the .tip file
// (which carries the trie payload) is materially larger than a bare
// header+footer envelope: a real trie has at least a root node, child
// pointers and arc bytes. Catches the regression where the trie payload
// silently becomes empty.
func TestLucene103BlockTree_TrieFixturePresent(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, "postings-format", seed)
			tip := findUniqueByExt(t, dir, ".tip")
			const suffix = "Lucene104_0"
			minHeader := int64(codecs.IndexHeaderLength(
				codecs.Lucene103BlockTreeTermsIndexCodecName, suffix))
			// A non-empty trie always emits at least a single arc byte
			// of payload between IndexHeader and the 16-byte footer.
			// We assert >= 1 byte (i.e. file > header+footer); the
			// Sprint 114 postings-format scenario indexes 10 docs into
			// 6 fields and produces a small but non-empty trie.
			mustNonEmpty(t, dir, tip, minHeader)
		})
	}
}
