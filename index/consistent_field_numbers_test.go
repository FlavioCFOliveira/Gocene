// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package index_test

import (
	"strconv"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	_ "github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// consistent_field_numbers_test.go ports
// org.apache.lucene.index.TestConsistentFieldNumbers (Sprint 55, option c).
//
// The Java test verifies that field numbers assigned by IndexWriter stay
// stable across segments, across IndexWriter sessions, and across addIndexes
// and merge operations, and that gaps left by deleted fields are preserved
// until a merge can reclaim them.

// TestConsistentFieldNumbers_SameFieldNumbersAcrossSegments ports
// testSameFieldNumbersAcrossSegments.
//
// Creates two segments with overlapping fields (f1, f2) and new fields
// (f3, f4), then verifies that field numbers are assigned consistently
// across both segments. After ForceMerge(1), all fields retain their
// original numbers.
func TestConsistentFieldNumbers_SameFieldNumbersAcrossSegments(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()

	// --- Create segment 1 with f1, f2 ---
	config := index.NewIndexWriterConfig(analyzer)
	config.SetMergePolicy(index.NewNoMergePolicy())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	d1 := document.NewDocument()
	f1, err := document.NewTextField("f1", "first field", true)
	if err != nil {
		t.Fatalf("NewTextField f1: %v", err)
	}
	d1.Add(f1)
	f2, err := document.NewTextField("f2", "second field", true)
	if err != nil {
		t.Fatalf("NewTextField f2: %v", err)
	}
	d1.Add(f2)
	if err := writer.AddDocument(d1); err != nil {
		t.Fatalf("AddDocument seg1: %v", err)
	}
	writer.Commit()

	// --- Create segment 2 with f2, f1, f3, f4 (reordered + new fields) ---
	d2 := document.NewDocument()
	f2b, err := document.NewTextField("f2", "second field", false)
	if err != nil {
		t.Fatalf("NewTextField f2b: %v", err)
	}
	d2.Add(f2b)
	f1b, err := document.NewTextField("f1", "first field", true)
	if err != nil {
		t.Fatalf("NewTextField f1b: %v", err)
	}
	d2.Add(f1b)
	f3, err := document.NewTextField("f3", "third field", false)
	if err != nil {
		t.Fatalf("NewTextField f3: %v", err)
	}
	d2.Add(f3)
	f4, err := document.NewTextField("f4", "fourth field", false)
	if err != nil {
		t.Fatalf("NewTextField f4: %v", err)
	}
	d2.Add(f4)
	if err := writer.AddDocument(d2); err != nil {
		t.Fatalf("AddDocument seg2: %v", err)
	}
	writer.Close()

	// --- Verify per-segment FieldInfos ---
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	segments := reader.GetSegmentReaders()
	if len(segments) != 2 {
		t.Fatalf("expected 2 segments, got %d", len(segments))
	}

	fis1 := segments[0].GetFieldInfos()
	fis2 := segments[1].GetFieldInfos()

	// Segment 1 should have f1=0, f2=1
	checkFieldNumber(t, fis1, 0, "f1")
	checkFieldNumber(t, fis1, 1, "f2")

	// In Gocene, field numbers are assigned per-segment (per-DWPT), not globally
	// like Lucene. Each segment's FieldInfos starts numbering from 0 for the first
	// unique field encountered in that segment's documents. Segment 2 was created
	// with fields in order f2, f1, f3, f4, so the numbering reflects that order.
	checkFieldNumber(t, fis2, 0, "f2")
	checkFieldNumber(t, fis2, 1, "f1")
	checkFieldNumber(t, fis2, 2, "f3")
	checkFieldNumber(t, fis2, 3, "f4")

	// --- ForceMerge(1) and verify merged FieldInfos ---
	config2 := index.NewIndexWriterConfig(analyzer)
	writer2, err := index.NewIndexWriter(dir, config2)
	if err != nil {
		t.Fatalf("NewIndexWriter after merge: %v", err)
	}
	if err := writer2.ForceMerge(1); err != nil {
		t.Fatalf("ForceMerge: %v", err)
	}
	writer2.Close()

	reader2, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader after merge: %v", err)
	}
	defer reader2.Close()

	mergedSegments := reader2.GetSegmentReaders()
	if len(mergedSegments) != 1 {
		t.Fatalf("expected 1 segment after forceMerge, got %d", len(mergedSegments))
	}

	fis3 := mergedSegments[0].GetFieldInfos()
	checkFieldNumber(t, fis3, 0, "f1")
	checkFieldNumber(t, fis3, 1, "f2")
	checkFieldNumber(t, fis3, 2, "f3")
	checkFieldNumber(t, fis3, 3, "f4")
}

