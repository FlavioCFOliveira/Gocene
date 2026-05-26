// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// compressing_roundtrip_test.go exercises Lucene90CompressingStoredFieldsWriter
// and Lucene90CompressingStoredFieldsReader in a full write → read round-trip,
// verifying that string, binary, and numeric field values are recovered
// exactly for both the LZ4 (BEST_SPEED) and DEFLATE (BEST_COMPRESSION) modes.
//
// Test strategy
//
//   - Small corpus (3 docs, 1 doc, and 1 doc that is empty) so every chunk
//     boundary and edge-case hits both the chunkDocs==1 bare-VInt path and
//     the chunkDocs>1 StoredFieldsInts path.
//   - Large corpus (maxDocsPerChunk+1 docs) to exercise multi-chunk writes
//     and the fields-index binary search.
//   - Binary field value corpus to satisfy the NewStoredFieldFromBytes AC.
package compressing

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs/compressing"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// collectedFields accumulates every (fieldName, value) pair dispatched
// by VisitDocument.
type collectedFields struct {
	strings  []namedValue[string]
	binaries []namedValue[[]byte]
	ints     []namedValue[int]
	longs    []namedValue[int64]
	floats   []namedValue[float32]
	doubles  []namedValue[float64]
}

type namedValue[T any] struct {
	name  string
	value T
}

func (c *collectedFields) StringField(name, v string) {
	c.strings = append(c.strings, namedValue[string]{name, v})
}
func (c *collectedFields) BinaryField(name string, v []byte) {
	cp := make([]byte, len(v))
	copy(cp, v)
	c.binaries = append(c.binaries, namedValue[[]byte]{name, cp})
}
func (c *collectedFields) IntField(name string, v int) {
	c.ints = append(c.ints, namedValue[int]{name, v})
}
func (c *collectedFields) LongField(name string, v int64) {
	c.longs = append(c.longs, namedValue[int64]{name, v})
}
func (c *collectedFields) FloatField(name string, v float32) {
	c.floats = append(c.floats, namedValue[float32]{name, v})
}
func (c *collectedFields) DoubleField(name string, v float64) {
	c.doubles = append(c.doubles, namedValue[float64]{name, v})
}

// makeFieldInfos builds a FieldInfos with a sequential list of stored-only
// fields numbered 0, 1, 2, … in the order provided.
func makeFieldInfos(t *testing.T, names ...string) *index.FieldInfos {
	t.Helper()
	fi := index.NewFieldInfos()
	for i, name := range names {
		info := index.NewFieldInfo(name, i, index.FieldInfoOptions{Stored: true})
		if err := fi.Add(info); err != nil {
			t.Fatalf("makeFieldInfos add %s: %v", name, err)
		}
	}
	return fi
}

