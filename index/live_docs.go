// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// This file provides an in-package, byte-faithful reader/writer for the
// Lucene 10.4.0 live-docs (.liv) file so that IndexWriter.Commit can persist
// deletions against committed segments and OpenDirectoryReader can read them
// back without taking a dependency on package codecs (which would create an
// import cycle: codecs imports index).
//
// Reference: org.apache.lucene.codecs.lucene90.Lucene90LiveDocsFormat
// (Apache Lucene 10.4.0, tag releases/lucene/10.4.0, commit 9983b7c).
//
//	Deletions (.liv) -> IndexHeader, Bits, Footer
//	  - IndexHeader  -> CodecUtil.writeIndexHeader with codec "Lucene90LiveDocs",
//	                    version 0, the 16-byte segment ID, and the del-generation
//	                    encoded as Long.toString(gen, Character.MAX_RADIX) (base 36).
//	  - Bits         -> bits2words(maxDoc) little-endian Int64 words; bit=1 means
//	                    the document is LIVE, bit=0 means it is deleted.  Ghost
//	                    bits past maxDoc in the final word are cleared.
//	  - Footer       -> CodecUtil.writeFooter.
//
// DEVIATION (documented): this is a second byte-faithful implementation of the
// same wire format already provided by codecs/live_docs_format.go
// (Lucene90LiveDocsFormat). The two are kept in lock-step. Lucene's read path
// may materialise a SparseFixedBitSet for deletion rates <= 1%, but its write
// path is always dense (Lucene90LiveDocsFormat.writeBits), so the on-disk bytes
// are identical regardless of the in-memory representation. Gocene's reader
// always returns a dense util.FixedBitSet.
//
// The header/footer envelope is produced through the package-level forwarders
// in codec_util.go (writeIndexHeader / writeFooter), which delegate to
// spi.WriteIndexHeader / spi.WriteFooter — the exact same routines used by
// codecs/live_docs_format.go — guaranteeing identical envelope bytes.

const (
	// liveDocsCodecName is the codec string stamped into the .liv IndexHeader.
	liveDocsCodecName = "Lucene90LiveDocs"

	// liveDocsVersionStart / liveDocsVersionCurrent bracket the supported
	// version range. Lucene 10.4.0 uses VERSION_START == VERSION_CURRENT == 0.
	liveDocsVersionStart   int32 = 0
	liveDocsVersionCurrent int32 = liveDocsVersionStart

	// liveDocsExtension is the file extension for the .liv file.
	liveDocsExtension = "liv"
)

// liveDocsFileName mirrors IndexFileNames.fileNameFromGeneration: a generation
// of 0 yields "<segment>.liv"; any other generation appends "_<gen in base 36>".
func liveDocsFileName(segmentName string, gen int64) string {
	return fileNameFromGenerationBase36(segmentName, liveDocsExtension, gen)
}

// fileNameFromGenerationBase36 is the Go port of
// IndexFileNames.fileNameFromGeneration restricted to the base-36 generation
// suffix used by the live-docs format.
func fileNameFromGenerationBase36(segmentName, ext string, gen int64) string {
	if gen == 0 {
		return segmentName + "." + ext
	}
	return segmentName + "_" + base36(gen) + "." + ext
}

// base36 encodes gen as a lowercase base-36 string, equivalent to
// Long.toString(gen, Character.MAX_RADIX) in Java.
func base36(gen int64) string {
	if gen == 0 {
		return "0"
	}
	neg := gen < 0
	g := uint64(gen)
	if neg {
		g = uint64(-gen)
	}
	const alphabet = "0123456789abcdefghijklmnopqrstuvwxyz"
	var buf [13]byte // a 64-bit unsigned value fits in 13 base-36 digits
	i := len(buf)
	for g > 0 {
		i--
		buf[i] = alphabet[g%36]
		g /= 36
	}
	out := string(buf[i:])
	if neg {
		out = "-" + out
	}
	return out
}

// writeLiveDocs writes the .liv file for the given segment at delGen. live is a
// FixedBitSet of length maxDoc whose set bits mark LIVE documents; ghost bits
// past maxDoc must already be cleared. Returns the on-disk deleted count
// (maxDoc - cardinality) so the caller can cross-check it against the expected
// delCount, mirroring Lucene90LiveDocsFormat.writeLiveDocs.
func writeLiveDocs(dir store.Directory, segmentName string, segmentID []byte, delGen int64, live *util.FixedBitSet) (int, error) {
	name := liveDocsFileName(segmentName, delGen)
	rawOut, err := dir.CreateOutput(name, store.IOContextWrite)
	if err != nil {
		return 0, err
	}
	out := store.NewChecksumIndexOutput(rawOut)

	if err := writeIndexHeader(out, liveDocsCodecName, liveDocsVersionCurrent, segmentID, base36(delGen)); err != nil {
		_ = out.Close()
		return 0, fmt.Errorf("live docs: header: %w", err)
	}

	delCount, err := writeLiveDocsBits(out, live)
	if err != nil {
		_ = out.Close()
		return 0, fmt.Errorf("live docs: bits: %w", err)
	}

	if err := writeFooter(out); err != nil {
		_ = out.Close()
		return 0, fmt.Errorf("live docs: footer: %w", err)
	}
	if err := out.Close(); err != nil {
		return 0, err
	}
	return delCount, nil
}

