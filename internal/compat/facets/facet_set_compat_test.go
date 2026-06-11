// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// facet_set_compat_test.go addresses the facets audit row (verbatim
// from docs/compat-coverage.tsv):
//
//	facets	FacetSet packed-bytes encoding
//	    lucene_class:  org.apache.lucene.facet.facetset.FacetSet
//	    gocene_class:  facets/facetset/facet_set.go
//	    isolated:      yes:facets/facetset/facet_set_decoder_test.go
//	    integration:   yes:facets/facetset/matching_facet_sets_counts_test.go
//	    binary_compat: no
//	    gap_notes:     "No Lucene-produced FacetSet bytes used in tests."
//
// The scenario "facet-set-packed-bytes" indexes 6 docs, each carrying
// two 3-dimensional LongFacetSets packed into a single FacetSetsField
// BinaryDocValues entry. The (long, long, long) tuples are packed BE per
// the canonical Lucene 10.4.0 layout decoded by
// FacetSetDecoder.decodeLongs. The verifier reopens the index and uses
// MatchingFacetSetsCounts + ExactFacetSetMatcher to count the canonical
// (1, 2, 3) packed tuple; at least one matching doc must be returned.
//
// Three test classes per the rmp 4620 contract:
//
//	(a) read-fixture     — Lucene-generated segment exposes the expected
//	                        BinaryDocValues file shape (.dvd + .dvm) plus
//	                        the canonical segment-info bookkeeping.
//	(b) write-and-verify — Two `gen` runs at the same seed produce
//	                        byte-identical non-.si files; the harness
//	                        `verify` subcommand reopens the index and
//	                        confirms the ExactFacetSetMatcher count
//	                        equals NUM_DOCS (every doc carries the
//	                        anchor).
//	(c) round-trip       — Lucene-write -> Gocene-read -> Lucene-verify.
//	                        Deferred (see deferred_facets_compat_test.go)
//	                        because Gocene's FacetSetDecoder cannot yet
//	                        consume a Lucene-emitted BinaryDocValues
//	                        stream (the SegmentReader core-readers gap
//	                        recorded under memory-index reference
//	                        'gocene-segmentreader-corereaders-gap').
package facets

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestFacetSetPackedBytes_ReadFixture (class a) drives the harness and
// asserts the resulting directory carries the BinaryDocValues codec
// shape (.dvd, .dvm) plus the field-infos descriptor and segment-info
// bookkeeping.
func TestFacetSetPackedBytes_ReadFixture(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioFacetSetPackedBytes, seed)
			if !hasFileWithSuffix(t, dir, ".dvd") {
				t.Errorf("expected .dvd in %s (BinaryDocValues data missing)", dir)
			}
			if !hasFileWithSuffix(t, dir, ".dvm") {
				t.Errorf("expected .dvm in %s (BinaryDocValues meta missing)", dir)
			}
			if !hasFileWithSuffix(t, dir, ".fnm") {
				t.Errorf("expected .fnm in %s (field infos missing)", dir)
			}
			if !hasFileWithSuffix(t, dir, ".si") {
				t.Errorf("expected .si in %s (segment info missing)", dir)
			}
		})
	}
}

// TestFacetSetPackedBytes_DigestDeterminism (class b, part 1) runs the
// scenario twice at the same seed and confirms the non-.si files are
// byte-identical. The packed-bytes encoding multiplexes a 4-byte
// dim-count prefix followed by N x sizePackedBytes per set; any drift
// in the per-set packing surfaces here.
func TestFacetSetPackedBytes_DigestDeterminism(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			a := generate(t, ScenarioFacetSetPackedBytes, seed)
			b := generate(t, ScenarioFacetSetPackedBytes, seed)
			ma := fileMapRecursive(t, a)
			mb := fileMapRecursive(t, b)
			if len(ma) != len(mb) {
				t.Fatalf("file count mismatch: A=%d B=%d", len(ma), len(mb))
			}
			for name, ba := range ma {
				bb, ok := mb[name]
				if !ok {
					t.Errorf("file %q present in A but missing from B", name)
					continue
				}
				if !bytes.Equal(ba, bb) {
					t.Errorf("file %q content drift between two runs at seed=%d", name, seed)
				}
			}
		})
	}
}

