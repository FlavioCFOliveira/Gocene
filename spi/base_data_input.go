// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spi

import (
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// BaseDataInput provides default implementations for DataInput.
// Implementations of DataInput should embed BaseDataInput and set its Core field.
type BaseDataInput struct {
	Core DataInputCore
}

// ReadBytesBuffered reads a specified number of bytes into an array at the specified offset
// with control over whether the read should be buffered.
func (b *BaseDataInput) ReadBytesBuffered(buf []byte, offset, len int, useBuffer bool) error {
	// Default to ignoring useBuffer entirely
	return b.Core.ReadBytes(buf, offset, len)
}

func (b *BaseDataInput) ReadShort() (int16, error) {
	b1, err := b.Core.ReadByte()
	if err != nil {
		return 0, err
	}
	b2, err := b.Core.ReadByte()
	if err != nil {
		return 0, err
	}
	return int16(((int16(b2) & 0xFF) << 8) | (int16(b1) & 0xFF)), nil
}

func (b *BaseDataInput) ReadInt() (int32, error) {
	b1, err := b.Core.ReadByte()
	if err != nil {
		return 0, err
	}
	b2, err := b.Core.ReadByte()
	if err != nil {
		return 0, err
	}
	b3, err := b.Core.ReadByte()
	if err != nil {
		return 0, err
	}
	b4, err := b.Core.ReadByte()
	if err != nil {
		return 0, err
	}
	return int32((int32(b4) << 24) | (int32(b3) << 16) | (int32(b2) << 8) | int32(b1)), nil
}

func (b *BaseDataInput) ReadVInt() (int32, error) {
	bVal, err := b.Core.ReadByte()
	if err != nil {
		return 0, err
	}
	i := int32(bVal & 0x7F)
	for shift := 7; (bVal & 0x80) != 0; shift += 7 {
		bVal, err = b.Core.ReadByte()
		if err != nil {
			return 0, err
		}
		i |= int32(bVal & 0x7F) << shift
	}
	return i, nil
}

func (b *BaseDataInput) ReadZInt() (int32, error) {
	v, err := b.ReadVInt()
	if err != nil {
		return 0, err
	}
	return int32(util.ZigZagDecodeInt(int(v))), nil
}

func (b *BaseDataInput) ReadLong() (int64, error) {
	low, err := b.ReadInt()
	if err != nil {
		return 0, err
	}
	high, err := b.ReadInt()
	if err != nil {
		return 0, err
	}
	return int64(uint32(low)) | (int64(high) << 32), nil
}

func (b *BaseDataInput) ReadLongs(dst []int64, offset, length int) error {
	if offset+length > len(dst) {
		return fmt.Errorf("index out of bounds")
	}
	for i := 0; i < length; i++ {
		v, err := b.ReadLong()
		if err != nil {
			return err
		}
		dst[offset+i] = v
	}
	return nil
}

func (b *BaseDataInput) ReadInts(dst []int32, offset, length int) error {
	if offset+length > len(dst) {
		return fmt.Errorf("index out of bounds")
	}
	for i := 0; i < length; i++ {
		v, err := b.ReadInt()
		if err != nil {
			return err
		}
		dst[offset+i] = v
	}
	return nil
}

func (b *BaseDataInput) ReadFloats(dst []float32, offset, length int) error {
	if offset+length > len(dst) {
		return fmt.Errorf("index out of bounds")
	}
	for i := 0; i < length; i++ {
		v, err := b.ReadInt()
		if err != nil {
			return err
		}
		dst[offset+i] = math.Float32frombits(uint32(v))
	}
	return nil
}

func (b *BaseDataInput) ReadVLong() (int64, error) {
	bVal, err := b.Core.ReadByte()
	if err != nil {
		return 0, err
	}
	i := int64(bVal & 0x7F)
	for shift := 7; (bVal & 0x80) != 0; shift += 7 {
		bVal, err = b.Core.ReadByte()
		if err != nil {
			return 0, err
		}
		i |= int64(bVal & 0x7F) << shift
	}
	return i, nil
}

func (b *BaseDataInput) ReadZLong() (int64, error) {
	v, err := b.ReadVLong()
	if err != nil {
		return 0, err
	}
	return util.ZigZagDecodeInt64(v), nil
}

func (b *BaseDataInput) ReadString() (string, error) {
	length, err := b.ReadVInt()
	if err != nil {
		return "", err
	}
	bytes := make([]byte, length)
	err = b.Core.ReadBytes(bytes, 0, int(length))
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func (b *BaseDataInput) ReadMapOfStrings() (map[string]string, error) {
	count, err := b.ReadVInt()
	if err != nil {
		return nil, err
	}
	if count == 0 {
		return make(map[string]string), nil
	}
	res := make(map[string]string, count)
	for i := 0; i < int(count); i++ {
		k, err := b.ReadString()
		if err != nil {
			return nil, err
		}
		v, err := b.ReadString()
		if err != nil {
			return nil, err
		}
		res[k] = v
	}
	return res, nil
}

func (b *BaseDataInput) ReadSetOfStrings() ([]string, error) {
	count, err := b.ReadVInt()
	if err != nil {
		return nil, err
	}
	if count == 0 {
		return []string{}, nil
	}
	res := make([]string, 0, count)
	for i := 0; i < int(count); i++ {
		s, err := b.ReadString()
		if err != nil {
			return nil, err
		}
		res = append(res, s)
	}
	return res, nil
}

func (b *BaseDataInput) SkipBytes(numBytes int64) error {
	return b.Core.SkipBytes(numBytes)
}
