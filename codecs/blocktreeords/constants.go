// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package blocktreeords

// outputFlagsNumBits is the number of flag bits stored in the low-order
// bits of the root-block output long.  Mirrors
// OrdsBlockTreeTermsWriter.OUTPUT_FLAGS_NUM_BITS = 2.
const outputFlagsNumBits = 2

// outputFlagIsFloor is the flag bit indicating this block is the root of a
// floor sequence.  Mirrors OrdsBlockTreeTermsWriter.OUTPUT_FLAG_IS_FLOOR = 0x1.
const outputFlagIsFloor = int64(0x1)

// outputFlagHasTerms is the flag bit indicating this block (or at least
// one sub-block under it in the floor chain) contains terms. Mirrors
// OrdsBlockTreeTermsWriter.OUTPUT_FLAG_HAS_TERMS = 0x2.
const outputFlagHasTerms = int64(0x2)
