// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

// This file pins the defect that made every compressed read path in this
// package return garbage: lz4Decompress was
//
//	func lz4Decompress(data []byte, uncompressedLen int) ([]byte, error) {
//	    // Placeholder: in production, use a real LZ4 library.
//	    return data, nil
//	}
//
// It is the decompressor CompressionMode returns for CompressionModeLZ4Fast
// and CompressionModeLZ4High, so CompressingStoredFieldsReader.visit and
// the term-vectors reader in this package both handed the *compressed* bytes on
// as if they were the document payload. Apache Lucene's counterpart is the
// anonymous LZ4_DECOMPRESSOR in
// org.apache.lucene.codecs.compressing.CompressionMode, which delegates to
// org.apache.lucene.util.compress.LZ4.decompress.
//
// The evidence below is a real block written by Apache Lucene 10.4.0, not a
// block this port produced: the compressed LZ4 body of the single term-vector
// chunk in testdata/lucene-10.4.0-fixtures. Term vectors are the right witness
// because Lucene90TermVectorsFormat is declared as
//
//	super("Lucene90TermVectorsData", "", CompressionMode.FAST, 1 << 12, 128, 10);
//
// (Lucene90TermVectorsFormat.java:170), and CompressionMode.FAST's decompressor
// *is* LZ4_DECOMPRESSOR — the exact contract lz4Decompress now implements.
// (Stored fields under BEST_SPEED use LZ4WithPresetDictCompressionMode instead,
// which this package does not yet model; see the CompressionMode port gap.)

const (
	// tvdBlockStart is the offset inside _0.tvd at which the chunk's LZ4 body
	// begins: everything before it is the CodecUtil index header plus the
	// chunk header written by Lucene90CompressingTermVectorsWriter.flush
	// (docBase, chunkDocs, then flushFields/Flags/NumTerms/TermLengths/
	// TermFreqs/Positions/Offsets/PayloadLengths).
	tvdBlockStart = 409
	// tvdBlockLen is the length of that LZ4 body. It runs to exactly the start
	// of the 16-byte CodecUtil footer: 409 + 150 == 575 - 16.
	tvdBlockLen = 150
	// tvdOriginalLen is the decompressed length of the block — the concatenated
	// term suffixes of the "body" field across all 20 documents.
	tvdOriginalLen = 1030
)

// tvdOriginalText is the plaintext Apache Lucene compressed into that block.
// Per document d in 0..19 the term suffixes of the "body" field are emitted in
// sorted term order: the document's own number, then "document", "for",
// "lucene", "number", "postings", "some", "testing", "with", "ords"
// (the last two being the shared "with"/"words" prefix split).
func tvdOriginalText() []byte {
	var b bytes.Buffer
	for d := 0; d < 20; d++ {
		b.WriteString(itoa(d))
		b.WriteString("documentforlucenenumberpostingssometestingwithords")
	}
	return b.Bytes()
}

func itoa(i int) string {
	if i < 10 {
		return string(rune('0' + i))
	}
	return string(rune('0'+i/10)) + string(rune('0'+i%10))
}

