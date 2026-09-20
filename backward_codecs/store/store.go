// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package store implements the endianness-reversing I/O wrappers from
// org.apache.lucene.backward_codecs.store (Lucene 10.4.0).
//
// Old Lucene codecs (up to ~Lucene 8.x) persisted multi-byte values
// (short, int, long) in big-endian order.  Current Lucene/Gocene uses
// little-endian.  Reading those legacy segments therefore requires
// byte-swapping every multi-byte integer on the fly; that is what every
// type in this package does.
package store

import (
	"fmt"
	"math"
	"math/bits"

	"github.com/FlavioCFOliveira/Gocene/spi"
	gstore "github.com/FlavioCFOliveira/Gocene/store"
)

// ─────────────────────────────────────────────────────────────────────────────
// EndiannessReverserDataInput
// ─────────────────────────────────────────────────────────────────────────────

// EndiannessReverserDataInput wraps a DataInput and byte-swaps every
// multi-byte integer read.  Single bytes and raw byte slices are passed
// through unchanged.
//
// Port of org.apache.lucene.backward_codecs.store.EndiannessReverserDataInput
// (Lucene 10.4.0, package-private).
type EndiannessReverserDataInput struct {
	In gstore.DataInput
}

// NewEndiannessReverserDataInput wraps in so that short/int/long reads are
// byte-swapped.
func NewEndiannessReverserDataInput(in gstore.DataInput) *EndiannessReverserDataInput {
	return &EndiannessReverserDataInput{In: in}
}

func (r *EndiannessReverserDataInput) ReadInts(dst []int32, offset, length int) error {
	if offset+length > len(dst) {
		return fmt.Errorf("index out of bounds")
	}
	for i := 0; i < length; i++ {
		v, err := r.ReadInt()
		if err != nil {
			return err
		}
		dst[offset+i] = v
	}
	return nil
}

func (r *EndiannessReverserDataInput) ReadMapOfStrings() (map[string]string, error) {
	return r.In.ReadMapOfStrings()
}

func (r *EndiannessReverserDataInput) ReadSetOfStrings() ([]string, error) {
	return r.In.ReadSetOfStrings()
}

func (r *EndiannessReverserDataInput) SkipBytes(numBytes int64) error {
	return r.In.SkipBytes(numBytes)
}

func (r *EndiannessReverserDataInput) ReadVInt() (int32, error) {
	return r.In.ReadVInt()
}

func (r *EndiannessReverserDataInput) ReadZInt() (int32, error) {
	return r.In.ReadZInt()
}

func (r *EndiannessReverserDataInput) ReadVLong() (int64, error) {
	return r.In.ReadVLong()
}

func (r *EndiannessReverserDataInput) ReadZLong() (int64, error) {
	return r.In.ReadZLong()
}

func (r *EndiannessReverserDataInput) ReadLongs(dst []int64, offset, length int) error {
	if offset+length > len(dst) {
		return fmt.Errorf("index out of bounds")
	}
	for i := 0; i < length; i++ {
		v, err := r.ReadLong()
		if err != nil {
			return err
		}
		dst[offset+i] = v
	}
	return nil
}

func (r *EndiannessReverserDataInput) ReadByte() (byte, error) { return r.In.ReadByte() }

func (r *EndiannessReverserDataInput) ReadBytes(b []byte, offset, length int) error {
	return r.In.ReadBytes(b, offset, length)
}

func (r *EndiannessReverserDataInput) ReadBytesN(n int) ([]byte, error) {
	out := make([]byte, n)
	if err := r.In.ReadBytes(out, 0, n); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *EndiannessReverserDataInput) ReadShort() (int16, error) {
	v, err := r.In.ReadShort()
	return int16(bits.ReverseBytes16(uint16(v))), err
}

func (r *EndiannessReverserDataInput) ReadInt() (int32, error) {
	v, err := r.In.ReadInt()
	return int32(bits.ReverseBytes32(uint32(v))), err
}

