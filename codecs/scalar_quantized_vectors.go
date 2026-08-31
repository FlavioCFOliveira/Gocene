// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Source: lucene/core/src/java/org/apache/lucene/codecs/lucene104/Lucene104ScalarQuantizedVectorsFormat.java
// Purpose: per-vector optimized scalar quantization for vector storage —
// compresses float vectors to quantized byte representations, byte-for-byte
// compatible with Apache Lucene 10.4.0.
//
// This file defines the ScalarEncoding enum and shared helpers for scalar
// quantization writers.
//
// Note: Lucene104ScalarQuantizedVectorsFormat and its reader/writer are now
// located in the codecs/lucene104 package.

package codecs

import (
	"fmt"
)

// ScalarEncoding represents the encoding type for scalar quantized vectors.
// This is the Go equivalent of Lucene's
// Lucene104ScalarQuantizedVectorsFormat.ScalarEncoding enum (Lucene 10.4.0);
// the iota order matches the Java declaration order so the enum ordinals line
// up, but on the wire the encoding is identified by its wire number (see
// [ScalarEncoding.GetWireNumber]), not its ordinal.
type ScalarEncoding int

const (
	// ScalarEncodingUnsignedByte quantizes each dimension to 8 bits, treated
	// as an unsigned value. Wire number 0.
	ScalarEncodingUnsignedByte ScalarEncoding = iota
	// ScalarEncodingPackedNibble quantizes each dimension to 4 bits, packing
	// two values into each output byte. Wire number 1.
	ScalarEncodingPackedNibble
	// ScalarEncodingSevenBit quantizes each dimension to 7 bits, treated as a
	// signed value (backwards compatible with older scalar quantization). Wire
	// number 2.
	ScalarEncodingSevenBit
	// ScalarEncodingSingleBitQueryNibble quantizes each dimension to a single
	// bit; query vectors are quantized to 4 bits. Wire number 3.
	ScalarEncodingSingleBitQueryNibble
	// ScalarEncodingDibitQueryNibble quantizes each dimension to 2 bits; query
	// vectors are quantized to 4 bits. Wire number 4.
	ScalarEncodingDibitQueryNibble
)

// scalarEncodingParams maps each encoding to its (bits, bitsPerDim, queryBits,
// queryBitsPerDim). Mirrors the per-constant constructor arguments of the Java
// ScalarEncoding enum.
type scalarEncodingParams struct {
	wireNumber      int
	bits            int
	bitsPerDim      int
	queryBits       int
	queryBitsPerDim int
}

// scalarEncodingTable holds the Java enum constructor parameters in
// declaration (ordinal) order. Mirrors the Java enum constants:
//
//	UNSIGNED_BYTE(0, 8, 8)
//	PACKED_NIBBLE(1, 4, 4)
//	SEVEN_BIT(2, 7, 8)
//	SINGLE_BIT_QUERY_NIBBLE(3, 1, 1, 4, 4)
//	DIBIT_QUERY_NIBBLE(4, 2, 2, 4, 4)
var scalarEncodingTable = [...]scalarEncodingParams{
	ScalarEncodingUnsignedByte:         {wireNumber: 0, bits: 8, bitsPerDim: 8, queryBits: 8, queryBitsPerDim: 8},
	ScalarEncodingPackedNibble:         {wireNumber: 1, bits: 4, bitsPerDim: 4, queryBits: 4, queryBitsPerDim: 4},
	ScalarEncodingSevenBit:             {wireNumber: 2, bits: 7, bitsPerDim: 8, queryBits: 7, queryBitsPerDim: 8},
	ScalarEncodingSingleBitQueryNibble: {wireNumber: 3, bits: 1, bitsPerDim: 1, queryBits: 4, queryBitsPerDim: 4},
	ScalarEncodingDibitQueryNibble:     {wireNumber: 4, bits: 2, bitsPerDim: 2, queryBits: 4, queryBitsPerDim: 4},
}

// String returns the string representation of the ScalarEncoding, matching the
// Java enum constant names.
func (se ScalarEncoding) String() string {
	switch se {
	case ScalarEncodingUnsignedByte:
		return "UNSIGNED_BYTE"
	case ScalarEncodingSevenBit:
		return "SEVEN_BIT"
	case ScalarEncodingPackedNibble:
		return "PACKED_NIBBLE"
	case ScalarEncodingSingleBitQueryNibble:
		return "SINGLE_BIT_QUERY_NIBBLE"
	case ScalarEncodingDibitQueryNibble:
		return "DIBIT_QUERY_NIBBLE"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", int(se))
	}
}

// GetBits returns the number of bits used per dimension for the document-side
// quantization. Mirrors Java's ScalarEncoding.getBits().
func (se ScalarEncoding) GetBits() int {
	return scalarEncodingTable[se].bits
}

// GetWireNumber returns the number used to identify this encoding on the wire,
// independent of the enum ordinal. Mirrors Java's
// ScalarEncoding.getWireNumber().
func (se ScalarEncoding) GetWireNumber() int {
	return scalarEncodingTable[se].wireNumber
}

