// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import "fmt"

// InvertableType describes how an IndexableField is processed when inverted
// (indexed). This is the Go port of Lucene 10.4.0's
// org.apache.lucene.document.InvertableType.
//
// Implemented as a typed int constant set rather than a Java enum; each value
// maps to a single Lucene ordinal (BINARY=0, TOKEN_STREAM=1) and the String
// method returns the canonical Lucene name for parity with Java's
// Enum#name().
type InvertableType int

const (
	// InvertableTypeBinary indicates the field should be treated as a single
	// value whose binary content is returned by BinaryValue(). The term
	// frequency is assumed to be one.
	InvertableTypeBinary InvertableType = iota

	// InvertableTypeTokenStream indicates the field should be inverted
	// through its TokenStream().
	InvertableTypeTokenStream
)

// String returns the canonical Lucene name for the InvertableType.
func (it InvertableType) String() string {
	switch it {
	case InvertableTypeBinary:
		return "BINARY"
	case InvertableTypeTokenStream:
		return "TOKEN_STREAM"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", int(it))
	}
}

// Ordinal returns the zero-based ordinal of the InvertableType, mirroring
// Java's Enum#ordinal().
func (it InvertableType) Ordinal() int { return int(it) }