func (r *EndiannessReverserDataInput) ReadLong() (int64, error) {
	v, err := r.In.ReadLong()
	return int64(bits.ReverseBytes64(uint64(v))), err
}

func (r *EndiannessReverserDataInput) ReadFloats(dst []float32, offset, length int) error {
	return r.In.ReadFloats(dst, offset, length)
}

func (r *EndiannessReverserDataInput) ReadString() (string, error) {
	return r.In.ReadString()
}

var _ gstore.DataInput = (*EndiannessReverserDataInput)(nil)

// ─────────────────────────────────────────────────────────────────────────────
// EndiannessReverserDataOutput
// ─────────────────────────────────────────────────────────────────────────────

// EndiannessReverserDataOutput wraps a DataOutput and byte-swaps every
// multi-byte integer written.  Single bytes, raw byte slices, and strings
// are forwarded unchanged.
//
// Port of org.apache.lucene.backward_codecs.store.EndiannessReverserDataOutput
// (Lucene 10.4.0, package-private).
type EndiannessReverserDataOutput struct {
	Out gstore.DataOutput
}

// NewEndiannessReverserDataOutput wraps out so that short/int/long writes are
// byte-swapped.
func NewEndiannessReverserDataOutput(out gstore.DataOutput) *EndiannessReverserDataOutput {
	return &EndiannessReverserDataOutput{Out: out}
}

func (w *EndiannessReverserDataOutput) WriteByte(b byte) error { return w.Out.WriteByte(b) }

func (w *EndiannessReverserDataOutput) WriteBytes(b []byte, offset, length int) error {
	return w.Out.WriteBytes(b, offset, length)
}

func (w *EndiannessReverserDataOutput) WriteBytesN(b []byte, n int) error {
	return w.Out.WriteBytesN(b, n)
}

func (w *EndiannessReverserDataOutput) CopyBytes(input gstore.DataInput, numBytes int64) error {
	return w.Out.CopyBytes(input, numBytes)
}

func (w *EndiannessReverserDataOutput) WriteShort(v int16) error {
	return w.Out.WriteShort(int16(bits.ReverseBytes16(uint16(v))))
}

func (w *EndiannessReverserDataOutput) WriteInt(v int32) error {
	return w.Out.WriteInt(int32(bits.ReverseBytes32(uint32(v))))
}

func (w *EndiannessReverserDataOutput) WriteLong(v int64) error {
	return w.Out.WriteLong(int64(bits.ReverseBytes64(uint64(v))))
}

func (w *EndiannessReverserDataOutput) WriteVInt(v int32) error {
	return w.Out.WriteVInt(v)
}

func (w *EndiannessReverserDataOutput) WriteZInt(v int32) error {
	return w.Out.WriteZInt(v)
}

func (w *EndiannessReverserDataOutput) WriteVLong(v int64) error {
	return w.Out.WriteVLong(v)
}

func (w *EndiannessReverserDataOutput) WriteZLong(v int64) error {
	return w.Out.WriteZLong(v)
}

func (w *EndiannessReverserDataOutput) WriteGroupVInts(values []int32, limit int) error {
	return w.Out.WriteGroupVInts(values, limit)
}

func (w *EndiannessReverserDataOutput) WriteMapOfStrings(m map[string]string) error {
	return w.Out.WriteMapOfStrings(m)
}

func (w *EndiannessReverserDataOutput) WriteSetOfStrings(s []string) error {
	return w.Out.WriteSetOfStrings(s)
}

func (w *EndiannessReverserDataOutput) WriteString(s string) error { return w.Out.WriteString(s) }

var _ gstore.DataOutput = (*EndiannessReverserDataOutput)(nil)

// ─────────────────────────────────────────────────────────────────────────────
// EndiannessReverserIndexInput
// ─────────────────────────────────────────────────────────────────────────────

