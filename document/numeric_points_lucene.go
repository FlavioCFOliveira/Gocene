// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// This file adds Lucene 10.4.0-canonical helpers for IntPoint, LongPoint,
// FloatPoint and DoublePoint. The pre-existing pack/unpack helpers in
// point.go produce big-endian byte encodings that do NOT match Lucene's
// NumericUtils.intToSortableBytes (which flips the sign bit for correct
// unsigned-byte ordering). The new EncodeDimension*Lucene / DecodeDimension*Lucene
// helpers below match Lucene exactly.
//
// Static query factories are deferred — see backlog #2695.

// IntPointTYPE returns the FieldType for an N-dimensional IntPoint with
// numBytes=4. Mirrors Lucene's IntPoint.getType(int numDims).
func IntPointTYPE(numDims int) *FieldType {
	ft := NewFieldType()
	ft.SetIndexed(true)
	ft.SetDimensions(numDims, 4)
	ft.Freeze()
	return ft
}

// LongPointTYPE returns the FieldType for an N-dimensional LongPoint
// (numBytes=8).
func LongPointTYPE(numDims int) *FieldType {
	ft := NewFieldType()
	ft.SetIndexed(true)
	ft.SetDimensions(numDims, 8)
	ft.Freeze()
	return ft
}

// FloatPointTYPE returns the FieldType for an N-dimensional FloatPoint
// (numBytes=4).
func FloatPointTYPE(numDims int) *FieldType {
	ft := NewFieldType()
	ft.SetIndexed(true)
	ft.SetDimensions(numDims, 4)
	ft.Freeze()
	return ft
}

// DoublePointTYPE returns the FieldType for an N-dimensional DoublePoint
// (numBytes=8).
func DoublePointTYPE(numDims int) *FieldType {
	ft := NewFieldType()
	ft.SetIndexed(true)
	ft.SetDimensions(numDims, 8)
	ft.Freeze()
	return ft
}

// EncodeDimensionIntLucene writes a single int32 dimension into dest at
// offset using Lucene's sign-flipped sortable-bytes encoding.
func EncodeDimensionIntLucene(value int32, dest []byte, offset int) {
	util.IntToSortableBytes(value, dest, offset)
}

// DecodeDimensionIntLucene reads a Lucene-encoded int32 dimension from src.
func DecodeDimensionIntLucene(src []byte, offset int) int32 {
	return util.SortableBytesToInt(src, offset)
}

// EncodeDimensionLongLucene writes a single int64 dimension into dest at
// offset using Lucene's sortable-bytes encoding.
func EncodeDimensionLongLucene(value int64, dest []byte, offset int) {
	util.LongToSortableBytes(value, dest, offset)
}

// DecodeDimensionLongLucene reads a Lucene-encoded int64 dimension from src.
func DecodeDimensionLongLucene(src []byte, offset int) int64 {
	return util.SortableBytesToLong(src, offset)
}

// EncodeDimensionFloatLucene writes a single float32 dimension into dest
// at offset using Lucene's two-stage encoding (FloatToSortableInt then
// IntToSortableBytes).
func EncodeDimensionFloatLucene(value float32, dest []byte, offset int) {
	util.IntToSortableBytes(util.FloatToSortableInt(value), dest, offset)
}

// DecodeDimensionFloatLucene reads a Lucene-encoded float32 dimension.
func DecodeDimensionFloatLucene(src []byte, offset int) float32 {
	return util.SortableIntToFloat(util.SortableBytesToInt(src, offset))
}

// EncodeDimensionDoubleLucene writes a single float64 dimension using
// Lucene's two-stage encoding (DoubleToSortableLong then LongToSortableBytes).
func EncodeDimensionDoubleLucene(value float64, dest []byte, offset int) {
	util.LongToSortableBytes(util.DoubleToSortableLong(value), dest, offset)
}

// DecodeDimensionDoubleLucene reads a Lucene-encoded float64 dimension.
func DecodeDimensionDoubleLucene(src []byte, offset int) float64 {
	return util.SortableLongToDouble(util.SortableBytesToLong(src, offset))
}

// PackIntsLucene packs N int32 dimensions using Lucene's sortable-bytes
// encoding, suitable for direct use with the BKD index.
func PackIntsLucene(values ...int32) []byte {
	out := make([]byte, 4*len(values))
	for i, v := range values {
		EncodeDimensionIntLucene(v, out, i*4)
	}
	return out
}

// PackLongsLucene packs N int64 dimensions using Lucene's sortable-bytes
// encoding.
func PackLongsLucene(values ...int64) []byte {
	out := make([]byte, 8*len(values))
	for i, v := range values {
		EncodeDimensionLongLucene(v, out, i*8)
	}
	return out
}

// PackFloatsLucene packs N float32 dimensions using Lucene's sortable-bytes
// encoding.
func PackFloatsLucene(values ...float32) []byte {
	out := make([]byte, 4*len(values))
	for i, v := range values {
		EncodeDimensionFloatLucene(v, out, i*4)
	}
	return out
}

// PackDoublesLucene packs N float64 dimensions using Lucene's
// sortable-bytes encoding.
func PackDoublesLucene(values ...float64) []byte {
	out := make([]byte, 8*len(values))
	for i, v := range values {
		EncodeDimensionDoubleLucene(v, out, i*8)
	}
	return out
}

// NewIntPointLucene creates an IntPoint that uses Lucene's
// sortable-bytes encoding. Mirrors Lucene's IntPoint(String, int...).
func NewIntPointLucene(name string, point ...int32) (*Point, error) {
	if len(point) == 0 {
		return nil, fmt.Errorf("IntPoint requires at least one dimension value")
	}
	return NewPoint(name, IntPointTYPE(len(point)), PackIntsLucene(point...), len(point), 4)
}

// NewLongPointLucene creates a LongPoint with Lucene-compatible encoding.
func NewLongPointLucene(name string, point ...int64) (*Point, error) {
	if len(point) == 0 {
		return nil, fmt.Errorf("LongPoint requires at least one dimension value")
	}
	return NewPoint(name, LongPointTYPE(len(point)), PackLongsLucene(point...), len(point), 8)
}

// NewFloatPointLucene creates a FloatPoint with Lucene-compatible encoding.
func NewFloatPointLucene(name string, point ...float32) (*Point, error) {
	if len(point) == 0 {
		return nil, fmt.Errorf("FloatPoint requires at least one dimension value")
	}
	return NewPoint(name, FloatPointTYPE(len(point)), PackFloatsLucene(point...), len(point), 4)
}

// NewDoublePointLucene creates a DoublePoint with Lucene-compatible
// encoding.
func NewDoublePointLucene(name string, point ...float64) (*Point, error) {
	if len(point) == 0 {
		return nil, fmt.Errorf("DoublePoint requires at least one dimension value")
	}
	return NewPoint(name, DoublePointTYPE(len(point)), PackDoublesLucene(point...), len(point), 8)
}