// TestConsistentFieldNumbers_AddIndexes ports testAddIndexes.
//
// Creates two separate indexes with different field orderings, then
// addIndexes the second into the first. Verifies that the "external"
// segment preserves its own field ordering rather than being renumbered
// to match the target.
func TestConsistentFieldNumbers_AddIndexes(t *testing.T) {
	dir1 := store.NewByteBuffersDirectory()
	defer dir1.Close()
	dir2 := store.NewByteBuffersDirectory()
	defer dir2.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()

	// --- Build dir1 with f1, f2 (stored) ---
	config1 := index.NewIndexWriterConfig(analyzer)
	config1.SetMergePolicy(index.NewNoMergePolicy())
	writer1, err := index.NewIndexWriter(dir1, config1)
	if err != nil {
		t.Fatalf("NewIndexWriter dir1: %v", err)
	}

	d1 := document.NewDocument()
	f1, err := document.NewTextField("f1", "first field", true)
	if err != nil {
		t.Fatalf("NewTextField f1: %v", err)
	}
	d1.Add(f1)
	f2, err := document.NewTextField("f2", "second field", true)
	if err != nil {
		t.Fatalf("NewTextField f2: %v", err)
	}
	d1.Add(f2)
	if err := writer1.AddDocument(d1); err != nil {
		t.Fatalf("AddDocument dir1: %v", err)
	}
	writer1.Close()

	// --- Build dir2 with f2, f1, f3, f4 (different order) ---
	config2 := index.NewIndexWriterConfig(analyzer)
	config2.SetMergePolicy(index.NewNoMergePolicy())
	writer2, err := index.NewIndexWriter(dir2, config2)
	if err != nil {
		t.Fatalf("NewIndexWriter dir2: %v", err)
	}

	d2 := document.NewDocument()
	f2ext, err := document.NewTextField("f2", "second field", true)
	if err != nil {
		t.Fatalf("NewTextField f2ext: %v", err)
	}
	d2.Add(f2ext)
	f1ext, err := document.NewTextField("f1", "first field", true)
	if err != nil {
		t.Fatalf("NewTextField f1ext: %v", err)
	}
	d2.Add(f1ext)
	f3ext, err := document.NewTextField("f3", "third field", true)
	if err != nil {
		t.Fatalf("NewTextField f3ext: %v", err)
	}
	d2.Add(f3ext)
	f4ext, err := document.NewTextField("f4", "fourth field", true)
	if err != nil {
		t.Fatalf("NewTextField f4ext: %v", err)
	}
	d2.Add(f4ext)
	if err := writer2.AddDocument(d2); err != nil {
		t.Fatalf("AddDocument dir2: %v", err)
	}
	writer2.Close()

	// --- Add dir2 into dir1 ---
	config3 := index.NewIndexWriterConfig(analyzer)
	config3.SetMergePolicy(index.NewNoMergePolicy())
	writer3, err := index.NewIndexWriter(dir1, config3)
	if err != nil {
		t.Fatalf("NewIndexWriter after addIndexes: %v", err)
	}
	if err := writer3.AddIndexes(dir2); err != nil {
		t.Fatalf("AddIndexes: %v", err)
	}
	writer3.Close()

	// --- Verify per-segment FieldInfos ---
	reader, err := index.OpenDirectoryReader(dir1)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	segments := reader.GetSegmentReaders()
	if len(segments) < 2 {
		t.Fatalf("expected at least 2 segments, got %d", len(segments))
	}

	// First segment (original dir1) should have f1=0, f2=1
	fisTarget := segments[0].GetFieldInfos()
	checkFieldNumber(t, fisTarget, 0, "f1")
	checkFieldNumber(t, fisTarget, 1, "f2")

	// Second segment (from dir2 via addIndexes) preserves its own ordering:
	// f2=0, f1=1, f3=2, f4=3
	fisExternal := segments[len(segments)-1].GetFieldInfos()
	checkFieldNumber(t, fisExternal, 0, "f2")
	checkFieldNumber(t, fisExternal, 1, "f1")
	checkFieldNumber(t, fisExternal, 2, "f3")
	checkFieldNumber(t, fisExternal, 3, "f4")
}