// EndiannessReverserIndexInput wraps an IndexInput and byte-swaps every
// multi-byte integer read.
//
// Port of org.apache.lucene.backward_codecs.store.EndiannessReverserIndexInput
// (Lucene 10.4.0, package-private).  The Java version extends FilterIndexInput;
// here we embed *gstore.FilterIndexInput and shadow the read methods.
type EndiannessReverserIndexInput struct {
	*gstore.FilterIndexInput
}

// NewEndiannessReverserIndexInput wraps in so that short/int/long reads are
// byte-swapped.
func NewEndiannessReverserIndexInput(in gstore.IndexInput) *EndiannessReverserIndexInput {
	return &EndiannessReverserIndexInput{
		FilterIndexInput: gstore.NewFilterIndexInput("EndiannessReverserIndexInput", in),
	}
}

func (r *EndiannessReverserIndexInput) ReadShort() (int16, error) {
	v, err := r.FilterIndexInput.ReadShort()
	return int16(bits.ReverseBytes16(uint16(v))), err
}

func (r *EndiannessReverserIndexInput) ReadInt() (int32, error) {
	v, err := r.FilterIndexInput.ReadInt()
	return int32(bits.ReverseBytes32(uint32(v))), err
}

func (r *EndiannessReverserIndexInput) ReadLong() (int64, error) {
	v, err := r.FilterIndexInput.ReadLong()
	return int64(bits.ReverseBytes64(uint64(v))), err
}

// Clone wraps the cloned inner input in a fresh EndiannessReverserIndexInput.
func (r *EndiannessReverserIndexInput) Clone() gstore.IndexInput {
	inner := r.FilterIndexInput.GetDelegate().Clone()
	if inner == nil {
		return nil
	}
	return NewEndiannessReverserIndexInput(inner)
}

// Slice wraps the sliced inner input in a fresh EndiannessReverserIndexInput.
func (r *EndiannessReverserIndexInput) Slice(desc string, offset, length int64) (gstore.IndexInput, error) {
	inner, err := r.FilterIndexInput.GetDelegate().Slice(desc, offset, length)
	if err != nil {
		return nil, err
	}
	return NewEndiannessReverserIndexInput(inner), nil
}

var _ gstore.IndexInput = (*EndiannessReverserIndexInput)(nil)

// ─────────────────────────────────────────────────────────────────────────────
// EndiannessReverserIndexOutput
// ─────────────────────────────────────────────────────────────────────────────

// EndiannessReverserIndexOutput wraps an IndexOutput and byte-swaps every
// multi-byte integer written.
//
// Port of org.apache.lucene.backward_codecs.store.EndiannessReverserIndexOutput
// (Lucene 10.4.0, package-private).
type EndiannessReverserIndexOutput struct {
	*gstore.FilterIndexOutput
}

// NewEndiannessReverserIndexOutput wraps out so that short/int/long writes are
// byte-swapped.
func NewEndiannessReverserIndexOutput(out gstore.IndexOutput) *EndiannessReverserIndexOutput {
	return &EndiannessReverserIndexOutput{
		FilterIndexOutput: gstore.NewFilterIndexOutput("EndiannessReverserIndexOutput", out.GetName(), out),
	}
}

func (w *EndiannessReverserIndexOutput) WriteShort(v int16) error {
	return w.FilterIndexOutput.WriteShort(int16(bits.ReverseBytes16(uint16(v))))
}

func (w *EndiannessReverserIndexOutput) WriteInt(v int32) error {
	return w.FilterIndexOutput.WriteInt(int32(bits.ReverseBytes32(uint32(v))))
}

func (w *EndiannessReverserIndexOutput) WriteLong(v int64) error {
	return w.FilterIndexOutput.WriteLong(int64(bits.ReverseBytes64(uint64(v))))
}

