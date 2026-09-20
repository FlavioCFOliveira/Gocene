// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
	"bytes"
	"errors"
	"fmt"
)

// This file is the Go port of org.apache.lucene.codecs.CodecUtil
// (Apache Lucene 10.5.0, lucene/core/src/java/org/apache/lucene/codecs/CodecUtil.java).
//
// Byte order, read this before changing anything here: Lucene's
// DataOutput.writeInt / DataOutput.writeLong are LITTLE-endian (DataOutput.java:73,223),
// but every fixed-width field of a codec HEADER and of a codec FOOTER is written
// through CodecUtil.writeBEInt / CodecUtil.writeBELong and is therefore
// BIG-endian (CodecUtil.java:83,85,308,310,410,411,653,661). The magic number
// 0x3FD76C17 consequently appears on disk as the bytes 3f d7 6c 17, which is
// exactly how every file Apache Lucene writes begins.

const (
	// CODEC_MAGIC is the constant that identifies the start of a codec header.
	//
	// Port of CodecUtil.CODEC_MAGIC (CodecUtil.java:46).
	CODEC_MAGIC int32 = 0x3FD76C17
	// FOOTER_MAGIC is the constant that identifies the start of a codec footer.
	//
	// Port of CodecUtil.FOOTER_MAGIC (CodecUtil.java:49).
	FOOTER_MAGIC int32 = ^0x3FD76C17
)

// idLength is the length of a segment/object identifier, mirroring
// org.apache.lucene.util.StringHelper.ID_LENGTH.
const idLength = 16

// checksumWriter is the Go rendering of the getChecksum() method Lucene declares
// on org.apache.lucene.store.IndexOutput. Gocene's spi.IndexOutput does not carry
// it, so CodecUtil recovers it through a type assertion.
type checksumWriter interface {
	IndexOutput
	GetChecksum() uint32
}

// HeaderLength computes the length of a codec header.
//
// Port of CodecUtil.headerLength (CodecUtil.java:144).
func HeaderLength(codec string) int {
	return 9 + len(codec)
}

// IndexHeaderLength computes the length of an index header.
//
// Port of CodecUtil.indexHeaderLength (CodecUtil.java:155).
func IndexHeaderLength(codec string, suffix string) int {
	return HeaderLength(codec) + idLength + 1 + len(suffix)
}

// FooterLength computes the length of a codec footer.
//
// Port of CodecUtil.footerLength (CodecUtil.java:421).
func FooterLength() int {
	return 16
}

// WriteHeader writes a codec header, which records both a string to identify the
// file and a version number.
//
//	CodecHeader --> Magic,CodecName,Version
//
// Magic is a big-endian uint32, CodecName is a Lucene string (vInt length +
// UTF-8 bytes) and Version is a big-endian uint32.
//
// Port of CodecUtil.writeHeader (CodecUtil.java:77-86).
func WriteHeader(out DataOutput, codec string, version int32) error {
	if err := checkCodecName(codec); err != nil {
		return err
	}
	if err := WriteBEInt(out, CODEC_MAGIC); err != nil {
		return err
	}
	if err := out.WriteString(codec); err != nil {
		return err
	}
	return WriteBEInt(out, version)
}

// WriteIndexHeader writes a codec header for an index file, which records a
// string identifying the format, a version number, and data identifying the file
// instance (a 16-byte ID and an auxiliary suffix such as a generation).
//
//	IndexHeader --> CodecHeader,ObjectID,ObjectSuffix
//
// Port of CodecUtil.writeIndexHeader (CodecUtil.java:121-135).
func WriteIndexHeader(out DataOutput, codec string, version int32, id []byte, suffix string) error {
	if len(id) != idLength {
		return fmt.Errorf("Invalid id: %s", idToString(id))
	}
	if err := WriteHeader(out, codec, version); err != nil {
		return err
	}
	if err := out.WriteBytes(id, 0, len(id)); err != nil {
		return err
	}
	return writeSuffix(out, suffix)
}

