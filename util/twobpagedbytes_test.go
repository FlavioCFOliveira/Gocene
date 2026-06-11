// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"fmt"
	"testing"
)

// Test2BPagedBytes is the Go port of Lucene's Test2BPagedBytes "monster" test
// (org.apache.lucene.util.Test2BPagedBytes). The original Java test is marked
// with @Monster("You must increase heap to > 2 G to run this") and is excluded
// from default Lucene test runs because it allocates roughly 1.1 * Integer.MAX_VALUE
// bytes (~2.36 GiB) of paged storage.
//
// In Gocene, monster tests are guarded behind the GOCENE_RUN_MONSTERS environment
// variable so that `go test ./...` remains fast and memory-safe by default. The
// full port of the body is intentionally deferred: it requires a stable
// store.Directory implementation with multi-gigabyte file support and the
// PagedBytes.Copy(IndexInput, length) entry point used by the Java test.
//
// Skipping here is the contract: the test is registered, discoverable via
// `go test -run Test2BPagedBytes`, and acts as a placeholder for the full
// port that will land alongside the matching Directory/IndexInput support.
func Test2BPagedBytes(t *testing.T) {
	// Replacement for the upstream @Monster test that requires >2GiB heap.
	// Validates that PagedBytes correctly handles block-spanning writes at
	// various block sizes without requiring multi-GiB allocations.
	for _, blockBits := range []int{8, 12, 16} {
		t.Run(fmt.Sprintf("blockBits=%d", blockBits), func(t *testing.T) {
			pb, err := NewPagedBytes(blockBits)
			if err != nil {
				t.Fatalf("NewPagedBytes(%d): %v", blockBits, err)
			}
			out := pb.GetDataOutput()
			// Write data that spans multiple blocks. Use (2^blockBits + 42) bytes
			// to force a partial last block.
			dataSize := (1 << blockBits) + 42
			data := make([]byte, dataSize)
			for i := range data {
				data[i] = byte(i % 251) // non-repeating-ish pattern
			}
			if err := out.WriteBytes(data); err != nil {
				t.Fatalf("WriteBytes: %v", err)
			}
			reader, err := pb.Freeze(true)
			if err != nil {
				t.Fatalf("Freeze: %v", err)
			}
			// Verify every byte.
			for i, want := range data {
				got := reader.GetByte(int64(i))
				if got != want {
					t.Fatalf("byte[%d] = %d, want %d", i, got, want)
				}
			}
			// RamBytesUsed must be positive.
			if pb.RamBytesUsed() <= 0 {
				t.Fatal("PagedBytes.RamBytesUsed() <= 0")
			}
		})
	}
}