// GetChecksum delegates to the wrapped output if it exposes a checksum, or
// panics.  This satisfies the codecs.checksumWriter contract so that
// store.WriteFooter can record the running CRC32 into the file.
func (w *EndiannessReverserIndexOutput) GetChecksum() uint32 {
	type checksummer interface{ GetChecksum() uint32 }
	if cw, ok := w.FilterIndexOutput.GetDelegate().(checksummer); ok {
		return cw.GetChecksum()
	}
	// The underlying output does not track a checksum; callers must ensure they
	// only invoke GetChecksum() on outputs that were opened via a checksum-aware
	// factory (e.g. a ChecksumIndexOutput delegate).
	panic("EndiannessReverserIndexOutput: delegate does not support GetChecksum()")
}

var _ gstore.IndexOutput = (*EndiannessReverserIndexOutput)(nil)

// ─────────────────────────────────────────────────────────────────────────────
// EndiannessReverserChecksumIndexInput
// ─────────────────────────────────────────────────────────────────────────────

// EndiannessReverserChecksumIndexInput wraps an IndexInput as a
// *gstore.BufferedChecksumIndexInput and additionally byte-swaps every
// multi-byte integer read.  The checksum is computed over the raw (pre-swap)
// bytes, matching Lucene's semantics.
//
// Port of org.apache.lucene.backward_codecs.store.EndiannessReverserChecksumIndexInput
// (Lucene 10.4.0, package-private).
type EndiannessReverserChecksumIndexInput struct {
	// BaseDataInput carries the concrete members Java's DataInput supplies to
	// this class (readVInt, readVLong, readZInt, readZLong, readMapOfStrings,
	// readSetOfStrings), built on the overridden readByte and readBytes. Its
	// Core is this input.
	spi.BaseDataInput

	inner *gstore.BufferedChecksumIndexInput
}

// NewEndiannessReverserChecksumIndexInput wraps in so bytes are checksummed
// and multi-byte integers are byte-swapped.
func NewEndiannessReverserChecksumIndexInput(in gstore.IndexInput) *EndiannessReverserChecksumIndexInput {
	r := &EndiannessReverserChecksumIndexInput{
		inner: gstore.NewBufferedChecksumIndexInput(in),
	}
	r.Core = r
	return r
}

// SkipBytes renders DataInput.skipBytes, which reads and discards the bytes
// through readBytes; the checksumming input it delegates to does the same.
func (r *EndiannessReverserChecksumIndexInput) SkipBytes(numBytes int64) error {
	return r.inner.SkipBytes(numBytes)
}

// ReadInts renders DataInput.readInts, which reads every value through the
// overridden, byte-swapping readInt.
func (r *EndiannessReverserChecksumIndexInput) ReadInts(dst []int32, offset, length int) error {
	if offset < 0 || length < 0 || offset+length > len(dst) {
		return fmt.Errorf("index out of bounds: offset=%d length=%d size=%d", offset, length, len(dst))
	}
	for i := 0; i < length; i++ {
		v, err := r.ReadInt()
		if err != nil {
			return err
		}
		dst[offset+i] = v
	}
	return nil
}

// ReadLongs renders DataInput.readLongs, which reads every value through the
// overridden, byte-swapping readLong.
func (r *EndiannessReverserChecksumIndexInput) ReadLongs(dst []int64, offset, length int) error {
	if offset < 0 || length < 0 || offset+length > len(dst) {
		return fmt.Errorf("index out of bounds: offset=%d length=%d size=%d", offset, length, len(dst))
	}
	for i := 0; i < length; i++ {
		v, err := r.ReadLong()
		if err != nil {
			return err
		}
		dst[offset+i] = v
	}
	return nil
}

// ReadFloats renders DataInput.readFloats: Float.intBitsToFloat(readInt()) for
// every value, through the overridden, byte-swapping readInt.
func (r *EndiannessReverserChecksumIndexInput) ReadFloats(dst []float32, offset, length int) error {
	if offset < 0 || length < 0 || offset+length > len(dst) {
		return fmt.Errorf("index out of bounds: offset=%d length=%d size=%d", offset, length, len(dst))
	}
	for i := 0; i < length; i++ {
		v, err := r.ReadInt()
		if err != nil {
			return err
		}
		dst[offset+i] = math.Float32frombits(uint32(v))
	}
	return nil
}