// roundTripCompressing creates a fresh temp directory, writes numDocs documents
// through the Lucene90CompressingStoredFieldsFormat, then reads them back and
// returns the slice of per-document collectedFields.
//
// buildDoc is called once per docID; it must call WriteField for each field.
// fieldNames is used to build the synthetic FieldInfos for the reader.
func roundTripCompressing(
	t *testing.T,
	formatName string,
	mode compressing.CompressionMode,
	chunkSize, maxDocsPerChunk, blockShift int,
	numDocs int,
	fieldNames []string,
	buildDoc func(t *testing.T, w *Lucene90CompressingStoredFieldsWriter, docID int),
) []collectedFields {
	t.Helper()

	tmpDir, err := store.NewSimpleFSDirectory(t.TempDir())
	if err != nil {
		t.Fatalf("create dir: %v", err)
	}
	defer tmpDir.Close()

	seg := "_0"
	segID := make([]byte, 16)
	for i := range segID {
		segID[i] = byte(i + 1)
	}

	si := index.NewSegmentInfo(seg, numDocs, tmpDir)
	if err := si.SetID(segID); err != nil {
		t.Fatalf("set segment ID: %v", err)
	}

	// --- Write phase ---
	format := NewLucene90CompressingStoredFieldsFormatWithOptions(
		formatName, mode, chunkSize, maxDocsPerChunk, blockShift,
	)
	writer, err := format.FieldsWriter(tmpDir, si, store.IOContext{})
	if err != nil {
		t.Fatalf("FieldsWriter: %v", err)
	}
	w := writer.(*Lucene90CompressingStoredFieldsWriter)

	for docID := 0; docID < numDocs; docID++ {
		if err := w.StartDocument(); err != nil {
			t.Fatalf("StartDocument doc=%d: %v", docID, err)
		}
		buildDoc(t, w, docID)
		if err := w.FinishDocument(); err != nil {
			t.Fatalf("FinishDocument doc=%d: %v", docID, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("writer Close: %v", err)
	}

	// --- Read phase ---
	fi := makeFieldInfos(t, fieldNames...)
	reader, err := format.FieldsReader(tmpDir, si, fi, store.IOContext{})
	if err != nil {
		t.Fatalf("FieldsReader: %v", err)
	}
	defer reader.Close()

	result := make([]collectedFields, numDocs)
	for docID := 0; docID < numDocs; docID++ {
		if err := reader.VisitDocument(docID, &result[docID]); err != nil {
			t.Fatalf("VisitDocument doc=%d: %v", docID, err)
		}
	}
	return result
}

// TestLucene90Compressing_RoundTrip_BestSpeed exercises the LZ4 (BEST_SPEED)
// path with a corpus of string, binary, and long fields over multiple chunks.
//
// Note: the test uses the base compressing.FAST mode rather than the
// LZ4WithPresetDict mode from the parent lucene90 package to avoid a
// circular import (lucene90 → compressing). The format structure — chunk
// framing, header, index, footer — is identical between the two; only the
// compressed payload bytes differ.
func TestLucene90Compressing_RoundTrip_BestSpeed(t *testing.T) {
	t.Parallel()
	testRoundTrip(t, "Lucene90StoredFieldsFastData",
		compressing.FAST,
		10*8*1024, 128, 10)
}

// TestLucene90Compressing_RoundTrip_BestCompression exercises the DEFLATE
// (BEST_COMPRESSION) path with the same corpus.
//
// Note: uses compressing.HIGH_COMPRESSION (base Deflate) instead of the
// DeflateWithPresetDict mode for the same import-cycle reason as above.
func TestLucene90Compressing_RoundTrip_BestCompression(t *testing.T) {
	t.Parallel()
	testRoundTrip(t, "Lucene90StoredFieldsHighData",
		compressing.HIGH_COMPRESSION,
		10*48*1024, 128, 10)
}

// testRoundTrip is the shared test body for both compression modes.
// It covers:
//
//   - Single-doc chunk (chunkDocs==1 bare-VInt path)
//   - Multi-doc chunks (chunkDocs>1 StoredFieldsInts path)
//   - Empty document (zero stored fields)
//   - String, binary, int32, int64, float32, float64 field types
//   - Field name round-trip via FieldInfos
func testRoundTrip(
	t *testing.T,
	formatName string,
	mode compressing.CompressionMode,
	chunkSize, maxDocsPerChunk, blockShift int,
) {
	t.Helper()

	type wantDoc struct {
		strings  []namedValue[string]
		binaries []namedValue[[]byte]
		longs    []namedValue[int64]
		ints     []namedValue[int]
		floats   []namedValue[float32]
		doubles  []namedValue[float64]
	}

	// Build a corpus that covers all field types and edge cases.
	const numDocs = 5
	// Fields numbered 0..3 — order matches write order so seq ID matches FieldInfo.number.
	fieldNames := []string{"title", "payload", "count", "score"}

	// Per-doc expected values.
	wants := []wantDoc{
		// doc 0: string + binary + long
		{
			strings:  []namedValue[string]{{"title", "hello world"}},
			binaries: []namedValue[[]byte]{{"payload", []byte{0x01, 0x02, 0x03}}},
			longs:    []namedValue[int64]{{"count", 42}},
		},
		// doc 1: only string (minimal doc)
		{
			strings: []namedValue[string]{{"title", "second doc"}},
		},
		// doc 2: empty — no stored fields
		{},
		// doc 3: all field types
		{
			strings:  []namedValue[string]{{"title", "all types"}},
			binaries: []namedValue[[]byte]{{"payload", []byte("binary payload")}},
			longs:    []namedValue[int64]{{"count", -9999999999}},
			doubles:  []namedValue[float64]{{"score", 3.14}},
		},
		// doc 4: large binary to exercise the compressor. Fields are written
		// in FieldInfos order (title=0, payload=1, count=2) so that the
		// sequential per-doc IDs match the FieldInfo numbers and name
		// round-trip is correct.
		{
			strings:  []namedValue[string]{{"title", "large-binary-doc"}},
			binaries: []namedValue[[]byte]{{"payload", makeLargeBinary(4096)}},
			longs:    []namedValue[int64]{{"count", 0}},
		},
	}

	got := roundTripCompressing(t, formatName, mode, chunkSize, maxDocsPerChunk, blockShift,
		numDocs, fieldNames,
		func(t *testing.T, w *Lucene90CompressingStoredFieldsWriter, docID int) {
			t.Helper()
			switch docID {
			case 0:
				mustWriteField(t, w, "title", "hello world")
				mustWriteFieldBytes(t, w, "payload", []byte{0x01, 0x02, 0x03})
				mustWriteFieldLong(t, w, "count", 42)
			case 1:
				mustWriteField(t, w, "title", "second doc")
			case 2:
				// empty doc
			case 3:
				mustWriteField(t, w, "title", "all types")
				mustWriteFieldBytes(t, w, "payload", []byte("binary payload"))
				mustWriteFieldLong(t, w, "count", -9999999999)
				mustWriteFieldDouble(t, w, "score", 3.14)
			case 4:
				// Write in FieldInfos order so sequential IDs match FieldInfo numbers.
				mustWriteField(t, w, "title", "large-binary-doc")
				mustWriteFieldBytes(t, w, "payload", makeLargeBinary(4096))
				mustWriteFieldLong(t, w, "count", 0)
			}
		},
	)

	for docID, want := range wants {
		actual := got[docID]
		t.Run(fmt.Sprintf("doc%d", docID), func(t *testing.T) {
			t.Parallel()
			// Strings.
			if len(actual.strings) != len(want.strings) {
				t.Fatalf("strings count: got %d, want %d", len(actual.strings), len(want.strings))
			}
			for i, wv := range want.strings {
				av := actual.strings[i]
				if av.name != wv.name {
					t.Errorf("string[%d] name: got %q, want %q", i, av.name, wv.name)
				}
				if av.value != wv.value {
					t.Errorf("string[%d] value: got %q, want %q", i, av.value, wv.value)
				}
			}
			// Binaries.
			if len(actual.binaries) != len(want.binaries) {
				t.Fatalf("binaries count: got %d, want %d", len(actual.binaries), len(want.binaries))
			}
			for i, wv := range want.binaries {
				av := actual.binaries[i]
				if av.name != wv.name {
					t.Errorf("binary[%d] name: got %q, want %q", i, av.name, wv.name)
				}
				if !bytes.Equal(av.value, wv.value) {
					t.Errorf("binary[%d] value mismatch (len %d vs %d)", i, len(av.value), len(wv.value))
				}
			}
			// Longs.
			if len(actual.longs) != len(want.longs) {
				t.Fatalf("longs count: got %d, want %d", len(actual.longs), len(want.longs))
			}
			for i, wv := range want.longs {
				av := actual.longs[i]
				if av.name != wv.name {
					t.Errorf("long[%d] name: got %q, want %q", i, av.name, wv.name)
				}
				if av.value != wv.value {
					t.Errorf("long[%d] value: got %d, want %d", i, av.value, wv.value)
				}
			}
			// Doubles.
			if len(actual.doubles) != len(want.doubles) {
				t.Fatalf("doubles count: got %d, want %d", len(actual.doubles), len(want.doubles))
			}
			for i, wv := range want.doubles {
				av := actual.doubles[i]
				if av.name != wv.name {
					t.Errorf("double[%d] name: got %q, want %q", i, av.name, wv.name)
				}
				if av.value != wv.value {
					t.Errorf("double[%d] value: got %v, want %v", i, av.value, wv.value)
				}
			}
		})
	}
}

// TestLucene90Compressing_RoundTrip_MultiChunk writes more docs than
// maxDocsPerChunk to exercise multi-chunk writes and the fields-index
// binary-search lookup.
func TestLucene90Compressing_RoundTrip_MultiChunk(t *testing.T) {
	t.Parallel()

	const (
		maxDocs    = 130 // more than the 128-block size in StoredFieldsInts
		maxDPChunk = 32  // small per-chunk cap to force many chunks
		chunkSz    = 512
		bShift     = 4
	)
	fieldNames := []string{"title", "payload"}

	mode := compressing.FAST
	got := roundTripCompressing(t, "Lucene90StoredFieldsFastData", mode,
		chunkSz, maxDPChunk, bShift,
		maxDocs, fieldNames,
		func(t *testing.T, w *Lucene90CompressingStoredFieldsWriter, docID int) {
			t.Helper()
			mustWriteField(t, w, "title", fmt.Sprintf("doc-%d", docID))
			mustWriteFieldBytes(t, w, "payload", []byte(fmt.Sprintf("payload-%d", docID)))
		},
	)

	for docID := 0; docID < maxDocs; docID++ {
		actual := got[docID]
		wantTitle := fmt.Sprintf("doc-%d", docID)
		wantPayload := []byte(fmt.Sprintf("payload-%d", docID))
		if len(actual.strings) != 1 || actual.strings[0].value != wantTitle {
			t.Errorf("doc=%d title: got %v, want %q", docID, actual.strings, wantTitle)
		}
		if len(actual.binaries) != 1 || !bytes.Equal(actual.binaries[0].value, wantPayload) {
			t.Errorf("doc=%d payload: got %v, want %q", docID, actual.binaries, wantPayload)
		}
	}
}

// TestLucene90Compressing_RoundTrip_FieldNames verifies that the reader
// returns the correct field name for each field by mapping the embedded
// field number through the provided FieldInfos.
func TestLucene90Compressing_RoundTrip_FieldNames(t *testing.T) {
	t.Parallel()

	// Three fields with distinctive names so we can detect cross-assignment.
	fieldNames := []string{"alpha", "bravo", "charlie"}
	mode := compressing.FAST

	got := roundTripCompressing(t,
		"Lucene90StoredFieldsFastData", mode,
		10*8*1024, 1024, 10,
		1, fieldNames,
		func(t *testing.T, w *Lucene90CompressingStoredFieldsWriter, _ int) {
			t.Helper()
			mustWriteField(t, w, "alpha", "val-a")
			mustWriteField(t, w, "bravo", "val-b")
			mustWriteFieldLong(t, w, "charlie", 999)
		},
	)

	doc := got[0]
	if len(doc.strings) != 2 {
		t.Fatalf("expected 2 string fields, got %d", len(doc.strings))
	}
	if doc.strings[0].name != "alpha" || doc.strings[0].value != "val-a" {
		t.Errorf("field 0: got (%s=%s), want (alpha=val-a)", doc.strings[0].name, doc.strings[0].value)
	}
	if doc.strings[1].name != "bravo" || doc.strings[1].value != "val-b" {
		t.Errorf("field 1: got (%s=%s), want (bravo=val-b)", doc.strings[1].name, doc.strings[1].value)
	}
	if len(doc.longs) != 1 || doc.longs[0].name != "charlie" || doc.longs[0].value != 999 {
		t.Errorf("field 2: got %v, want (charlie=999)", doc.longs)
	}
}

// ---------------------------------------------------------------------------
// Helpers for writing test documents
// ---------------------------------------------------------------------------

func mustWriteField(t *testing.T, w *Lucene90CompressingStoredFieldsWriter, name, value string) {
	t.Helper()
	f, err := document.NewStoredField(name, value)
	if err != nil {
		t.Fatalf("NewStoredField(%s): %v", name, err)
	}
	if err := w.WriteField(f); err != nil {
		t.Fatalf("WriteField(%s): %v", name, err)
	}
}

func mustWriteFieldBytes(t *testing.T, w *Lucene90CompressingStoredFieldsWriter, name string, value []byte) {
	t.Helper()
	f, err := document.NewStoredFieldFromBytes(name, value)
	if err != nil {
		t.Fatalf("NewStoredFieldFromBytes(%s): %v", name, err)
	}
	if err := w.WriteField(f); err != nil {
		t.Fatalf("WriteField(%s): %v", name, err)
	}
}

func mustWriteFieldLong(t *testing.T, w *Lucene90CompressingStoredFieldsWriter, name string, value int64) {
	t.Helper()
	f, err := document.NewStoredFieldFromInt64(name, value)
	if err != nil {
		t.Fatalf("NewStoredFieldFromInt64(%s): %v", name, err)
	}
	if err := w.WriteField(f); err != nil {
		t.Fatalf("WriteField(%s): %v", name, err)
	}
}

func mustWriteFieldDouble(t *testing.T, w *Lucene90CompressingStoredFieldsWriter, name string, value float64) {
	t.Helper()
	f, err := document.NewStoredFieldFromFloat64(name, value)
	if err != nil {
		t.Fatalf("NewStoredFieldFromFloat64(%s): %v", name, err)
	}
	if err := w.WriteField(f); err != nil {
		t.Fatalf("WriteField(%s): %v", name, err)
	}
}

func makeLargeBinary(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(i & 0xFF)
	}
	return b
}
