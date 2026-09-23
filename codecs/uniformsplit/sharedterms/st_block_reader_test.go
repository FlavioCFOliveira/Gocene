// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package sharedterms

// Port of
// lucene/codecs/src/test/org/apache/lucene/codecs/uniformsplit/sharedterms/TestSTBlockReader.java
// (Apache Lucene 10.5.0). The Java MockSTBlockReader subclass overrides the
// protected BlockReader methods readTermState, compareToMiddleAndJump,
// readLineInBlock, initializeHeader and readHeader; Go renders it as a type
// embedding *STBlockReader that installs itself as the BlockReader Overrides.
// MockTermStateFactory.create() (lucene/codecs/src/test/.../lucene90/tests)
// returns a new Lucene104PostingsFormat.IntBlockTermState.

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/codecs/uniformsplit"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

const mockBlockOutputName = "TestSTBlockReader.tmp"

// stBlockReaderFixture holds the TestSTBlockReader fields built by setUp().
type stBlockReaderFixture struct {
	fieldInfos *index.FieldInfos
	blockLines []*mockSTBlockLine
	supplier   uniformsplit.IndexDictionaryBrowserSupplier
	directory  *store.ByteBuffersDirectory
}

// dictionaryBrowserSupplier is the anonymous IndexDictionary.BrowserSupplier
// of setUp().
type dictionaryBrowserSupplier struct {
	indexDictionary uniformsplit.IndexDictionary
}

func (s *dictionaryBrowserSupplier) Get() (uniformsplit.IndexDictionaryBrowser, error) {
	return s.indexDictionary.Browser()
}

func newSTBlockReaderFixture(t *testing.T) *stBlockReaderFixture {
	t.Helper()
	f := &stBlockReaderFixture{fieldInfos: mockFieldInfos()}
	vocab := []blockLineDefinition{
		blockLineDef(1, "abaco", "f1", "f3"),
		blockLineDef(2, "amiga", "f1", "f2", "f4"),
		blockLineDef(5, "amigo", "f1", "f2", "f3", "f4"),
		blockLineDef(2, "arco", "f1"),
		blockLineDef(1, "bloom", "f2"),
		blockLineDef(1, "frien", "f2"),
		blockLineDef(6, "frienchies", "f3"),
	}

	f.blockLines = generateBlockLines(vocab)
	f.directory = store.NewByteBuffersDirectory()
	blockOutput, err := f.directory.CreateOutput(mockBlockOutputName, store.IOContextDefault)
	if err != nil {
		t.Fatal(err)
	}
	if err := blockOutput.WriteVInt(5); err != nil {
		t.Fatal(err)
	}
	if err := blockOutput.Close(); err != nil {
		t.Fatal(err)
	}
	builder, err := uniformsplit.NewFSTDictionaryBuilder()
	if err != nil {
		t.Fatal(err)
	}
	if err := builder.Add(util.NewBytesRef([]byte("a")), 0); err != nil {
		t.Fatal(err)
	}
	indexDictionary, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	f.supplier = &dictionaryBrowserSupplier{indexDictionary: indexDictionary}
	t.Cleanup(func() {
		// tearDown()
		f.blockLines = nil
		if err := f.directory.Close(); err != nil {
			t.Error(err)
		}
	})
	return f
}

func assertSTTerm(t *testing.T, r *mockSTBlockReader, want string) {
	t.Helper()
	term := r.Term()
	if term == nil {
		t.Fatalf("term() = null, want %q", want)
	}
	if got := term.BytesValue().String(); got != want {
		t.Fatalf("term() = %q, want %q", got, want)
	}
}

func TestSTBlockReader_testSeekExactIgnoreFieldF1(t *testing.T) {
	f := newSTBlockReaderFixture(t)
	// when block reader for field 1 -> f1
	blockReader := newMockSTBlockReader(t, f.supplier, f.blockLines, f.directory,
		f.fieldInfos.FieldInfo("f1"), // last term "arco"
		f.fieldInfos)

	// when seekCeil
	if _, err := blockReader.SeekCeilBytes(util.NewBytesRef([]byte("arco2"))); err != nil {
		t.Fatal(err)
	}
	// then
	if term := blockReader.Term(); term != nil {
		t.Fatalf("term() = %q, want null", term.BytesValue().String())
	}

	// when seekCeilIgnoreField
	if _, err := blockReader.seekCeilIgnoreField(util.NewBytesRef([]byte("arco2"))); err != nil {
		t.Fatal(err)
	}
	// then
	assertSTTerm(t, blockReader, "bloom")
}

func TestSTBlockReader_testSeekExactIgnoreFieldF2(t *testing.T) {
	f := newSTBlockReaderFixture(t)
	blockReader := newMockSTBlockReader(t, f.supplier, f.blockLines, f.directory,
		f.fieldInfos.FieldInfo("f2"), // last term "frien"
		f.fieldInfos)

	// when seekCeil
	if _, err := blockReader.seekCeilIgnoreField(util.NewBytesRef([]byte("arco2"))); err != nil {
		t.Fatal(err)
	}
	// then
	assertSTTerm(t, blockReader, "bloom")
}