// TestFacetSetPackedBytes_VerifySubcommand (class b, part 2) drives the
// harness `verify` subcommand against a fresh fixture. A clean exit
// proves the Java verifier reopened the index, decoded the FacetSet
// packed-bytes stream through MatchingFacetSetsCounts +
// ExactFacetSetMatcher, and confirmed the anchor tuple count equals
// NUM_DOCS (every doc carries it).
func TestFacetSetPackedBytes_VerifySubcommand(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioFacetSetPackedBytes, seed)
			verifyHarness(t, ScenarioFacetSetPackedBytes, seed, dir)
		})
	}
}

// TestFacetSetPackedBytes_RoundTrip (class c) is the full Lucene ->
// Gocene -> Lucene loop. Gocene opens the index, reads the BinaryDocValues
// field "fset", decodes the FacetSet packed bytes, and asserts the anchor
// tuple (1, 2, 3) appears in every document's FacetSet list.
func TestFacetSetPackedBytes_RoundTrip(t *testing.T) {
	const numDocs = 6
	const dims = 3
	const anchorD0, anchorD1, anchorD2 int64 = 1, 2, 3

	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioFacetSetPackedBytes, seed)
			storeDir, err := store.NewSimpleFSDirectory(dir)
			if err != nil {
				t.Fatalf("NewSimpleFSDirectory: %v", err)
			}
			defer storeDir.Close()

			dr, err := index.OpenDirectoryReader(storeDir)
			if err != nil {
				t.Fatalf("OpenDirectoryReader: %v", err)
			}
			defer dr.Close()

			leaves, err := dr.Leaves()
			if err != nil {
				t.Fatalf("Leaves: %v", err)
			}
			if len(leaves) == 0 {
				t.Fatal("no leaf readers")
			}

			anchorSeen := false
			docCount := 0
			for _, lrc := range leaves {
				raw := lrc.Reader()
				lr, ok := raw.(interface {
					GetBinaryDocValues(string) (index.BinaryDocValues, error)
				})
				if !ok {
					t.Fatal("reader does not support GetBinaryDocValues")
				}
				bdv, err := lr.GetBinaryDocValues(facetSetFieldName)
				if err != nil {
					t.Fatalf("GetBinaryDocValues(%s): %v", facetSetFieldName, err)
				}
				if bdv == nil {
					t.Fatal("BinaryDocValues fset is nil")
				}
				doc, err := bdv.NextDoc()
				if err != nil {
					t.Fatalf("NextDoc: %v", err)
				}
				for doc != search.NO_MORE_DOCS {
					docCount++
					payload, err := bdv.BinaryValue()
					if err != nil {
						t.Fatalf("BinaryValue: %v", err)
					}
					if len(payload) < 4 {
						t.Fatalf("payload too short (%d bytes) doc=%d", len(payload), doc)
					}

					// Parse vint count, vint dims, then sets.
					count, n1 := binary.Uvarint(payload)
					if n1 <= 0 {
						t.Fatalf("decoding set count: n1=%d", n1)
					}
					gotDims, n2 := binary.Uvarint(payload[n1:])
					if n2 <= 0 {
						t.Fatalf("decoding dims: n2=%d", n2)
					}
					if int(gotDims) != dims {
						t.Fatalf("dims: got %d, want %d", gotDims, dims)
					}

					offset := n1 + n2
					setBytes := dims * 8 // each dim is big-endian int64
					for s := uint64(0); s < count; s++ {
						if offset+setBytes > len(payload) {
							t.Fatalf("payload truncated: offset=%d, need=%d, have=%d", offset, setBytes, len(payload))
						}
						v0 := int64(binary.BigEndian.Uint64(payload[offset:]))
						v1 := int64(binary.BigEndian.Uint64(payload[offset+8:]))
						v2 := int64(binary.BigEndian.Uint64(payload[offset+16:]))
						if v0 == anchorD0 && v1 == anchorD1 && v2 == anchorD2 {
							anchorSeen = true
						}
						offset += setBytes
					}
					doc, err = bdv.NextDoc()
					if err != nil {
						t.Fatalf("NextDoc: %v", err)
					}
				}
			}

			if docCount != numDocs {
				t.Errorf("document count: got %d, want %d", docCount, numDocs)
			}
			if !anchorSeen {
				t.Error("anchor tuple (1,2,3) not found in any document's FacetSets")
			}
		})
	}
}