// CodecUtilWriteIndexHeader is an alias for WriteIndexHeader.
func CodecUtilWriteIndexHeader(out IndexOutput, codec string, version int32, id []byte, suffix string) error {
	return WriteIndexHeader(out, codec, version, id, suffix)
}

// writeSuffix writes the length-prefixed object suffix of an index header.
//
// Port of the tail of CodecUtil.writeIndexHeader (CodecUtil.java:129-134).
func writeSuffix(out DataOutput, suffix string) error {
	if !isSimpleASCII(suffix) || len(suffix) >= 256 {
		return fmt.Errorf("suffix must be simple ASCII, less than 256 characters in length [got %s]", suffix)
	}
	if err := out.WriteByte(byte(len(suffix))); err != nil {
		return err
	}
	return out.WriteBytes([]byte(suffix), 0, len(suffix))
}

// checkCodecName reproduces the codec-name validation of CodecUtil.writeHeader
// (CodecUtil.java:78-82).
func checkCodecName(codec string) error {
	if !isSimpleASCII(codec) || len(codec) >= 128 {
		return fmt.Errorf("codec must be simple ASCII, less than 128 characters in length [got %s]", codec)
	}
	return nil
}

// isSimpleASCII reports whether s encodes to one byte per character, which is
// Lucene's `bytes.length != s.length()` test expressed for Go's UTF-8 strings.
func isSimpleASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] > 127 {
			return false
		}
	}
	return true
}

// idToString renders an object ID the way org.apache.lucene.util.StringHelper
// does for diagnostics.
func idToString(id []byte) string {
	if id == nil {
		return "(null)"
	}
	return fmt.Sprintf("%x", id)
}

// CheckHeader reads and validates a header previously written with WriteHeader.
//
// Port of CodecUtil.checkHeader (CodecUtil.java:182-194).
func CheckHeader(in DataInput, codec string, minVersion, maxVersion int32) (int32, error) {
	// Safety to guard against reading a bogus string:
	actualHeader, err := ReadBEInt(in)
	if err != nil {
		return 0, err
	}
	if actualHeader != CODEC_MAGIC {
		return 0, fmt.Errorf("codec header mismatch: actual header=%d vs expected header=%d", actualHeader, CODEC_MAGIC)
	}
	return CheckHeaderNoMagic(in, codec, minVersion, maxVersion)
}

// CheckHeaderNoMagic is like CheckHeader except that it assumes the first int
// has already been read and validated from the input.
//
// Port of CodecUtil.checkHeaderNoMagic (CodecUtil.java:201-217).
func CheckHeaderNoMagic(in DataInput, codec string, minVersion, maxVersion int32) (int32, error) {
	actualCodec, err := in.ReadString()
	if err != nil {
		return 0, err
	}
	if actualCodec != codec {
		return 0, fmt.Errorf("codec mismatch: actual codec=%s vs expected codec=%s", actualCodec, codec)
	}

	actualVersion, err := ReadBEInt(in)
	if err != nil {
		return 0, err
	}
	if actualVersion < minVersion {
		return 0, fmt.Errorf("Format version is not supported (resource=%s): %d (needs to be between %d and %d)", codec, actualVersion, minVersion, maxVersion)
	}
	if actualVersion > maxVersion {
		return 0, fmt.Errorf("Format version is not supported (resource=%s): %d (needs to be between %d and %d)", codec, actualVersion, minVersion, maxVersion)
	}

	return actualVersion, nil
}

// CheckIndexHeader reads and validates a header previously written with
// WriteIndexHeader.
//
// Port of CodecUtil.checkIndexHeader (CodecUtil.java:246-257).
func CheckIndexHeader(in DataInput, codec string, minVersion, maxVersion int32, expectedID []byte, expectedSuffix string) (int32, error) {
	version, err := CheckHeader(in, codec, minVersion, maxVersion)
	if err != nil {
		return 0, err
	}
	if _, err := CheckIndexHeaderID(in, expectedID); err != nil {
		return 0, err
	}
	if _, err := CheckIndexHeaderSuffix(in, expectedSuffix); err != nil {
		return 0, err
	}
	return version, nil
}