func TestSTBlockReader_testSeekExactIgnoreFieldF3(t *testing.T) {
	f := newSTBlockReaderFixture(t)
	blockReader := newMockSTBlockReader(t, f.supplier, f.blockLines, f.directory,
		f.fieldInfos.FieldInfo("f3"), // last term "frienchies"
		f.fieldInfos)

	// when seekCeilIgnoreField
	if _, err := blockReader.seekCeilIgnoreField(util.NewBytesRef([]byte("arco2"))); err != nil {
		t.Fatal(err)
	}
	// then
	assertSTTerm(t, blockReader, "bloom")

	// when seekCeil
	if _, err := blockReader.SeekCeilBytes(util.NewBytesRef([]byte("arco2"))); err != nil {
		t.Fatal(err)
	}
	// then
	assertSTTerm(t, blockReader, "frienchies")
}

func TestSTBlockReader_testSeekExactIgnoreFieldF4(t *testing.T) {
	f := newSTBlockReaderFixture(t)
	blockReader := newMockSTBlockReader(t, f.supplier, f.blockLines, f.directory,
		f.fieldInfos.FieldInfo("f4"), // last term "amigo"
		f.fieldInfos)

	// when seekCeilIgnoreField
	if _, err := blockReader.seekCeilIgnoreField(util.NewBytesRef([]byte("abaco"))); err != nil {
		t.Fatal(err)
	}
	// then
	assertSTTerm(t, blockReader, "abaco")

	// when seekCeil
	if _, err := blockReader.SeekCeilBytes(util.NewBytesRef([]byte("abaco"))); err != nil {
		t.Fatal(err)
	}
	// then
	assertSTTerm(t, blockReader, "amiga")
}

func mockFieldInfos() *index.FieldInfos {
	return spi.NewFieldInfos(
		mockFieldInfo("f1", 0),
		mockFieldInfo("f2", 1),
		mockFieldInfo("f3", 2),
		mockFieldInfo("f4", 3),
	)
}

func mockFieldInfo(fieldName string, number int) *index.FieldInfo {
	opts := spi.DefaultFieldInfoOptions()
	opts.StoreTermVectors = false
	opts.OmitNorms = false
	opts.IndexOptions = spi.IndexOptionsDocsAndFreqsAndPositionsAndOffsets
	opts.DocValuesType = spi.DocValuesTypeNone
	opts.DocValuesSkipIndexType = spi.DocValuesSkipIndexTypeNone
	opts.DocValuesGen = -1
	opts.VectorEncoding = util.VectorEncodingFloat32
	opts.VectorSimilarityFunction = util.EuclideanSim
	fi := spi.NewFieldInfo(fieldName, number, opts)
	fi.SetStorePayloads() // storePayloads = true
	return fi
}

// blockLineDefinition renders the private record BlockLineDefinition.
type blockLineDefinition struct {
	termBytes *uniformsplit.TermBytes
	fields    []string
}

func blockLineDef(mdpLength int, term string, fields ...string) blockLineDefinition {
	return blockLineDefinition{
		termBytes: uniformsplit.NewTermBytes(mdpLength, util.NewBytesRef([]byte(term))),
		fields:    fields,
	}
}

func generateBlockLines(blockLineDefinitions []blockLineDefinition) []*mockSTBlockLine {
	lines := make([]*mockSTBlockLine, 0, len(blockLineDefinitions))
	for _, def := range blockLineDefinitions {
		lines = append(lines, newMockSTBlockLine(def.termBytes, def.fields))
	}
	return lines
}

// mockSTBlockLine renders the private static class MockSTBlockLine, which
// extends STBlockLine.
type mockSTBlockLine struct {
	*STBlockLine
	termStates map[string]index.TermState
}

func newMockSTBlockLine(termBytes *uniformsplit.TermBytes, fields []string) *mockSTBlockLine {
	l := &mockSTBlockLine{
		STBlockLine: NewSTBlockLine(termBytes, []*FieldMetadataTermState{NewFieldMetadataTermState(nil, nil)}),
		termStates:  make(map[string]index.TermState),
	}
	for _, field := range fields {
		l.termStates[field] = codecs.NewIntBlockTermState() // MockTermStateFactory.create()
	}
	return l
}

func (l *mockSTBlockLine) getFields() map[string]index.TermState {
	return l.termStates
}

// mockSTBlockReader renders the private static class MockSTBlockReader, which
// extends STBlockReader.
type mockSTBlockReader struct {
	*STBlockReader
	lines []*mockSTBlockLine
}