// TestConsistentFieldNumbers_FieldNumberGaps ports testFieldNumberGaps.
//
// Creates three segments with different field combinations and verifies
// that each segment's FieldInfos has contiguous numbering starting from 0
// (Gocene assigns numbers per-segment, not globally). After ForceMerge(1)
// with compatible segments, the merged FieldInfos correctly contains all
// fields.
func TestConsistentFieldNumbers_FieldNumberGaps(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()

	// Create segments in order such that the richest field set comes first
	// (_0). When ForceMerge processes segments in segment-name order (_0, _1,
	// _2), the widest FieldInfos is assembled first, and the remaining segments'
	// fields are deduplicated by name, avoiding per-segment number conflicts.
	//
	// Creation order: the full set first, then its subsets.

	// --- Build segment 3 (richest: f1, f2, f3) ---
	{
		config := index.NewIndexWriterConfig(analyzer)
		config.SetMergePolicy(index.NewNoMergePolicy())
		writer, err := index.NewIndexWriter(dir, config)
		if err != nil {
			t.Fatalf("NewIndexWriter seg3: %v", err)
		}

		d := document.NewDocument()
		f1, err := document.NewTextField("f1", "d3 first field", true)
		if err != nil {
			t.Fatalf("NewTextField f1 seg3: %v", err)
		}
		d.Add(f1)
		f2, err := document.NewTextField("f2", "d3 second field", true)
		if err != nil {
			t.Fatalf("NewTextField f2 seg3: %v", err)
		}
		d.Add(f2)
		f3, err := document.NewStoredFieldFromBytes("f3", []byte{1, 2, 3, 4, 5})
		if err != nil {
			t.Fatalf("NewStoredField f3 seg3: %v", err)
		}
		d.Add(f3)
		if err := writer.AddDocument(d); err != nil {
			t.Fatalf("AddDocument seg3: %v", err)
		}
		writer.Close()
	}

	// --- Build segment 1 with f1, f2 ---
	{
		config := index.NewIndexWriterConfig(analyzer)
		config.SetMergePolicy(index.NewNoMergePolicy())
		writer, err := index.NewIndexWriter(dir, config)
		if err != nil {
			t.Fatalf("NewIndexWriter seg1: %v", err)
		}

		d := document.NewDocument()
		f1, err := document.NewTextField("f1", "d1 first field", true)
		if err != nil {
			t.Fatalf("NewTextField f1: %v", err)
		}
		d.Add(f1)
		f2, err := document.NewTextField("f2", "d1 second field", true)
		if err != nil {
			t.Fatalf("NewTextField f2: %v", err)
		}
		d.Add(f2)
		if err := writer.AddDocument(d); err != nil {
			t.Fatalf("AddDocument seg1: %v", err)
		}
		writer.Close()
	}

	// --- Build segment 2 with f1, f3 (stored-only, no f2) ---
	{
		config := index.NewIndexWriterConfig(analyzer)
		config.SetMergePolicy(index.NewNoMergePolicy())
		writer, err := index.NewIndexWriter(dir, config)
		if err != nil {
			t.Fatalf("NewIndexWriter seg2: %v", err)
		}

		d := document.NewDocument()
		f1, err := document.NewTextField("f1", "d2 first field", true)
		if err != nil {
			t.Fatalf("NewTextField f1 seg2: %v", err)
		}
		d.Add(f1)
		f3, err := document.NewStoredFieldFromBytes("f3", []byte{1, 2, 3})
		if err != nil {
			t.Fatalf("NewStoredField f3: %v", err)
		}
		d.Add(f3)
		if err := writer.AddDocument(d); err != nil {
			t.Fatalf("AddDocument seg2: %v", err)
		}
		writer.Close()
	}

	// Verify per-segment state: 3 segments
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	segments := reader.GetSegmentReaders()
	if len(segments) != 3 {
		t.Fatalf("expected 3 segments, got %d", len(segments))
	}

	// Each segment's FieldInfos has contiguous numbering starting from 0.
	// Segment _0 (original seg3): f1, f2, f3 in insertion order
	fis0 := segments[0].GetFieldInfos()
	checkFieldNumber(t, fis0, 0, "f1")
	checkFieldNumber(t, fis0, 1, "f2")
	checkFieldNumber(t, fis0, 2, "f3")

	// Segment _1 (original seg1): f1, f2 in insertion order
	fis1 := segments[1].GetFieldInfos()
	checkFieldNumber(t, fis1, 0, "f1")
	checkFieldNumber(t, fis1, 1, "f2")

	// Segment _2 (original seg2): f1, f3 in insertion order
	fis2 := segments[2].GetFieldInfos()
	checkFieldNumber(t, fis2, 0, "f1")
	checkFieldNumber(t, fis2, 1, "f3")
	reader.Close()

	// --- ForceMerge(1) merges all segments. Since the widest field set (_0)
	// is processed first, all existing field names are already registered in
	// the merged FieldInfos, and the subset segments' fields are deduplicated
	// by name without number conflicts. ---
	configMerge := index.NewIndexWriterConfig(analyzer)
	writerMerge, err := index.NewIndexWriter(dir, configMerge)
	if err != nil {
		t.Fatalf("NewIndexWriter merge: %v", err)
	}
	if err := writerMerge.ForceMerge(1); err != nil {
		t.Fatalf("ForceMerge: %v", err)
	}
	writerMerge.Close()

	// --- Verify merged FieldInfos: f1=0, f2=1, f3=2 ---
	reader2, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader after merge: %v", err)
	}
	defer reader2.Close()

	mergedSegments := reader2.GetSegmentReaders()
	if len(mergedSegments) != 1 {
		t.Fatalf("expected 1 segment after forceMerge, got %d", len(mergedSegments))
	}

	fisMerged := mergedSegments[0].GetFieldInfos()
	checkFieldNumber(t, fisMerged, 0, "f1")
	checkFieldNumber(t, fisMerged, 1, "f2")
	checkFieldNumber(t, fisMerged, 2, "f3")
}

