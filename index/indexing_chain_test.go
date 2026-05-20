// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// fakeFieldType is a minimal IndexableFieldType for exercising the indexing
// chain's schema-building logic without depending on package document.
type fakeFieldType struct {
	stored                   bool
	tokenized                bool
	storeTermVectors         bool
	storeTermVectorOffsets   bool
	storeTermVectorPositions bool
	storeTermVectorPayloads  bool
	omitNorms                bool
	indexOptions             IndexOptions
	docValuesType            DocValuesType
	docValuesSkipIndexType   DocValuesSkipIndexType
	pointDimensionCount      int
	pointIndexDimensionCount int
	pointNumBytes            int
	vectorDimension          int
	vectorEncoding           VectorEncoding
	vectorSimilarityFunction VectorSimilarityFunction
	attributes               map[string]string
}

func (f *fakeFieldType) Stored() bool                   { return f.stored }
func (f *fakeFieldType) Tokenized() bool                { return f.tokenized }
func (f *fakeFieldType) StoreTermVectors() bool         { return f.storeTermVectors }
func (f *fakeFieldType) StoreTermVectorOffsets() bool   { return f.storeTermVectorOffsets }
func (f *fakeFieldType) StoreTermVectorPositions() bool { return f.storeTermVectorPositions }
func (f *fakeFieldType) StoreTermVectorPayloads() bool  { return f.storeTermVectorPayloads }
func (f *fakeFieldType) OmitNorms() bool                { return f.omitNorms }
func (f *fakeFieldType) IndexOptions() IndexOptions     { return f.indexOptions }
func (f *fakeFieldType) DocValuesType() DocValuesType   { return f.docValuesType }
func (f *fakeFieldType) DocValuesSkipIndexType() DocValuesSkipIndexType {
	return f.docValuesSkipIndexType
}
func (f *fakeFieldType) PointDimensionCount() int       { return f.pointDimensionCount }
func (f *fakeFieldType) PointIndexDimensionCount() int  { return f.pointIndexDimensionCount }
func (f *fakeFieldType) PointNumBytes() int             { return f.pointNumBytes }
func (f *fakeFieldType) VectorDimension() int           { return f.vectorDimension }
func (f *fakeFieldType) VectorEncoding() VectorEncoding { return f.vectorEncoding }
func (f *fakeFieldType) VectorSimilarityFunction() VectorSimilarityFunction {
	return f.vectorSimilarityFunction
}
func (f *fakeFieldType) GetAttributes() map[string]string { return f.attributes }

var _ IndexableFieldType = (*fakeFieldType)(nil)

// ---------------------------------------------------------------------------
// stringHashCode
// ---------------------------------------------------------------------------