func newMockSTBlockReader(t *testing.T, supplier uniformsplit.IndexDictionaryBrowserSupplier, lines []*mockSTBlockLine,
	directory store.Directory, fieldInfo *index.FieldInfo, fieldInfos *index.FieldInfos) *mockSTBlockReader {
	t.Helper()
	blockInput, err := directory.OpenInput(mockBlockOutputName, store.IOContextDefault)
	if err != nil {
		t.Fatal(err)
	}
	fieldMetadata, err := mockFieldMetadata(fieldInfo, getLastTermForField(lines, fieldInfo.Name()))
	if err != nil {
		t.Fatal(err)
	}
	base, err := NewSTBlockReader(supplier, blockInput, getMockPostingReaderBase(), fieldMetadata, nil, fieldInfos)
	if err != nil {
		t.Fatal(err)
	}
	r := &mockSTBlockReader{STBlockReader: base, lines: lines}
	// Java's `this` in the BlockReader bodies is the MockSTBlockReader.
	r.Overrides = r
	return r
}

// mockPostingsReaderBase is the anonymous PostingsReaderBase of
// getMockPostingReaderBase().
type mockPostingsReaderBase struct{}

func (mockPostingsReaderBase) Init(termsIn store.IndexInput, state *codecs.SegmentReadState) error {
	return nil
}
func (mockPostingsReaderBase) NewTermState() index.TermState { return nil }
func (mockPostingsReaderBase) DecodeTerm(in store.DataInput, fieldInfo *index.FieldInfo, termState index.TermState, absolute bool) error {
	return nil
}
func (mockPostingsReaderBase) Postings(fieldInfo *index.FieldInfo, termState index.TermState, reuse index.PostingsEnum, flags int) (index.PostingsEnum, error) {
	return nil, nil
}
func (mockPostingsReaderBase) Impacts(fieldInfo *index.FieldInfo, termState index.TermState, flags int) (index.ImpactsEnum, error) {
	return nil, nil
}
func (mockPostingsReaderBase) CheckIntegrity() error { return nil }
func (mockPostingsReaderBase) Close() error          { return nil }

func getMockPostingReaderBase() codecs.PostingsReaderBase {
	return mockPostingsReaderBase{}
}

func mockFieldMetadata(fieldInfo *index.FieldInfo, lastTerm *util.BytesRef) (*uniformsplit.FieldMetadata, error) {
	fieldMetadata, err := uniformsplit.NewFieldMetadata(fieldInfo, 1)
	if err != nil {
		return nil, err
	}
	fieldMetadata.SetLastTerm(lastTerm)
	fieldMetadata.SetLastBlockStartFP(1)
	return fieldMetadata, nil
}

func getLastTermForField(lines []*mockSTBlockLine, fieldName string) *util.BytesRef {
	var lastTerm *util.BytesRef
	for _, line := range lines {
		if _, ok := line.getFields()[fieldName]; ok {
			lastTerm = line.GetTermBytes().GetTerm()
		}
	}
	return lastTerm
}

// ReadTermState overrides BlockReader.readTermState.
func (r *mockSTBlockReader) ReadTermState() (index.TermState, error) {
	r.CurrentTermState = r.lines[r.LineIndexInBlock-1].termStates[r.FieldMetadata.GetFieldInfo().Name()]
	return r.CurrentTermState, nil
}

// CompareToMiddleAndJump overrides BlockReader.compareToMiddleAndJump.
func (r *mockSTBlockReader) CompareToMiddleAndJump(searchedTerm *util.BytesRef) (int, error) {
	r.BlockLine = r.lines[len(r.lines)>>1].BlockLine
	r.LineIndexInBlock = r.BlockHeader.MiddleLineIndex()
	compare := searchedTerm.BytesRefCompareTo(r.TermBytes())
	if compare < 0 {
		r.LineIndexInBlock = 0
	}
	return compare, nil
}

// ReadLineInBlock overrides BlockReader.readLineInBlock.
func (r *mockSTBlockReader) ReadLineInBlock() (*uniformsplit.BlockLine, error) {
	if int(r.LineIndexInBlock) >= len(r.lines) {
		r.BlockLine = nil
		return nil, nil
	}
	r.BlockLine = r.lines[r.LineIndexInBlock].BlockLine
	r.LineIndexInBlock++
	return r.BlockLine, nil
}

// InitializeHeader overrides BlockReader.initializeHeader.
func (r *mockSTBlockReader) InitializeHeader(searchedTerm *util.BytesRef, startBlockLinePos int64) error {
	// Force blockStartFP to an impossible value so we never trigger the optimization
	// that keeps the current block with our mock block reader.
	r.BlockStartFP = -1
	return r.STBlockReader.InitializeHeader(searchedTerm, startBlockLinePos)
}

// ReadHeader overrides BlockReader.readHeader.
func (r *mockSTBlockReader) ReadHeader() (*uniformsplit.BlockHeader, error) {
	if int(r.LineIndexInBlock) >= len(r.lines) {
		r.BlockHeader = nil
	} else {
		r.BlockHeader = newMockBlockHeader(int32(len(r.lines)))
	}
	return r.BlockHeader, nil
}

// newMockBlockHeader renders the private static class MockBlockHeader.
func newMockBlockHeader(linesCount int32) *uniformsplit.BlockHeader {
	return uniformsplit.NewBlockHeader(linesCount, 0, 0, 0, 1, 0)
}
