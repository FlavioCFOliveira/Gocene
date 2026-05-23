// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package hhmm

import (
	"encoding/binary"
	"fmt"
	"io"
)

// javaObjStream is a minimal reader for the Java Object Serialization Stream
// Protocol (java.io.Serializable).
//
// Only the types written by WordDictionary.saveToObj and
// BigramDictionary.saveToObj are supported:
//   - short[]
//   - char[]
//   - int[]
//   - long[]
//   - char[][]  (nullable elements)
//   - char[][][] (nullable 2-D slices)
//   - int[][]   (nullable elements)
//
// Protocol reference:
// https://docs.oracle.com/javase/8/docs/platform/serialization/spec/protocol.html
//
// Key insight: handles are implicit — the deserializer assigns a running
// counter starting at 0x7E0000; there is no explicit handle encoded in the
// stream.

const (
	jsMagic   = 0xACED
	jsVersion = 5

	tcNull          byte = 0x70
	tcReference     byte = 0x71
	tcClassDesc     byte = 0x72
	tcObject        byte = 0x73
	tcString        byte = 0x74
	tcArray         byte = 0x75
	tcEndBlockData  byte = 0x78
)

type javaObjStream struct {
	r   io.Reader
}

func newJavaObjStream(r io.Reader) (*javaObjStream, error) {
	s := &javaObjStream{r: r}
	magic, err := s.readUint16()
	if err != nil {
		return nil, err
	}
	if magic != jsMagic {
		return nil, fmt.Errorf("java serial: bad magic 0x%04X", magic)
	}
	ver, err := s.readUint16()
	if err != nil {
		return nil, err
	}
	if ver != jsVersion {
		return nil, fmt.Errorf("java serial: unsupported version %d", ver)
	}
	return s, nil
}

func (s *javaObjStream) readFull(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := io.ReadFull(s.r, b)
	return b, err
}

func (s *javaObjStream) readByte() (byte, error) {
	b, err := s.readFull(1)
	if err != nil {
		return 0, err
	}
	return b[0], nil
}

func (s *javaObjStream) readUint16() (uint16, error) {
	b, err := s.readFull(2)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint16(b), nil
}

func (s *javaObjStream) readInt32() (int32, error) {
	b, err := s.readFull(4)
	if err != nil {
		return 0, err
	}
	return int32(binary.BigEndian.Uint32(b)), nil
}

func (s *javaObjStream) readInt64() (int64, error) {
	b, err := s.readFull(8)
	if err != nil {
		return 0, err
	}
	return int64(binary.BigEndian.Uint64(b)), nil
}

// skipClassDesc consumes a classDesc (TC_CLASSDESC tag already read).
func (s *javaObjStream) skipClassDesc() error {
	// className (UTF-8, length-prefixed)
	nameLen, err := s.readUint16()
	if err != nil {
		return err
	}
	if _, err := s.readFull(int(nameLen)); err != nil {
		return err
	}
	// serialVersionUID (8 bytes)
	if _, err := s.readFull(8); err != nil {
		return err
	}
	// classDescInfo: classDescFlags (1 byte)
	if _, err := s.readFull(1); err != nil {
		return err
	}
	// fields count
	fieldCount, err := s.readUint16()
	if err != nil {
		return err
	}
	for i := 0; i < int(fieldCount); i++ {
		typeCode, err := s.readByte()
		if err != nil {
			return err
		}
		fnLen, err := s.readUint16()
		if err != nil {
			return err
		}
		if _, err := s.readFull(int(fnLen)); err != nil {
			return err
		}
		// object/array field carries an additional className string
		if typeCode == '[' || typeCode == 'L' {
			if err := s.skipAnyValue(); err != nil {
				return err
			}
		}
	}
	// classAnnotation: values terminated by TC_ENDBLOCKDATA
	for {
		tag, err := s.readByte()
		if err != nil {
			return err
		}
		if tag == tcEndBlockData {
			break
		}
		if err := s.skipValueWithTag(tag); err != nil {
			return err
		}
	}
	// superClassDesc
	return s.skipAnyValue()
}

// skipAnyValue reads one value (reading its tag first).
func (s *javaObjStream) skipAnyValue() error {
	tag, err := s.readByte()
	if err != nil {
		return err
	}
	return s.skipValueWithTag(tag)
}

