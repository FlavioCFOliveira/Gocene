// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package blocktree

import (
	"fmt"

	bcstore "github.com/FlavioCFOliveira/Gocene/backward_codecs/store"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util/compress"
)

// CompressionAlgorithm identifies the compression scheme used for term-suffix
// bytes within a block of the Lucene 4.0 block-tree terms dictionary.
//
// Port of the package-private enum
// org.apache.lucene.backward_codecs.lucene40.blocktree.CompressionAlgorithm
// (Lucene 10.4.0).
type CompressionAlgorithm int

const (
	// CompressionNone means suffix bytes are stored without compression.
	CompressionNone CompressionAlgorithm = 0x00

	// CompressionLowercaseASCII means suffixes are compressed with the
	// lowercase-ASCII scheme (only valid when all bytes are in [a-z]).
	CompressionLowercaseASCII CompressionAlgorithm = 0x01

	// CompressionLZ4 means suffixes are compressed with LZ4.
	CompressionLZ4 CompressionAlgorithm = 0x02
)

// Code returns the wire code of the algorithm.
func (c CompressionAlgorithm) Code() int { return int(c) }

// String returns a human-readable name for the algorithm.
func (c CompressionAlgorithm) String() string {
	switch c {
	case CompressionNone:
		return "NO_COMPRESSION"
	case CompressionLowercaseASCII:
		return "LOWERCASE_ASCII"
	case CompressionLZ4:
		return "LZ4"
	default:
		return fmt.Sprintf("CompressionAlgorithm(%d)", int(c))
	}
}

// CompressionAlgorithmByCode returns the CompressionAlgorithm for the given
// wire code, or an error if the code is unknown.
func CompressionAlgorithmByCode(code int) (CompressionAlgorithm, error) {
	switch CompressionAlgorithm(code) {
	case CompressionNone, CompressionLowercaseASCII, CompressionLZ4:
		return CompressionAlgorithm(code), nil
	default:
		return 0, fmt.Errorf("blocktree: illegal compression algorithm code: %d", code)
	}
}

// compressInput is the minimal surface required for decompression.
// It is satisfied by any store.IndexInput (all of which implement ReadVInt).
type compressInput interface {
	store.DataInput
	ReadVInt() (int32, error)
}

// Decompress reads compressed data from in into out[0:length].
// The exact behaviour mirrors CompressionAlgorithm.read() in Java.
func (c CompressionAlgorithm) Decompress(in compressInput, out []byte, length int) error {
	switch c {
	case CompressionNone:
		return in.ReadBytes(out[:length])
	case CompressionLowercaseASCII:
		return compress.Decompress(in, out, length)
	case CompressionLZ4:
		// Java wraps in with EndiannessReverserUtil.wrapDataInput for LZ4
		// because the LZ4 framing is big-endian in the Lucene 4.0 format.
		wrapped := bcstore.NewEndiannessReverserDataInput(in)
		_, err := compress.LZ4Decompress(wrapped, length, out, 0)
		return err
	default:
		return fmt.Errorf("blocktree: unknown compression algorithm %d", int(c))
	}
}
