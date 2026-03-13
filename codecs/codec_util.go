// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"bytes"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/store"
)

const (
	// CODEC_MAGIC is the magic number for codec headers.
	CODEC_MAGIC int32 = 0x3FD76C17
	// FOOTER_MAGIC is the magic number for codec footers.
	FOOTER_MAGIC int32 = ^0x3FD76C17
)

// checksumWriter is an interface for IndexOutput that provides a checksum.
type checksumWriter interface {
	store.IndexOutput
	GetChecksum() uint32
}

// HeaderLength returns the length of a codec header.
func HeaderLength(codec string) int {
	return 9 + len(codec)
}

// IndexHeaderLength returns the length of an index header.
func IndexHeaderLength(codec string, suffix string) int {
	return HeaderLength(codec) + 16 + 1 + len(suffix)
}

// FooterLength returns the length of a codec footer.
func FooterLength() int {
	return 16
}

// WriteHeader writes a codec header, which identifies the file format and version.
func WriteHeader(out store.IndexOutput, codec string, version int32) error {
	if err := checkCodecName(codec); err != nil {
		return err
	}
	if err := store.WriteInt32(out, CODEC_MAGIC); err != nil {
		return err
	}
	if err := store.WriteString(out, codec); err != nil {
		return err
	}
	return store.WriteInt32(out, version)
}

// WriteIndexHeader writes an index header, which includes a unique ID and suffix.
func WriteIndexHeader(out store.IndexOutput, codec string, version int32, id []byte, suffix string) error {
	if len(id) != 16 {
		return fmt.Errorf("invalid id length: %d (expected 16)", len(id))
	}
	if err := WriteHeader(out, codec, version); err != nil {
		return err
	}
	if err := out.WriteBytes(id); err != nil {
		return err
	}
	return writeSuffix(out, suffix)
}

// writeSuffix writes the suffix string with length prefix.
func writeSuffix(out store.IndexOutput, suffix string) error {
	if len(suffix) > 255 {
		return fmt.Errorf("suffix too long: %d (max 255)", len(suffix))
	}
	for i := 0; i < len(suffix); i++ {
		if suffix[i] > 127 {
			return fmt.Errorf("non-ascii character in suffix: %s", suffix)
		}
	}
	if err := out.WriteByte(byte(len(suffix))); err != nil {
		return err
	}
	// Write actual suffix bytes directly
	return out.WriteBytes([]byte(suffix))
}

// checkCodecName verifies the codec name is valid.
func checkCodecName(codec string) error {
	if len(codec) >= 128 {
		return fmt.Errorf("codec name too long: %d (max 127)", len(codec))
	}
	for i := 0; i < len(codec); i++ {
		if codec[i] > 127 {
			return fmt.Errorf("non-ascii character in codec name: %s", codec)
		}
	}
	return nil
}

// CheckHeader reads and validates a codec header.
func CheckHeader(in store.IndexInput, codec string, minVersion, maxVersion int32) (int32, error) {
	magic, err := store.ReadInt32(in)
	if err != nil {
		return 0, err
	}
	if magic != CODEC_MAGIC {
		return 0, fmt.Errorf("invalid magic number: %x (expected %x)", magic, CODEC_MAGIC)
	}
	actualCodec, err := store.ReadString(in)
	if err != nil {
		return 0, err
	}
	if actualCodec != codec {
		return 0, fmt.Errorf("invalid codec name: %s (expected %s)", actualCodec, codec)
	}
	version, err := store.ReadInt32(in)
	if err != nil {
		return 0, err
	}
	if version < minVersion || version > maxVersion {
		return 0, fmt.Errorf("invalid version: %d (expected between %d and %d)", version, minVersion, maxVersion)
	}
	return version, nil
}

// CheckIndexHeader reads and validates an index header.
func CheckIndexHeader(in store.IndexInput, codec string, minVersion, maxVersion int32, expectedId []byte, expectedSuffix string) (int32, error) {
	version, err := CheckHeader(in, codec, minVersion, maxVersion)
	if err != nil {
		return 0, err
	}
	id, err := in.ReadBytesN(16)
	if err != nil {
		return 0, err
	}
	if expectedId != nil && !bytes.Equal(id, expectedId) {
		return 0, fmt.Errorf("mismatched index id")
	}

	suffixLen, err := in.ReadByte()
	if err != nil {
		return 0, err
	}
	suffixBytes, err := in.ReadBytesN(int(suffixLen))
	if err != nil {
		return 0, err
	}
	suffix := string(suffixBytes)
	if suffix != expectedSuffix {
		return 0, fmt.Errorf("mismatched suffix: %s (expected %s)", suffix, expectedSuffix)
	}
	return version, nil
}