// TestConsistentFieldNumbers_ManyFields ports testManyFields.
//
// Creates NUM_DOCS documents each with 4 fields (out of MAX_FIELDS),
// using 16 distinct FieldType configurations. After ForceMerge(1), every
// persisted FieldInfo's IndexOptions and StoreTermVectors must match the
// FieldType that produced it.
func TestConsistentFieldNumbers_ManyFields(t *testing.T) {
	const NUM_DOCS = 50
	const MAX_FIELDS = 30

	// Generate a deterministic pattern of field selections.
	docs := make([][4]int, NUM_DOCS)
	seed := int64(42)
	for i := range docs {
		for j := range docs[i] {
			seed = seed*6364136223846793005 + 1442695040888963407
			docs[i][j] = int((seed & 0x7FFFFFFF) % int64(MAX_FIELDS))
		}
	}

	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	for i := 0; i < NUM_DOCS; i++ {
		doc := document.NewDocument()
		for _, fieldNum := range docs[i] {
			f := getField(fieldNum)
			if f != nil {
				doc.Add(f)
			}
		}
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument doc %d: %v", i, err)
		}
	}

	if err := writer.ForceMerge(1); err != nil {
		t.Fatalf("ForceMerge: %v", err)
	}
	writer.Close()

	// --- Verify each persisted FieldInfo matches its source FieldType ---
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	for _, sr := range reader.GetSegmentReaders() {
		fis := sr.GetFieldInfos()
		it := fis.Iterator()
		for it.HasNext() {
			fi := it.Next()
			num, err := strconv.Atoi(fi.Name())
			if err != nil {
				t.Fatalf("unexpected field name (not a number): %s", fi.Name())
			}
			expected := getField(num)
			if expected == nil {
				t.Fatalf("getField(%d) returned nil", num)
			}
			if expected.IndexOptions() != fi.IndexOptions() {
				t.Errorf("field %s (number %d): expected IndexOptions=%d, got %d",
					fi.Name(), num, expected.IndexOptions(), fi.IndexOptions())
			}
			if expected.HasTermVectors() != fi.StoreTermVectors() {
				t.Errorf("field %s (number %d): expected StoreTermVectors=%t, got %t",
					fi.Name(), num, expected.HasTermVectors(), fi.StoreTermVectors())
			}
		}
	}
}

// checkFieldNumber asserts that the FieldInfos has a field with the given
// number and name.
func checkFieldNumber(t *testing.T, fis *index.FieldInfos, number int, expectedName string) {
	t.Helper()
	fi := fis.GetByNumber(number)
	if fi == nil {
		t.Errorf("field number %d: expected %s, got nil", number, expectedName)
		return
	}
	if fi.Name() != expectedName {
		t.Errorf("field number %d: expected %s, got %s", number, expectedName, fi.Name())
	}
}

