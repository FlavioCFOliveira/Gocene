// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package valuesource

import (
	"fmt"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/queries/function"
)

// ConstKnnByteVectorValueSource is a function that returns a constant byte vector value for every document.
type ConstKnnByteVectorValueSource struct {
	function.BaseValueSource
	vector []byte
}

// NewConstKnnByteVectorValueSource creates a new ConstKnnByteVectorValueSource.
func NewConstKnnByteVectorValueSource(constVector []byte) *ConstKnnByteVectorValueSource {
	return &ConstKnnByteVectorValueSource{
		vector: constVector,
	}
}

// GetValues returns the FunctionValues for the given context and reader.
func (v *ConstKnnByteVectorValueSource) GetValues(ctx function.Context, readerContext *index.LeafReaderContext) (function.FunctionValues, error) {
	return &constKnnByteVectorValues{
		vector: v.vector,
		desc:   v.Description(),
	}, nil
}

// Equals reports value-equality.
func (v *ConstKnnByteVectorValueSource) Equals(other function.ValueSource) bool {
	that, ok := other.(*ConstKnnByteVectorValueSource)
	if !ok {
		return false
	}
	if len(v.vector) != len(that.vector) {
		return false
	}
	for i := range v.vector {
		if v.vector[i] != that.vector[i] {
			return false
		}
	}
	return true
}

// HashCode returns a stable hash.
func (v *ConstKnnByteVectorValueSource) HashCode() int32 {
	var h int32 = 1
	for _, b := range v.vector {
		h = 31*h + int32(b)
	}
	return h
}

// Description renders a human-readable representation.
func (v *ConstKnnByteVectorValueSource) Description() string {
	return fmt.Sprintf("ConstKnnByteVectorValueSource(%s)", formatBytes(v.vector))
}

func formatBytes(b []byte) string {
	var sb strings.Builder
	sb.WriteByte('[')
	for i, val := range b {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(fmt.Sprintf("%d", val))
	}
	sb.WriteByte(']')
	return sb.String()
}

type constKnnByteVectorValues struct {
	function.BaseFunctionValues
	vector []byte
	desc   string
}

func (v *constKnnByteVectorValues) ByteVectorVal(_ int) ([]byte, error) {
	return v.vector, nil
}

func (v *constKnnByteVectorValues) StrVal(doc int) (string, error) {
	return formatBytes(v.vector), nil
}

func (v *constKnnByteVectorValues) ToString(doc int) (string, error) {
	s, err := v.StrVal(doc)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s=%s", v.desc, s), nil
}

// ConstKnnFloatValueSource is a function that returns a constant float vector value for every document.
type ConstKnnFloatValueSource struct {
	function.BaseValueSource
	vector []float32
}

// NewConstKnnFloatValueSource creates a new ConstKnnFloatValueSource.
func NewConstKnnFloatValueSource(constVector []float32) *ConstKnnFloatValueSource {
	return &ConstKnnFloatValueSource{
		vector: constVector,
	}
}

// GetValues returns the FunctionValues for the given context and reader.
func (v *ConstKnnFloatValueSource) GetValues(ctx function.Context, readerContext *index.LeafReaderContext) (function.FunctionValues, error) {
	return &constKnnFloatVectorValues{
		vector: v.vector,
		desc:   v.Description(),
	}, nil
}

// Equals reports value-equality.
func (v *ConstKnnFloatValueSource) Equals(other function.ValueSource) bool {
	that, ok := other.(*ConstKnnFloatValueSource)
	if !ok {
		return false
	}
	if len(v.vector) != len(that.vector) {
		return false
	}
	for i := range v.vector {
		if v.vector[i] != that.vector[i] {
			return false
		}
	}
	return true
}

// HashCode returns a stable hash.
func (v *ConstKnnFloatValueSource) HashCode() int32 {
	var h int32 = 1
	for _, f := range v.vector {
		h = 31*h + int32(f) // Simple cast for hash
	}
	return h
}

// Description renders a human-readable representation.
func (v *ConstKnnFloatValueSource) Description() string {
	return fmt.Sprintf("ConstKnnFloatValueSource(%s)", formatFloats(v.vector))
}

func formatFloats(f []float32) string {
	var sb strings.Builder
	sb.WriteByte('[')
	for i, val := range f {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(fmt.Sprintf("%g", val))
	}
	sb.WriteByte(']')
	return sb.String()
}

type constKnnFloatVectorValues struct {
	function.BaseFunctionValues
	vector []float32
	desc   string
}

func (v *constKnnFloatVectorValues) FloatVectorVal(_ int) ([]float32, error) {
	return v.vector, nil
}

func (v *constKnnFloatVectorValues) StrVal(doc int) (string, error) {
	return formatFloats(v.vector), nil
}

func (v *constKnnFloatVectorValues) ToString(doc int) (string, error) {
	s, err := v.StrVal(doc)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s=%s", v.desc, s), nil
}
