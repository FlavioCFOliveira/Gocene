// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spi

import (
	"bytes"
	"fmt"
)

func writeVInt(b []byte, v int32) int {
	i := v
	var n int
	for i >= 0x80 {
		b[n] = byte(i) | 0x80
		i >>= 7
		n++
	}
	b[n] = byte(i)
	return n + 1
}

func writeVLong(b []byte, v int64) int {
	i := v
	var n int
	for i >= 0x80 {
		b[n] = byte(i) | 0x80
		i >>= 7
		n++
	}
	b[n] = byte(i)
	return n + 1
}

// ReadInt32 reads an int32 from the input.
func ReadInt32(in IndexInput) (int32, error) {
	return in.ReadInt()
}

// ReadInt64 reads an int64 from the input.
func ReadInt64(in IndexInput) (int64, error) {
	return in.ReadLong()
}

// GetSegmentComponentName returns the file name for a segment component.
func GetSegmentComponentName(segmentName string, segmentSuffix string, ext string) string {
	if segmentSuffix == "" {
		if ext != "" {
			return segmentName + "." + ext
		}
		return segmentName
	}
	if ext != "" {
		return segmentName + "_" + segmentSuffix + "." + ext
	}
	return segmentName + "_" + segmentSuffix
}

// CodecMagic is the magic number written at the start of every Lucene index file.
// Mirrors org.apache.lucene.codecs.CodecUtil.MAGIC = 0x3FD76C17.
const CodecMagic int32 = 0x3FD76C17

// FooterMagic is the magic number written in codec footers.
// Mirrors org.apache.lucene.codecs.CodecUtil.FOOTER_MAGIC = ^0x3FD76C17.
const FooterMagic int32 = ^0x3FD76C17

// WriteBEInt writes an int value on header / footer with big endian order.
//
// Port of org.apache.lucene.codecs.CodecUtil#writeBEInt (CodecUtil.java:653).
// Lucene's DataOutput.writeInt is LITTLE-endian; every fixed-width field of a
// codec header or footer goes through this big-endian writer instead.
func WriteBEInt(out DataOutput, i int32) error {
	if err := out.WriteByte(byte(i >> 24)); err != nil {
		return err
	}
	if err := out.WriteByte(byte(i >> 16)); err != nil {
		return err
	}
	if err := out.WriteByte(byte(i >> 8)); err != nil {
		return err
	}
	return out.WriteByte(byte(i))
}

// WriteBELong writes a long value on header / footer with big endian order.
//
// Port of org.apache.lucene.codecs.CodecUtil#writeBELong (CodecUtil.java:661).
func WriteBELong(out DataOutput, l int64) error {
	if err := WriteBEInt(out, int32(l>>32)); err != nil {
		return err
	}
	return WriteBEInt(out, int32(l))
}

// ReadBEInt reads an int value from header / footer with big endian order.
//
// Port of org.apache.lucene.codecs.CodecUtil#readBEInt (CodecUtil.java:667).
func ReadBEInt(in DataInput) (int32, error) {
	b1, err := in.ReadByte()
	if err != nil {
		return 0, err
	}
	b2, err := in.ReadByte()
	if err != nil {
		return 0, err
	}
	b3, err := in.ReadByte()
	if err != nil {
		return 0, err
	}
	b4, err := in.ReadByte()
	if err != nil {
		return 0, err
	}
	return int32(b1)<<24 | int32(b2)<<16 | int32(b3)<<8 | int32(b4), nil
}

// ReadBELong reads a long value from header / footer with big endian order.
//
// Port of org.apache.lucene.codecs.CodecUtil#readBELong (CodecUtil.java:675).
func ReadBELong(in DataInput) (int64, error) {
	hi, err := ReadBEInt(in)
	if err != nil {
		return 0, err
	}
	lo, err := ReadBEInt(in)
	if err != nil {
		return 0, err
	}
	return int64(hi)<<32 | (int64(lo) & 0xFFFFFFFF), nil
}