func (r *EndiannessReverserChecksumIndexInput) ReadByte() (byte, error) {
	return r.inner.ReadByte()
}

func (r *EndiannessReverserChecksumIndexInput) ReadBytes(b []byte, offset, length int) error {
	return r.inner.ReadBytes(b, offset, length)
}

func (r *EndiannessReverserChecksumIndexInput) ReadBytesN(n int) ([]byte, error) {
	return r.inner.ReadBytesN(n)
}

func (r *EndiannessReverserChecksumIndexInput) ReadShort() (int16, error) {
	v, err := r.inner.ReadShort()
	return int16(bits.ReverseBytes16(uint16(v))), err
}

func (r *EndiannessReverserChecksumIndexInput) ReadInt() (int32, error) {
	v, err := r.inner.ReadInt()
	return int32(bits.ReverseBytes32(uint32(v))), err
}

func (r *EndiannessReverserChecksumIndexInput) ReadLong() (int64, error) {
	v, err := r.inner.ReadLong()
	return int64(bits.ReverseBytes64(uint64(v))), err
}

func (r *EndiannessReverserChecksumIndexInput) ReadString() (string, error) {
	return r.inner.ReadString()
}

// GetChecksum returns the CRC32 checksum over all bytes read so far (raw,
// pre-swap bytes, matching Lucene semantics).
func (r *EndiannessReverserChecksumIndexInput) GetChecksum() uint32 {
	return r.inner.GetChecksum()
}

func (r *EndiannessReverserChecksumIndexInput) GetFilePointer() int64 {
	return r.inner.GetFilePointer()
}

func (r *EndiannessReverserChecksumIndexInput) Length() int64 { return r.inner.Length() }

func (r *EndiannessReverserChecksumIndexInput) SetPosition(pos int64) error {
	return r.inner.SetPosition(pos)
}

func (r *EndiannessReverserChecksumIndexInput) Close() error { return r.inner.Close() }

// Slice delegates to the inner BufferedChecksumIndexInput.  The result is
// wrapped in a fresh EndiannessReverserIndexInput (without checksum tracking)
// matching Lucene's slice semantics in this class.
func (r *EndiannessReverserChecksumIndexInput) Slice(desc string, offset, length int64) (gstore.IndexInput, error) {
	inner, err := r.inner.Slice(desc, offset, length)
	if err != nil {
		return nil, err
	}
	return NewEndiannessReverserIndexInput(inner), nil
}

// Clone is not supported; returns nil (matching Lucene's UnsupportedOperationException).
func (r *EndiannessReverserChecksumIndexInput) Clone() gstore.IndexInput { return nil }

// ─────────────────────────────────────────────────────────────────────────────
// EndiannessReverserUtil
// ─────────────────────────────────────────────────────────────────────────────

// EndiannessReverserUtil is a stateless factory that opens legacy (big-endian)
// index files through the appropriate endianness-reversing wrappers.
//
// Port of org.apache.lucene.backward_codecs.store.EndiannessReverserUtil
// (Lucene 10.4.0).  The Java class is final with no instances; here we use
// package-level functions.
type EndiannessReverserUtil struct{} // kept for any code that embeds the type

// OpenInput opens name from dir and wraps it in an
// EndiannessReverserIndexInput.
func OpenInput(dir gstore.Directory, name string, ctx gstore.IOContext) (*EndiannessReverserIndexInput, error) {
	in, err := dir.OpenInput(name, ctx)
	if err != nil {
		return nil, err
	}
	return NewEndiannessReverserIndexInput(in), nil
}

// OpenChecksumInput opens name from dir and wraps it in an
// EndiannessReverserChecksumIndexInput.
func OpenChecksumInput(dir gstore.Directory, name string, ctx gstore.IOContext) (*EndiannessReverserChecksumIndexInput, error) {
	in, err := dir.OpenInput(name, ctx)
	if err != nil {
		return nil, err
	}
	return NewEndiannessReverserChecksumIndexInput(in), nil
}

