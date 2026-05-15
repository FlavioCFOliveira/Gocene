// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package fst

// byteFormatFixtureABC is the frozen byte stream produced by the FST
// compiler for the inputs {"a"->1, "b"->2, "c"->3} with
// InputTypeByte1 and PositiveIntOutputs. Captured by
// TestFSTCompilerByteFormatFixture and frozen as a regression guard
// against byte-format drift.
//
// This fixture is NOT a Lucene-validated golden file — it was
// captured from this implementation on 2026-05-15 because no JVM was
// available to cross-validate against the Java reference. Structural
// correctness is asserted independently by TestFSTCompilerRoundTrip
// and TestFSTCompilerSaveRoundTrip; this fixture only locks the
// byte layout against subsequent regressions in the Go port.
var byteFormatFixtureABC = []byte{
	// Byte 0: the leading padding sentinel reserved by addNode so that
	// no real node ever lands at offset 0 (which would collide with
	// the NonFinalEndNode virtual address).
	0x00,
	// Bytes 1-9: the root node, written tail-first by FSTCompiler
	// (scratchBytes is reversed in place before flushing). The node
	// holds three arcs: 'a'->1, 'b'->2, 'c'->3, all marked
	// BIT_FINAL_ARC + BIT_STOP_NODE + BIT_ARC_HAS_OUTPUT. Arcs 'a' and
	// 'b' also carry BIT_TARGET_NEXT because dedupHash advanced
	// lastFrozenNode to FinalEndNode before they were written; the
	// reader ignores that bit when BIT_STOP_NODE is set, so the
	// behaviour is byte-identical to Lucene and reader-equivalent.
	0x03, 0x63, 0x1F, // arc 'c' in reverse: VLong(3), label 'c'=0x63, flags 0x1F (FINAL|LAST|TARGET_NEXT|STOP|HAS_OUTPUT)
	0x02, 0x62, 0x1D, // arc 'b' in reverse: VLong(2), label 'b'=0x62, flags 0x1D (FINAL|TARGET_NEXT|STOP|HAS_OUTPUT)
	0x01, 0x61, 0x1D, // arc 'a' in reverse: VLong(1), label 'a'=0x61, flags 0x1D
}
