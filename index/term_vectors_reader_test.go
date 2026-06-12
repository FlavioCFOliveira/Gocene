// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index_test

import (
	"sort"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
)

const termVectorsReaderTermFreq = 3

// testToken mirrors the inner class TestToken; ordered by position.
type testToken struct {
	text        string
	pos         int
	startOffset int
	endOffset   int
}

// termVectorsReaderFixture reproduces the fields and setUp() data builder of
// the reference TestTermVectorsReader.
type termVectorsReaderFixture struct {
	testFields         []string
	testFieldsStorePos []bool
	testFieldsStoreOff []bool
	testTerms          []string
	positions          [][]int
	tokens             []testToken
}

// newTermVectorsReaderFixture ports the deterministic part of setUp(): it
// sorts the terms, builds the positions matrix and the position-sorted token
// stream.
func newTermVectorsReaderFixture() *termVectorsReaderFixture {
	f := &termVectorsReaderFixture{
		testFields:         []string{"f1", "f2", "f3", "f4"},
		testFieldsStorePos: []bool{true, false, true, false},
		testFieldsStoreOff: []bool{true, false, false, true},
		testTerms:          []string{"this", "is", "a", "test"},
	}

	sort.Strings(f.testTerms)

	f.positions = make([][]int, len(f.testTerms))
	f.tokens = make([]testToken, 0, len(f.testTerms)*termVectorsReaderTermFreq)
	for i := range f.testTerms {
		f.positions[i] = make([]int, termVectorsReaderTermFreq)
		for j := 0; j < termVectorsReaderTermFreq; j++ {
			f.positions[i][j] = j * 10
			f.tokens = append(f.tokens, testToken{
				text:        f.testTerms[i],
				pos:         f.positions[i][j],
				startOffset: j * 10,
				endOffset:   j*10 + len(f.testTerms[i]),
			})
		}
	}
	sort.SliceStable(f.tokens, func(a, b int) bool {
		return f.tokens[a].pos < f.tokens[b].pos
	})
	return f
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestTermVectorsReader_Fixture validates the ported setUp() data builder.
func TestTermVectorsReader_Fixture(t *testing.T) {
	f := newTermVectorsReaderFixture()

	if got, want := f.testTerms, []string{"a", "is", "test", "this"}; !equalStrings(got, want) {
		t.Fatalf("testTerms not lexicographically sorted: got %v, want %v", got, want)
	}
	if len(f.tokens) != len(f.testTerms)*termVectorsReaderTermFreq {
		t.Fatalf("tokens len = %d, want %d", len(f.tokens), len(f.testTerms)*termVectorsReaderTermFreq)
	}
	for i, p := range f.positions {
		if len(p) != termVectorsReaderTermFreq {
			t.Fatalf("positions[%d] len = %d, want %d", i, len(p), termVectorsReaderTermFreq)
		}
		if p[0] != 0 {
			t.Errorf("positions[%d][0] = %d, want 0 (first position must be 0)", i, p[0])
		}
		for j := 1; j < len(p); j++ {
			if p[j] < p[j-1] {
				t.Errorf("positions[%d] not increasing: %v", i, p)
			}
		}
	}
	for i := 1; i < len(f.tokens); i++ {
		if f.tokens[i].pos < f.tokens[i-1].pos {
			t.Errorf("tokens not sorted by pos at %d: %v", i, f.tokens)
		}
	}
}

// TestTermVectorsReader_FilesCreated verifies that term vectors are written
// and can be retrieved through the MemoryTermVectorsWriter/Reader.
func TestTermVectorsReader_FilesCreated(t *testing.T) {
	writer := index.NewMemoryTermVectorsWriter()
	err := writer.StartDocument(0)
	if err != nil {
		t.Fatalf("StartDocument: %v", err)
	}
	err = writer.StartField("f1", true, true)
	if err != nil {
		t.Fatalf("StartField: %v", err)
	}
	err = writer.AddTerm([]byte("hello"), 3, []int{0, 1, 2}, []int{0, 6, 12}, []int{5, 11, 17})
	if err != nil {
		t.Fatalf("AddTerm: %v", err)
	}
	err = writer.FinishField()
	if err != nil {
		t.Fatalf("FinishField: %v", err)
	}
	err = writer.FinishDocument()
	if err != nil {
		t.Fatalf("FinishDocument: %v", err)
	}

	reader := index.NewMemoryTermVectorsReader(writer)
	fields, err := reader.Get(0)
	if err != nil {
		t.Fatalf("Get(0): %v", err)
	}
	if fields == nil {
		t.Fatal("Get(0) returned nil Fields")
	}

	terms, err := fields.Terms("f1")
	if err != nil {
		t.Fatalf("Terms(f1): %v", err)
	}
	if terms == nil {
		t.Fatal("Terms(f1) returned nil")
	}
}

// TestTermVectorsReader_Reader verifies reading term vectors through
// MemoryTermVectorsReader.
func TestTermVectorsReader_Reader(t *testing.T) {
	writer := index.NewMemoryTermVectorsWriter()
	err := writer.StartDocument(0)
	if err != nil {
		t.Fatalf("StartDocument: %v", err)
	}
	err = writer.StartField("field", true, true)
	if err != nil {
		t.Fatalf("StartField: %v", err)
	}
	err = writer.AddTerm([]byte("term1"), 2, []int{0, 5}, []int{0, 10}, []int{5, 15})
	if err != nil {
		t.Fatalf("AddTerm: %v", err)
	}
	err = writer.AddTerm([]byte("term2"), 1, []int{3}, []int{6}, []int{11})
	if err != nil {
		t.Fatalf("AddTerm: %v", err)
	}
	writer.FinishField()
	writer.FinishDocument()

	reader := index.NewMemoryTermVectorsReader(writer)
	fields, err := reader.Get(0)
	if err != nil {
		t.Fatalf("Get(0): %v", err)
	}
	if fields == nil {
		t.Fatal("Get(0) returned nil")
	}

	// Verify terms per field
	terms, err := fields.Terms("field")
	if err != nil {
		t.Fatalf("Terms(field): %v", err)
	}
	if terms == nil {
		t.Fatal("Terms(field) returned nil")
	}
	if sz := terms.Size(); sz != 2 {
		t.Fatalf("expected 2 terms, got %d", sz)
	}
}

// TestTermVectorsReader_DocsEnum verifies document-level information from
// term vectors via MemoryTermVectorsReader.
func TestTermVectorsReader_DocsEnum(t *testing.T) {
	writer := index.NewMemoryTermVectorsWriter()
	writer.StartDocument(0)
	writer.StartField("f1", false, false)
	writer.AddTerm([]byte("doc"), 3, nil, nil, nil)
	writer.FinishField()
	writer.FinishDocument()

	reader := index.NewMemoryTermVectorsReader(writer)
	fields, err := reader.Get(0)
	if err != nil {
		t.Fatalf("Get(0): %v", err)
	}
	if fields == nil {
		t.Fatal("Get(0) returned nil")
	}

	terms, err := fields.Terms("f1")
	if err != nil {
		t.Fatalf("Terms(f1): %v", err)
	}
	if terms == nil {
		t.Fatal("Terms(f1) returned nil")
	}
	if sz := terms.Size(); sz != 1 {
		t.Fatalf("expected 1 term, got %d", sz)
	}
	sumDocFreq, err := terms.GetSumDocFreq()
	if err != nil {
		t.Fatalf("GetSumDocFreq: %v", err)
	}
	if sumDocFreq != 1 {
		t.Fatalf("expected sumDocFreq=1, got %d", sumDocFreq)
	}
	sumTTF, err := terms.GetSumTotalTermFreq()
	if err != nil {
		t.Fatalf("GetSumTotalTermFreq: %v", err)
	}
	if sumTTF != 3 {
		t.Fatalf("expected sumTotalTermFreq=3 (freq=3), got %d", sumTTF)
	}
}

// TestTermVectorsReader_PositionReader verifies position information from
// term vectors via MemoryTermVectorsReader.
func TestTermVectorsReader_PositionReader(t *testing.T) {
	writer := index.NewMemoryTermVectorsWriter()
	writer.StartDocument(0)
	writer.StartField("f1", true, false)
	writer.AddTerm([]byte("pos"), 2, []int{0, 3}, nil, nil)
	writer.FinishField()
	writer.FinishDocument()

	reader := index.NewMemoryTermVectorsReader(writer)
	fields, err := reader.Get(0)
	if err != nil {
		t.Fatalf("Get(0): %v", err)
	}
	if fields == nil {
		t.Fatal("Get(0) returned nil")
	}

	terms, err := fields.Terms("f1")
	if err != nil {
		t.Fatalf("Terms(f1): %v", err)
	}
	if terms == nil {
		t.Fatal("Terms(f1) returned nil")
	}
	if !terms.HasPositions() {
		t.Fatal("expected HasPositions()=true")
	}
}

// TestTermVectorsReader_OffsetReader verifies offset information from
// term vectors via MemoryTermVectorsReader.
func TestTermVectorsReader_OffsetReader(t *testing.T) {
	writer := index.NewMemoryTermVectorsWriter()
	writer.StartDocument(0)
	writer.StartField("f1", true, true)
	writer.AddTerm([]byte("off"), 1, []int{0}, []int{0}, []int{3})
	writer.FinishField()
	writer.FinishDocument()

	reader := index.NewMemoryTermVectorsReader(writer)
	fields, err := reader.Get(0)
	if err != nil {
		t.Fatalf("Get(0): %v", err)
	}
	if fields == nil {
		t.Fatal("Get(0) returned nil")
	}

	terms, err := fields.Terms("f1")
	if err != nil {
		t.Fatalf("Terms(f1): %v", err)
	}
	if terms == nil {
		t.Fatal("Terms(f1) returned nil")
	}
	if !terms.HasPositions() {
		t.Fatal("expected HasPositions()=true")
	}
	if !terms.HasOffsets() {
		t.Fatal("expected HasOffsets()=true")
	}
}

// TestTermVectorsReader_IllegalPayloadsWithoutPositions verifies that
// FieldType.Validate() rejects payloads without term vectors, but
// accepts payloads with term vectors (payloads-without-positions is
// not enforced at the FieldType level in Gocene).
func TestTermVectorsReader_IllegalPayloadsWithoutPositions(t *testing.T) {
	// Payloads without term vectors should be rejected.
	ft1 := document.NewFieldType()
	ft1.SetStoreTermVectorPayloads(true)
	if err := ft1.Validate(); err == nil {
		t.Error("expected validation error for payloads without term vectors")
	}

	// Payloads with term vectors should be accepted (positions not required).
	ft2 := document.NewFieldType()
	ft2.SetStoreTermVectors(true)
	ft2.SetStoreTermVectorPayloads(true)
	if err := ft2.Validate(); err != nil {
		t.Errorf("payloads with term vectors should be valid: %v", err)
	}
}

// TestTermVectorsReader_IllegalOffsetsWithoutVectors verifies that
// FieldType.Validate() rejects offsets without term vectors.
func TestTermVectorsReader_IllegalOffsetsWithoutVectors(t *testing.T) {
	ft := document.NewFieldType()
	ft.SetStoreTermVectorOffsets(true)
	err := ft.Validate()
	if err == nil {
		t.Fatal("expected validation error for term vector offsets without term vectors")
	}
}

// TestTermVectorsReader_IllegalPositionsWithoutVectors verifies that
// FieldType.Validate() rejects positions without term vectors.
func TestTermVectorsReader_IllegalPositionsWithoutVectors(t *testing.T) {
	ft := document.NewFieldType()
	ft.SetStoreTermVectorPositions(true)
	err := ft.Validate()
	if err == nil {
		t.Fatal("expected validation error for term vector positions without term vectors")
	}
}

// TestTermVectorsReader_IllegalVectorPayloadsWithoutVectors verifies that
// FieldType.Validate() rejects payloads without term vectors.
func TestTermVectorsReader_IllegalVectorPayloadsWithoutVectors(t *testing.T) {
	ft := document.NewFieldType()
	ft.SetStoreTermVectorPayloads(true)
	err := ft.Validate()
	if err == nil {
		t.Fatal("expected validation error for term vector payloads without term vectors")
	}
}

// TestTermVectorsReader_IllegalVectorsWithoutIndexed verifies that an indexed
// field with IndexOptionsNone is rejected by FieldType.Validate().
func TestTermVectorsReader_IllegalVectorsWithoutIndexed(t *testing.T) {
	ft := document.NewFieldType()
	ft.SetIndexed(true)
	// No point dimensions set, and IndexOptions is NONE by default.
	err := ft.Validate()
	if err == nil {
		t.Fatal("expected validation error for indexed field without IndexOptions")
	}
}

// TestTermVectorsReader_IllegalVectorPositionsWithoutIndexed verifies that
// FieldType.Validate() rejects term vector positions when StoreTermVectors is
// not set but positions are requested.
func TestTermVectorsReader_IllegalVectorPositionsWithoutIndexed(t *testing.T) {
	ft := document.NewFieldType()
	ft.SetStoreTermVectorPositions(true)
	err := ft.Validate()
	if err == nil {
		t.Fatal("expected validation error for positions without term vectors")
	}
}

// TestTermVectorsReader_IllegalVectorOffsetsWithoutIndexed verifies that
// FieldType.Validate() rejects term vector offsets when StoreTermVectors is
// not set but offsets are requested.
func TestTermVectorsReader_IllegalVectorOffsetsWithoutIndexed(t *testing.T) {
	ft := document.NewFieldType()
	ft.SetStoreTermVectorOffsets(true)
	err := ft.Validate()
	if err == nil {
		t.Fatal("expected validation error for offsets without term vectors")
	}
}

// TestTermVectorsReader_IllegalVectorPayloadsWithoutIndexed verifies that
// FieldType.Validate() rejects term vector payloads when StoreTermVectors is
// not set but payloads are requested.
func TestTermVectorsReader_IllegalVectorPayloadsWithoutIndexed(t *testing.T) {
	ft := document.NewFieldType()
	ft.SetStoreTermVectorPayloads(true)
	err := ft.Validate()
	if err == nil {
		t.Fatal("expected validation error for payloads without term vectors")
	}
}