// skipValueWithTag discards a value whose tag has already been read.
func (s *javaObjStream) skipValueWithTag(tag byte) error {
	switch tag {
	case tcNull:
		return nil
	case tcReference:
		// back-reference: 4-byte handle (this IS encoded explicitly)
		_, err := s.readFull(4)
		return err
	case tcString:
		n, err := s.readUint16()
		if err != nil {
			return err
		}
		_, err = s.readFull(int(n))
		return err
	case tcClassDesc:
		return s.skipClassDesc()
	case tcArray:
		// classDesc (not null for concrete arrays)
		if err := s.skipAnyValue(); err != nil {
			return err
		}
		// handle is implicit
		// length
		length, err := s.readInt32()
		if err != nil {
			return err
		}
		for i := int32(0); i < length; i++ {
			if err := s.skipAnyValue(); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("java serial: unexpected tag 0x%02X during skip", tag)
	}
}

// readArrayHeader expects TC_ARRAY, reads classDesc (consuming it) and
// returns the array length. Returns -1, nil when the next tag is TC_NULL.
func (s *javaObjStream) readArrayHeader() (int32, error) {
	tag, err := s.readByte()
	if err != nil {
		return 0, err
	}
	if tag == tcNull {
		return -1, nil
	}
	if tag != tcArray {
		return 0, fmt.Errorf("java serial: expected TC_ARRAY (0x75) or TC_NULL, got 0x%02X", tag)
	}
	// classDesc
	if err := s.skipAnyValue(); err != nil {
		return 0, err
	}
	// handle is implicit — no bytes
	return s.readInt32()
}

// ReadShortArray reads a short[] from the stream.
func (s *javaObjStream) ReadShortArray() ([]int16, error) {
	length, err := s.readArrayHeader()
	if err != nil {
		return nil, err
	}
	if length < 0 {
		return nil, nil
	}
	result := make([]int16, length)
	for i := int32(0); i < length; i++ {
		b, err := s.readFull(2)
		if err != nil {
			return nil, err
		}
		result[i] = int16(binary.BigEndian.Uint16(b))
	}
	return result, nil
}

// ReadCharArray reads a char[] from the stream (possibly null element).
func (s *javaObjStream) ReadCharArray() ([]rune, error) {
	length, err := s.readArrayHeader()
	if err != nil {
		return nil, err
	}
	if length < 0 {
		return nil, nil
	}
	result := make([]rune, length)
	for i := int32(0); i < length; i++ {
		b, err := s.readFull(2)
		if err != nil {
			return nil, err
		}
		result[i] = rune(binary.BigEndian.Uint16(b))
	}
	return result, nil
}

// ReadIntArray reads an int[] from the stream (possibly null).
func (s *javaObjStream) ReadIntArray() ([]int32, error) {
	length, err := s.readArrayHeader()
	if err != nil {
		return nil, err
	}
	if length < 0 {
		return nil, nil
	}
	result := make([]int32, length)
	for i := int32(0); i < length; i++ {
		v, err := s.readInt32()
		if err != nil {
			return nil, err
		}
		result[i] = v
	}
	return result, nil
}

// ReadLongArray reads a long[] from the stream.
func (s *javaObjStream) ReadLongArray() ([]int64, error) {
	length, err := s.readArrayHeader()
	if err != nil {
		return nil, err
	}
	if length < 0 {
		return nil, nil
	}
	result := make([]int64, length)
	for i := int32(0); i < length; i++ {
		v, err := s.readInt64()
		if err != nil {
			return nil, err
		}
		result[i] = v
	}
	return result, nil
}

// ReadChar3D reads a char[][][] from the stream.
func (s *javaObjStream) ReadChar3D() ([][][]rune, error) {
	length, err := s.readArrayHeader()
	if err != nil {
		return nil, err
	}
	if length < 0 {
		return nil, nil
	}
	result := make([][][]rune, length)
	for i := int32(0); i < length; i++ {
		row, err := s.readChar2D()
		if err != nil {
			return nil, fmt.Errorf("char[][][] row %d: %w", i, err)
		}
		result[i] = row
	}
	return result, nil
}

// readChar2D reads a char[][] (nullable) from the stream.
func (s *javaObjStream) readChar2D() ([][]rune, error) {
	length, err := s.readArrayHeader()
	if err != nil {
		return nil, err
	}
	if length < 0 {
		return nil, nil
	}
	result := make([][]rune, length)
	for j := int32(0); j < length; j++ {
		row, err := s.ReadCharArray()
		if err != nil {
			return nil, fmt.Errorf("char[][] col %d: %w", j, err)
		}
		result[j] = row
	}
	return result, nil
}

// ReadInt2D reads an int[][] from the stream.
func (s *javaObjStream) ReadInt2D() ([][]int32, error) {
	length, err := s.readArrayHeader()
	if err != nil {
		return nil, err
	}
	if length < 0 {
		return nil, nil
	}
	result := make([][]int32, length)
	for i := int32(0); i < length; i++ {
		row, err := s.readInt1D()
		if err != nil {
			return nil, fmt.Errorf("int[][] row %d: %w", i, err)
		}
		result[i] = row
	}
	return result, nil
}

// readInt1D reads an int[] (nullable) from the stream.
func (s *javaObjStream) readInt1D() ([]int32, error) {
	length, err := s.readArrayHeader()
	if err != nil {
		return nil, err
	}
	if length < 0 {
		return nil, nil
	}
	result := make([]int32, length)
	for j := int32(0); j < length; j++ {
		v, err := s.readInt32()
		if err != nil {
			return nil, err
		}
		result[j] = v
	}
	return result, nil
}
