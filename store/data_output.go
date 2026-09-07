package store

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// DataOutput is an interface for performing write operations of Lucene's low-level data types.
// It may only be used from one thread, because it is not thread safe (it keeps
// internal state like file position).
//
// This is a port of org.apache.lucene.store.DataOutput.
type DataOutput interface {
	WriteByte(b byte) error
	WriteBytes(b []byte, offset, length int) error
	WriteBytesN(b []byte, n int) error
	WriteInt(i int32) error
	WriteShort(i int16) error
	WriteVInt(i int32) error
	WriteZInt(i int32) error
	WriteLong(i int64) error
	WriteVLong(i int64) error
	WriteZLong(i int64) error
	WriteString(s string) error
	CopyBytes(input DataInput, numBytes int64) error
	WriteMapOfStrings(m map[string]string) error
	WriteSetOfStrings(s []string) error
	WriteGroupVInts(values []int32, limit int) error
}

// PrimitiveWriter is an internal interface used by BaseDataOutput to delegate
// the most basic write operations.
type PrimitiveWriter interface {
	WriteByte(b byte) error
	WriteBytes(b []byte, offset, length int) error
}

// BaseDataOutput provides default implementations for the DataOutput interface.
// It delegates the basic WriteByte and WriteBytes operations to an underlying PrimitiveWriter.
type BaseDataOutput struct {
	primitive      PrimitiveWriter
	copyBuffer     []byte
	groupVIntBytes []byte
}

// NewBaseDataOutput creates a new BaseDataOutput that delegates basic operations to the provided PrimitiveWriter.
func NewBaseDataOutput(primitive PrimitiveWriter) *BaseDataOutput {
	return &BaseDataOutput{
		primitive: primitive,
	}
}

func (b *BaseDataOutput) WriteByte(val byte) error {
	return b.primitive.WriteByte(val)
}

func (b *BaseDataOutput) WriteBytes(val []byte, offset, length int) error {
	return b.primitive.WriteBytes(val, offset, length)
}

// WriteBytesN writes n bytes from the beginning of the slice.
func (b *BaseDataOutput) WriteBytesN(val []byte, n int) error {
	return b.WriteBytes(val, 0, n)
}

func (b *BaseDataOutput) WriteInt(i int32) error {
	if err := b.WriteByte(byte(i)); err != nil {
		return err
	}
	if err := b.WriteByte(byte(i >> 8)); err != nil {
		return err
	}
	if err := b.WriteByte(byte(i >> 16)); err != nil {
		return err
	}
	return b.WriteByte(byte(i >> 24))
}

func (b *BaseDataOutput) WriteShort(i int16) error {
	if err := b.WriteByte(byte(i)); err != nil {
		return err
	}
	return b.WriteByte(byte(i >> 8))
}

func (b *BaseDataOutput) WriteVInt(i int32) error {
	val := uint32(i)
	for (val & ^uint32(0x7F)) != 0 {
		if err := b.WriteByte(byte((val & 0x7F) | 0x80)); err != nil {
			return err
		}
		val >>= 7
	}
	return b.WriteByte(byte(val))
}

func (b *BaseDataOutput) WriteZInt(i int32) error {
	return b.WriteVInt(int32(util.ZigZagEncodeInt(int(i))))
}

func (b *BaseDataOutput) WriteLong(i int64) error {
	if err := b.WriteInt(int32(i)); err != nil {
		return err
	}
	return b.WriteInt(int32(i >> 32))
}

func (b *BaseDataOutput) WriteVLong(i int64) error {
	if i < 0 {
		return fmt.Errorf("cannot write negative vLong (got: %d)", i)
	}
	return b.writeSignedVLong(i)
}

func (b *BaseDataOutput) writeSignedVLong(i int64) error {
	val := uint64(i)
	for (val & ^uint64(0x7F)) != 0 {
		if err := b.WriteByte(byte((val & 0x7F) | 0x80)); err != nil {
			return err
		}
		val >>= 7
	}
	return b.WriteByte(byte(val))
}

func (b *BaseDataOutput) WriteZLong(i int64) error {
	return b.writeSignedVLong(util.ZigZagEncodeInt64(i))
}

func (b *BaseDataOutput) WriteString(s string) error {
	utf8Result := util.NewBytesRef([]byte(s))
	if err := b.WriteVInt(int32(utf8Result.Length)); err != nil {
		return err
	}
	return b.WriteBytes(utf8Result.Bytes, utf8Result.Offset, utf8Result.Length)
}

const copyBufferSize = 16384

func (b *BaseDataOutput) CopyBytes(input DataInput, numBytes int64) error {
	if numBytes < 0 {
		return fmt.Errorf("numBytes must be non-negative, got %d", numBytes)
	}

	left := numBytes
	if b.copyBuffer == nil {
		b.copyBuffer = make([]byte, copyBufferSize)
	}

	for left > 0 {
		toCopy := int(left)
		if left > copyBufferSize {
			toCopy = copyBufferSize
		}

		if err := input.ReadBytes(b.copyBuffer, 0, toCopy); err != nil {
			return err
		}
		if err := b.WriteBytes(b.copyBuffer, 0, toCopy); err != nil {
			return err
		}
		left -= int64(toCopy)
	}
	return nil
}

func (b *BaseDataOutput) WriteMapOfStrings(m map[string]string) error {
	if err := b.WriteVInt(int32(len(m))); err != nil {
		return err
	}
	for k, v := range m {
		if err := b.WriteString(k); err != nil {
			return err
		}
		if err := b.WriteString(v); err != nil {
			return err
		}
	}
	return nil
}

func (b *BaseDataOutput) WriteSetOfStrings(s []string) error {
	if err := b.WriteVInt(int32(len(s))); err != nil {
		return err
	}
	for _, v := range s {
		if err := b.WriteString(v); err != nil {
			return err
		}
	}
	return nil
}

func (b *BaseDataOutput) WriteGroupVInts(values []int32, limit int) error {
	if b.groupVIntBytes == nil {
		b.groupVIntBytes = make([]byte, util.GroupVIntMaxLengthPerGroup)
	}
	return util.WriteGroupVInts(b, b.groupVIntBytes, values, limit)
}
