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

// WriteIndexHeader writes a codec index header: magic (int32) + codec name
// (string) + version (int32) + id (16 bytes) + suffix (len byte + bytes).
// Byte-for-byte identical to org.apache.lucene.codecs.CodecUtil.writeIndexHeader.
func WriteIndexHeader(out IndexOutput, codec string, version int32, id []byte, suffix string) error {
	if len(id) != 16 {
		return fmt.Errorf("WriteIndexHeader: id length must be 16, got %d", len(id))
	}
	if len(suffix) > 255 {
		return fmt.Errorf("WriteIndexHeader: suffix too long (%d > 255)", len(suffix))
	}
	if err := out.WriteInt(CodecMagic); err != nil {
		return err
	}
	if err := out.WriteString(codec); err != nil {
		return err
	}
	if err := out.WriteInt(version); err != nil {
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
	magic, err := in.ReadInt()
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
	version, err := in.ReadInt()
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

// WriteFooter writes the codec footer: FooterMagic (int32) + algo=0 (int32) +
// CRC32 checksum (int64). out must be a *ChecksumIndexOutput.
// Mirrors org.apache.lucene.codecs.CodecUtil.writeFooter.
func WriteFooter(out *ChecksumIndexOutput) error {
	if err := out.WriteInt(FooterMagic); err != nil {
		return err
	}
	if err := out.WriteInt(0); err != nil { // algo = CRC32
		return err
	}
	checksum := out.GetChecksum()
	return out.WriteLong(int64(checksum))
}

// CheckFooter validates the codec footer and returns the checksum.
// in must be positioned just before the footer (FooterMagic field).
// Mirrors org.apache.lucene.codecs.CodecUtil.checkFooter.
func CheckFooter(in *ChecksumIndexInput) (int64, error) {
	magic, err := in.ReadInt()
	if err != nil {
		return 0, err
	}
	if magic != FooterMagic {
		return 0, fmt.Errorf("CheckFooter: invalid footer magic 0x%x (expected 0x%x)", magic, FooterMagic)
	}
	algo, err := in.ReadInt()
	if err != nil {
		return 0, err
	}
	if algo != 0 {
		return 0, fmt.Errorf("CheckFooter: unknown checksum algorithm %d", algo)
	}
	actualChecksum := in.GetChecksum()
	expectedChecksum, err := in.ReadLong()
	if err != nil {
		return 0, err
	}
	if int64(actualChecksum) != expectedChecksum {
		return 0, fmt.Errorf("CheckFooter: checksum mismatch (actual 0x%x, expected 0x%x)", actualChecksum, expectedChecksum)
	}
	return expectedChecksum, nil
}
