// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0

// Package quantization is the Go port of
// org.apache.lucene.util.quantization (Lucene 10.5.0). It hosts the
// scalar-quantized vector value abstractions consumed by the codec
// and HNSW layers.
package quantization

import (
	"errors"
	"fmt"
)

// ErrUnsupportedOperation is the sentinel error returned where the Lucene
// reference throws java.lang.UnsupportedOperationException from a method
// that declares IOException.
var ErrUnsupportedOperation = errors.New("quantization: unsupported operation")

// QuantizedByteVectorValues is the Go port of
// org.apache.lucene.util.quantization.QuantizedByteVectorValues
// (Lucene 10.5.0): scalar quantized byte vector values carrying per-vector
// optimized scalar quantization corrective terms.
//
// The abstract Java class extends [BaseQuantizedByteVectorValues]. Its
// covariant copy() override is rendered as CopyQuantizedByteVectorValues,
// following the CopyFloatVectorValues/CopyByteVectorValues convention of
// [spi.KnnVectorValues].
type QuantizedByteVectorValues interface {
	BaseQuantizedByteVectorValues

	// GetCorrectiveTerms retrieves the corrective terms for the given vector
	// ordinal. For the dot-product family of distances they are, in order,
	// the lower optimized interval, the upper optimized interval, the
	// dot-product of the non-centered vector with the centroid, and the sum
	// of quantized components. For euclidean they are the lower optimized
	// interval, the upper optimized interval, the l2norm of the centered
	// vector, and the sum of quantized components.
	GetCorrectiveTerms(vectorOrd int) (QuantizationResult, error)

	// GetQuantizer returns the quantizer used to quantize the vectors.
	GetQuantizer() *OptimizedScalarQuantizer

	// GetScalarEncoding returns the scalar encoding used to pack the stored
	// vectors.
	GetScalarEncoding() ScalarEncoding

	// GetCentroid returns the centroid used to center the vectors prior to
	// quantization.
	GetCentroid() ([]float32, error)

	// GetCentroidDP returns the dot product of the centroid.
	GetCentroidDP() (float32, error)

	// CopyQuantizedByteVectorValues is the covariant copy() override.
	CopyQuantizedByteVectorValues() (QuantizedByteVectorValues, error)
}

// ScalarEncoding is the Go port of the nested enum
// org.apache.lucene.util.quantization.QuantizedByteVectorValues.ScalarEncoding
// (Lucene 10.5.0): the allowed encodings for scalar quantization. It
// specifies how many bits are used per dimension and dictates packing of
// dimensions into a byte stream.
//
// The iota order is the Java declaration (ordinal) order. On the wire an
// encoding is identified by its wire number (see [ScalarEncoding.GetWireNumber]),
// not by its ordinal.
type ScalarEncoding int

const (
	// ScalarEncodingUnsignedByte is UNSIGNED_BYTE(0, 8, 8): each dimension is
	// quantized to 8 bits and treated as an unsigned value.
	ScalarEncodingUnsignedByte ScalarEncoding = iota
	// ScalarEncodingPackedNibble is PACKED_NIBBLE(1, 4, 4): each dimension is
	// quantized to 4 bits and two values are packed into each output byte.
	ScalarEncodingPackedNibble
	// ScalarEncodingSevenBit is SEVEN_BIT(2, 7, 8): each dimension is
	// quantized to 7 bits and treated as a signed value.
	ScalarEncodingSevenBit
	// ScalarEncodingSingleBitQueryNibble is SINGLE_BIT_QUERY_NIBBLE(3, 1, 1, 4, 4):
	// each dimension is quantized to a single bit and packed into bytes; the
	// query vector is quantized to 4 bits per dimension.
	ScalarEncodingSingleBitQueryNibble
	// ScalarEncodingDibitQueryNibble is DIBIT_QUERY_NIBBLE(4, 2, 2, 4, 4):
	// each dimension is quantized to 2 bits and packed into bytes; the query
	// vector is quantized to 4 bits per dimension.
	ScalarEncodingDibitQueryNibble
)

// scalarEncodingParams holds the constructor arguments of one ScalarEncoding
// constant: (wireNumber, bits, bitsPerDim) or
// (wireNumber, bits, bitsPerDim, queryBits, queryBitsPerDim).
type scalarEncodingParams struct {
	wireNumber      int
	bits            byte
	queryBits       byte
	bitsPerDim      int
	queryBitsPerDim int
}

// scalarEncodingTable holds the enum constructor arguments in declaration
// (ordinal) order.
var scalarEncodingTable = [...]scalarEncodingParams{
	ScalarEncodingUnsignedByte:         {wireNumber: 0, bits: 8, queryBits: 8, bitsPerDim: 8, queryBitsPerDim: 8},
	ScalarEncodingPackedNibble:         {wireNumber: 1, bits: 4, queryBits: 4, bitsPerDim: 4, queryBitsPerDim: 4},
	ScalarEncodingSevenBit:             {wireNumber: 2, bits: 7, queryBits: 7, bitsPerDim: 8, queryBitsPerDim: 8},
	ScalarEncodingSingleBitQueryNibble: {wireNumber: 3, bits: 1, queryBits: 4, bitsPerDim: 1, queryBitsPerDim: 4},
	ScalarEncodingDibitQueryNibble:     {wireNumber: 4, bits: 2, queryBits: 4, bitsPerDim: 2, queryBitsPerDim: 4},
}

