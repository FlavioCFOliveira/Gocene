// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import "github.com/FlavioCFOliveira/Gocene/util"

// BytesTermAttribute is the Go port of Lucene's
// org.apache.lucene.analysis.tokenattributes.BytesTermAttribute.
//
// Use BytesTermAttribute when raw term bytes are already available and
// should be indexed directly (binary terms), as a replacement for
// [CharTermAttribute]. The interface extends [TermToBytesRefAttribute]
// so consumers can read the current term through the unified
// GetBytesRef hook.
//
// This is a Lucene internal API.
//
// Lucene 10.4.0 reference:
//
//	lucene/core/src/java/org/apache/lucene/analysis/tokenattributes/BytesTermAttribute.java
type BytesTermAttribute interface {
	TermToBytesRefAttribute

	// SetBytesRef sets the BytesRef of the current term.
	SetBytesRef(bytes *util.BytesRef)
}
