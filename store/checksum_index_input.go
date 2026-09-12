// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import "github.com/FlavioCFOliveira/Gocene/spi"

// The checksum-wrapping stores live on the SPI surface so that packages below
// store (which may not import store) can still frame a file with the exact
// CodecUtil envelope Lucene writes. This file is the store-package view of
// them: every declaration below is an alias of, or a delegate to, the single
// definition in spi/checksum_index.go.
//
// These are the Go ports of org.apache.lucene.store.ChecksumIndexInput and of
// the checksum-tracking IndexOutput Lucene builds inside CodecUtil.
//
// The store package previously carried a byte-for-byte copy of that file. The
// copy had drifted: it was missing the seven DataOutput-derived writers
// (WriteVInt, WriteVLong, WriteGroupVInts, WriteZInt, WriteZLong,
// WriteMapOfStrings, WriteSetOfStrings), so *store.ChecksumIndexOutput no
// longer satisfied spi.IndexOutput and every codec that framed a file through
// store.NewChecksumIndexOutput failed to compile. Aliasing removes the
// duplicate rather than re-diverging it.

// ChecksumType selects the checksum algorithm used by the wrappers below.
type ChecksumType = spi.ChecksumType

const (
	// ChecksumAdler32 uses the Adler32 algorithm (faster, less robust).
	ChecksumAdler32 = spi.ChecksumAdler32
	// ChecksumCRC32 uses the CRC32 algorithm (slower, more robust). This is
	// the algorithm Lucene's CodecUtil footer records.
	ChecksumCRC32 = spi.ChecksumCRC32
	// ChecksumXXHash32 uses the XXHash32 algorithm (very fast, good distribution).
	ChecksumXXHash32 = spi.ChecksumXXHash32
)

// ChecksumIndexInput is an IndexInput that computes a checksum over every byte
// it reads. Go port of org.apache.lucene.store.ChecksumIndexInput.
type ChecksumIndexInput = spi.ChecksumIndexInput

// NewChecksumIndexInput wraps input so that all bytes read are checksummed
// with CRC32.
func NewChecksumIndexInput(input IndexInput) *ChecksumIndexInput {
	return spi.NewChecksumIndexInput(input)
}

// NewChecksumIndexInputWithType wraps input with the given checksum algorithm.
func NewChecksumIndexInputWithType(input IndexInput, checksumType ChecksumType) *ChecksumIndexInput {
	return spi.NewChecksumIndexInputWithType(input, checksumType)
}

// ChecksumIndexOutput is an IndexOutput that computes a checksum over every
// byte it writes.
type ChecksumIndexOutput = spi.ChecksumIndexOutput

// NewChecksumIndexOutput wraps output so that all bytes written are
// checksummed with CRC32.
func NewChecksumIndexOutput(output IndexOutput) *ChecksumIndexOutput {
	return spi.NewChecksumIndexOutput(output)
}

// NewChecksumIndexOutputWithType wraps output with the given checksum algorithm.
func NewChecksumIndexOutputWithType(output IndexOutput, checksumType ChecksumType) *ChecksumIndexOutput {
	return spi.NewChecksumIndexOutputWithType(output, checksumType)
}

// ChecksumException reports a checksum mismatch between the computed and the
// expected value.
type ChecksumException = spi.ChecksumException

// NewChecksumException builds a ChecksumException for the given values.
func NewChecksumException(computed, expected uint32) *ChecksumException {
	return spi.NewChecksumException(computed, expected)
}

// ChecksumError reports a malformed checksum operation.
type ChecksumError = spi.ChecksumError

// NewChecksumError builds a ChecksumError with the given message.
func NewChecksumError(msg string) error {
	return spi.NewChecksumError(msg)
}

// ErrInvalidBuffer is returned when a checksum operation is handed a buffer it
// cannot use.
var ErrInvalidBuffer = spi.ErrInvalidBuffer