// readLiveDocs reads the .liv file for the given segment at delGen and returns
// a dense FixedBitSet of length maxDoc (1 = live). Returns (nil, nil) when the
// file does not exist (segment has no deletions at that generation).
func readLiveDocs(dir store.Directory, segmentName string, segmentID []byte, delGen int64, maxDoc int) (*util.FixedBitSet, error) {
	name := liveDocsFileName(segmentName, delGen)
	if !dir.FileExists(name) {
		return nil, nil
	}
	raw, err := dir.OpenInput(name, store.IOContextRead)
	if err != nil {
		return nil, err
	}
	defer raw.Close()
	in := store.NewChecksumIndexInput(raw)

	if _, err := checkIndexHeader(in, liveDocsCodecName, liveDocsVersionStart, liveDocsVersionCurrent, segmentID, base36(delGen)); err != nil {
		return nil, fmt.Errorf("live docs: header: %w", err)
	}

	bits, err := readLiveDocsBits(in, maxDoc)
	if err != nil {
		return nil, err
	}
	if _, err := checkFooter(in); err != nil {
		return nil, fmt.Errorf("live docs: footer: %w", err)
	}
	return bits, nil
}

// writeLiveDocsBits writes the FixedBitSet as little-endian Int64 words in
// 1024-bit batches, matching Lucene90LiveDocsFormat.writeBits. Ghost bits past
// the bitset length are cleared in the final batch. Returns the deleted count
// (length - cardinality).
func writeLiveDocsBits(out store.IndexOutput, live *util.FixedBitSet) (int, error) {
	length := live.Length()
	delCount := length

	const batchBits = 1024
	const numLongs = batchBits / 64 // 16
	for offset := 0; offset < length; offset += batchBits {
		numBitsToCopy := batchBits
		if length-offset < numBitsToCopy {
			numBitsToCopy = length - offset
		}
		// Start with every bit set, then clear ghost bits beyond numBitsToCopy.
		var words [numLongs]uint64
		for i := range words {
			words[i] = ^uint64(0)
		}
		for b := numBitsToCopy; b < batchBits; b++ {
			words[b>>6] &^= uint64(1) << uint(b&63)
		}
		// Apply the source live bits: clear positions that are not live.
		for b := 0; b < numBitsToCopy; b++ {
			if !live.Get(offset + b) {
				words[b>>6] &^= uint64(1) << uint(b&63)
			}
		}
		// Subtract this batch's live count from the running deleted total, then
		// emit exactly bits2words(numBitsToCopy) longs (the meaningful prefix).
		longCount := (numBitsToCopy + 63) / 64
		for i := 0; i < longCount; i++ {
			delCount -= popcount64(words[i])
			if err := writeInt64LE(out, words[i]); err != nil {
				return 0, err
			}
		}
	}
	return delCount, nil
}

// readLiveDocsBits reads bits2words(maxDoc) little-endian Int64 words into a
// dense FixedBitSet of length maxDoc.
func readLiveDocsBits(in store.DataInput, maxDoc int) (*util.FixedBitSet, error) {
	numLongs := (maxDoc + 63) / 64
	words := make([]uint64, numLongs)
	for i := 0; i < numLongs; i++ {
		v, err := readInt64LE(in)
		if err != nil {
			return nil, err
		}
		words[i] = v
	}
	return util.NewFixedBitSetOfBits(words, maxDoc)
}

// writeInt64LE writes a 64-bit value little-endian via WriteByte to stay
// endian-correct regardless of the IndexOutput implementation's WriteLong
// convention (Gocene has a documented BE/LE divergence across store types).
func writeInt64LE(out store.IndexOutput, v uint64) error {
	for i := 0; i < 8; i++ {
		if err := out.WriteByte(byte(v >> (8 * uint(i)))); err != nil {
			return err
		}
	}
	return nil
}

// readInt64LE reads a 64-bit little-endian value via ReadByte.
func readInt64LE(in store.DataInput) (uint64, error) {
	var v uint64
	for i := 0; i < 8; i++ {
		b, err := in.ReadByte()
		if err != nil {
			return 0, err
		}
		v |= uint64(b) << (8 * uint(i))
	}
	return v, nil
}

// popcount64 returns the number of set bits in v.
func popcount64(v uint64) int {
	v = v - ((v >> 1) & 0x5555555555555555)
	v = (v & 0x3333333333333333) + ((v >> 2) & 0x3333333333333333)
	v = (v + (v >> 4)) & 0x0F0F0F0F0F0F0F0F
	return int((v * 0x0101010101010101) >> 56)
}
