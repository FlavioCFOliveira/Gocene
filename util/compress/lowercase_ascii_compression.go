// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package compress is the Go port of org.apache.lucene.util.compress.
//
// It contains LZ4 and LowercaseAsciiCompression, the two byte-level
// compressors Lucene uses internally for fast-path encoding of small
// payloads (stored fields, terms dictionaries).
package compress

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// isCompressible reports whether the byte b lies in the lowercase
// ASCII compressible ranges [0x1F, 0x3F) or [0x5F, 0x7F).
// Matches Lucene's static helper bit-for-bit.
func isCompressible(b int) bool {
	high3 := (b + 1) & ^0x1F
	return high3 == 0x20 || high3 == 0x60
}

// lowercaseAsciiInput is the minimal DataInput surface Decompress needs.
type lowercaseAsciiInput interface {
	store.DataInput
	ReadVInt() (int32, error)
}

// lowercaseAsciiOutput is the minimal DataOutput surface Compress needs.
type lowercaseAsciiOutput interface {
	store.DataOutput
	WriteVInt(i int32) error
}

// Compress tries to encode in[0:len_] into the output stream using
// Lucene's lowercase-ASCII scheme. It returns false (with no bytes
// written) when the input has too many exceptions; the caller should
// fall back to another encoding. When true is returned the number of
// bytes written is guaranteed to be < len_.
//
// tmp is a scratch buffer at least len_ bytes long; reusing it across
// calls avoids per-call allocations.
//
// This is the Go port of org.apache.lucene.util.compress.LowercaseAsciiCompression
// in Apache Lucene 10.4.0.
func Compress(in []byte, len_ int, tmp []byte, out lowercaseAsciiOutput) (bool, error) {
	if len_ < 8 {
		return false, nil
	}
	if len(tmp) < len_ {
		return false, fmt.Errorf("compress: tmp buffer must be at least len bytes, got %d need %d", len(tmp), len_)
	}

	// 1. Count exceptions; fail if there are more than len/32.
	maxExceptions := len_ >> 5
	previousExceptionIndex := 0
	numExceptions := 0
	for i := 0; i < len_; i++ {
		b := int(in[i]) & 0xFF
		if !isCompressible(b) {
			for i-previousExceptionIndex > 0xFF {
				numExceptions++
				previousExceptionIndex += 0xFF
			}
			numExceptions++
			if numExceptions > maxExceptions {
				return false, nil
			}
			previousExceptionIndex = i
		}
	}

	// 2. Pack into 6-bit codepoints in tmp[0:len_].
	compressedLen := len_ - (len_ >> 2)
	// Wipe tmp so the OR-folding below works (Lucene's array starts zero'd).
	for i := 0; i < len_; i++ {
		tmp[i] = 0
	}
	for i := 0; i < len_; i++ {
		b := (int(in[i]) & 0xFF) + 1
		tmp[i] = byte((b & 0x1F) | ((b & 0x40) >> 1))
	}

	// 3. Pack 4 ASCII chars into 3 bytes by folding bits 4-5, 2-3, 0-1
	//    of the tail into the top 2 bits of three slots in the head.
	o := 0
	for i := compressedLen; i < len_; i++ {
		tmp[o] |= byte((tmp[i] & 0x30) << 2)
		o++
	}
	for i := compressedLen; i < len_; i++ {
		tmp[o] |= byte((tmp[i] & 0x0C) << 4)
		o++
	}
	for i := compressedLen; i < len_; i++ {
		tmp[o] |= byte((tmp[i] & 0x03) << 6)
		o++
	}

	if err := out.WriteBytesN(tmp, compressedLen); err != nil {
		return false, err
	}

	// 4. Exceptions.
	if err := out.WriteVInt(int32(numExceptions)); err != nil {
		return false, err
	}
	if numExceptions > 0 {
		previousExceptionIndex = 0
		numExceptions2 := 0
		for i := 0; i < len_; i++ {
			b := int(in[i]) & 0xFF
			if !isCompressible(b) {
				for i-previousExceptionIndex > 0xFF {
					if err := out.WriteByte(0xFF); err != nil {
						return false, err
					}
					previousExceptionIndex += 0xFF
					if err := out.WriteByte(in[previousExceptionIndex]); err != nil {
						return false, err
					}
					numExceptions2++
				}
				if err := out.WriteByte(byte(i - previousExceptionIndex)); err != nil {
					return false, err
				}
				previousExceptionIndex = i
				if err := out.WriteByte(byte(b)); err != nil {
					return false, err
				}
				numExceptions2++
			}
		}
		if numExceptions != numExceptions2 {
			return false, fmt.Errorf("compress: exception count mismatch %d != %d", numExceptions, numExceptions2)
		}
	}
	return true, nil
}

// Decompress restores data previously written with Compress.
// len_ must be the ORIGINAL length, not the compressed byte count.
func Decompress(in lowercaseAsciiInput, out []byte, len_ int) error {
	if len(out) < len_ {
		return fmt.Errorf("compress: out buffer must be at least len bytes, got %d need %d", len(out), len_)
	}
	saved := len_ >> 2
	compressedLen := len_ - saved

	// 1. Copy the packed bytes back.
	if err := in.ReadBytes(out[:compressedLen]); err != nil {
		return err
	}

	// 2. Reconstruct the tail bytes by extracting their top 2 bits
	//    from three different slots in the head.
	for i := 0; i < saved; i++ {
		out[compressedLen+i] = byte(
			(int(out[i])&0xC0)>>2 |
				(int(out[saved+i])&0xC0)>>4 |
				(int(out[(saved<<1)+i])&0xC0)>>6)
	}

	// 3. Move back to the original ASCII range.
	for i := 0; i < len_; i++ {
		b := int(out[i])
		out[i] = byte(((b & 0x1F) | 0x20 | ((b & 0x20) << 1)) - 1)
	}

	// 4. Replay exceptions.
	numExceptions, err := in.ReadVInt()
	if err != nil {
		return err
	}
	i := 0
	for e := int32(0); e < numExceptions; e++ {
		delta, err := in.ReadByte()
		if err != nil {
			return err
		}
		i += int(delta) & 0xFF
		b, err := in.ReadByte()
		if err != nil {
			return err
		}
		out[i] = b
	}
	return nil
}