// WriteFooter writes a codec footer with a checksum.
func WriteFooter(out store.IndexOutput) error {
	if err := store.WriteInt32(out, FOOTER_MAGIC); err != nil {
		return err
	}
	if err := store.WriteInt32(out, 0); err != nil {
		return err
	}
	return WriteCRC(out)
}

// WriteCRC writes the checksum of the output.
func WriteCRC(out store.IndexOutput) error {
	if cw, ok := out.(checksumWriter); ok {
		checksum := cw.GetChecksum()
		return store.WriteInt64(out, int64(checksum))
	}
	return fmt.Errorf("output does not support checksums")
}

// ReadCRC reads a checksum from the input.
func ReadCRC(in store.IndexInput) (int64, error) {
	checksum, err := store.ReadInt64(in)
	if err != nil {
		return 0, err
	}
	if (checksum & ^int64(0xFFFFFFFF)) != 0 {
		return 0, fmt.Errorf("illegal checksum: %d", checksum)
	}
	return checksum, nil
}

// CheckFooter validates the footer and checksum.
func CheckFooter(in *store.ChecksumIndexInput) (int64, error) {
	if err := validateFooter(in); err != nil {
		return 0, err
	}

	actualChecksum := in.GetChecksum()
	expectedChecksum, err := ReadCRC(in)
	if err != nil {
		return 0, err
	}

	if int64(actualChecksum) != expectedChecksum {
		return 0, fmt.Errorf("checksum failed: %x (expected %x)", actualChecksum, expectedChecksum)
	}

	return expectedChecksum, nil
}

func validateFooter(in store.IndexInput) error {
	remaining := in.Length() - in.GetFilePointer()
	expected := int64(FooterLength())
	if remaining < expected {
		return fmt.Errorf("misplaced codec footer (file truncated?): remaining=%d, expected=%d, fp=%d", remaining, expected, in.GetFilePointer())
	} else if remaining > expected {
		return fmt.Errorf("misplaced codec footer (file extended?): remaining=%d, expected=%d, fp=%d", remaining, expected, in.GetFilePointer())
	}

	magic, err := store.ReadInt32(in)
	if err != nil {
		return err
	}
	if magic != FOOTER_MAGIC {
		return fmt.Errorf("codec footer mismatch: actual footer=%x vs expected footer=%x", magic, FOOTER_MAGIC)
	}

	algorithmID, err := store.ReadInt32(in)
	if err != nil {
		return err
	}
	if algorithmID != 0 {
		return fmt.Errorf("codec footer mismatch: unknown algorithmID: %d", algorithmID)
	}
	return nil
}

// ChecksumEntireFile computes and validates the checksum for the entire file.
func ChecksumEntireFile(input store.IndexInput) (int64, error) {
	clone := input.Clone()
	if err := clone.SetPosition(0); err != nil {
		return 0, err
	}
	in := store.NewChecksumIndexInput(clone)
	defer in.Close()

	if in.Length() < int64(FooterLength()) {
		return 0, fmt.Errorf("misplaced codec footer (file truncated?): length=%d but footerLength==16", in.Length())
	}

	if err := in.SetPosition(in.Length() - int64(FooterLength())); err != nil {
		return 0, err
	}
	return CheckFooter(in)
}

// RetrieveChecksum reads the checksum from the end of the file.
func RetrieveChecksum(in store.IndexInput) (int64, error) {
	if in.Length() < int64(FooterLength()) {
		return 0, fmt.Errorf("misplaced codec footer (file truncated?): length=%d but footerLength==16", in.Length())
	}
	if err := in.SetPosition(in.Length() - int64(FooterLength())); err != nil {
		return 0, err
	}
	if err := validateFooter(in); err != nil {
		return 0, err
	}
	return ReadCRC(in)
}
