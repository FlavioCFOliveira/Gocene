// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// writeVLongsToBytes returns the raw bytes produced by writing the given
// VLong values to a ByteBuffersDataOutput.
func writeVLongsToBytes(t *testing.T, values ...int64) []byte {
	t.Helper()
	out := store.NewByteBuffersDataOutput()
	for _, v := range values {
		if err := out.WriteVLong(v); err != nil {
			t.Fatalf("WriteVLong(%d): %v", v, err)
		}
	}
	return out.ToArrayCopy()
}

// openMemoryInput wraps b in an IndexInput exposed by a ByteBuffersDirectory;
// the resulting IndexInput natively supports random access.
func openMemoryInput(t *testing.T, name string, b []byte) (store.Directory, store.IndexInput) {
	t.Helper()
	dir := store.NewByteBuffersDirectory()
	out, err := dir.CreateOutput(name, store.IOContext{Context: store.ContextWrite})
	if err != nil {
		t.Fatalf("CreateOutput(%s): %v", name, err)
	}
	if err := out.WriteBytesN(b, len(b)); err != nil {
		t.Fatalf("WriteBytes(%s): %v", name, err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("Close(%s): %v", name, err)
	}
	in, err := dir.OpenInput(name, store.IOContext{Context: store.ContextRead})
	if err != nil {
		t.Fatalf("OpenInput(%s): %v", name, err)
	}
	return dir, in
}

func TestLucene103FieldReader_ConstructorReadsHeaderAndExposesStats(t *testing.T) {
	const (
		numTerms         = int64(42)
		sumTotalTermFreq = int64(123)
		sumDocFreq       = int64(99)
		docCount         = 17
		indexStart       = int64(64)
		rootFP           = int64(200)
		indexEnd         = int64(512)
	)
	metaBytes := writeVLongsToBytes(t, indexStart, rootFP, indexEnd)
	dirMeta, metaIn := openMemoryInput(t, "meta", metaBytes)
	defer dirMeta.Close()
	defer metaIn.Close()

	// indexIn needs to be at least indexEnd bytes long so the slice math
	// inside NewTrieReader succeeds when the caller eventually opens one.
	indexBytes := make([]byte, indexEnd+16)
	dirIndex, indexIn := openMemoryInput(t, "index", indexBytes)
	defer dirIndex.Close()
	defer indexIn.Close()

	fi := index.NewFieldInfo("field1", 5, index.FieldInfoOptions{
		IndexOptions: index.IndexOptionsDocsAndFreqs,
	})
	parent := newReaderForTest("_0")

	minTerm := util.NewBytesRef([]byte("alpha"))
	maxTerm := util.NewBytesRef([]byte("omega"))

	fr, err := NewLucene103FieldReader(parent, fi, numTerms, sumTotalTermFreq, sumDocFreq, docCount, metaIn, indexIn, minTerm, maxTerm)
	if err != nil {
		t.Fatalf("NewLucene103FieldReader: %v", err)
	}

	if got := fr.NumTerms(); got != numTerms {
		t.Errorf("NumTerms: want %d, got %d", numTerms, got)
	}
	if got := fr.RootFP(); got != rootFP {
		t.Errorf("RootFP: want %d, got %d", rootFP, got)
	}
	if got, _ := fr.GetDocCount(); got != docCount {
		t.Errorf("GetDocCount: want %d, got %d", docCount, got)
	}
	if got, _ := fr.GetSumDocFreq(); got != sumDocFreq {
		t.Errorf("GetSumDocFreq: want %d, got %d", sumDocFreq, got)
	}
	if got, _ := fr.GetSumTotalTermFreq(); got != sumTotalTermFreq {
		t.Errorf("GetSumTotalTermFreq: want %d, got %d", sumTotalTermFreq, got)
	}
	if got := fr.Size(); got != numTerms {
		t.Errorf("Size: want %d, got %d", numTerms, got)
	}

	gotMin, err := fr.GetMin()
	if err != nil || gotMin == nil || gotMin.Text() != "alpha" {
		t.Errorf("GetMin: want alpha, got %v err=%v", gotMin, err)
	}
	gotMax, err := fr.GetMax()
	if err != nil || gotMax == nil || gotMax.Text() != "omega" {
		t.Errorf("GetMax: want omega, got %v err=%v", gotMax, err)
	}
}

func TestLucene103FieldReader_HasFreqsOffsetsPositionsTracksIndexOptions(t *testing.T) {
	type tc struct {
		opts         index.IndexOptions
		hasFreqs     bool
		hasPositions bool
		hasOffsets   bool
	}
	cases := []tc{
		{index.IndexOptionsDocs, false, false, false},
		{index.IndexOptionsDocsAndFreqs, true, false, false},
		{index.IndexOptionsDocsAndFreqsAndPositions, true, true, false},
		{index.IndexOptionsDocsAndFreqsAndPositionsAndOffsets, true, true, true},
	}
	for _, c := range cases {
		c := c
		t.Run(c.opts.String(), func(t *testing.T) {
			dirMeta, metaIn := openMemoryInput(t, "meta", writeVLongsToBytes(t, 0, 0, 8))
			defer dirMeta.Close()
			defer metaIn.Close()
			dirIdx, indexIn := openMemoryInput(t, "index", make([]byte, 16))
			defer dirIdx.Close()
			defer indexIn.Close()

			fi := index.NewFieldInfo("f", 0, index.FieldInfoOptions{IndexOptions: c.opts})
			fr, err := NewLucene103FieldReader(nil, fi, 1, 0, 0, 0, metaIn, indexIn, util.NewBytesRefEmpty(), util.NewBytesRefEmpty())
			if err != nil {
				t.Fatalf("NewLucene103FieldReader: %v", err)
			}
			if got := fr.HasFreqs(); got != c.hasFreqs {
				t.Errorf("HasFreqs: want %v, got %v", c.hasFreqs, got)
			}
			if got := fr.HasPositions(); got != c.hasPositions {
				t.Errorf("HasPositions: want %v, got %v", c.hasPositions, got)
			}
			if got := fr.HasOffsets(); got != c.hasOffsets {
				t.Errorf("HasOffsets: want %v, got %v", c.hasOffsets, got)
			}
		})
	}
}

func TestLucene103FieldReader_RejectsInvalidArguments(t *testing.T) {
	metaBytes := writeVLongsToBytes(t, 0, 0, 4)
	dirMeta, metaIn := openMemoryInput(t, "meta", metaBytes)
	defer dirMeta.Close()
	defer metaIn.Close()
	dirIdx, indexIn := openMemoryInput(t, "index", make([]byte, 16))
	defer dirIdx.Close()
	defer indexIn.Close()

	fi := index.NewFieldInfo("f", 0, index.FieldInfoOptions{IndexOptions: index.IndexOptionsDocs})
	emptyMin := util.NewBytesRefEmpty()
	emptyMax := util.NewBytesRefEmpty()

	if _, err := NewLucene103FieldReader(nil, fi, 0, 0, 0, 0, metaIn, indexIn, emptyMin, emptyMax); err == nil {
		t.Error("numTerms <= 0 must error")
	}
	if _, err := NewLucene103FieldReader(nil, nil, 1, 0, 0, 0, metaIn, indexIn, emptyMin, emptyMax); err == nil {
		t.Error("nil fieldInfo must error")
	}
	if _, err := NewLucene103FieldReader(nil, fi, 1, 0, 0, 0, nil, indexIn, emptyMin, emptyMax); err == nil {
		t.Error("nil metaIn must error")
	}
	if _, err := NewLucene103FieldReader(nil, fi, 1, 0, 0, 0, metaIn, nil, emptyMin, emptyMax); err == nil {
		t.Error("nil indexIn must error")
	}
	if _, err := NewLucene103FieldReader(nil, fi, 1, 0, 0, 0, metaIn, indexIn, nil, emptyMax); err == nil {
		t.Error("nil minTerm must error")
	}
	if _, err := NewLucene103FieldReader(nil, fi, 1, 0, 0, 0, metaIn, indexIn, emptyMin, nil); err == nil {
		t.Error("nil maxTerm must error")
	}
}

func TestLucene103FieldReader_StringFormat(t *testing.T) {
	dirMeta, metaIn := openMemoryInput(t, "meta", writeVLongsToBytes(t, 0, 0, 8))
	defer dirMeta.Close()
	defer metaIn.Close()
	dirIdx, indexIn := openMemoryInput(t, "index", make([]byte, 16))
	defer dirIdx.Close()
	defer indexIn.Close()

	fi := index.NewFieldInfo("f", 0, index.FieldInfoOptions{IndexOptions: index.IndexOptionsDocs})
	parent := newReaderForTest("_42")
	fr, err := NewLucene103FieldReader(parent, fi, 7, 21, 14, 5, metaIn, indexIn, util.NewBytesRefEmpty(), util.NewBytesRefEmpty())
	if err != nil {
		t.Fatalf("NewLucene103FieldReader: %v", err)
	}
	s := fr.String()
	// Matches the Java FieldReader.toString format exactly: segment label,
	// terms / postings / positions / docs counters.
	want := "BlockTreeTerms(seg=_42 terms=7,postings=14,positions=21,docs=5)"
	if !strings.Contains(s, want) {
		t.Errorf("String() want %q, got %q", want, s)
	}
}

func TestLucene103FieldReader_NewTrieReaderSlicesIndexIn(t *testing.T) {
	// Build an indexIn whose [indexStart, indexEnd) slice begins with a
	// trie leaf node Lucene103TrieReader can parse. Easiest path: write a
	// trie via TrieBuilder.Save to a real IndexOutput and then point the
	// reader at it.
	dirIdx, indexOut := openWriteOutput(t, "index_real")
	// Build a single-key trie so the read side has something to parse.
	root := BytesRefToTrie(util.NewBytesRef([]byte("hi")), NewTrieOutput(123, true, nil))
	scratchMeta := store.NewByteBuffersDataOutput()
	if err := root.Save(scratchMeta, indexOut); err != nil {
		t.Fatalf("TrieBuilder.Save: %v", err)
	}
	indexFP := indexOut.GetFilePointer()
	if err := indexOut.Close(); err != nil {
		t.Fatalf("indexOut.Close: %v", err)
	}

	metaIn := openExistingInput(t, dirIdx, "index_real")
	// Drain the three VLongs the trie just wrote into scratchMeta.
	scratchBytes := scratchMeta.ToArrayCopy()
	bin := store.NewByteArrayDataInput(scratchBytes)
	indexStart, err := store.ReadVLong(bin)
	if err != nil {
		t.Fatalf("ReadVLong indexStart: %v", err)
	}
	rootFP, err := store.ReadVLong(bin)
	if err != nil {
		t.Fatalf("ReadVLong rootFP: %v", err)
	}
	indexEnd, err := store.ReadVLong(bin)
	if err != nil {
		t.Fatalf("ReadVLong indexEnd: %v", err)
	}
	if indexEnd != indexFP {
		t.Fatalf("recorded indexEnd %d != actual file end %d", indexEnd, indexFP)
	}

	// Build a meta input that holds those three VLongs verbatim, so the
	// FieldReader constructor reads the same values back.
	dirMeta, metaInForReader := openMemoryInput(t, "meta", writeVLongsToBytes(t, indexStart, rootFP, indexEnd))
	defer dirMeta.Close()
	defer metaInForReader.Close()
	defer metaIn.Close()

	fi := index.NewFieldInfo("f", 0, index.FieldInfoOptions{IndexOptions: index.IndexOptionsDocs})
	fr, err := NewLucene103FieldReader(nil, fi, 1, 0, 0, 0, metaInForReader, metaIn, util.NewBytesRefEmpty(), util.NewBytesRefEmpty())
	if err != nil {
		t.Fatalf("NewLucene103FieldReader: %v", err)
	}
	tr, err := fr.NewTrieReader()
	if err != nil {
		t.Fatalf("NewTrieReader: %v", err)
	}
	if tr.Root() == nil {
		t.Fatal("NewTrieReader.Root() must not be nil")
	}
}

// openWriteOutput opens a fresh IndexOutput on a brand-new in-memory
// directory and returns both.
func openWriteOutput(t *testing.T, name string) (store.Directory, store.IndexOutput) {
	t.Helper()
	dir := store.NewByteBuffersDirectory()
	out, err := dir.CreateOutput(name, store.IOContext{Context: store.ContextWrite})
	if err != nil {
		t.Fatalf("CreateOutput(%s): %v", name, err)
	}
	return dir, out
}

func openExistingInput(t *testing.T, dir store.Directory, name string) store.IndexInput {
	t.Helper()
	in, err := dir.OpenInput(name, store.IOContext{Context: store.ContextRead})
	if err != nil {
		t.Fatalf("OpenInput(%s): %v", name, err)
	}
	return in
}

// newReaderForTest constructs a minimal Lucene103BlockTreeTermsReader
// suitable for tests that only need a non-nil parent pointer and a stable
// segment name. The canonical constructor requires a PostingsReaderBase
// (backlog task #2691) which is not needed for FieldReader-level checks.
func newReaderForTest(segment string) *Lucene103BlockTreeTermsReader {
	return &Lucene103BlockTreeTermsReader{segment: segment}
}
