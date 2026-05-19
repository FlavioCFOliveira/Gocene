// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene's
// org.apache.lucene.codecs.lucene90.TestLucene90DocValuesFormatVariableSkipInterval
// (release tag releases/lucene/10.4.0).
//
// GOC-4119: Tests Lucene90DocValuesFormat with a custom skipper interval size.
//
// Test coverage parity with the upstream Java test class:
//   - testSkipIndexIntervalSize       (constructor argument validation)
//   - testSkipperAllEqualValue        (single-value skipper, single block)
//   - testSkipperFewValuesSorted      (skipper across sort-grouped intervals)
//   - testSkipperAllEqualValueWithGaps        (skipper across empty-doc gaps)
//   - testSkipperAllEqualValueWithMultiValues (skipper across multi-value gaps)
//
// Divergences from upstream:
//   - The upstream constructor throws IllegalArgumentException for
//     skipIndexIntervalSize < 2; the Go port panics. The validation test
//     therefore asserts a recovered panic carrying the canonical message
//     fragment "skipIndexIntervalSize must be > 1".
//   - Upstream uses RandomIndexWriter, BaseDocValuesFormatTestCase and the
//     static *DocValuesField.indexedField(...) helpers, which install the
//     skip-index side-table at write time. None of these helpers are yet
//     ported in Gocene, and LeafReader.GetDocValuesSkipper currently returns
//     (nil, nil) for all fields (see filter_directory_reader_test.go line
//     330). The three skipper-content tests are kept as faithful scaffolding
//     and gated with t.Skip until the DocValuesFormat reader/skipper wiring
//     lands, mirroring the TODO pattern in lucene90_doc_values_format_test.go.
package codecs_test

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// newVariableSkipIntervalFormat returns a Lucene90DocValuesFormat seeded
// with a small (4..15) skipIndexIntervalSize, matching upstream getCodec().
func newVariableSkipIntervalFormat(rng *rand.Rand) *codecs.Lucene90DocValuesFormat {
	// Upstream: random().nextInt(4, 16) -> [4, 16).
	interval := 4 + rng.Intn(12)
	return codecs.NewLucene90DocValuesFormatWithSkipInterval(interval)
}

// TestLucene90DocValuesFormatVariableSkipInterval_SkipIndexIntervalSize
// mirrors testSkipIndexIntervalSize: rejecting skipIndexIntervalSize < 2.
//
// Source: TestLucene90DocValuesFormatVariableSkipInterval.testSkipIndexIntervalSize()
func TestLucene90DocValuesFormatVariableSkipInterval_SkipIndexIntervalSize(t *testing.T) {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	// Upstream: random().nextInt(Integer.MIN_VALUE, 2) -> any int in [MIN_VALUE, 2).
	// In Go we exercise the full negative span as well as the boundary value 1.
	candidates := []int{
		1,
		0,
		-1,
		-rng.Intn(1 << 20),
		// Pin a deterministic deep-negative sample to cover the int range.
		int(int32(-1) << 30),
	}

	for _, sz := range candidates {
		t.Run(fmt.Sprintf("size_%d", sz), func(t *testing.T) {
			defer func() {
				r := recover()
				if r == nil {
					t.Fatalf("expected panic for skipIndexIntervalSize=%d, got none", sz)
				}
				msg := fmt.Sprintf("%v", r)
				if !strings.Contains(msg, "skipIndexIntervalSize must be > 1") {
					t.Fatalf("panic message %q does not contain %q", msg,
						"skipIndexIntervalSize must be > 1")
				}
			}()
			_ = codecs.NewLucene90DocValuesFormatWithSkipInterval(sz)
		})
	}
}

