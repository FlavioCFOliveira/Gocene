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
//   - The upstream testSkipperFewValuesSorted uses IndexWriter with
//     IndexSort; the Go port does not yet expose IndexSort at the
//     IndexWriter level, so this test writes varied values directly via
//     the codec consumer API to exercise the skipper with non-uniform
//     data.
//   - All skipper-content tests write at the codec level (bypassing
//     IndexWriter) because the LeafReader.GetDocValuesSkipper path is
//     not yet wired. The codec-level tests exercise the full consumer-
//     to-producer round-trip including the per-field skip-index side
//     table (DocValuesSkipIndexTypeRange).
//   - The Gocene DocValuesSkipper interface exposes SkipTo/GetDocID
//     only (the upstream Java version has a richer level-based API with
//     advance(level), minValue(level), maxValue(level), docCount(level),
//     minDocID(level), maxDocID(level)). The go-forward port should
//     extend the interface when the per-block level decode path lands.
//     Until then, the tests verify that the skipper is non-nil and that
//     SkipTo/GetDocID follow the documented contract.
package codecs_test

import (
	"crypto/rand"
	"fmt"
	"math"
	mathrand "math/rand"
	"strings"
	"testing"
	"time"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// testNoMoreDocs mirrors the codecs-internal dvNoMoreDocs sentinel
// (math.MaxInt32) used by the DocValuesSkipper implementation returned
// from Lucene90DocValuesProducer.GetSkipper.
const testNoMoreDocs = math.MaxInt32

// newVariableSkipIntervalFormat returns a Lucene90DocValuesFormat seeded
// with a small (4..15) skipIndexIntervalSize, matching upstream getCodec().
func newVariableSkipIntervalFormat(rng *mathrand.Rand) *codecs.Lucene90DocValuesFormat {
	// Upstream: random().nextInt(4, 16) -> [4, 16).
	interval := 4 + rng.Intn(12)
	return codecs.NewLucene90DocValuesFormatWithSkipInterval(interval)
}

// TestLucene90DocValuesFormatVariableSkipInterval_SkipIndexIntervalSize
// mirrors testSkipIndexIntervalSize: rejecting skipIndexIntervalSize < 2.
//
// Source: TestLucene90DocValuesFormatVariableSkipInterval.testSkipIndexIntervalSize()
func TestLucene90DocValuesFormatVariableSkipInterval_SkipIndexIntervalSize(t *testing.T) {
	rng := mathrand.New(mathrand.NewSource(time.Now().UnixNano()))
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

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// dvSkipperTestSegment returns a write/read state pair and a cleanup
// function for a codec-level doc-values round-trip with the given number
// of documents and a single numeric field named "dv" at field number 0.
//
// The field is configured with DocValuesSkipIndexTypeRange so the consumer
// writes the skip-index side-table and the producer returns a
// DocValuesSkipper from GetSkipper.
func dvSkipperTestSegment(t *testing.T, maxDoc int) (
	*codecs.SegmentWriteState, *codecs.SegmentReadState, *index.FieldInfo, func()) {
	t.Helper()

	dir := store.NewByteBuffersDirectory()
	si := index.NewSegmentInfo("_0", maxDoc, dir)
	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		t.Fatalf("rand.Read: %v", err)
	}
	if err := si.SetID(id); err != nil {
		t.Fatalf("SetID: %v", err)
	}

	fi := index.NewFieldInfo("dv", 0, index.FieldInfoOptions{
		DocValuesType:          index.DocValuesTypeNumeric,
		DocValuesSkipIndexType: index.DocValuesSkipIndexTypeRange,
	})
	fis := index.NewFieldInfos()
	if err := fis.Add(fi); err != nil {
		t.Fatalf("fis.Add: %v", err)
	}

	ws := &codecs.SegmentWriteState{
		Directory:   dir,
		SegmentInfo: si,
		FieldInfos:  fis,
	}
	rs := &codecs.SegmentReadState{
		Directory:   dir,
		SegmentInfo: si,
		FieldInfos:  fis,
	}
	cleanup := func() { _ = dir.Close() }
	return ws, rs, fi, cleanup
}

// simpleNumericIter implements codecs.NumericDocValuesIterator from
// parallel doc/value slices.
type simpleNumericIter struct {
	docs   []int
	values []int64
	pos    int
}

func (s *simpleNumericIter) Next() bool {
	s.pos++
	return s.pos < len(s.docs)
}
func (s *simpleNumericIter) DocID() int   { return s.docs[s.pos] }
func (s *simpleNumericIter) Value() int64 { return s.values[s.pos] }

// simpleSNIter implements codecs.SortedNumericDocValuesIterator for
// multi-value numeric tests.
type simpleSNIter struct {
	entries []snEntry
	pos     int
	valPos  int
}

type snEntry struct {
	doc    int
	values []int64
}

func (s *simpleSNIter) NextDoc() bool {
	s.pos++
	s.valPos = 0
	return s.pos < len(s.entries)
}
func (s *simpleSNIter) DocID() int { return s.entries[s.pos].doc }
func (s *simpleSNIter) NextValue() int64 {
	v := s.entries[s.pos].values[s.valPos]
	s.valPos++
	return v
}
func (s *simpleSNIter) DocValueCount() int { return len(s.entries[s.pos].values) }

// ---------------------------------------------------------------------------
// TestSkipperAllEqualValue
// ---------------------------------------------------------------------------

// TestLucene90DocValuesFormatVariableSkipInterval_SkipperAllEqualValue
// mirrors testSkipperAllEqualValue: writes 100 docs with the same constant
// value (0) via the codec consumer API, then reads back via the producer
// and verifies that GetSkipper returns a non-nil skipper with the expected
// SkipTo/GetDocID contract.
//
// Source: TestLucene90DocValuesFormatVariableSkipInterval.testSkipperAllEqualValue()
func TestLucene90DocValuesFormatVariableSkipInterval_SkipperAllEqualValue(t *testing.T) {
	rng := mathrand.New(mathrand.NewSource(time.Now().UnixNano()))
	format := newVariableSkipIntervalFormat(rng)
	maxDoc := 100

	ws, rs, fi, cleanup := dvSkipperTestSegment(t, maxDoc)
	defer cleanup()

	// Write: 100 docs with constant value 0.
	consumer, err := format.FieldsConsumer(ws)
	if err != nil {
		t.Fatalf("FieldsConsumer: %v", err)
	}
	docs := make([]int, maxDoc)
	vals := make([]int64, maxDoc)
	for i := range docs {
		docs[i] = i
		vals[i] = 0
	}
	if err := consumer.AddNumericField(fi, &simpleNumericIter{docs: docs, values: vals}); err != nil {
		t.Fatalf("AddNumericField: %v", err)
	}
	if err := consumer.Close(); err != nil {
		t.Fatalf("consumer.Close: %v", err)
	}

	// Read
	producer, err := format.FieldsProducer(rs)
	if err != nil {
		t.Fatalf("FieldsProducer: %v", err)
	}
	defer producer.Close() //nolint:errcheck

	skipper, err := producer.GetSkipper(fi)
	if err != nil {
		t.Fatalf("GetSkipper: %v", err)
	}
	if skipper == nil {
		t.Fatal("expected non-nil DocValuesSkipper when DocValuesSkipIndexTypeRange is set")
	}

	// GetDocID before SkipTo: should be -1.
	if d := skipper.GetDocID(); d != -1 {
		t.Fatalf("GetDocID before SkipTo: got %d, want -1", d)
	}

	// SkipTo(0) should return 0.
	docID, err := skipper.SkipTo(0)
	if err != nil {
		t.Fatalf("SkipTo(0): %v", err)
	}
	if docID != 0 {
		t.Fatalf("SkipTo(0): got %d, want 0", docID)
	}
	if d := skipper.GetDocID(); d != 0 {
		t.Fatalf("GetDocID after SkipTo(0): got %d, want 0", d)
	}

	// SkipTo past the last doc should return NO_MORE_DOCS.
	docID, err = skipper.SkipTo(maxDoc + 1)
	if err != nil {
		t.Fatalf("SkipTo(past end): %v", err)
	}
	if docID != testNoMoreDocs {
		t.Fatalf("SkipTo(past end): got %d, want %d (NO_MORE_DOCS)", docID, testNoMoreDocs)
	}
	if d := skipper.GetDocID(); d != testNoMoreDocs {
		t.Fatalf("GetDocID after exhaustion: got %d, want %d", d, testNoMoreDocs)
	}
}

// ---------------------------------------------------------------------------
// TestSkipperFewValuesSorted
// ---------------------------------------------------------------------------

// TestLucene90DocValuesFormatVariableSkipInterval_SkipperFewValuesSorted
// mirrors testSkipperFewValuesSorted: writes numeric values that change
// in groups to exercise the skipper with non-uniform data.
//
// Upstream uses IndexWriter with IndexSort to produce sorted doc-id order;
// this test writes values that vary per group (every skipIntervalSize docs
// the value increases) to create per-block boundary coverage in the skip
// index without requiring IndexSort.
//
// Source: TestLucene90DocValuesFormatVariableSkipInterval.testSkipperFewValuesSorted()
func TestLucene90DocValuesFormatVariableSkipInterval_SkipperFewValuesSorted(t *testing.T) {
	rng := mathrand.New(mathrand.NewSource(time.Now().UnixNano()))
	interval := 4 + rng.Intn(12) // [4, 16)
	format := codecs.NewLucene90DocValuesFormatWithSkipInterval(interval)

	// At least 2 skip-index blocks to exercise the accumulator flush.
	maxDoc := interval * 4

	ws, rs, fi, cleanup := dvSkipperTestSegment(t, maxDoc)
	defer cleanup()

	// Write: values that change every interval docs.
	consumer, err := format.FieldsConsumer(ws)
	if err != nil {
		t.Fatalf("FieldsConsumer: %v", err)
	}
	docs := make([]int, maxDoc)
	vals := make([]int64, maxDoc)
	for i := range docs {
		docs[i] = i
		vals[i] = int64(i / interval)
	}
	if err := consumer.AddNumericField(fi, &simpleNumericIter{docs: docs, values: vals}); err != nil {
		t.Fatalf("AddNumericField: %v", err)
	}
	if err := consumer.Close(); err != nil {
		t.Fatalf("consumer.Close: %v", err)
	}

	// Read
	producer, err := format.FieldsProducer(rs)
	if err != nil {
		t.Fatalf("FieldsProducer: %v", err)
	}
	defer producer.Close() //nolint:errcheck

	skipper, err := producer.GetSkipper(fi)
	if err != nil {
		t.Fatalf("GetSkipper: %v", err)
	}
	if skipper == nil {
		t.Fatal("expected non-nil DocValuesSkipper when DocValuesSkipIndexTypeRange is set")
	}

	// Validate SkipTo.
	docID, err := skipper.SkipTo(0)
	if err != nil {
		t.Fatalf("SkipTo(0): %v", err)
	}
	if docID != 0 {
		t.Fatalf("SkipTo(0): got %d, want 0", docID)
	}
	if d := skipper.GetDocID(); d != 0 {
		t.Fatalf("GetDocID after SkipTo(0): got %d, want 0", d)
	}

	// SkipTo(last doc) should return the last doc ID.
	docID, err = skipper.SkipTo(maxDoc - 1)
	if err != nil {
		t.Fatalf("SkipTo(last): %v", err)
	}
	if docID != maxDoc-1 {
		t.Fatalf("SkipTo(last): got %d, want %d", docID, maxDoc-1)
	}

	// SkipTo past the last doc should return NO_MORE_DOCS.
	docID, err = skipper.SkipTo(maxDoc)
	if err != nil {
		t.Fatalf("SkipTo(past end): %v", err)
	}
	if docID != testNoMoreDocs {
		t.Fatalf("SkipTo(past end): got %d, want %d", docID, testNoMoreDocs)
	}
}

// ---------------------------------------------------------------------------
// TestSkipperAllEqualValueWithGaps
// ---------------------------------------------------------------------------

// TestLucene90DocValuesFormatVariableSkipInterval_SkipperAllEqualValueWithGaps
// mirrors testSkipperAllEqualValueWithGaps: writes numeric values where
// only a subset of docs have a value (gaps), exercising the DISI and
// skip-index handling of sparse fields.
//
// Source: TestLucene90DocValuesFormatVariableSkipInterval.testSkipperAllEqualValueWithGaps()
func TestLucene90DocValuesFormatVariableSkipInterval_SkipperAllEqualValueWithGaps(t *testing.T) {
	rng := mathrand.New(mathrand.NewSource(time.Now().UnixNano()))
	format := newVariableSkipIntervalFormat(rng)
	maxDoc := 100 // total doc IDs

	ws, rs, fi, cleanup := dvSkipperTestSegment(t, maxDoc)
	defer cleanup()

	// Write: only even-numbered docs have a value; odd docs are gaps.
	var docs []int
	var vals []int64
	for i := 0; i < maxDoc; i += 2 {
		docs = append(docs, i)
		vals = append(vals, 0)
	}

	consumer, err := format.FieldsConsumer(ws)
	if err != nil {
		t.Fatalf("FieldsConsumer: %v", err)
	}
	if err := consumer.AddNumericField(fi, &simpleNumericIter{docs: docs, values: vals}); err != nil {
		t.Fatalf("AddNumericField: %v", err)
	}
	if err := consumer.Close(); err != nil {
		t.Fatalf("consumer.Close: %v", err)
	}

	// Read
	producer, err := format.FieldsProducer(rs)
	if err != nil {
		t.Fatalf("FieldsProducer: %v", err)
	}
	defer producer.Close() //nolint:errcheck

	skipper, err := producer.GetSkipper(fi)
	if err != nil {
		t.Fatalf("GetSkipper: %v", err)
	}
	if skipper == nil {
		t.Fatal("expected non-nil DocValuesSkipper when DocValuesSkipIndexTypeRange is set")
	}

	docID, err := skipper.SkipTo(0)
	if err != nil {
		t.Fatalf("SkipTo(0): %v", err)
	}
	if docID != 0 {
		t.Fatalf("SkipTo(0): got %d, want 0", docID)
	}

	docID, err = skipper.SkipTo(maxDoc + 1)
	if err != nil {
		t.Fatalf("SkipTo(past end): %v", err)
	}
	if docID != testNoMoreDocs {
		t.Fatalf("SkipTo(past end): got %d, want %d", docID, testNoMoreDocs)
	}
}

// ---------------------------------------------------------------------------
// TestSkipperAllEqualValueWithMultiValues
// ---------------------------------------------------------------------------

// TestLucene90DocValuesFormatVariableSkipInterval_SkipperAllEqualValueWithMultiValues
// mirrors testSkipperAllEqualValueWithMultiValues: writes sorted-numeric
// values (multiple values per doc) and verifies the skipper round-trips.
//
// Source: TestLucene90DocValuesFormatVariableSkipInterval.testSkipperAllEqualValueWithMultiValues()
func TestLucene90DocValuesFormatVariableSkipInterval_SkipperAllEqualValueWithMultiValues(t *testing.T) {
	rng := mathrand.New(mathrand.NewSource(time.Now().UnixNano()))
	format := newVariableSkipIntervalFormat(rng)
	maxDoc := 50

	// Sorted-numeric field needs DocValuesSkipIndexTypeRange as well.
	dir := store.NewByteBuffersDirectory()
	si := index.NewSegmentInfo("_0", maxDoc, dir)
	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		t.Fatalf("rand.Read: %v", err)
	}
	if err := si.SetID(id); err != nil {
		t.Fatalf("SetID: %v", err)
	}

	fi := index.NewFieldInfo("dv", 0, index.FieldInfoOptions{
		DocValuesType:          index.DocValuesTypeSortedNumeric,
		DocValuesSkipIndexType: index.DocValuesSkipIndexTypeRange,
	})
	fis := index.NewFieldInfos()
	if err := fis.Add(fi); err != nil {
		t.Fatalf("fis.Add: %v", err)
	}

	ws := &codecs.SegmentWriteState{
		Directory:   dir,
		SegmentInfo: si,
		FieldInfos:  fis,
	}
	rs := &codecs.SegmentReadState{
		Directory:   dir,
		SegmentInfo: si,
		FieldInfos:  fis,
	}
	defer dir.Close()

	// Write: 2 values per doc.
	var entries []snEntry
	for i := 0; i < maxDoc; i++ {
		entries = append(entries, snEntry{doc: i, values: []int64{1, 2}})
	}

	consumer, err := format.FieldsConsumer(ws)
	if err != nil {
		t.Fatalf("FieldsConsumer: %v", err)
	}
	if err := consumer.AddSortedNumericField(fi, &simpleSNIter{entries: entries}); err != nil {
		t.Fatalf("AddSortedNumericField: %v", err)
	}
	if err := consumer.Close(); err != nil {
		t.Fatalf("consumer.Close: %v", err)
	}

	// Read
	producer, err := format.FieldsProducer(rs)
	if err != nil {
		t.Fatalf("FieldsProducer: %v", err)
	}
	defer producer.Close() //nolint:errcheck

	skipper, err := producer.GetSkipper(fi)
	if err != nil {
		t.Fatalf("GetSkipper: %v", err)
	}
	if skipper == nil {
		t.Fatal("expected non-nil DocValuesSkipper when DocValuesSkipIndexTypeRange is set")
	}

	docID, err := skipper.SkipTo(0)
	if err != nil {
		t.Fatalf("SkipTo(0): %v", err)
	}
	if docID != 0 {
		t.Fatalf("SkipTo(0): got %d, want 0", docID)
	}

	docID, err = skipper.SkipTo(maxDoc + 1)
	if err != nil {
		t.Fatalf("SkipTo(past end): %v", err)
	}
	if docID != testNoMoreDocs {
		t.Fatalf("SkipTo(past end): got %d, want %d", docID, testNoMoreDocs)
	}
}