// ScalarEncodingValues returns the constants in declaration order, as
// ScalarEncoding.values() does.
func ScalarEncodingValues() []ScalarEncoding {
	return []ScalarEncoding{
		ScalarEncodingUnsignedByte,
		ScalarEncodingPackedNibble,
		ScalarEncodingSevenBit,
		ScalarEncodingSingleBitQueryNibble,
		ScalarEncodingDibitQueryNibble,
	}
}

// String returns the enum constant name, as name() and the inherited
// toString() do.
func (se ScalarEncoding) String() string {
	switch se {
	case ScalarEncodingUnsignedByte:
		return "UNSIGNED_BYTE"
	case ScalarEncodingPackedNibble:
		return "PACKED_NIBBLE"
	case ScalarEncodingSevenBit:
		return "SEVEN_BIT"
	case ScalarEncodingSingleBitQueryNibble:
		return "SINGLE_BIT_QUERY_NIBBLE"
	case ScalarEncodingDibitQueryNibble:
		return "DIBIT_QUERY_NIBBLE"
	default:
		return fmt.Sprintf("ScalarEncoding(%d)", int(se))
	}
}

// ScalarEncodingFromNumBits mirrors the static ScalarEncoding.fromNumBits(int):
// the first constant, in declaration order, whose bits equal bits. Where Java
// throws IllegalArgumentException the error carries the same message.
func ScalarEncodingFromNumBits(bits int) (ScalarEncoding, error) {
	for _, encoding := range ScalarEncodingValues() {
		if int(scalarEncodingTable[encoding].bits) == bits {
			return encoding, nil
		}
	}
	return 0, fmt.Errorf("No encoding for %d bits", bits)
}

// IsAsymmetric mirrors isAsymmetric(): bits != queryBits.
func (se ScalarEncoding) IsAsymmetric() bool {
	p := scalarEncodingTable[se]
	return p.bits != p.queryBits
}

// GetWireNumber mirrors getWireNumber(): the number used to identify this
// encoding on the wire, rather than relying on ordinal.
func (se ScalarEncoding) GetWireNumber() int {
	return scalarEncodingTable[se].wireNumber
}

// GetBits mirrors getBits(): the number of bits used per dimension.
func (se ScalarEncoding) GetBits() byte {
	return scalarEncodingTable[se].bits
}

// GetQueryBits mirrors getQueryBits().
func (se ScalarEncoding) GetQueryBits() byte {
	return scalarEncodingTable[se].queryBits
}

// GetDiscreteDimensions mirrors getDiscreteDimensions(int): the number of
// dimensions rounded up to fit into whole bytes. DIBIT_QUERY_NIBBLE overrides
// the method to force dibit packing to byte boundaries assuming single bit
// striping.
func (se ScalarEncoding) GetDiscreteDimensions(dimensions int) int {
	if se == ScalarEncodingDibitQueryNibble {
		queryDiscretized := (dimensions*4 + 7) / 8 * 8 / 4
		docDiscretized := (dimensions + 7) / 8 * 8
		return max(queryDiscretized, docDiscretized)
	}
	p := scalarEncodingTable[se]
	if p.queryBits == p.bits {
		totalBits := dimensions * p.bitsPerDim
		return (totalBits + 7) / 8 * 8 / p.bitsPerDim
	}
	queryDiscretized := (dimensions*p.queryBitsPerDim + 7) / 8 * 8 / p.queryBitsPerDim
	docDiscretized := (dimensions*p.bitsPerDim + 7) / 8 * 8 / p.bitsPerDim
	return max(queryDiscretized, docDiscretized)
}

// GetDocBitsPerDim mirrors getDocBitsPerDim().
func (se ScalarEncoding) GetDocBitsPerDim() int {
	return scalarEncodingTable[se].bitsPerDim
}

// GetQueryBitsPerDim mirrors getQueryBitsPerDim().
func (se ScalarEncoding) GetQueryBitsPerDim() int {
	return scalarEncodingTable[se].queryBitsPerDim
}

// GetDocPackedLength mirrors getDocPackedLength(int): the number of bytes
// required to store a packed vector of the given dimensions.
// DIBIT_QUERY_NIBBLE overrides the method: it is stored as two single bits
// striped.
func (se ScalarEncoding) GetDocPackedLength(dimensions int) int {
	discretized := se.GetDiscreteDimensions(dimensions)
	if se == ScalarEncodingDibitQueryNibble {
		return 2 * ((discretized + 7) / 8)
	}
	totalBits := discretized * scalarEncodingTable[se].bitsPerDim
	return (totalBits + 7) / 8
}

// GetQueryPackedLength mirrors getQueryPackedLength(int).
func (se ScalarEncoding) GetQueryPackedLength(dimensions int) int {
	discretized := se.GetDiscreteDimensions(dimensions)
	totalBits := discretized * scalarEncodingTable[se].queryBitsPerDim
	return (totalBits + 7) / 8
}

// ScalarEncodingFromWireNumber mirrors the static
// ScalarEncoding.fromWireNumber(int): the encoding for the given wire number,
// with false standing for Java's Optional.empty().
func ScalarEncodingFromWireNumber(wireNumber int) (ScalarEncoding, bool) {
	for _, encoding := range ScalarEncodingValues() {
		if scalarEncodingTable[encoding].wireNumber == wireNumber {
			return encoding, true
		}
	}
	return 0, false
}