// WriteIndexHeader writes a codec index header: magic (BE int32) + codec name
// (string) + version (BE int32) + id (16 bytes) + suffix (len byte + bytes).
// Byte-for-byte identical to org.apache.lucene.codecs.CodecUtil.writeIndexHeader
// (CodecUtil.java:121-135).
func WriteIndexHeader(out IndexOutput, codec string, version int32, id []byte, suffix string) error {
	if len(id) != 16 {
		return fmt.Errorf("WriteIndexHeader: id length must be 16, got %d", len(id))
	}
	if len(suffix) > 255 {
		return fmt.Errorf("WriteIndexHeader: suffix too long (%d > 255)", len(suffix))
	}
	if err := WriteBEInt(out, CodecMagic); err != nil {
		return err
	}
	if err := out.WriteString(codec); err != nil {
		return err
	}
	if err := WriteBEInt(out, version); err != nil {
		return err
	}
	if err := out.WriteBytes(id, 0, len(id)); err != nil {
		return err
	}
	if err := out.WriteByte(byte(len(suffix))); err != nil {
		return err
	}
	if len(suffix) > 0 {
		if err := out.WriteBytes([]byte(suffix), 0, len(suffix)); err != nil {
			return err
		}
	}
	return nil
}

// CheckIndexHeader reads and validates a codec index header.
// Returns the version number on success.
// Mirrors org.apache.lucene.codecs.CodecUtil.checkIndexHeader.
func CheckIndexHeader(in IndexInput, codec string, minVersion, maxVersion int32, expectedID []byte, expectedSuffix string) (int32, error) {
	magic, err := ReadBEInt(in)
	if err != nil {
		return 0, err
	}
	if magic != CodecMagic {
		return 0, fmt.Errorf("CheckIndexHeader: invalid magic 0x%x (expected 0x%x)", magic, CodecMagic)
	}
	actualCodec, err := in.ReadString()
	if err != nil {
		return 0, err
	}
	if actualCodec != codec {
		return 0, fmt.Errorf("CheckIndexHeader: codec mismatch %q (expected %q)", actualCodec, codec)
	}
	version, err := ReadBEInt(in)
	if err != nil {
		return 0, err
	}
	if version < minVersion || version > maxVersion {
		return 0, fmt.Errorf("CheckIndexHeader: version %d out of range [%d, %d]", version, minVersion, maxVersion)
	}
	id, err := in.ReadBytesN(16)
	if err != nil {
		return 0, err
	}
	if expectedID != nil && !bytes.Equal(id, expectedID) {
		return 0, fmt.Errorf("CheckIndexHeader: segment ID mismatch")
	}
	suffixLen, err := in.ReadByte()
	if err != nil {
		return 0, err
	}
	var suffix string
	if suffixLen > 0 {
		sb, err2 := in.ReadBytesN(int(suffixLen))
		if err2 != nil {
			return 0, err2
		}
		suffix = string(sb)
	}
	if suffix != expectedSuffix {
		return 0, fmt.Errorf("CheckIndexHeader: suffix mismatch %q (expected %q)", suffix, expectedSuffix)
	}
	return version, nil
}

// WriteFooter writes the codec footer: FooterMagic (BE int32) + algo=0
// (BE int32) + CRC32 checksum (BE int64). out must be a *ChecksumIndexOutput.
// Mirrors org.apache.lucene.codecs.CodecUtil.writeFooter (CodecUtil.java:409).
func WriteFooter(out *ChecksumIndexOutput) error {
	if err := WriteBEInt(out, FooterMagic); err != nil {
		return err
	}
	if err := WriteBEInt(out, 0); err != nil { // algo = CRC32
		return err
	}
	checksum := out.GetChecksum()
	return WriteBELong(out, int64(checksum))
}

// CheckFooter validates the codec footer and returns the checksum.
// in must be positioned just before the footer (FooterMagic field).
// Mirrors org.apache.lucene.codecs.CodecUtil.checkFooter.
func CheckFooter(in *ChecksumIndexInput) (int64, error) {
	magic, err := ReadBEInt(in)
	if err != nil {
		return 0, err
	}
	if magic != FooterMagic {
		return 0, fmt.Errorf("CheckFooter: invalid footer magic 0x%x (expected 0x%x)", magic, FooterMagic)
	}
	algo, err := ReadBEInt(in)
	if err != nil {
		return 0, err
	}
	if algo != 0 {
		return 0, fmt.Errorf("CheckFooter: unknown checksum algorithm %d", algo)
	}
	actualChecksum := in.GetChecksum()
	expectedChecksum, err := ReadBELong(in)
	if err != nil {
		return 0, err
	}
	if int64(actualChecksum) != expectedChecksum {
		return 0, fmt.Errorf("CheckFooter: checksum mismatch (actual 0x%x, expected 0x%x)", actualChecksum, expectedChecksum)
	}
	return expectedChecksum, nil
}
