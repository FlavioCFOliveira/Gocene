// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"encoding/binary"
	"fmt"
	"math"
	"strconv"
)

// PayloadHelper provides utility methods for encoding payload values.
//
// This is the Go port of org.apache.lucene.analysis.payloads.PayloadHelper
// from Apache Lucene 10.4.0.
//
// All multi-byte values are encoded in big-endian byte order to match
// Lucene's reference implementation (which uses BitUtil.VH_BE_FLOAT and
// BitUtil.VH_BE_INT). This preserves byte-for-byte wire compatibility
// with Lucene-produced payload data.
//
// The Lucene reference exposes these as static methods on a class with
// no fields. In Go they are package-level functions on the unexported
// payloadHelper namespace; the public entry points are
// EncodeFloatPayload, EncodeIntPayload, DecodeFloatPayload, and
// DecodeIntPayload to avoid collisions with similarly-named codec
// helpers elsewhere in the module.
type PayloadHelper struct{}

// EncodeFloatPayload encodes a float32 into a 4-byte big-endian slice.
//
// This is the Go port of PayloadHelper.encodeFloat(float).
func EncodeFloatPayload(payload float32) []byte {
	return EncodeFloatPayloadInto(payload, make([]byte, 4), 0)
}

// EncodeFloatPayloadInto writes the float32 to data at offset and returns
// the data slice for chaining. The caller is responsible for ensuring
// data has at least offset+4 bytes of capacity.
//
// This is the Go port of PayloadHelper.encodeFloat(float, byte[], int).
func EncodeFloatPayloadInto(payload float32, data []byte, offset int) []byte {
	binary.BigEndian.PutUint32(data[offset:offset+4], math.Float32bits(payload))
	return data
}

// EncodeIntPayload encodes an int32 into a 4-byte big-endian slice.
//
// This is the Go port of PayloadHelper.encodeInt(int).
func EncodeIntPayload(payload int32) []byte {
	return EncodeIntPayloadInto(payload, make([]byte, 4), 0)
}

// EncodeIntPayloadInto writes the int32 to data at offset and returns
// the data slice for chaining. The caller is responsible for ensuring
// data has at least offset+4 bytes of capacity.
//
// This is the Go port of PayloadHelper.encodeInt(int, byte[], int).
func EncodeIntPayloadInto(payload int32, data []byte, offset int) []byte {
	binary.BigEndian.PutUint32(data[offset:offset+4], uint32(payload))
	return data
}

// DecodeFloatPayload decodes a float32 from the first 4 bytes of bytes.
//
// This is the Go port of PayloadHelper.decodeFloat(byte[]).
func DecodeFloatPayload(bytes []byte) float32 {
	return DecodeFloatPayloadAt(bytes, 0)
}

// DecodeFloatPayloadAt decodes a float32 from bytes starting at offset.
//
// This is the Go port of PayloadHelper.decodeFloat(byte[], int).
func DecodeFloatPayloadAt(bytes []byte, offset int) float32 {
	return math.Float32frombits(binary.BigEndian.Uint32(bytes[offset : offset+4]))
}

// DecodeIntPayload decodes an int32 from bytes starting at offset.
//
// This is the Go port of PayloadHelper.decodeInt(byte[], int).
func DecodeIntPayload(bytes []byte, offset int) int32 {
	return int32(binary.BigEndian.Uint32(bytes[offset : offset+4]))
}

// PayloadEncoder converts a UTF-8 byte slice (Lucene's char[] in Java)
// into a payload byte slice.
//
// This is the Go port of the
// org.apache.lucene.analysis.payloads.PayloadEncoder interface.
//
// Deviation from Lucene: the reference interface takes a char[] of
// UTF-16 code units; Gocene's CharTermAttribute exposes its buffer as
// UTF-8 bytes (per Sprint 12 decisions), so the Encode methods consume
// []byte slices directly. This matches Gocene's pipeline convention
// and avoids a UTF-16/UTF-8 round trip on every token.
type PayloadEncoder interface {
	// Encode encodes the entire buffer.
	Encode(buffer []byte) []byte

	// EncodeSlice encodes buffer[offset:offset+length].
	EncodeSlice(buffer []byte, offset, length int) []byte
}

// AbstractPayloadEncoder is the base struct embedded by encoders that
// only override EncodeSlice. It provides the Encode convenience method
// that delegates to EncodeSlice over the full buffer.
//
// This is the Go port of
// org.apache.lucene.analysis.payloads.AbstractEncoder. The Java class
// is a one-method abstract; in Go we model it as a small embeddable
// struct + helper so concrete encoders compose rather than inherit.
type AbstractPayloadEncoder struct {
	// EncodeSliceFunc is set by concrete encoders to the implementation
	// of EncodeSlice. Encode invokes it through the helper to remain
	// faithful to the Lucene "convenience-method-on-base-class"
	// pattern.
	EncodeSliceFunc func(buffer []byte, offset, length int) []byte
}