// luceneFixtureBlob returns the bytes of one logical file stored inside the
// Lucene-written compound file testdata/lucene-10.4.0-fixtures/_0.cfs, located
// through its entry in _0.cfe. The .cfe layout is
// CodecUtil.writeIndexHeader + vInt(numEntries) + numEntries * (string name,
// long offset, long length) + CodecUtil.writeFooter, where the longs are
// little-endian (DataOutput.writeLong) while the header ints are big-endian
// (CodecUtil.writeBEInt).
func luceneFixtureBlob(t *testing.T, name string) []byte {
	t.Helper()
	dir := filepath.Join("..", "testdata", "lucene-10.4.0-fixtures")
	cfe, err := os.ReadFile(filepath.Join(dir, "_0.cfe"))
	if err != nil {
		t.Fatalf("read _0.cfe: %v", err)
	}
	cfs, err := os.ReadFile(filepath.Join(dir, "_0.cfs"))
	if err != nil {
		t.Fatalf("read _0.cfs: %v", err)
	}
	p := 4 // CODEC_MAGIC
	p += 1 + int(cfe[p])
	p += 4  // version
	p += 16 // object id
	p += 1 + int(cfe[p])
	numEntries := int(cfe[p]) // vInt; the fixture holds 24 entries
	p++
	for i := 0; i < numEntries; i++ {
		n := int(cfe[p])
		p++
		entry := string(cfe[p : p+n])
		p += n
		off := int64(binary.LittleEndian.Uint64(cfe[p:]))
		p += 8
		length := int64(binary.LittleEndian.Uint64(cfe[p:]))
		p += 8
		if entry == name {
			return cfs[off : off+length]
		}
	}
	t.Fatalf("no %s entry in _0.cfe", name)
	return nil
}

// TestLZ4DecompressRecoversLuceneTermVectorBlock is the regression test for the
// identity-function lz4Decompress. It fails if the stub ever returns: the stub
// hands back the 150 compressed bytes, which are neither tvdOriginalLen long
// nor the recorded plaintext.
func TestLZ4DecompressRecoversLuceneTermVectorBlock(t *testing.T) {
	tvd := luceneFixtureBlob(t, ".tvd")
	if got, want := len(tvd), 575; got != want {
		t.Fatalf("_0.tvd length = %d, want %d (fixture changed; re-derive the block constants)", got, want)
	}
	if tvdBlockStart+tvdBlockLen != len(tvd)-16 {
		t.Fatalf("block [%d,%d) does not end at the CodecUtil footer (%d)",
			tvdBlockStart, tvdBlockStart+tvdBlockLen, len(tvd)-16)
	}
	block := tvd[tvdBlockStart : tvdBlockStart+tvdBlockLen]

	got, err := lz4Decompress(block, tvdOriginalLen)
	if err != nil {
		t.Fatalf("lz4Decompress: %v", err)
	}
	if len(got) != tvdOriginalLen {
		t.Fatalf("decompressed length = %d, want %d", len(got), tvdOriginalLen)
	}
	if want := tvdOriginalText(); !bytes.Equal(got, want) {
		t.Fatalf("decompressed content mismatch:\n got %q\nwant %q", got, want)
	}
	// The defect itself: the decompressor must not be the identity.
	if bytes.Equal(got, block) {
		t.Fatal("lz4Decompress returned its input unchanged")
	}
}

// TestCompressionModeDecompressorIsLZ4 pins the wiring: both LZ4 modes must
// route to the real decompressor, mirroring CompressionMode.FAST and
// CompressionMode.FAST_DECOMPRESSION both returning LZ4_DECOMPRESSOR.
func TestCompressionModeDecompressorIsLZ4(t *testing.T) {
	tvd := luceneFixtureBlob(t, ".tvd")
	block := tvd[tvdBlockStart : tvdBlockStart+tvdBlockLen]
	want := tvdOriginalText()
	for _, mode := range []CompressionMode{CompressionModeLZ4Fast, CompressionModeLZ4High} {
		got, err := mode.decompressor()(block, tvdOriginalLen)
		if err != nil {
			t.Fatalf("mode %d: %v", mode, err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("mode %d did not recover the Lucene block", mode)
		}
	}
}

// TestLZ4DecompressRejectsTruncatedBlock pins the "lengths mismatch" guard and
// the error propagation LZ4_DECOMPRESSOR performs via CorruptIndexException:
// a truncated block must fail rather than quietly return short data.
func TestLZ4DecompressRejectsTruncatedBlock(t *testing.T) {
	tvd := luceneFixtureBlob(t, ".tvd")
	block := tvd[tvdBlockStart : tvdBlockStart+tvdBlockLen-1]
	if _, err := lz4Decompress(block, tvdOriginalLen); err == nil {
		t.Fatal("truncated block decompressed without error")
	}
}
