// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
	"hash"
	"hash/adler32"
	"hash/crc32"
)

// ChecksumType represents the type of checksum algorithm to use.
type ChecksumType int

const (
	// ChecksumAdler32 uses Adler32 checksum algorithm (faster, less robust)
	ChecksumAdler32 ChecksumType = iota
	// ChecksumCRC32 uses CRC32 checksum algorithm (slower, more robust)
	ChecksumCRC32
)

// String returns the string representation of the checksum type.
func (c ChecksumType) String() string {
	switch c {
	case ChecksumAdler32:
		return "Adler32"
	case ChecksumCRC32:
		return "CRC32"
	default:
		return "Unknown"
	}
}

// ChecksumIndexInput wraps another IndexInput and computes a checksum
// of the data as it is read. This is useful for verifying data integrity.
//
// This is the Go port of Lucene's org.apache.lucene.store.ChecksumIndexInput.
type ChecksumIndexInput struct {
	*BaseIndexInput
	input    IndexInput
	digest   hash.Hash32
	checksum ChecksumType
}

// NewChecksumIndexInput creates a new ChecksumIndexInput wrapping the given input.
// By default, uses CRC32 for checksum calculation.
func NewChecksumIndexInput(input IndexInput) *ChecksumIndexInput {
	return NewChecksumIndexInputWithType(input, ChecksumCRC32)
}

// NewChecksumIndexInputWithType creates a new ChecksumIndexInput with the specified checksum type.
func NewChecksumIndexInputWithType(input IndexInput, checksumType ChecksumType) *ChecksumIndexInput {
	var digest hash.Hash32
	switch checksumType {
	case ChecksumAdler32:
		digest = adler32.New()
	case ChecksumCRC32:
		digest = crc32.NewIEEE()
	default:
		digest = crc32.NewIEEE()
	}

	return &ChecksumIndexInput{
		BaseIndexInput: NewBaseIndexInput("ChecksumIndexInput", input.Length()),
		input:          input,
		digest:         digest,
		checksum:       checksumType,
	}
}

// ReadByte reads a single byte and updates the checksum.
func (in *ChecksumIndexInput) ReadByte() (byte, error) {
	b, err := in.input.ReadByte()
	if err != nil {
		return 0, err
	}

	// Update checksum
	in.digest.Write([]byte{b})

	// Update file pointer
	in.SetFilePointer(in.GetFilePointer() + 1)

	return b, nil
}

// ReadBytes reads len(b) bytes and updates the checksum.
func (in *ChecksumIndexInput) ReadBytes(b []byte) error {
	err := in.input.ReadBytes(b)
	if err != nil {
		return err
	}

	// Update checksum
	in.digest.Write(b)

	// Update file pointer
	in.SetFilePointer(in.GetFilePointer() + int64(len(b)))

	return nil
}

// ReadBytesN reads exactly n bytes and returns them, updating the checksum.
func (in *ChecksumIndexInput) ReadBytesN(n int) ([]byte, error) {
	b := make([]byte, n)
	if err := in.ReadBytes(b); err != nil {
		return nil, err
	}
	return b, nil
}

// SetPosition changes the current position in the file.
// If the new position is ahead of the current position, it skips bytes
// to update the checksum. If it's behind, it resets the checksum.
func (in *ChecksumIndexInput) SetPosition(pos int64) error {
	current := in.GetFilePointer()
	if pos == current {
		return nil
	}
	if pos < current {
		if err := in.input.SetPosition(pos); err != nil {
			return err
		}
		// Reset the checksum digest since we cannot maintain checksum across backward seeks
		in.digest.Reset()
		in.SetFilePointer(pos)
		return nil
	}

	// Forward seek: skip bytes to update checksum
	return in.SkipBytes(pos - current)
}

// SkipBytes skips n bytes forward in the input and updates the checksum.
func (in *ChecksumIndexInput) SkipBytes(n int64) error {
	if n < 0 {
		return NewChecksumError("cannot skip negative bytes")
	}
	if n == 0 {
		return nil
	}

	// We must read the bytes to update the checksum
	buffer := make([]byte, 1024)
	for n > 0 {
		toRead := n
		if toRead > int64(len(buffer)) {
			toRead = int64(len(buffer))
		}
		if err := in.ReadBytes(buffer[:toRead]); err != nil {
			return err
		}
		n -= toRead
	}
	return nil
}

// GetChecksum returns the current checksum value.
func (in *ChecksumIndexInput) GetChecksum() uint32 {
	return in.digest.Sum32()
}

// GetChecksumType returns the type of checksum being used.
func (in *ChecksumIndexInput) GetChecksumType() ChecksumType {
	return in.checksum
}

// VerifyChecksum compares the computed checksum against the expected value.
// Returns nil if the checksums match, otherwise returns an error.
func (in *ChecksumIndexInput) VerifyChecksum(expected uint32) error {
	if in.GetChecksum() != expected {
		return NewChecksumException(in.GetChecksum(), expected)
	}
	return nil
}

// Clone returns a clone of this ChecksumIndexInput.
// Note: The cloned input will have a fresh checksum digest.
func (in *ChecksumIndexInput) Clone() IndexInput {
	clonedInput := in.input.Clone()
	clone := &ChecksumIndexInput{
		BaseIndexInput: NewBaseIndexInput("ChecksumIndexInput", in.Length()),
		input:          clonedInput,
		digest:         in.cloneDigest(),
		checksum:       in.checksum,
	}
	clone.SetFilePointer(in.GetFilePointer())
	return clone
}

