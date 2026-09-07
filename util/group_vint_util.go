package util

import (
	"encoding/binary"
	"errors"
	"math/bits"
)

// GroupVIntMaxLengthPerGroup is the maximum length of a single group-varint.
// It consists of 1 byte for the flag and up to 4 bytes for each of the 4 integers.
const GroupVIntMaxLengthPerGroup = 1 + 4*4

var ErrGroupVIntOverflow = errors.New("group-varint integer overflow")

// DataInput defines the minimum read operations required for GroupVInt decoding.
type DataInput interface {
	ReadByte() (byte, error)
	ReadShort() (int16, error)
	ReadInt() (int32, error)
	ReadVInt() (int32, error)
}

// DataOutput defines the minimum write operations required for GroupVInt encoding.
type DataOutput interface {
	WriteByte(b byte) error
	WriteBytes(b []byte, offset, length int) error
	WriteVInt(i int32) error
}

// ToInt32 converts an int64 to an int32, returning ErrGroupVIntOverflow if the value overflows.
func ToInt32(v int64) (int32, error) {
	if v < -2147483648 || v > 2147483647 {
		return 0, ErrGroupVIntOverflow
	}
	return int32(v), nil
}

func numBytes(v int32) int {
	// Integer.BYTES - (Integer.numberOfLeadingZeros(v | 1) >> 3)
	return 4 - (bits.LeadingZeros32(uint32(v)|1) >> 3)
}

// WriteGroupVInts encodes the given values using group varint encoding.
// It uses a scratch buffer to construct each group before writing it to the output.
func WriteGroupVInts(out DataOutput, scratch []byte, values []int32, limit int) error {
	if len(scratch) < GroupVIntMaxLengthPerGroup {
		panic("scratch buffer too small")
	}

	readPos := 0
	for (limit - readPos) >= 4 {
		writePos := 0

		n1Minus1 := numBytes(values[readPos]) - 1
		n2Minus1 := numBytes(values[readPos+1]) - 1
		n3Minus1 := numBytes(values[readPos+2]) - 1
		n4Minus1 := numBytes(values[readPos+3]) - 1

		flag := byte((n1Minus1 << 6) | (n2Minus1 << 4) | (n3Minus1 << 2) | n4Minus1)
		scratch[writePos] = flag
		writePos++

		binary.LittleEndian.PutUint32(scratch[writePos:], uint32(values[readPos]))
		writePos += n1Minus1 + 1
		readPos++

		binary.LittleEndian.PutUint32(scratch[writePos:], uint32(values[readPos]))
		writePos += n2Minus1 + 1
		readPos++

		binary.LittleEndian.PutUint32(scratch[writePos:], uint32(values[readPos]))
		writePos += n3Minus1 + 1
		readPos++

		binary.LittleEndian.PutUint32(scratch[writePos:], uint32(values[readPos]))
		writePos += n4Minus1 + 1
		readPos++

		if err := out.WriteBytes(scratch, 0, writePos); err != nil {
			return err
		}
	}

	for ; readPos < limit; readPos++ {
		if err := out.WriteVInt(values[readPos]); err != nil {
			return err
		}
	}

	return nil
}

// ReadGroupVInts decodes group varints from the input into the destination array.
func ReadGroupVInts(in DataInput, dst []int32, length int) error {
	i := 0
	for i <= length-4 {
		if err := readGroupVInt(in, dst, i); err != nil {
			return err
		}
		i += 4
	}

	for ; i < length; i++ {
		v, err := in.ReadVInt()
		if err != nil {
			return err
		}
		dst[i] = v
	}

	return nil
}

func readGroupVInt(in DataInput, dst []int32, offset int) error {
	flag, err := in.ReadByte()
	if err != nil {
		return err
	}

	n1Minus1 := int(flag >> 6)
	n2Minus1 := int((flag >> 4) & 0x03)
	n3Minus1 := int((flag >> 2) & 0x03)
	n4Minus1 := int(flag & 0x03)

	v1, err := readIntInGroup(in, n1Minus1)
	if err != nil {
		return err
	}
	dst[offset] = v1

	v2, err := readIntInGroup(in, n2Minus1)
	if err != nil {
		return err
	}
	dst[offset+1] = v2

	v3, err := readIntInGroup(in, n3Minus1)
	if err != nil {
		return err
	}
	dst[offset+2] = v3

	v4, err := readIntInGroup(in, n4Minus1)
	if err != nil {
		return err
	}
	dst[offset+3] = v4

	return nil
}

func readIntInGroup(in DataInput, numBytesMinus1 int) (int32, error) {
	switch numBytesMinus1 {
	case 0:
		b, err := in.ReadByte()
		if err != nil {
			return 0, err
		}
		return int32(b), nil
	case 1:
		s, err := in.ReadShort()
		if err != nil {
			return 0, err
		}
		return int32(uint16(s)), nil
	case 2:
		s, err := in.ReadShort()
		if err != nil {
			return 0, err
		}
		b, err := in.ReadByte()
		if err != nil {
			return 0, err
		}
		return int32(uint16(s)) | (int32(b) << 16), nil
	default:
		return in.ReadInt()
	}
}
