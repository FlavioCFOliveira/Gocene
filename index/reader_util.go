// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

// ReaderUtil collects static helpers for working with IndexReader hierarchies.
// Mirrors org.apache.lucene.index.ReaderUtil from Apache Lucene 10.4.0.

// ReaderUtilSubIndex returns the index in starts of the slot whose docBase is
// the greatest value <= docID. Equivalent to Lucene's
// ReaderUtil.subIndex(docID, starts).
//
// starts must be sorted ascending; the sentinel entry starts[len(starts)-1]
// is interpreted as "one past the last valid docID".
func ReaderUtilSubIndex(docID int, starts []int) int {
	// Binary search the highest start that does not exceed docID.
	lo, hi := 0, len(starts)-1
	for lo < hi {
		mid := (lo + hi + 1) >> 1
		if starts[mid] <= docID {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return lo
}

// ReaderUtilGetTopLevelContext walks parent pointers up an IndexReaderContext
// chain until the top level is reached. Equivalent to Lucene's
// ReaderUtil.getTopLevelContext.
func ReaderUtilGetTopLevelContext(ctx IndexReaderContext) IndexReaderContext {
	for ctx != nil && ctx.Parent() != nil {
		ctx = ctx.Parent()
	}
	return ctx
}