// CheckIndexHeaderID reads and verifies the object ID of an index header.
//
// Port of CodecUtil.checkIndexHeaderID (CodecUtil.java:363-372).
func CheckIndexHeaderID(in DataInput, expectedID []byte) ([]byte, error) {
	id := make([]byte, idLength)
	if err := in.ReadBytes(id, 0, len(id)); err != nil {
		return nil, err
	}
	// Gocene keeps Lucene's semantics but tolerates a nil expectation, which the
	// module already relies on to mean "do not verify the id".
	if expectedID != nil && !bytes.Equal(id, expectedID) {
		return nil, fmt.Errorf("file mismatch, expected id=%s, got=%s", idToString(expectedID), idToString(id))
	}
	return id, nil
}

// CheckIndexHeaderSuffix reads and verifies the suffix of an index header.
//
// Port of CodecUtil.checkIndexHeaderSuffix (CodecUtil.java:378-387).
func CheckIndexHeaderSuffix(in DataInput, expectedSuffix string) (string, error) {
	suffixLength, err := in.ReadByte()
	if err != nil {
		return "", err
	}
	suffixBytes := make([]byte, int(suffixLength))
	if err := in.ReadBytes(suffixBytes, 0, len(suffixBytes)); err != nil {
		return "", err
	}
	suffix := string(suffixBytes)
	if suffix != expectedSuffix {
		return "", fmt.Errorf("file mismatch, expected suffix=%s, got=%s", expectedSuffix, suffix)
	}
	return suffix, nil
}

// VerifyAndCopyIndexHeader verifies that in carries an index header whose
// segment ID matches expectedID, then copies that header into out. This is what
// Lucene uses when building compound files.
//
// Port of CodecUtil.verifyAndCopyIndexHeader (CodecUtil.java:274-313).
func VerifyAndCopyIndexHeader(in IndexInput, out DataOutput, expectedID []byte) error {
	// make sure it's large enough to have a header and footer
	if in.Length() < int64(FooterLength()+HeaderLength("")) {
		return fmt.Errorf("compound sub-files must have a valid codec header and footer: file is too small (%d bytes)", in.Length())
	}

	actualHeader, err := ReadBEInt(in)
	if err != nil {
		return err
	}
	if actualHeader != CODEC_MAGIC {
		return fmt.Errorf("compound sub-files must have a valid codec header and footer: codec header mismatch: actual header=%d vs expected header=%d", actualHeader, CODEC_MAGIC)
	}

	// we can't verify these, so we pass-through:
	codec, err := in.ReadString()
	if err != nil {
		return err
	}
	version, err := ReadBEInt(in)
	if err != nil {
		return err
	}

	// verify id:
	if _, err := CheckIndexHeaderID(in, expectedID); err != nil {
		return err
	}

	// we can't verify extension either, so we pass-through:
	suffixLengthByte, err := in.ReadByte()
	if err != nil {
		return err
	}
	suffixLength := int(suffixLengthByte) & 0xFF
	suffixBytes := make([]byte, suffixLength)
	if err := in.ReadBytes(suffixBytes, 0, suffixLength); err != nil {
		return err
	}

	// now write the header we just verified
	if err := WriteBEInt(out, CODEC_MAGIC); err != nil {
		return err
	}
	if err := out.WriteString(codec); err != nil {
		return err
	}
	if err := WriteBEInt(out, version); err != nil {
		return err
	}
	if err := out.WriteBytes(expectedID, 0, len(expectedID)); err != nil {
		return err
	}
	if err := out.WriteByte(byte(suffixLength)); err != nil {
		return err
	}
	return out.WriteBytes(suffixBytes, 0, suffixLength)
}