// GetDiscreteDimensions returns the number of dimensions rounded up so the
// per-dimension bits fit into whole bytes. Mirrors Java's
// ScalarEncoding.getDiscreteDimensions(int), including the DIBIT_QUERY_NIBBLE
// override that forces dibit packing to byte boundaries assuming single-bit
// striping.
func (se ScalarEncoding) GetDiscreteDimensions(dimensions int) int {
	p := scalarEncodingTable[se]
	if se == ScalarEncodingDibitQueryNibble {
		queryDiscretized := (dimensions*4 + 7) / 8 * 8 / 4
		docDiscretized := (dimensions + 7) / 8 * 8
		if queryDiscretized > docDiscretized {
			return queryDiscretized
		}
		return docDiscretized
	}
	if p.queryBits == p.bits {
		totalBits := dimensions * p.bitsPerDim
		return (totalBits + 7) / 8 * 8 / p.bitsPerDim
	}
	queryDiscretized := (dimensions*p.queryBitsPerDim + 7) / 8 * 8 / p.queryBitsPerDim
	docDiscretized := (dimensions*p.bitsPerDim + 7) / 8 * 8 / p.bitsPerDim
	if queryDiscretized > docDiscretized {
		return queryDiscretized
	}
	return docDiscretized
}

// GetDocPackedLength returns the number of bytes required to store a packed
// document vector of the given (raw, not yet discretized) dimensions. Mirrors
// Java's ScalarEncoding.getDocPackedLength(int), including the
// DIBIT_QUERY_NIBBLE override that stores two single-bit stripes.
func (se ScalarEncoding) GetDocPackedLength(dimensions int) int {
	p := scalarEncodingTable[se]
	discretized := se.GetDiscreteDimensions(dimensions)
	if se == ScalarEncodingDibitQueryNibble {
		// DIBIT is stored as two single-bit stripes.
		return 2 * ((discretized + 7) / 8)
	}
	totalBits := discretized * p.bitsPerDim
	return (totalBits + 7) / 8
}

// GetQueryPackedLength returns the number of bytes required to store a packed
// query vector of the given dimensions. Mirrors Java's
// ScalarEncoding.getQueryPackedLength(int).
func (se ScalarEncoding) GetQueryPackedLength(dimensions int) int {
	p := scalarEncodingTable[se]
	discretized := se.GetDiscreteDimensions(dimensions)
	totalBits := discretized * p.queryBitsPerDim
	return (totalBits + 7) / 8
}

// IsAsymmetric reports whether the document-side and query-side bit-widths
// differ. Mirrors Java's ScalarEncoding.isAsymmetric().
func (se ScalarEncoding) IsAsymmetric() bool {
	p := scalarEncodingTable[se]
	return p.bits != p.queryBits
}

// ScalarEncodingValues returns all scalar encoding values in ordinal order.
func ScalarEncodingValues() []ScalarEncoding {
	return []ScalarEncoding{
		ScalarEncodingUnsignedByte,
		ScalarEncodingPackedNibble,
		ScalarEncodingSevenBit,
		ScalarEncodingSingleBitQueryNibble,
		ScalarEncodingDibitQueryNibble,
	}
}

// ScalarEncodingFromWireNumber returns the encoding for the given wire number.
// Mirrors Java's ScalarEncoding.fromWireNumber(int).
func ScalarEncodingFromWireNumber(wireNumber int) (ScalarEncoding, error) {
	for i := range scalarEncodingTable {
		if scalarEncodingTable[i].wireNumber == wireNumber {
			return ScalarEncoding(i), nil
		}
	}
	return 0, fmt.Errorf("lucene104 sq: no ScalarEncoding for wire number %d", wireNumber)
}

// PackNibbles packs the per-dimension 4-bit values in unpacked into packed,
// striped so packed[i] = (unpacked[i] << 4) | unpacked[len(packed)+i]. Mirrors
// org.apache.lucene.codecs.lucene104.OffHeapScalarQuantizedVectorValues.packNibbles
// (Lucene 10.4.0) and is the exact inverse of the read-side unpackNibblesPacked.
func PackNibbles(unpacked, packed []byte) error {
	if len(unpacked) != len(packed)*2 {
		return fmt.Errorf("lucene104 sq: packNibbles: unpacked len %d != 2*packed len %d", len(unpacked), len(packed))
	}
	n := len(packed)
	for i := 0; i < n; i++ {
		packed[i] = byte(int(unpacked[i])<<4 | int(unpacked[n+i]))
	}
	return nil
}

// FlatDelegateFieldWriter wraps a KnnFieldVectorsWriter so that non-FLOAT32
// (BYTE) fields registered through a scalar writer still flow their values
// to the raw flat writer.
type FlatDelegateFieldWriter struct {
	Delegate KnnFieldVectorsWriter
}

// AddValue forwards the value to the flat field writer.
func (f *FlatDelegateFieldWriter) AddValue(docID int, vectorValue any) error {
	return f.Delegate.AddValue(docID, vectorValue)
}

// RamBytesUsed reports the delegate's footprint.
func (f *FlatDelegateFieldWriter) RamBytesUsed() int64 {
	return f.Delegate.RamBytesUsed()
}

// Finish marks the delegate field complete.
func (f *FlatDelegateFieldWriter) Finish() error {
	return f.Delegate.Finish()
}