func TestStringHashCode(t *testing.T) {
	// Reference values from java.lang.String.hashCode.
	cases := []struct {
		in   string
		want int
	}{
		{"", 0},
		{"a", 97},
		{"hello", 99162322},
		{"title", 110371416},
	}
	for _, tc := range cases {
		if got := stringHashCode(tc.in); got != tc.want {
			t.Errorf("stringHashCode(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// InvertableType
// ---------------------------------------------------------------------------

func TestInvertableTypeString(t *testing.T) {
	if got := InvertableTypeBinary.String(); got != "BINARY" {
		t.Errorf("InvertableTypeBinary.String() = %q, want BINARY", got)
	}
	if got := InvertableTypeTokenStream.String(); got != "TOKEN_STREAM" {
		t.Errorf("InvertableTypeTokenStream.String() = %q, want TOKEN_STREAM", got)
	}
	if int(InvertableTypeBinary) != 0 || int(InvertableTypeTokenStream) != 1 {
		t.Errorf("InvertableType ordinals diverge from Lucene: BINARY=%d TOKEN_STREAM=%d",
			InvertableTypeBinary, InvertableTypeTokenStream)
	}
}

// ---------------------------------------------------------------------------
// toInt64
// ---------------------------------------------------------------------------

func TestToInt64(t *testing.T) {
	cases := []struct {
		in   any
		want int64
	}{
		{int(7), 7},
		{int32(-3), -3},
		{int64(1 << 40), 1 << 40},
		{float32(2.9), 2},
		{float64(-4.7), -4},
		{"not-a-number", 0},
	}
	for _, tc := range cases {
		if got := toInt64(tc.in); got != tc.want {
			t.Errorf("toInt64(%v) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// fieldSchema: per-document schema consistency
// ---------------------------------------------------------------------------

func TestFieldSchemaSetIndexOptionsConsistent(t *testing.T) {
	s := newFieldSchema("body")
	if err := s.setIndexOptions(IndexOptionsDocsAndFreqs, false, false); err != nil {
		t.Fatalf("first setIndexOptions: %v", err)
	}
	// Same options again: must be accepted.
	if err := s.setIndexOptions(IndexOptionsDocsAndFreqs, false, false); err != nil {
		t.Fatalf("repeat setIndexOptions: %v", err)
	}
}

func TestFieldSchemaSetIndexOptionsInconsistent(t *testing.T) {
	s := newFieldSchema("body")
	if err := s.setIndexOptions(IndexOptionsDocs, false, false); err != nil {
		t.Fatalf("first setIndexOptions: %v", err)
	}
	if err := s.setIndexOptions(IndexOptionsDocsAndFreqs, false, false); err == nil {
		t.Fatal("expected error for divergent index options, got nil")
	}
}

func TestFieldSchemaSetDocValuesInconsistent(t *testing.T) {
	s := newFieldSchema("price")
	if err := s.setDocValues(DocValuesTypeNumeric, DocValuesSkipIndexTypeNone); err != nil {
		t.Fatalf("first setDocValues: %v", err)
	}
	if err := s.setDocValues(DocValuesTypeSorted, DocValuesSkipIndexTypeNone); err == nil {
		t.Fatal("expected error for divergent doc values type, got nil")
	}
}

func TestFieldSchemaSetPointsInconsistent(t *testing.T) {
	s := newFieldSchema("loc")
	if err := s.setPoints(2, 2, 4); err != nil {
		t.Fatalf("first setPoints: %v", err)
	}
	if err := s.setPoints(3, 3, 4); err == nil {
		t.Fatal("expected error for divergent point dimensions, got nil")
	}
}

func TestFieldSchemaSetVectorsInconsistent(t *testing.T) {
	s := newFieldSchema("vec")
	if err := s.setVectors(VectorEncodingFloat32, VectorSimilarityFunctionEuclidean, 128); err != nil {
		t.Fatalf("first setVectors: %v", err)
	}
	if err := s.setVectors(VectorEncodingFloat32, VectorSimilarityFunctionEuclidean, 64); err == nil {
		t.Fatal("expected error for divergent vector dimension, got nil")
	}
}

func TestFieldSchemaReset(t *testing.T) {
	s := newFieldSchema("f")
	if err := s.setIndexOptions(IndexOptionsDocsAndFreqs, true, true); err != nil {
		t.Fatalf("setIndexOptions: %v", err)
	}
	if err := s.setPoints(1, 1, 4); err != nil {
		t.Fatalf("setPoints: %v", err)
	}
	s.reset(5)
	if s.docID != 5 {
		t.Errorf("after reset docID = %d, want 5", s.docID)
	}
	if s.indexOptions != IndexOptionsNone || s.pointDimensionCount != 0 || s.omitNorms {
		t.Errorf("reset did not clear schema: %+v", s)
	}
	// After a reset the schema must accept fresh, different options.
	if err := s.setIndexOptions(IndexOptionsDocs, false, false); err != nil {
		t.Fatalf("setIndexOptions after reset: %v", err)
	}
}

func TestFieldSchemaAssertSameSchema(t *testing.T) {
	s := newFieldSchema("title")
	if err := s.setIndexOptions(IndexOptionsDocsAndFreqs, false, false); err != nil {
		t.Fatalf("setIndexOptions: %v", err)
	}
	opts := DefaultFieldInfoOptions()
	opts.IndexOptions = IndexOptionsDocsAndFreqs
	fi := NewFieldInfo("title", 0, opts)
	if err := s.assertSameSchema(fi); err != nil {
		t.Fatalf("assertSameSchema on matching FieldInfo: %v", err)
	}

	// A FieldInfo with a different index option must be rejected.
	optsBad := DefaultFieldInfoOptions()
	optsBad.IndexOptions = IndexOptionsDocs
	fiBad := NewFieldInfo("title", 0, optsBad)
	if err := s.assertSameSchema(fiBad); err == nil {
		t.Fatal("assertSameSchema accepted a divergent FieldInfo, want error")
	}
}

// ---------------------------------------------------------------------------
// updateDocFieldSchema
// ---------------------------------------------------------------------------

func TestUpdateDocFieldSchemaIndexed(t *testing.T) {
	s := newFieldSchema("body")
	ft := &fakeFieldType{
		indexOptions:  IndexOptionsDocsAndFreqs,
		docValuesType: DocValuesTypeNumeric,
		attributes:    map[string]string{"k": "v"},
	}
	if err := updateDocFieldSchema("body", s, ft); err != nil {
		t.Fatalf("updateDocFieldSchema: %v", err)
	}
	if s.indexOptions != IndexOptionsDocsAndFreqs {
		t.Errorf("indexOptions = %v, want DocsAndFreqs", s.indexOptions)
	}
	if s.docValuesType != DocValuesTypeNumeric {
		t.Errorf("docValuesType = %v, want Numeric", s.docValuesType)
	}
	if s.attributes["k"] != "v" {
		t.Errorf("attribute k not propagated: %v", s.attributes)
	}
}

func TestUpdateDocFieldSchemaUnindexedWithTermVectorsFails(t *testing.T) {
	s := newFieldSchema("f")
	ft := &fakeFieldType{indexOptions: IndexOptionsNone, storeTermVectors: true}
	if err := updateDocFieldSchema("f", s, ft); err == nil {
		t.Fatal("expected error: term vectors on an unindexed field")
	}
}

func TestUpdateDocFieldSchemaSkipIndexWithoutDocValuesFails(t *testing.T) {
	s := newFieldSchema("f")
	// docValuesType NONE but a non-NONE skip index type: illegal.
	ft := &fakeFieldType{
		indexOptions:           IndexOptionsNone,
		docValuesType:          DocValuesTypeNone,
		docValuesSkipIndexType: DocValuesSkipIndexType(1),
	}
	if err := updateDocFieldSchema("f", s, ft); err == nil {
		t.Fatal("expected error: skip index without doc values")
	}
}

// ---------------------------------------------------------------------------
// verifyUnIndexedFieldType
// ---------------------------------------------------------------------------

func TestVerifyUnIndexedFieldType(t *testing.T) {
	if err := verifyUnIndexedFieldType("ok", &fakeFieldType{}); err != nil {
		t.Fatalf("plain unindexed field rejected: %v", err)
	}
	bad := []*fakeFieldType{
		{storeTermVectors: true},
		{storeTermVectorPositions: true},
		{storeTermVectorOffsets: true},
		{storeTermVectorPayloads: true},
	}
	for i, ft := range bad {
		if err := verifyUnIndexedFieldType("f", ft); err == nil {
			t.Errorf("case %d: expected error for term-vector option on unindexed field", i)
		}
	}
}

// ---------------------------------------------------------------------------
// dvWriterBox
// ---------------------------------------------------------------------------

func TestDVWriterBoxNumeric(t *testing.T) {
	opts := DefaultFieldInfoOptions()
	opts.DocValuesType = DocValuesTypeNumeric
	fi := NewFieldInfo("n", 0, opts)
	box := newDVWNumeric(NewNumericDocValuesWriter(fi, util.NewCounter()))
	if err := box.addNumeric(0, 42); err != nil {
		t.Fatalf("addNumeric: %v", err)
	}
	// addBinary on a numeric box is a no-op and must not panic.
	if err := box.addBinary(1, util.NewBytesRef([]byte("x"))); err != nil {
		t.Fatalf("addBinary on numeric box: %v", err)
	}
}

func TestDVWriterBoxBinary(t *testing.T) {
	opts := DefaultFieldInfoOptions()
	opts.DocValuesType = DocValuesTypeBinary
	fi := NewFieldInfo("b", 0, opts)
	w, err := NewBinaryDocValuesWriter(fi, util.NewCounter())
	if err != nil {
		t.Fatalf("NewBinaryDocValuesWriter: %v", err)
	}
	box := newDVWBinary(w)
	if err := box.addBinary(0, util.NewBytesRef([]byte("hello"))); err != nil {
		t.Fatalf("addBinary: %v", err)
	}
}

// ---------------------------------------------------------------------------
// IndexingChain construction + getOrAddPerField / rehash
// ---------------------------------------------------------------------------

// stubFieldInfosBuilder is a minimal FieldInfosBuilderHandle for tests.
type stubFieldInfosBuilder struct{}

func (stubFieldInfosBuilder) Add(fi *FieldInfo) (*FieldInfo, error) { return fi, nil }
func (stubFieldInfosBuilder) Finish() *FieldInfos                   { return nil }
func (stubFieldInfosBuilder) SoftDeletesFieldName() string          { return "" }
func (stubFieldInfosBuilder) ParentFieldName() string               { return "" }

// stubStoredFieldsConsumer is a no-op StoredFieldsConsumerHandle for tests.
type stubStoredFieldsConsumer struct{}

func (stubStoredFieldsConsumer) StartDocument(int) error                         { return nil }
func (stubStoredFieldsConsumer) WriteField(*FieldInfo, IndexingChainField) error { return nil }
func (stubStoredFieldsConsumer) FinishDocument() error                           { return nil }
func (stubStoredFieldsConsumer) Finish(int) error                                { return nil }
func (stubStoredFieldsConsumer) Flush(*SegmentWriteState, SorterDocMap) error    { return nil }
func (stubStoredFieldsConsumer) Abort()                                          {}
func (stubStoredFieldsConsumer) RamBytesUsed() int64                             { return 0 }

// stubVectorValuesConsumer is a no-op VectorValuesConsumerHandle for tests.
type stubVectorValuesConsumer struct{}

func (stubVectorValuesConsumer) AddField(*FieldInfo) (KnnFieldVectorsWriterHandle, error) {
	return nil, nil
}
func (stubVectorValuesConsumer) Flush(*SegmentWriteState, SorterDocMap) error { return nil }
func (stubVectorValuesConsumer) Abort()                                       {}
func (stubVectorValuesConsumer) RamBytesUsed() int64                          { return 0 }

// newTestChain builds an IndexingChain wired to no-op collaborators. It reuses
// stubTermsHash / newStubTermsHash from terms_hash_test.go.
func newTestChain(t *testing.T) *IndexingChain {
	t.Helper()
	c, err := NewIndexingChain(
		10,
		stubFieldInfosBuilder{},
		nil, // no config: index sort off, no similarity
		func(error) {},
		newStubTermsHash(util.NewCounter(), nil),
		stubStoredFieldsConsumer{},
		stubVectorValuesConsumer{},
		nil,
	)
	if err != nil {
		t.Fatalf("NewIndexingChain: %v", err)
	}
	return c
}

func TestNewIndexingChainNilGuards(t *testing.T) {
	_, err := NewIndexingChain(0, nil, nil, nil, nil, nil, nil, nil)
	if err == nil {
		t.Fatal("expected error for nil abortingExceptionConsumer")
	}
	_, err = NewIndexingChain(0, nil, nil, func(error) {}, nil, nil, nil, nil)
	if err == nil {
		t.Fatal("expected error for nil fieldInfos")
	}
}

func TestGetOrAddPerFieldStableAndRehashes(t *testing.T) {
	c := newTestChain(t)

	// Re-adding the same name must return the identical PerField pointer.
	a1 := c.getOrAddPerField("alpha")
	a2 := c.getOrAddPerField("alpha")
	if a1 != a2 {
		t.Fatal("getOrAddPerField returned distinct PerField for the same name")
	}

	// Add enough distinct fields to force at least one rehash. The table
	// starts at length 2 and grows at a 50% load factor.
	names := []string{"f0", "f1", "f2", "f3", "f4", "f5", "f6", "f7", "f8", "f9"}
	for _, n := range names {
		c.getOrAddPerField(n)
	}
	if c.totalFieldCount != len(names)+1 { // +1 for "alpha"
		t.Fatalf("totalFieldCount = %d, want %d", c.totalFieldCount, len(names)+1)
	}
	if len(c.fieldHash) < 2*c.totalFieldCount {
		t.Errorf("fieldHash not grown to maintain <=50%% load: len=%d totalFieldCount=%d",
			len(c.fieldHash), c.totalFieldCount)
	}

	// Every field must still be retrievable after rehash, by both
	// getOrAddPerField and getPerField.
	for _, n := range append(names, "alpha") {
		if pf := c.getPerField(n); pf == nil || pf.fieldName != n {
			t.Errorf("getPerField(%q) lost the field after rehash", n)
		}
	}
}

func TestSortPerFieldsByName(t *testing.T) {
	pfs := []*indexingPerField{
		{fieldName: "charlie"},
		{fieldName: "alpha"},
		{fieldName: "bravo"},
	}
	sortPerFieldsByName(pfs)
	want := []string{"alpha", "bravo", "charlie"}
	for i, w := range want {
		if pfs[i].fieldName != w {
			t.Errorf("sortPerFieldsByName: index %d = %q, want %q", i, pfs[i].fieldName, w)
		}
	}
}

func TestMaybeSortSegmentRejectsConfiguredSort(t *testing.T) {
	c := newTestChain(t)
	c.indexWriterConfig = sortOnConfig{}
	if _, err := c.maybeSortSegment(nil); err == nil {
		t.Fatal("maybeSortSegment must fail loudly when an index sort is configured (GAP)")
	}
}

// sortOnConfig is an IndexingChainConfig that reports a configured index sort,
// used to assert the index-sorting GAP fails loudly.
type sortOnConfig struct{}

func (sortOnConfig) HasIndexSort() bool           { return true }
func (sortOnConfig) Similarity() SimilarityHandle { return nil }

func TestResetFieldInvertState(t *testing.T) {
	s := NewFieldInvertState(10, "f", IndexOptionsDocsAndFreqs)
	s.SetPosition(7)
	s.SetLength(9)
	s.SetNumOverlap(2)
	s.SetOffset(11)
	resetFieldInvertState(s)
	if s.Position() != 0 || s.Length() != 0 || s.NumOverlap() != 0 || s.Offset() != 0 {
		t.Errorf("resetFieldInvertState did not zero the counters: %+v", s)
	}
}