// ReadIndexHeader retrieves the full index header from in.
//
// Port of CodecUtil.readIndexHeader (CodecUtil.java:320-336).
func ReadIndexHeader(in IndexInput) ([]byte, error) {
	if err := in.SetPosition(0); err != nil {
		return nil, err
	}
	actualHeader, err := ReadBEInt(in)
	if err != nil {
		return nil, err
	}
	if actualHeader != CODEC_MAGIC {
		return nil, fmt.Errorf("codec header mismatch: actual header=%d vs expected header=%d", actualHeader, CODEC_MAGIC)
	}
	codec, err := in.ReadString()
	if err != nil {
		return nil, err
	}
	if _, err := ReadBEInt(in); err != nil {
		return nil, err
	}
	if err := in.SetPosition(in.GetFilePointer() + idLength); err != nil {
		return nil, err
	}
	suffixLengthByte, err := in.ReadByte()
	if err != nil {
		return nil, err
	}
	suffixLength := int(suffixLengthByte) & 0xFF
	b := make([]byte, HeaderLength(codec)+idLength+1+suffixLength)
	if err := in.SetPosition(0); err != nil {
		return nil, err
	}
	if err := in.ReadBytes(b, 0, len(b)); err != nil {
		return nil, err
	}
	return b, nil
}

// ReadFooter retrieves the full footer from in.
//
// Port of CodecUtil.readFooter (CodecUtil.java:345-356).
func ReadFooter(in IndexInput) ([]byte, error) {
	if in.Length() < int64(FooterLength()) {
		return nil, fmt.Errorf("misplaced codec footer (file truncated?): length=%d but footerLength==%d", in.Length(), FooterLength())
	}
	if err := in.SetPosition(in.Length() - int64(FooterLength())); err != nil {
		return nil, err
	}
	if err := validateFooter(in); err != nil {
		return nil, err
	}
	if err := in.SetPosition(in.Length() - int64(FooterLength())); err != nil {
		return nil, err
	}
	b := make([]byte, FooterLength())
	if err := in.ReadBytes(b, 0, len(b)); err != nil {
		return nil, err
	}
	return b, nil
}

// WriteFooter writes a codec footer, which records a checksum algorithm ID and a
// checksum.
//
//	CodecFooter --> Magic,AlgorithmID,Checksum
//
// Magic and AlgorithmID are big-endian uint32; Checksum is a big-endian uint64.
//
// Port of CodecUtil.writeFooter (CodecUtil.java:409-413).
func WriteFooter(out IndexOutput) error {
	if err := WriteBEInt(out, FOOTER_MAGIC); err != nil {
		return err
	}
	if err := WriteBEInt(out, 0); err != nil {
		return err
	}
	return WriteCRC(out)
}

// CodecUtilWriteFooter is an alias for WriteFooter.
func CodecUtilWriteFooter(out IndexOutput) error {
	return WriteFooter(out)
}

// WriteCRC writes the output's CRC-32 value as a big-endian 64-bit long.
//
// Port of CodecUtil.writeCRC (CodecUtil.java:643-650).
func WriteCRC(out IndexOutput) error {
	cw, ok := out.(checksumWriter)
	if !ok {
		return fmt.Errorf("output does not support checksums")
	}
	value := int64(cw.GetChecksum())
	if (value & ^int64(0xFFFFFFFF)) != 0 {
		return fmt.Errorf("Illegal CRC-32 checksum: %d (resource=%v)", value, out)
	}
	return WriteBELong(out, value)
}

// ReadCRC reads a CRC-32 value as a big-endian 64-bit long from the input.
//
// Port of CodecUtil.readCRC (CodecUtil.java:629-635).
func ReadCRC(in DataInput) (int64, error) {
	value, err := ReadBELong(in)
	if err != nil {
		return 0, err
	}
	if (value & ^int64(0xFFFFFFFF)) != 0 {
		return 0, fmt.Errorf("Illegal CRC-32 checksum: %d", value)
	}
	return value, nil
}

// CheckFooter validates the codec footer previously written by WriteFooter and
// returns the actual checksum value.
//
// Port of CodecUtil.checkFooter(ChecksumIndexInput) (CodecUtil.java:432-444).
func CheckFooter(in *ChecksumIndexInput) (int64, error) {
	if err := validateFooter(in); err != nil {
		return 0, err
	}

	actualChecksum := int64(in.GetChecksum())
	expectedChecksum, err := ReadCRC(in)
	if err != nil {
		return 0, err
	}

	if expectedChecksum != actualChecksum {
		return 0, fmt.Errorf("checksum failed (hardware problem?) : expected=%x actual=%x", expectedChecksum, actualChecksum)
	}

	return actualChecksum, nil
}

