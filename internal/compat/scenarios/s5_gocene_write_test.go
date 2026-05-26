// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package scenarios

// This file implements the Gocene-write leg for S5 (combined-suggester-fst).
//
// The Java-side leg (TestS5_SuggesterFst) verifies that Java's
// AnalyzingSuggester can build, store, and reload a seeded corpus.  This
// test proves the inverse: that Gocene's AnalyzingSuggester produces an FST
// blob + TSV that Java's AnalyzingSuggester.load() can consume and verify.

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	gcompat "github.com/FlavioCFOliveira/Gocene/internal/compat"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/suggest"
	"github.com/FlavioCFOliveira/Gocene/suggest/analyzing"
)

// TestS5_GoceneWriteLeg produces an AnalyzingSuggester FST from Gocene and
// verifies it with the Java harness.
//
// The corpus is generated using the same deterministic formula as
// CompletionFstScenario.seededEntries (Sprint 114 T13 / rmp 4621):
//
//	tag  = fmt.Sprintf("%08x", seed & 0xFFFFFFFF)
//	surf = "termN-" + tag  (N = 0..9)
//	mix  = (uint64(seed) * 0x9E3779B97F4A7C15) ^ (uint64(N) * 0xBF58476D1CE4E5B9)
//	wt   = 1 + int((mix >>> 1) & 0x3FFF)
//
// A secondary TSV (s5-suggestions.tsv) is produced that matches the layout
// CombinedSuggesterFstScenario.verify() re-derives internally.
func TestS5_GoceneWriteLeg(t *testing.T) {
	const (
		prefixCount = 5
		topN        = 3
	)
	for _, seed := range canarySeeds {
		seed := seed
		t.Run(strconv.FormatInt(seed, 10), func(t *testing.T) {
			requireHarness(t)

			dir := t.TempDir()

			// Build seeded entries.
			entries := s5SeededEntries(seed)

			// Build AnalyzingSuggester with the same analyzer as Java.
			a := analysis.NewStandardAnalyzer()
			s := analyzing.NewAnalyzingSuggester(a, scenarioS5+"-"+strconv.FormatInt(seed, 10))
			if err := s.Build(s5NewIter(entries)); err != nil {
				t.Fatalf("Build: %v", err)
			}

			// Write FST blob.
			fstPath := filepath.Join(dir, "s5-completion.fst")
			fstFile, err := os.Create(fstPath)
			if err != nil {
				t.Fatalf("create s5-completion.fst: %v", err)
			}
			bw := bufio.NewWriter(fstFile)
			out := store.NewOutputStreamDataOutput(bw)
			ok, storeErr := s.Store(out)
			flushErr := bw.Flush()
			fstFile.Close()
			if storeErr != nil {
				t.Fatalf("Store: %v", storeErr)
			}
			if flushErr != nil {
				t.Fatalf("flush FST: %v", flushErr)
			}
			if !ok {
				t.Fatalf("Store returned false (empty suggester)")
			}

			// Derive the 5 prefixes from entries (same logic as Java
			// CombinedSuggesterFstScenario.seededPrefixes).
			prefixes := make([]string, 0, prefixCount)
			for i := 0; i < prefixCount && i < len(entries); i++ {
				surf := entries[i].surface
				plen := 5
				if plen > len(surf) {
					plen = len(surf)
				}
				prefixes = append(prefixes, surf[:plen])
			}

			// Run lookups and build TSV rows.
			type row struct {
				prefix     string
				rank       int
				suggestion string
			}
			var rows []row
			for _, prefix := range prefixes {
				results, lerr := s.LookupResults(prefix, nil, false, topN)
				if lerr != nil {
					t.Fatalf("LookupResults(%q): %v", prefix, lerr)
				}
				for rank, r := range results {
					rows = append(rows, row{prefix, rank, r.Key})
				}
			}
			// Sort by (prefix asc, rank asc) — same as Java.
			sort.Slice(rows, func(i, j int) bool {
				if rows[i].prefix != rows[j].prefix {
					return rows[i].prefix < rows[j].prefix
				}
				return rows[i].rank < rows[j].rank
			})

			// Validate TSV shape.
			if len(rows) == 0 {
				t.Fatal("no suggestion rows produced")
			}
			prefixSet := make(map[string]struct{})
			for _, r := range rows {
				if !strings.HasPrefix(r.suggestion, r.prefix) {
					t.Errorf("row (%s,%d,%s): suggestion does not start with prefix",
						r.prefix, r.rank, r.suggestion)
				}
				prefixSet[r.prefix] = struct{}{}
			}
			if len(prefixSet) != prefixCount {
				t.Fatalf("expected %d distinct prefixes, got %d: %v",
					prefixCount, len(prefixSet), prefixSet)
			}

			// Write TSV.
			tsvPath := filepath.Join(dir, "s5-suggestions.tsv")
			tsvFile, err := os.Create(tsvPath)
			if err != nil {
				t.Fatalf("create s5-suggestions.tsv: %v", err)
			}
			w := bufio.NewWriter(tsvFile)
			fmt.Fprintf(w, "# prefix\trank\tsuggestion\n")
			for _, r := range rows {
				fmt.Fprintf(w, "%s\t%d\t%s\n", r.prefix, r.rank, r.suggestion)
			}
			if err := w.Flush(); err != nil {
				tsvFile.Close()
				t.Fatalf("flush TSV: %v", err)
			}
			tsvFile.Close()

			// Verify with Java harness.
			if err := gcompat.Verify(scenarioS5, seed, dir); err != nil {
				t.Fatalf("harness verify: %v", err)
			}
		})
	}
}

// ----------------------- Helpers -----------------------

// s5Entry is a single (surface, weight) pair for the seeded corpus.
type s5Entry struct {
	surface string
	weight  int64
}

// s5SeededEntries replicates CompletionFstScenario.seededEntries(seed).
func s5SeededEntries(seed int64) []s5Entry {
	const count = 10
	tag := s5Hex8(uint32(seed & 0xFFFFFFFF))
	out := make([]s5Entry, count)
	for i := 0; i < count; i++ {
		surface := "term" + string(rune('0'+i)) + "-" + tag
		mix := uint64(seed)*0x9E3779B97F4A7C15 ^ uint64(i)*0xBF58476D1CE4E5B9
		weight := int64(1) + int64((mix>>1)&0x3FFF)
		out[i] = s5Entry{surface: surface, weight: weight}
	}
	return out
}

// s5Hex8 formats v as an 8-char lowercase hex string with leading zeros.
func s5Hex8(v uint32) string {
	const digits = "0123456789abcdef"
	b := [8]byte{}
	for i := 7; i >= 0; i-- {
		b[i] = digits[v&0xF]
		v >>= 4
	}
	return string(b[:])
}

// s5Iterator implements suggest.InputIterator for a []s5Entry.
type s5Iterator struct {
	entries []s5Entry
	pos     int
}

func s5NewIter(entries []s5Entry) *s5Iterator {
	return &s5Iterator{entries: entries, pos: -1}
}

func (it *s5Iterator) Next() (term []byte, weight int64, payload []byte, contexts [][]byte, ok bool, err error) {
	it.pos++
	if it.pos >= len(it.entries) {
		return nil, 0, nil, nil, false, nil
	}
	e := it.entries[it.pos]
	return []byte(e.surface), e.weight, nil, nil, true, nil
}

func (it *s5Iterator) HasPayloads() bool { return false }
func (it *s5Iterator) HasContexts() bool { return false }

var _ suggest.InputIterator = (*s5Iterator)(nil)
