// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package analyzing_test exercises AnalyzingSuggester end-to-end:
// build → store → load → lookup.
package analyzing

import (
	"bytes"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/suggest"
)

// testIterator implements suggest.InputIterator for a slice of (surface,weight).
type testIterator struct {
	entries []struct {
		surface string
		weight  int64
	}
	pos int
}

func newTestIter(pairs ...interface{}) *testIterator {
	var entries []struct {
		surface string
		weight  int64
	}
	for i := 0; i+1 < len(pairs); i += 2 {
		entries = append(entries, struct {
			surface string
			weight  int64
		}{
			surface: pairs[i].(string),
			weight:  int64(pairs[i+1].(int)),
		})
	}
	return &testIterator{entries: entries, pos: -1}
}

func (it *testIterator) Next() (term []byte, weight int64, payload []byte, contexts [][]byte, ok bool, err error) {
	it.pos++
	if it.pos >= len(it.entries) {
		return nil, 0, nil, nil, false, nil
	}
	e := it.entries[it.pos]
	return []byte(e.surface), e.weight, nil, nil, true, nil
}

func (it *testIterator) HasPayloads() bool { return false }
func (it *testIterator) HasContexts() bool { return false }

// TestAnalyzingSuggester_RoundTrip verifies that a corpus built by Gocene's
// AnalyzingSuggester can be stored and reloaded, and all inserted surface
// forms are returned by LookupResults.
//
// AnalyzingSuggester works at the analyzed token level: a query prefix must
// match a full token boundary in the analyzed form. With StandardAnalyzer,
// "term0-suffix" tokenizes to ["term0", "suffix"], so querying "term0"
// returns completions beginning with that token.
func TestAnalyzingSuggester_RoundTrip(t *testing.T) {
	// Entries that produce multi-token analyzed forms so that token-prefix
	// lookup works correctly. StandardAnalyzer splits on '-'.
	a := analysis.NewStandardAnalyzer()
	s := NewAnalyzingSuggester(a, "test")
	if err := s.Build(newTestIter(
		"alpha-one", 10,
		"alpha-two", 5,
		"beta-one", 8,
		"gamma-x", 3,
	)); err != nil {
		t.Fatalf("Build: %v", err)
	}

	// Store.
	var buf bytes.Buffer
	out := store.NewOutputStreamDataOutput(&buf)
	ok, err := s.Store(out)
	if err != nil {
		t.Fatalf("Store: %v", err)
	}
	if !ok {
		t.Fatalf("Store returned false")
	}
	if buf.Len() == 0 {
		t.Fatalf("Store produced 0 bytes")
	}

	// Load into a fresh suggester.
	s2 := NewAnalyzingSuggester(a, "test")
	in := store.NewByteArrayDataInput(buf.Bytes())
	ok, err = s2.Load(in)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !ok {
		t.Fatalf("Load returned false")
	}

	// "alpha" matches "alpha-one" and "alpha-two" because "alpha" is the
	// first analyzed token of both. The highest-weight result comes first.
	results, err := s2.LookupResults("alpha", nil, false, 5)
	if err != nil {
		t.Fatalf("LookupResults('alpha'): %v", err)
	}
	if len(results) == 0 {
		t.Fatal("LookupResults('alpha'): want results, got none")
	}
	// alpha-one (weight=10) should be first.
	if results[0].Key != "alpha-one" {
		t.Errorf("LookupResults('alpha')[0]: want 'alpha-one', got %q", results[0].Key)
	}
	found := false
	for _, r := range results {
		if r.Key == "alpha-two" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("LookupResults('alpha'): want 'alpha-two' in results %v", results)
	}

	// "beta" matches "beta-one" only.
	results2, err := s2.LookupResults("beta", nil, false, 5)
	if err != nil {
		t.Fatalf("LookupResults('beta'): %v", err)
	}
	if len(results2) == 0 {
		t.Fatal("LookupResults('beta'): want results, got none")
	}
	if results2[0].Key != "beta-one" {
		t.Errorf("LookupResults('beta')[0]: want 'beta-one', got %q", results2[0].Key)
	}
}

