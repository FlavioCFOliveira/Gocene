// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License. You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

package codecs

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util/compress"
)

// CompressionAlgorithm enumerates the compression schemes that may be applied
// to the term-suffix bytes inside a Lucene103 block-tree block. It is the Go
// port of org.apache.lucene.codecs.lucene103.blocktree.CompressionAlgorithm
// from Apache Lucene 10.4.0.
//
// Reference: lucene/core/src/java/org/apache/lucene/codecs/lucene103/blocktree/
// CompressionAlgorithm.java
//
// The wire-format code values are fixed (0x00 / 0x01 / 0x02) and must not
// change: they are stored inline with every block header.
type CompressionAlgorithm uint8

const (
	// CompressionNoCompression stores the suffix bytes verbatim.
	CompressionNoCompression CompressionAlgorithm = 0x00
	// CompressionLowercaseASCII uses LowercaseAsciiCompression for blocks whose
	// suffixes are lowercase 7-bit ASCII.
	CompressionLowercaseASCII CompressionAlgorithm = 0x01
	// CompressionLZ4 uses LZ4 for blocks whose suffixes show enough redundancy.
	CompressionLZ4 CompressionAlgorithm = 0x02
)

// compressionAlgorithmByCode mirrors the Java BY_CODE table (size 3).
var compressionAlgorithmByCode = [3]CompressionAlgorithm{
	CompressionNoCompression,
	CompressionLowercaseASCII,
	CompressionLZ4,
}

// Code returns the on-disk code byte used inside the block header.
// Mirrors CompressionAlgorithm#code in Java.
func (a CompressionAlgorithm) Code() int {
	return int(a)
}

// String returns the algorithm name (mirrors the Java enum's name()).
func (a CompressionAlgorithm) String() string {
	switch a {
	case CompressionNoCompression:
		return "NO_COMPRESSION"
	case CompressionLowercaseASCII:
		return "LOWERCASE_ASCII"
	case CompressionLZ4:
		return "LZ4"
	default:
		return fmt.Sprintf("CompressionAlgorithm(%d)", uint8(a))
	}
}

// CompressionAlgorithmByCode returns the CompressionAlgorithm with the given
// wire-format code, or an error for an unknown code.
// Mirrors CompressionAlgorithm.byCode(int) in Java.
func CompressionAlgorithmByCode(code int) (CompressionAlgorithm, error) {
	if code < 0 || code >= len(compressionAlgorithmByCode) {
		return 0, fmt.Errorf("illegal code for a compression algorithm: %d", code)
	}
	return compressionAlgorithmByCode[code], nil
}

// CompressionInput is the input surface required by [CompressionAlgorithm.Read].
// It mirrors org.apache.lucene.store.DataInput's relevant subset: byte/range
// reads plus variable-length integer reads (used by the LOWERCASE_ASCII path).
type CompressionInput interface {
	store.DataInput
	store.VariableLengthInput
}

// Read decompresses length bytes from in into out[0:length], using this
// algorithm. Mirrors CompressionAlgorithm#read(DataInput, byte[], int) in Java.
//
// Note on input type: Lucene's org.apache.lucene.store.DataInput exposes both
// the byte-oriented methods and readVInt/readVLong. Gocene splits those into
// [store.DataInput] and [store.VariableLengthInput], so Read accepts the
// composite [CompressionInput] to satisfy both the LOWERCASE_ASCII and LZ4
// code paths uniformly. All store-package inputs that ship VInt support
// (e.g. ByteArrayDataInput, BufferedIndexInput-derived inputs) satisfy it
// directly.
func (a CompressionAlgorithm) Read(in CompressionInput, out []byte, length int) error {
	if length < 0 || length > len(out) {
		return fmt.Errorf("CompressionAlgorithm.Read: length %d out of range for out cap %d", length, len(out))
	}
	switch a {
	case CompressionNoCompression:
		return in.ReadBytes(out[:length])
	case CompressionLowercaseASCII:
		return compress.Decompress(in, out, length)
	case CompressionLZ4:
		_, err := compress.LZ4Decompress(in, length, out, 0)
		return err
	default:
		return fmt.Errorf("unknown CompressionAlgorithm: %s", a)
	}
}