// Encode encodes the entire buffer by delegating to EncodeSliceFunc.
func (a *AbstractPayloadEncoder) Encode(buffer []byte) []byte {
	return a.EncodeSliceFunc(buffer, 0, len(buffer))
}

// IdentityPayloadEncoder copies the input buffer verbatim into a new
// payload slice. The Lucene reference re-encodes the char[] to UTF-8
// (or a caller-specified charset); since Gocene's buffer is already
// UTF-8, the identity transform is a copy.
//
// This is the Go port of
// org.apache.lucene.analysis.payloads.IdentityEncoder.
//
// Deviation from Lucene: the reference accepts an arbitrary
// java.nio.charset.Charset. Gocene fixes the encoding to UTF-8 since
// the source buffer is already UTF-8 and Go's standard library has no
// equivalent runtime-pluggable charset registry. If non-UTF-8 output
// is ever required, callers should convert the buffer themselves
// before invoking Encode.
type IdentityPayloadEncoder struct {
	AbstractPayloadEncoder
}

// NewIdentityPayloadEncoder returns a fresh identity encoder.
func NewIdentityPayloadEncoder() *IdentityPayloadEncoder {
	e := &IdentityPayloadEncoder{}
	e.EncodeSliceFunc = e.encodeSlice
	return e
}

// EncodeSlice copies buffer[offset:offset+length] into a fresh slice.
func (e *IdentityPayloadEncoder) EncodeSlice(buffer []byte, offset, length int) []byte {
	return e.encodeSlice(buffer, offset, length)
}

func (e *IdentityPayloadEncoder) encodeSlice(buffer []byte, offset, length int) []byte {
	out := make([]byte, length)
	copy(out, buffer[offset:offset+length])
	return out
}

// FloatPayloadEncoder parses buffer as a decimal floating-point number
// and encodes it via EncodeFloatPayload.
//
// This is the Go port of
// org.apache.lucene.analysis.payloads.FloatEncoder.
type FloatPayloadEncoder struct {
	AbstractPayloadEncoder
}

// NewFloatPayloadEncoder returns a fresh float encoder.
func NewFloatPayloadEncoder() *FloatPayloadEncoder {
	e := &FloatPayloadEncoder{}
	e.EncodeSliceFunc = e.encodeSlice
	return e
}

// EncodeSlice parses buffer[offset:offset+length] as a float32 and
// encodes it. Invalid input is encoded as +0.0 (matching no exception
// is propagated; Lucene's Float.parseFloat throws but this filter
// chain has no error channel, so the convention here is to use 0.0 on
// parse failure to preserve the no-error TokenFilter contract).
func (e *FloatPayloadEncoder) EncodeSlice(buffer []byte, offset, length int) []byte {
	return e.encodeSlice(buffer, offset, length)
}

func (e *FloatPayloadEncoder) encodeSlice(buffer []byte, offset, length int) []byte {
	s := string(buffer[offset : offset+length])
	v, err := strconv.ParseFloat(s, 32)
	if err != nil {
		v = 0
	}
	return EncodeFloatPayload(float32(v))
}

// IntegerPayloadEncoder parses buffer as a decimal integer and encodes
// it via EncodeIntPayload.
//
// This is the Go port of
// org.apache.lucene.analysis.payloads.IntegerEncoder.
type IntegerPayloadEncoder struct {
	AbstractPayloadEncoder
}

// NewIntegerPayloadEncoder returns a fresh integer encoder.
func NewIntegerPayloadEncoder() *IntegerPayloadEncoder {
	e := &IntegerPayloadEncoder{}
	e.EncodeSliceFunc = e.encodeSlice
	return e
}

// EncodeSlice parses buffer[offset:offset+length] as an int32 and
// encodes it. Invalid input is encoded as 0 for the same reason
// documented on FloatPayloadEncoder.EncodeSlice.
func (e *IntegerPayloadEncoder) EncodeSlice(buffer []byte, offset, length int) []byte {
	return e.encodeSlice(buffer, offset, length)
}

func (e *IntegerPayloadEncoder) encodeSlice(buffer []byte, offset, length int) []byte {
	s := string(buffer[offset : offset+length])
	v, err := strconv.ParseInt(s, 10, 32)
	if err != nil {
		v = 0
	}
	return EncodeIntPayloadInto(int32(v), make([]byte, 4), 0)
}

// Ensure the encoders implement the PayloadEncoder contract.
var (
	_ PayloadEncoder = (*IdentityPayloadEncoder)(nil)
	_ PayloadEncoder = (*FloatPayloadEncoder)(nil)
	_ PayloadEncoder = (*IntegerPayloadEncoder)(nil)
)

// payloadEncoderError is returned by encoder constructors when the
// caller supplies an invalid configuration. It is currently unused
// (all encoders take no arguments) but kept for symmetry with the
// rest of the package.
func payloadEncoderError(msg string) error {
	return fmt.Errorf("payload encoder: %s", msg)
}