// CreateOutput creates name in dir and wraps it in an
// EndiannessReverserIndexOutput.
func CreateOutput(dir gstore.Directory, name string, ctx gstore.IOContext) (*EndiannessReverserIndexOutput, error) {
	out, err := dir.CreateOutput(name, ctx)
	if err != nil {
		return nil, err
	}
	return NewEndiannessReverserIndexOutput(out), nil
}

// ─────────────────────────────────────────────────────────────────────────────
// EndiannessReverserIndexInput.EndiannessReverserRandomAccessInput
// ─────────────────────────────────────────────────────────────────────────────

// EndiannessReverserRandomAccessInput is a RandomAccessInput wrapper that
// changes the endianness of the provided index input.
//
// Port of the public static nested class
// org.apache.lucene.backward_codecs.store.EndiannessReverserIndexInput.EndiannessReverserRandomAccessInput
// (Lucene 10.5.0, @lucene.internal).
type EndiannessReverserRandomAccessInput struct {
	in gstore.RandomAccessInput
}

// NewEndiannessReverserRandomAccessInput wraps in so that short/int/long reads
// are byte-swapped.
//
// Mirrors {@code public EndiannessReverserRandomAccessInput(RandomAccessInput in)}.
func NewEndiannessReverserRandomAccessInput(in gstore.RandomAccessInput) *EndiannessReverserRandomAccessInput {
	return &EndiannessReverserRandomAccessInput{in: in}
}

// Length reproduces {@code return in.length();}.
func (r *EndiannessReverserRandomAccessInput) Length() int64 { return r.in.Length() }

// ReadByteAt reproduces {@code return in.readByte(pos);}.
func (r *EndiannessReverserRandomAccessInput) ReadByteAt(pos int64) (byte, error) {
	return r.in.ReadByteAt(pos)
}

// ReadShortAt reproduces {@code return Short.reverseBytes(in.readShort(pos));}.
func (r *EndiannessReverserRandomAccessInput) ReadShortAt(pos int64) (int16, error) {
	v, err := r.in.ReadShortAt(pos)
	return int16(bits.ReverseBytes16(uint16(v))), err
}

// ReadIntAt reproduces {@code return Integer.reverseBytes(in.readInt(pos));}.
func (r *EndiannessReverserRandomAccessInput) ReadIntAt(pos int64) (int32, error) {
	v, err := r.in.ReadIntAt(pos)
	return int32(bits.ReverseBytes32(uint32(v))), err
}

// ReadLongAt reproduces {@code return Long.reverseBytes(in.readLong(pos));}.
func (r *EndiannessReverserRandomAccessInput) ReadLongAt(pos int64) (int64, error) {
	v, err := r.in.ReadLongAt(pos)
	return int64(bits.ReverseBytes64(uint64(v))), err
}

var _ gstore.RandomAccessInput = (*EndiannessReverserRandomAccessInput)(nil)

// RandomAccessSlice reproduces
//
//	return new EndiannessReverserRandomAccessInput(in.randomAccessSlice(offset, length));
//
// org.apache.lucene.store.IndexInput declares randomAccessSlice with a default
// body that slices the input and returns the slice itself when it already
// supports random access; store.IndexInput does not carry the member, so this
// renders that body here — the same spelling
// codecs/lucene95.OrdToDocDISIReaderConfiguration uses.
func (r *EndiannessReverserIndexInput) RandomAccessSlice(offset, length int64) (gstore.RandomAccessInput, error) {
	slice, err := r.FilterIndexInput.GetDelegate().Slice("randomaccess", offset, length)
	if err != nil {
		return nil, err
	}
	randomAccess, ok := slice.(gstore.RandomAccessInput)
	if !ok {
		return nil, fmt.Errorf("backward_codecs/store: slice %T of %T does not support random access",
			slice, r.FilterIndexInput.GetDelegate())
	}
	return NewEndiannessReverserRandomAccessInput(randomAccess), nil
}