// CheckFooterWithPriorError validates the codec footer previously written by
// WriteFooter, optionally passing an error that has already occurred.
//
// When priorError is non-nil this reports whether the checksum for the stream
// passes, fails, or cannot be computed, and returns that verdict joined to the
// prior error. Otherwise it behaves the same as CheckFooter.
//
// Go rendering of CodecUtil.checkFooter(ChecksumIndexInput, Throwable)
// (CodecUtil.java:470-514): Java's Throwable.addSuppressed is rendered as
// errors.Join, and IOUtils.rethrowAlways(priorException) as returning the
// joined error whose first member is the one Java would have rethrown.
func CheckFooterWithPriorError(in *ChecksumIndexInput, priorError error) error {
	if priorError == nil {
		_, err := CheckFooter(in)
		return err
	}

	// If we have evidence of corruption then we return the corruption as the
	// main error and the prior error is suppressed. Otherwise we return the
	// prior error with a suppressed error that notifies the user that
	// checksums matched.
	remaining := in.Length() - in.GetFilePointer()
	if remaining < int64(FooterLength()) {
		// corruption caused us to read into the checksum footer already: we can't proceed
		return errors.Join(
			fmt.Errorf("checksum status indeterminate: remaining=%d; please run checkindex for more details", remaining),
			priorError,
		)
	}

	// otherwise, skip any unread bytes.
	if err := in.SkipBytes(remaining - int64(FooterLength())); err != nil {
		// catch-all for things that shouldn't go wrong but could...
		return errors.Join(priorError, fmt.Errorf("checksum status indeterminate: unexpected error: %w", err))
	}

	// now check the footer
	checksum, err := CheckFooter(in)
	if err != nil {
		return errors.Join(err, priorError)
	}
	return errors.Join(
		priorError,
		fmt.Errorf("checksum passed (%x). possibly transient resource issue, or a Lucene or JVM bug", checksum),
	)
}

// validateFooter checks the position, the footer magic and the algorithm ID.
//
// Port of CodecUtil.validateFooter (CodecUtil.java:560-597).
func validateFooter(in IndexInput) error {
	remaining := in.Length() - in.GetFilePointer()
	expected := int64(FooterLength())
	if remaining < expected {
		return fmt.Errorf("misplaced codec footer (file truncated?): remaining=%d, expected=%d, fp=%d", remaining, expected, in.GetFilePointer())
	} else if remaining > expected {
		return fmt.Errorf("misplaced codec footer (file extended?): remaining=%d, expected=%d, fp=%d", remaining, expected, in.GetFilePointer())
	}

	magic, err := ReadBEInt(in)
	if err != nil {
		return err
	}
	if magic != FOOTER_MAGIC {
		return fmt.Errorf("codec footer mismatch (file truncated?): actual footer=%d vs expected footer=%d", magic, FOOTER_MAGIC)
	}

	algorithmID, err := ReadBEInt(in)
	if err != nil {
		return err
	}
	if algorithmID != 0 {
		return fmt.Errorf("codec footer mismatch: unknown algorithmID: %d", algorithmID)
	}
	return nil
}

// ChecksumEntireFile clones input, reads every byte of the file, and calls
// CheckFooter.
//
// Port of CodecUtil.checksumEntireFile (CodecUtil.java:606-620).
func ChecksumEntireFile(input IndexInput) (int64, error) {
	clone := input.Clone()
	if err := clone.SetPosition(0); err != nil {
		return 0, err
	}
	in := NewChecksumIndexInput(clone)
	defer in.Close()

	if in.Length() < int64(FooterLength()) {
		return 0, fmt.Errorf("misplaced codec footer (file truncated?): length=%d but footerLength==%d", in.Length(), FooterLength())
	}

	if err := in.SetPosition(in.Length() - int64(FooterLength())); err != nil {
		return 0, err
	}
	return CheckFooter(in)
}

