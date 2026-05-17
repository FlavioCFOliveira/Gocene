// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"encoding/binary"
	"fmt"
	"math"
)

// LateInteractionField stores a multi-vector (matrix) per document for
// late-interaction retrieval models (e.g. ColBERT).
//
// Go port of Lucene 10.4.0's
// org.apache.lucene.document.LateInteractionField. Each document holds a
// 2-D float32 matrix where all rows share the same dimensionality. The
// payload is encoded with the per-vector dimensionality in the first 4
// bytes (little-endian uint32) followed by the row-major float32s.
type LateInteractionField struct {
	*BinaryDocValuesField
	value [][]float32
}

// NewLateInteractionField creates a new LateInteractionField from a
// multi-vector matrix. All rows must be non-empty and share the same
// length.
func NewLateInteractionField(name string, value [][]float32) (*LateInteractionField, error) {
	encoded, err := EncodeLateInteraction(value)
	if err != nil {
		return nil, err
	}
	b, err := NewBinaryDocValuesField(name, encoded)
	if err != nil {
		return nil, err
	}
	dup := cloneFloat32Matrix(value)
	return &LateInteractionField{BinaryDocValuesField: b, value: dup}, nil
}

// GetValue returns a defensive copy of the stored multi-vector matrix.
func (f *LateInteractionField) GetValue() [][]float32 {
	return cloneFloat32Matrix(f.value)
}

// SetValue replaces the stored matrix.
func (f *LateInteractionField) SetValue(value [][]float32) error {
	encoded, err := EncodeLateInteraction(value)
	if err != nil {
		return err
	}
	f.BinaryDocValuesField.Field.SetBytesValue(encoded)
	f.value = cloneFloat32Matrix(value)
	return nil
}

// EncodeLateInteraction encodes a [][]float32 into the Lucene payload
// format: first 4 bytes = per-vector dim (little-endian uint32), then
// row-major float32 little-endian payload.
func EncodeLateInteraction(value [][]float32) ([]byte, error) {
	if len(value) == 0 {
		return nil, fmt.Errorf("LateInteractionField requires at least one vector")
	}
	dim := len(value[0])
	if dim == 0 {
		return nil, fmt.Errorf("vector 0 is empty")
	}
	for i, v := range value {
		if len(v) != dim {
			return nil, fmt.Errorf("vector %d has dimension %d, expected %d", i, len(v), dim)
		}
	}
	out := make([]byte, 4+len(value)*dim*4)
	binary.LittleEndian.PutUint32(out[:4], uint32(dim))
	off := 4
	for _, v := range value {
		for _, f := range v {
			binary.LittleEndian.PutUint32(out[off:], math.Float32bits(f))
			off += 4
		}
	}
	return out, nil
}

// DecodeLateInteraction decodes a payload back to a [][]float32.
func DecodeLateInteraction(payload []byte) ([][]float32, error) {
	if len(payload) < 4 {
		return nil, fmt.Errorf("payload too short")
	}
	dim := int(binary.LittleEndian.Uint32(payload[:4]))
	if dim == 0 {
		return nil, fmt.Errorf("payload declares zero-dim vectors")
	}
	body := payload[4:]
	if len(body)%(dim*4) != 0 {
		return nil, fmt.Errorf("payload body length %d not a multiple of dim*4 (%d)", len(body), dim*4)
	}
	rows := len(body) / (dim * 4)
	out := make([][]float32, rows)
	off := 0
	for i := 0; i < rows; i++ {
		row := make([]float32, dim)
		for j := 0; j < dim; j++ {
			row[j] = math.Float32frombits(binary.LittleEndian.Uint32(body[off:]))
			off += 4
		}
		out[i] = row
	}
	return out, nil
}

func cloneFloat32Matrix(in [][]float32) [][]float32 {
	out := make([][]float32, len(in))
	for i, row := range in {
		r := make([]float32, len(row))
		copy(r, row)
		out[i] = r
	}
	return out
}