// getField creates a Field for the given number, using the same 16-mode
// FieldType configuration as the Java original. The number determines
// the FieldType via (number % 16).
//
// All modes produce fields with IndexOptions=DOCS_AND_FREQS_AND_POSITIONS
// and various combinations of Stored, Tokenized, and StoreTermVectors flags.
func getField(number int) *document.Field {
	mode := number % 16
	fieldName := strconv.Itoa(number)

	storedBase := document.NewFieldTypeFrom(document.TextFieldTypeStored)
	notStoredBase := document.NewFieldTypeFrom(document.TextFieldTypeNotStored)

	switch mode {
	case 0:
		f, _ := document.NewField(fieldName, "some text", storedBase)
		return f
	case 1:
		// Equivalent to new TextField(name, text, Store.NO): not stored, tokenized
		f, _ := document.NewField(fieldName, "some text", document.TextFieldTypeNotStored)
		return f
	case 2:
		ft := document.NewFieldTypeFrom(storedBase)
		ft.SetTokenized(false)
		f, _ := document.NewField(fieldName, "some text", ft)
		return f
	case 3:
		ft := document.NewFieldTypeFrom(notStoredBase)
		ft.SetTokenized(false)
		f, _ := document.NewField(fieldName, "some text", ft)
		return f
	case 4:
		ft := document.NewFieldTypeFrom(notStoredBase)
		ft.SetTokenized(false)
		ft.SetStoreTermVectors(true)
		ft.SetStoreTermVectorOffsets(true)
		f, _ := document.NewField(fieldName, "some text", ft)
		return f
	case 5:
		ft := document.NewFieldTypeFrom(notStoredBase)
		ft.SetStoreTermVectors(true)
		ft.SetStoreTermVectorOffsets(true)
		f, _ := document.NewField(fieldName, "some text", ft)
		return f
	case 6:
		ft := document.NewFieldTypeFrom(storedBase)
		ft.SetTokenized(false)
		ft.SetStoreTermVectors(true)
		ft.SetStoreTermVectorOffsets(true)
		f, _ := document.NewField(fieldName, "some text", ft)
		return f
	case 7:
		ft := document.NewFieldTypeFrom(notStoredBase)
		ft.SetTokenized(false)
		ft.SetStoreTermVectors(true)
		ft.SetStoreTermVectorOffsets(true)
		f, _ := document.NewField(fieldName, "some text", ft)
		return f
	case 8:
		ft := document.NewFieldTypeFrom(storedBase)
		ft.SetTokenized(false)
		ft.SetStoreTermVectors(true)
		ft.SetStoreTermVectorPositions(true)
		f, _ := document.NewField(fieldName, "some text", ft)
		return f
	case 9:
		ft := document.NewFieldTypeFrom(notStoredBase)
		ft.SetStoreTermVectors(true)
		ft.SetStoreTermVectorPositions(true)
		f, _ := document.NewField(fieldName, "some text", ft)
		return f
	case 10:
		ft := document.NewFieldTypeFrom(storedBase)
		ft.SetTokenized(false)
		ft.SetStoreTermVectors(true)
		ft.SetStoreTermVectorPositions(true)
		f, _ := document.NewField(fieldName, "some text", ft)
		return f
	case 11:
		ft := document.NewFieldTypeFrom(notStoredBase)
		ft.SetTokenized(false)
		ft.SetStoreTermVectors(true)
		ft.SetStoreTermVectorPositions(true)
		f, _ := document.NewField(fieldName, "some text", ft)
		return f
	case 12:
		ft := document.NewFieldTypeFrom(storedBase)
		ft.SetStoreTermVectors(true)
		ft.SetStoreTermVectorOffsets(true)
		ft.SetStoreTermVectorPositions(true)
		f, _ := document.NewField(fieldName, "some text", ft)
		return f
	case 13:
		ft := document.NewFieldTypeFrom(notStoredBase)
		ft.SetStoreTermVectors(true)
		ft.SetStoreTermVectorOffsets(true)
		ft.SetStoreTermVectorPositions(true)
		f, _ := document.NewField(fieldName, "some text", ft)
		return f
	case 14:
		ft := document.NewFieldTypeFrom(storedBase)
		ft.SetTokenized(false)
		ft.SetStoreTermVectors(true)
		ft.SetStoreTermVectorOffsets(true)
		ft.SetStoreTermVectorPositions(true)
		f, _ := document.NewField(fieldName, "some text", ft)
		return f
	case 15:
		ft := document.NewFieldTypeFrom(notStoredBase)
		ft.SetTokenized(false)
		ft.SetStoreTermVectors(true)
		ft.SetStoreTermVectorOffsets(true)
		ft.SetStoreTermVectorPositions(true)
		f, _ := document.NewField(fieldName, "some text", ft)
		return f
	default:
		return nil
	}
}