// cloneDigest creates a new digest of the same type.
func (in *ChecksumIndexInput) cloneDigest() hash.Hash32 {
	switch in.checksum {
	case ChecksumAdler32:
		return adler32.New()
	case ChecksumCRC32:
		return crc32.NewIEEE()
	default:
		return crc32.NewIEEE()
	}
}

// Slice returns a subset of this IndexInput.
// Note: The sliced input will have a fresh checksum digest.
func (in *ChecksumIndexInput) Slice(desc string, offset int64, length int64) (IndexInput, error) {
	slicedInput, err := in.input.Slice(desc, offset, length)
	if err != nil {
		return nil, err
	}

	return &ChecksumIndexInput{
		BaseIndexInput: NewBaseIndexInput(desc, length),
		input:          slicedInput,
		digest:         in.cloneDigest(),
		checksum:       in.checksum,
	}, nil
}

// Close closes this ChecksumIndexInput and the underlying input.
func (in *ChecksumIndexInput) Close() error {
	return in.input.Close()
}

// Length returns the total length of the file.
func (in *ChecksumIndexInput) Length() int64 {
	return in.input.Length()
}

// GetWrappedInput returns the underlying IndexInput.
func (in *ChecksumIndexInput) GetWrappedInput() IndexInput {
	return in.input
}

// ChecksumException is returned when checksum verification fails.
type ChecksumException struct {
	Computed uint32
	Expected uint32
}

// NewChecksumException creates a new ChecksumException.
func NewChecksumException(computed, expected uint32) *ChecksumException {
	return &ChecksumException{
		Computed: computed,
		Expected: expected,
	}
}

// Error returns the error message.
func (e *ChecksumException) Error() string {
	return "checksum verification failed"
}

// ChecksumIndexOutput wraps another IndexOutput and computes a checksum
// of the data as it is written. This is useful for verifying data integrity on read.
//
// This is the Go port of Lucene's org.apache.lucene.store.ChecksumIndexOutput.
type ChecksumIndexOutput struct {
	*BaseIndexOutput
	output   IndexOutput
	digest   hash.Hash32
	checksum ChecksumType
}

// NewChecksumIndexOutput creates a new ChecksumIndexOutput wrapping the given output.
// By default, uses CRC32 for checksum calculation.
func NewChecksumIndexOutput(output IndexOutput) *ChecksumIndexOutput {
	return NewChecksumIndexOutputWithType(output, ChecksumCRC32)
}

// NewChecksumIndexOutputWithType creates a new ChecksumIndexOutput with the specified checksum type.
func NewChecksumIndexOutputWithType(output IndexOutput, checksumType ChecksumType) *ChecksumIndexOutput {
	var digest hash.Hash32
	switch checksumType {
	case ChecksumAdler32:
		digest = adler32.New()
	case ChecksumCRC32:
		digest = crc32.NewIEEE()
	default:
		digest = crc32.NewIEEE()
	}

	return &ChecksumIndexOutput{
		BaseIndexOutput: NewBaseIndexOutput(output.GetName()),
		output:          output,
		digest:          digest,
		checksum:        checksumType,
	}
}

// WriteByte writes a single byte and updates the checksum.
func (out *ChecksumIndexOutput) WriteByte(b byte) error {
	if err := out.output.WriteByte(b); err != nil {
		return err
	}

	// Update checksum
	out.digest.Write([]byte{b})

	out.IncrementFilePointer(1)
	return nil
}

// WriteBytes writes all bytes from b and updates the checksum.
func (out *ChecksumIndexOutput) WriteBytes(b []byte) error {
	if err := out.output.WriteBytes(b); err != nil {
		return err
	}

	// Update checksum
	out.digest.Write(b)

	out.IncrementFilePointer(int64(len(b)))
	return nil
}

// WriteBytesN writes exactly n bytes from b and updates the checksum.
func (out *ChecksumIndexOutput) WriteBytesN(b []byte, n int) error {
	if n > len(b) {
		return ErrInvalidBuffer
	}
	return out.WriteBytes(b[:n])
}

// Length returns the current length of the file.
func (out *ChecksumIndexOutput) Length() int64 {
	return out.output.Length()
}

// GetChecksum returns the current checksum value.
func (out *ChecksumIndexOutput) GetChecksum() uint32 {
	return out.digest.Sum32()
}

// GetChecksumType returns the type of checksum being used.
func (out *ChecksumIndexOutput) GetChecksumType() ChecksumType {
	return out.checksum
}

// Close closes this ChecksumIndexOutput and the underlying output.
func (out *ChecksumIndexOutput) Close() error {
	return out.output.Close()
}

// GetWrappedOutput returns the underlying IndexOutput.
func (out *ChecksumIndexOutput) GetWrappedOutput() IndexOutput {
	return out.output
}

// ErrInvalidBuffer is returned when buffer operations fail.
var ErrInvalidBuffer = NewChecksumError("invalid buffer")

// ChecksumError represents a checksum-related error.
type ChecksumError struct {
	msg string
}

// NewChecksumError creates a new ChecksumError.
func NewChecksumError(msg string) error {
	return &ChecksumError{msg: msg}
}

// Error returns the error message.
func (e *ChecksumError) Error() string {
	return e.msg
}