// RetrieveChecksum returns (but does not validate) the checksum previously
// written by WriteFooter.
//
// Port of CodecUtil.retrieveChecksum(IndexInput) (CodecUtil.java:525-535).
func RetrieveChecksum(in IndexInput) (int64, error) {
	if in.Length() < int64(FooterLength()) {
		return 0, fmt.Errorf("misplaced codec footer (file truncated?): length=%d but footerLength==%d", in.Length(), FooterLength())
	}
	if err := in.SetPosition(in.Length() - int64(FooterLength())); err != nil {
		return 0, err
	}
	if err := validateFooter(in); err != nil {
		return 0, err
	}
	return ReadCRC(in)
}

// RetrieveChecksumWithExpectedLength returns (but does not validate) the
// checksum previously written by WriteFooter, first asserting that the file has
// exactly the expected length.
//
// Port of CodecUtil.retrieveChecksum(IndexInput, long) (CodecUtil.java:545-558).
func RetrieveChecksumWithExpectedLength(in IndexInput, expectedLength int64) (int64, error) {
	if expectedLength < int64(FooterLength()) {
		return 0, fmt.Errorf("expectedLength cannot be less than the footer length")
	}
	if in.Length() < expectedLength {
		return 0, fmt.Errorf("truncated file: length=%d but expectedLength==%d", in.Length(), expectedLength)
	} else if in.Length() > expectedLength {
		return 0, fmt.Errorf("file too long: length=%d but expectedLength==%d", in.Length(), expectedLength)
	}
	return RetrieveChecksum(in)
}

// GetSegmentFileName returns the file name for a segment component.
func GetSegmentFileName(segmentName string, segmentSuffix string, ext string) string {
	return IndexFileNamesSegmentFileName(segmentName, segmentSuffix, ext)
}

// IndexFileNamesSegmentFileName is an alias for GetSegmentFileName.
func IndexFileNamesSegmentFileName(segmentName string, segmentSuffix string, ext string) string {
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

// WriteInt32 writes a 32-bit value in Lucene's LITTLE-endian DataOutput.writeInt
// encoding (DataOutput.java:73). It is NOT the codec header/footer encoding —
// those use WriteBEInt.
func WriteInt32(out DataOutput, v int32) error {
	return out.WriteInt(v)
}

// ReadInt32 reads a 32-bit value in Lucene's LITTLE-endian DataInput.readInt
// encoding. It is NOT the codec header/footer encoding — those use ReadBEInt.
func ReadInt32(in DataInput) (int32, error) {
	return in.ReadInt()
}

// WriteInt64 writes a 64-bit value in Lucene's LITTLE-endian
// DataOutput.writeLong encoding (DataOutput.java:223). It is NOT the codec
// footer checksum encoding — that uses WriteBELong.
func WriteInt64(out DataOutput, v int64) error {
	return out.WriteLong(v)
}

// ReadInt64 reads a 64-bit value in Lucene's LITTLE-endian DataInput.readLong
// encoding. It is NOT the codec footer checksum encoding — that uses ReadBELong.
func ReadInt64(in DataInput) (int64, error) {
	return in.ReadLong()
}

// WriteString writes a Lucene string (vInt length + UTF-8 bytes).
func WriteString(out DataOutput, s string) error {
	return out.WriteString(s)
}

// ReadString reads a Lucene string (vInt length + UTF-8 bytes).
func ReadString(in DataInput) (string, error) {
	return in.ReadString()
}

// WriteBEInt writes an int value on header / footer with big endian order.
//
// Port of CodecUtil.writeBEInt (CodecUtil.java:653-658).
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
// Port of CodecUtil.writeBELong (CodecUtil.java:661-664).
func WriteBELong(out DataOutput, l int64) error {
	if err := WriteBEInt(out, int32(l>>32)); err != nil {
		return err
	}
	return WriteBEInt(out, int32(l))
}

// ReadBEInt reads an int value from header / footer with big endian order.
//
// Port of CodecUtil.readBEInt (CodecUtil.java:667-672).
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
// Port of CodecUtil.readBELong (CodecUtil.java:675-677).
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
