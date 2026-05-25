// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"bytes"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// codecMagic is the magic number written at the start of every Lucene index file.
// Mirrors org.apache.lucene.codecs.CodecUtil.MAGIC = 0x3FD76C17.
const codecMagic int32 = 0x3FD76C17

// footerMagic is the magic number written in codec footers.
// Mirrors org.apache.lucene.codecs.CodecUtil.FOOTER_MAGIC = ^0x3FD76C17.
const footerMagic int32 = ^0x3FD76C17

// writeIndexHeader writes a codec index header: magic (int32) + codec name
// (string) + version (int32) + id (16 bytes) + suffix (len byte + bytes).
// Byte-for-byte identical to org.apache.lucene.codecs.CodecUtil.writeIndexHeader.
func writeIndexHeader(out store.IndexOutput, codec string, version int32, id []byte, suffix string) error {
	if len(id) != 16 {
		return fmt.Errorf("writeIndexHeader: id length must be 16, got %d", len(id))
	}
	if len(suffix) > 255 {
		return fmt.Errorf("writeIndexHeader: suffix too long (%d > 255)", len(suffix))
	}
	// Write magic
	if err := store.WriteInt32(out, codecMagic); err != nil {
		return err
	}
	// Write codec name
	if err := store.WriteString(out, codec); err != nil {
		return err
	}
	// Write version
	if err := store.WriteInt32(out, version); err != nil {
		return err
	}
	// Write 16-byte ID
	if err := out.WriteBytes(id); err != nil {
		return err
	}
	// Write suffix: 1-byte length + bytes
	if err := out.WriteByte(byte(len(suffix))); err != nil {
		return err
	}
	if len(suffix) > 0 {
		if err := out.WriteBytes([]byte(suffix)); err != nil {
			return err
		}
	}
	return nil
}

// checkIndexHeader reads and validates a codec index header.
// Returns the version number on success.
// Mirrors org.apache.lucene.codecs.CodecUtil.checkIndexHeader.
func checkIndexHeader(in store.IndexInput, codec string, minVersion, maxVersion int32, expectedID []byte, expectedSuffix string) (int32, error) {
	magic, err := store.ReadInt32(in)
	if err != nil {
		return 0, err
	}
	if magic != codecMagic {
		return 0, fmt.Errorf("checkIndexHeader: invalid magic 0x%x (expected 0x%x)", magic, codecMagic)
	}
	actualCodec, err := store.ReadString(in)
	if err != nil {
		return 0, err
	}
	if actualCodec != codec {
		return 0, fmt.Errorf("checkIndexHeader: codec mismatch %q (expected %q)", actualCodec, codec)
	}
	version, err := store.ReadInt32(in)
	if err != nil {
		return 0, err
	}
	if version < minVersion || version > maxVersion {
		return 0, fmt.Errorf("checkIndexHeader: version %d out of range [%d, %d]", version, minVersion, maxVersion)
	}
	id, err := in.ReadBytesN(16)
	if err != nil {
		return 0, err
	}
	if expectedID != nil && !bytes.Equal(id, expectedID) {
		return 0, fmt.Errorf("checkIndexHeader: segment ID mismatch")
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
		return 0, fmt.Errorf("checkIndexHeader: suffix mismatch %q (expected %q)", suffix, expectedSuffix)
	}
	return version, nil
}

// writeFooter writes the codec footer: footerMagic (int32) + algo=0 (int32) +
// CRC32 checksum (int64). out must be a *store.ChecksumIndexOutput.
// Mirrors org.apache.lucene.codecs.CodecUtil.writeFooter.
func writeFooter(out *store.ChecksumIndexOutput) error {
	if err := store.WriteInt32(out, footerMagic); err != nil {
		return err
	}
	if err := store.WriteInt32(out, 0); err != nil { // algo = CRC32
		return err
	}
	checksum := out.GetChecksum()
	return store.WriteInt64(out, int64(checksum))
}

// checkFooter validates the codec footer and returns the checksum.
// in must be positioned just before the footer (footerMagic field).
// Mirrors org.apache.lucene.codecs.CodecUtil.checkFooter.
func checkFooter(in *store.ChecksumIndexInput) (int64, error) {
	magic, err := store.ReadInt32(in)
	if err != nil {
		return 0, err
	}
	if magic != footerMagic {
		return 0, fmt.Errorf("checkFooter: invalid footer magic 0x%x (expected 0x%x)", magic, footerMagic)
	}
	algo, err := store.ReadInt32(in)
	if err != nil {
		return 0, err
	}
	if algo != 0 {
		return 0, fmt.Errorf("checkFooter: unknown checksum algorithm %d", algo)
	}
	actualChecksum := in.GetChecksum()
	expectedChecksum, err := store.ReadInt64(in)
	if err != nil {
		return 0, err
	}
	if int64(actualChecksum) != expectedChecksum {
		return 0, fmt.Errorf("checkFooter: checksum mismatch (actual 0x%x, expected 0x%x)", actualChecksum, expectedChecksum)
	}
	return expectedChecksum, nil
}
