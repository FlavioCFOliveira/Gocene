// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package uniformsplit

// Port of
// lucene/codecs/src/test/org/apache/lucene/codecs/uniformsplit/TestTermBytesComparator.java
// (Apache Lucene 10.5.0): tests the TermBytes comparator. The Java
// MockBlockReader subclass overrides the protected BlockReader methods
// compareToMiddleAndJump, readLineInBlock, initializeHeader and readHeader; Go
// renders it as a type embedding *BlockReader that installs itself as the
// BlockReader Overrides.

import (
	"math"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

func TestTermBytesComparator_testComparison(t *testing.T) {
	vocab := []*TermBytes{
		termBytesOf(1, "abaco"),
		termBytesOf(2, "amiga"),
		termBytesOf(5, "amigo"),
		termBytesOf(2, "arco"),
		termBytesOf(1, "bloom"),
		termBytesOf(1, "frien"),
		termBytesOf(6, "frienchies"),
		termBytesOf(6, "friend"),
		termBytesOf(7, "friendalan"),
		termBytesOf(7, "friende"),
		termBytesOf(8, "friendez"),
	}
	lines := generateTermBytesBlockLines(vocab)
	directory := store.NewByteBuffersDirectory()
	defer directory.Close()
	indexOutput, err := directory.CreateOutput("temp.bin", store.IOContextDefault)
	if err != nil {
		t.Fatal(err)
	}
	if err := indexOutput.WriteVInt(5); err != nil {
		t.Fatal(err)
	}
	if err := indexOutput.Close(); err != nil {
		t.Fatal(err)
	}

	blockReader := newTermBytesMockBlockReader(t, lines, directory)

	assertAlwaysGreater(t, blockReader, util.NewBytesRef([]byte("z")))

	assertGreaterUntil(t, 1, blockReader, util.NewBytesRef([]byte("abacu")))

	assertGreaterUntil(t, 4, blockReader, util.NewBytesRef([]byte("bar")))

	assertGreaterUntil(t, 2, blockReader, util.NewBytesRef([]byte("amigas")))

	assertGreaterUntil(t, 10, blockReader, util.NewBytesRef([]byte("friendez")))
}

func assertGreaterUntil(t *testing.T, expectedPosition int, blockReader *termBytesMockBlockReader, lookedTerm *util.BytesRef) spi.SeekStatus {
	t.Helper()
	seekStatus, err := blockReader.SeekInBlock(lookedTerm)
	if err != nil {
		t.Fatalf("seekInBlock(%s): %v", lookedTerm.String(), err)
	}
	if got := int(blockReader.LineIndexInBlock) - 1; got != expectedPosition {
		t.Fatalf("looked Term: %s expected:<%d> but was:<%d>", lookedTerm.String(), expectedPosition, got)
	}

	// reset the state
	blockReader.reset()
	return seekStatus
}

func assertAlwaysGreater(t *testing.T, blockReader *termBytesMockBlockReader, lookedTerm *util.BytesRef) {
	t.Helper()
	seekStatus := assertGreaterUntil(t, -1, blockReader, lookedTerm)
	if seekStatus != spi.SeekStatusEnd {
		t.Fatalf("seekStatus = %v, want END", seekStatus)
	}
}

func generateTermBytesBlockLines(words []*TermBytes) []*BlockLine {
	lines := make([]*BlockLine, 0, len(words))
	for _, word := range words {
		lines = append(lines, NewBlockLineForWriting(word, nil))
	}
	return lines
}

// termBytesMockBlockReader renders the inner class MockBlockReader, which
// extends BlockReader.
type termBytesMockBlockReader struct {
	*BlockReader
	lines []*BlockLine
}

func newTermBytesMockBlockReader(t *testing.T, lines []*BlockLine, directory store.Directory) *termBytesMockBlockReader {
	t.Helper()
	blockInput, err := directory.OpenInput("temp.bin", store.IOContextDefault)
	if err != nil {
		t.Fatal(err)
	}
	fieldMetadata, err := NewFieldMetadata(nil, 1)
	if err != nil {
		t.Fatal(err)
	}
	base, err := NewBlockReader(nil, blockInput, createMockPostingReaderBase(), fieldMetadata, nil)
	if err != nil {
		t.Fatal(err)
	}
	r := &termBytesMockBlockReader{BlockReader: base, lines: lines}
	// Java's `this` in the BlockReader bodies is the MockBlockReader.
	r.Overrides = r
	return r
}

// CompareToMiddleAndJump overrides BlockReader.compareToMiddleAndJump.
func (r *termBytesMockBlockReader) CompareToMiddleAndJump(searchedTerm *util.BytesRef) (int, error) {
	// Do not jump in test.
	return -1, nil
}

// ReadLineInBlock overrides BlockReader.readLineInBlock.
func (r *termBytesMockBlockReader) ReadLineInBlock() (*BlockLine, error) {
	if int(r.LineIndexInBlock) >= len(r.lines) {
		r.LineIndexInBlock = 0
		r.BlockLine = nil
		return nil, nil
	}
	r.BlockLine = r.lines[r.LineIndexInBlock]
	r.LineIndexInBlock++
	return r.BlockLine, nil
}

// InitializeHeader overrides BlockReader.initializeHeader.
func (r *termBytesMockBlockReader) InitializeHeader(searchedTerm *util.BytesRef, targetBlockStartFP int64) error {
	// Force blockStartFP to an impossible value so we never trigger the optimization
	// that keeps the current block with our mock block reader.
	r.BlockStartFP = math.MinInt64
	return r.BlockReader.InitializeHeader(searchedTerm, targetBlockStartFP)
}

// ReadHeader overrides BlockReader.readHeader.
func (r *termBytesMockBlockReader) ReadHeader() (*BlockHeader, error) {
	if int(r.LineIndexInBlock) >= len(r.lines) {
		r.BlockHeader = nil
	} else {
		r.BlockHeader = NewBlockHeader(int32(len(r.lines)), 0, 0, 0, 0, 0)
	}
	return r.BlockHeader, nil
}

func (r *termBytesMockBlockReader) reset() {
	r.LineIndexInBlock = 0
	r.BlockHeader = nil
	r.BlockLine = nil
}

func termBytesOf(mdpLength int, term string) *TermBytes {
	return NewTermBytes(mdpLength, util.NewBytesRef([]byte(term)))
}

// termBytesMockPostingsReaderBase is the anonymous PostingsReaderBase of
// createMockPostingReaderBase().
type termBytesMockPostingsReaderBase struct{}

func (termBytesMockPostingsReaderBase) Init(termsIn store.IndexInput, state *codecs.SegmentReadState) error {
	return nil
}
func (termBytesMockPostingsReaderBase) NewTermState() index.TermState { return nil }
func (termBytesMockPostingsReaderBase) DecodeTerm(in store.DataInput, fieldInfo *index.FieldInfo, termState index.TermState, absolute bool) error {
	return nil
}
func (termBytesMockPostingsReaderBase) Postings(fieldInfo *index.FieldInfo, termState index.TermState, reuse index.PostingsEnum, flags int) (index.PostingsEnum, error) {
	return nil, nil
}
func (termBytesMockPostingsReaderBase) Impacts(fieldInfo *index.FieldInfo, termState index.TermState, flags int) (index.ImpactsEnum, error) {
	return nil, nil
}
func (termBytesMockPostingsReaderBase) CheckIntegrity() error { return nil }
func (termBytesMockPostingsReaderBase) Close() error          { return nil }

func createMockPostingReaderBase() codecs.PostingsReaderBase {
	return termBytesMockPostingsReaderBase{}
}