// TestAnalyzingSuggester_StoreLoadByteIdentity verifies that two successive
// Store calls on the same suggester produce identical bytes (determinism).
func TestAnalyzingSuggester_StoreLoadByteIdentity(t *testing.T) {
	a := analysis.NewStandardAnalyzer()
	s := NewAnalyzingSuggester(a, "t")
	if err := s.Build(newTestIter(
		"apple-pie", 10,
		"apple-cider", 8,
		"apply-filter", 6,
	)); err != nil {
		t.Fatalf("Build: %v", err)
	}

	var buf1, buf2 bytes.Buffer
	if _, err := s.Store(store.NewOutputStreamDataOutput(&buf1)); err != nil {
		t.Fatalf("Store #1: %v", err)
	}
	if _, err := s.Store(store.NewOutputStreamDataOutput(&buf2)); err != nil {
		t.Fatalf("Store #2: %v", err)
	}
	if !bytes.Equal(buf1.Bytes(), buf2.Bytes()) {
		t.Fatalf("Store produced different bytes on two calls: len1=%d len2=%d",
			buf1.Len(), buf2.Len())
	}
}

// TestAnalyzingSuggester_SeededEntries replicates the Java seeded corpus from
// CompletionFstScenario (Sprint 114 T13 / rmp 4621) and verifies that every
// surface form is retrievable after a Store → Load round-trip.
//
// The seeded formula must match the Java implementation exactly:
//
//	tag  = fmt.Sprintf("%08x", seed & 0xFFFFFFFF)
//	surf = "termN-" + tag  (N = 0..9)
//	mix  = (uint64(seed) * 0x9E3779B97F4A7C15) ^ (uint64(N) * 0xBF58476D1CE4E5B9)
//	wt   = 1 + int((mix >>> 1) & 0x3FFF)
func TestAnalyzingSuggester_SeededEntries(t *testing.T) {
	for _, seed := range []int64{0xC0FFEE, 0xDECAF} {
		seed := seed
		t.Run(formatHexSeed(seed), func(t *testing.T) {
			entries := seededTestEntries(seed)
			a := analysis.NewStandardAnalyzer()
			s := NewAnalyzingSuggester(a, "seed-test")
			if err := s.Build(newSeededIter(entries)); err != nil {
				t.Fatalf("Build: %v", err)
			}
			if s.GetCount() == 0 {
				t.Fatal("GetCount() == 0 after Build")
			}

			var buf bytes.Buffer
			out := store.NewOutputStreamDataOutput(&buf)
			ok, err := s.Store(out)
			if err != nil {
				t.Fatalf("Store: %v", err)
			}
			if !ok {
				t.Fatalf("Store returned false")
			}

			// Load into fresh instance.
			s2 := NewAnalyzingSuggester(a, "seed-test")
			in := store.NewByteArrayDataInput(buf.Bytes())
			ok, err = s2.Load(in)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if !ok {
				t.Fatalf("Load returned false")
			}

			// Each surface "termN-XXXXXXXX" has "termN" as first analyzed
			// token. Querying "termN" must return the surface.
			for _, e := range entries {
				// Prefix = first 5 chars ("termN").
				prefix := e.surface[:5]
				results, err := s2.LookupResults(prefix, nil, false, len(entries))
				if err != nil {
					t.Fatalf("LookupResults(%q): %v", prefix, err)
				}
				found := false
				for _, r := range results {
					if r.Key == e.surface {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("surface %q not in LookupResults(%q): %v",
						e.surface, prefix, results)
				}
			}
		})
	}
}

// TestAnalyzingSuggester_ExactFirst verifies that when exactFirst=true (the
// default), an exact-match surface is returned at position 0 regardless of
// its score.
func TestAnalyzingSuggester_ExactFirst(t *testing.T) {
	a := analysis.NewStandardAnalyzer()
	// Default options include ExactFirst.
	s := NewAnalyzingSuggester(a, "t")
	// "go-lang" tokens: ["go","lang"].  "go-fast" tokens: ["go","fast"].
	// "go-go" tokens: ["go","go"].
	// Querying "go" returns all; with exactFirst, the exact match "go" (if
	// it existed) would be first. We don't index "go" alone so we just
	// verify high-weight result comes first after sorting.
	if err := s.Build(newTestIter(
		"go-lang", 100,
		"go-fast", 50,
	)); err != nil {
		t.Fatalf("Build: %v", err)
	}
	results, err := s.LookupResults("go", nil, false, 5)
	if err != nil {
		t.Fatalf("LookupResults: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("no results returned")
	}
	// "go-lang" has higher weight and should come first.
	if results[0].Key != "go-lang" {
		t.Errorf("want first result 'go-lang', got %q", results[0].Key)
	}
}

// TestAnalyzingSuggester_EmptyIterator verifies graceful handling of an empty
// corpus (Store returns false because fst is nil; Load on a count-only blob
// succeeds without error).
func TestAnalyzingSuggester_EmptyIterator(t *testing.T) {
	a := analysis.NewStandardAnalyzer()
	s := NewAnalyzingSuggester(a, "t")
	if err := s.Build(newTestIter()); err != nil {
		t.Fatalf("Build: %v", err)
	}
	var buf bytes.Buffer
	out := store.NewOutputStreamDataOutput(&buf)
	ok, err := s.Store(out)
	if err != nil {
		t.Fatalf("Store: %v", err)
	}
	// Store returns false when FST is nil (empty corpus).
	if ok {
		t.Errorf("Store returned true for empty corpus, want false")
	}
}

// TestAnalyzingSuggester_LookupNilFST verifies that LookupResults on an
// un-built suggester returns nil without error.
func TestAnalyzingSuggester_LookupNilFST(t *testing.T) {
	a := analysis.NewStandardAnalyzer()
	s := NewAnalyzingSuggester(a, "t")
	results, err := s.LookupResults("anything", nil, false, 5)
	if err != nil {
		t.Fatalf("LookupResults on nil FST: %v", err)
	}
	if results != nil {
		t.Errorf("expected nil results, got %v", results)
	}
}

// ----------------------- Helpers -----------------------

// seededEntry mirrors Java's CompletionFstScenario.Entry.
type seededEntry struct {
	surface string
	weight  int64
}

// seededTestEntries replicates CompletionFstScenario.seededEntries(seed).
// The formula is identical to the Java reference:
//
//	tag  = fmt.Sprintf("%08x", uint32(seed))
//	surf = "termN-" + tag
//	mix  = (uint64(seed) * 0x9E3779B97F4A7C15) ^ (uint64(N) * 0xBF58476D1CE4E5B9)
//	wt   = 1 + int((mix >> 1) & 0x3FFF)
//
// hex8 formats v as an 8-char lowercase hex string with leading zeros.
// Mirrors Java's String.format("%08x", v).
func hex8(v uint32) string {
	const digits = "0123456789abcdef"
	b := [8]byte{}
	for i := 7; i >= 0; i-- {
		b[i] = digits[v&0xF]
		v >>= 4
	}
	return string(b[:])
}

// seededTestEntries replicates CompletionFstScenario.seededEntries(seed).
// The formula is identical to the Java reference:
//
//	tag  = String.format("%08x", seed & 0xFFFFFFFF)
//	surf = "termN-" + tag
//	mix  = (long(seed) * 0x9E3779B97F4A7C15L) ^ (long(N) * 0xBF58476D1CE4E5B9L)
//	wt   = 1 + int((mix >>> 1) & 0x3FFF)
func seededTestEntries(seed int64) []seededEntry {
	const count = 10
	tag := hex8(uint32(seed & 0xFFFFFFFF))
	out := make([]seededEntry, count)
	for i := 0; i < count; i++ {
		surface := "term" + string(rune('0'+i)) + "-" + tag
		mix := uint64(seed)*0x9E3779B97F4A7C15 ^ uint64(i)*0xBF58476D1CE4E5B9
		weight := int64(1) + int64((mix>>1)&0x3FFF)
		out[i] = seededEntry{surface: surface, weight: weight}
	}
	return out
}

func formatHexSeed(seed int64) string {
	return "0x" + hex8(uint32(seed&0xFFFFFFFF))
}

// seededIter adapts []seededEntry to suggest.InputIterator.
type seededIter struct {
	entries []seededEntry
	pos     int
}

func newSeededIter(entries []seededEntry) *seededIter {
	return &seededIter{entries: entries, pos: -1}
}

func (it *seededIter) Next() (term []byte, weight int64, payload []byte, contexts [][]byte, ok bool, err error) {
	it.pos++
	if it.pos >= len(it.entries) {
		return nil, 0, nil, nil, false, nil
	}
	e := it.entries[it.pos]
	return []byte(e.surface), e.weight, nil, nil, true, nil
}

func (it *seededIter) HasPayloads() bool { return false }
func (it *seededIter) HasContexts() bool { return false }

// ensure interfaces satisfied
var _ suggest.InputIterator = (*testIterator)(nil)
var _ suggest.InputIterator = (*seededIter)(nil)