// TestLucene90DocValuesFormatVariableSkipInterval_SkipperAllEqualValue
// mirrors testSkipperAllEqualValue.
//
// Source: TestLucene90DocValuesFormatVariableSkipInterval.testSkipperAllEqualValue()
func TestLucene90DocValuesFormatVariableSkipInterval_SkipperAllEqualValue(t *testing.T) {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	_ = newVariableSkipIntervalFormat(rng) // Documents intent; format wiring TBD.

	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	numDocs := 100 // Upstream: atLeast(100).
	for i := 0; i < numDocs; i++ {
		doc := document.NewDocument()
		dvField, err := document.NewNumericDocValuesField("dv", 0)
		if err != nil {
			t.Fatalf("NewNumericDocValuesField: %v", err)
		}
		doc.Add(dvField)
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument: %v", err)
		}
	}
	if err := writer.ForceMerge(1); err != nil {
		t.Fatalf("ForceMerge: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()
	if err := writer.Close(); err != nil {
		t.Fatalf("writer.Close: %v", err)
	}

	leaves, _ := reader.Leaves()
	if len(leaves) != 1 {
		t.Fatalf("expected 1 leaf, got %d", len(leaves))
	}

	leaf, ok := leaves[0].Reader().(*index.LeafReader)
	if !ok {
		t.Skipf("leaf reader %T does not expose GetDocValuesSkipper directly; deferred", leaves[0].Reader())
	}
	skipper, err := leaf.GetDocValuesSkipper("dv")
	if err != nil {
		t.Fatalf("GetDocValuesSkipper: %v", err)
	}
	if skipper == nil {
		t.Skip("DocValuesSkipper plumbing not yet wired; see GOC follow-up for codec reader path")
	}
	// Reserved for when the skipper machinery lands:
	//   skipper.Advance(0); assert min/max/docCount; advance past last doc;
	//   assert minDocID == NO_MORE_DOCS.
}

// TestLucene90DocValuesFormatVariableSkipInterval_SkipperFewValuesSorted
// mirrors testSkipperFewValuesSorted.
//
// Source: TestLucene90DocValuesFormatVariableSkipInterval.testSkipperFewValuesSorted()
func TestLucene90DocValuesFormatVariableSkipInterval_SkipperFewValuesSorted(t *testing.T) {
	// Upstream relies on:
	//   - IndexWriterConfig.setIndexSort(new Sort(new SortField("dv", LONG, reverse)))
	//   - NumericDocValuesField.indexedField(...) to attach a skip side-table.
	//   - LeafReader.getDocValuesSkipper returning a non-nil reader-backed skipper.
	// None of these are yet ported. The test body is intentionally left as a
	// stub until the DocValuesFormat reader exposes the skipper side-table.
	t.Skip("requires index-sort + indexedField helpers + reader-backed DocValuesSkipper (deferred)")
}

// TestLucene90DocValuesFormatVariableSkipInterval_SkipperAllEqualValueWithGaps
// mirrors testSkipperAllEqualValueWithGaps.
//
// Source: TestLucene90DocValuesFormatVariableSkipInterval.testSkipperAllEqualValueWithGaps()
func TestLucene90DocValuesFormatVariableSkipInterval_SkipperAllEqualValueWithGaps(t *testing.T) {
	// Same gating as TestLucene90DocValuesFormatVariableSkipInterval_SkipperFewValuesSorted:
	// depends on SortedNumericDocValuesField.indexedField + reader skipper plumbing.
	t.Skip("requires index-sort + indexedField helpers + reader-backed DocValuesSkipper (deferred)")
}

// TestLucene90DocValuesFormatVariableSkipInterval_SkipperAllEqualValueWithMultiValues
// mirrors testSkipperAllEqualValueWithMultiValues.
//
// Source: TestLucene90DocValuesFormatVariableSkipInterval.testSkipperAllEqualValueWithMultiValues()
func TestLucene90DocValuesFormatVariableSkipInterval_SkipperAllEqualValueWithMultiValues(t *testing.T) {
	// Same gating: requires SortedNumericDocValuesField.indexedField with
	// genuine multi-value support and reader-backed DocValuesSkipper.
	t.Skip("requires index-sort + indexedField helpers + reader-backed DocValuesSkipper (deferred)")
}
