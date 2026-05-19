// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import "testing"

// TestCodecLoadingDeadlock ports Lucene's TestCodecLoadingDeadlock
// (org/apache/lucene/codecs/TestCodecLoadingDeadlock.java).
//
// Status: STUB.
//
// The original test forks a separate JVM and races 14 goroutine-equivalents
// against a CyclicBarrier to stress the static class-initialization path of
// Codec/PostingsFormat/DocValuesFormat SPI loaders (a classloader-deadlock
// regression check).
//
// In Gocene the SPI surface is replaced by name-keyed registries
// (PostingsFormatByName / DocValuesFormatByName / Codec lookups) without
// JVM-style lazy static initialization, so the original deadlock vector
// does not exist. A faithful port still needs to:
//   - exercise concurrent ForName / Available* calls across all three
//     registries from N goroutines released by a sync.WaitGroup barrier,
//   - run under -race,
//   - bound the run with a context deadline (Go analogue of the 30s cap).
//
// Subprocess re-execution (Lucene's ProcessBuilder fork) is intentionally
// not ported: Gocene init ordering is deterministic per-binary, so a
// single-process race suffices once the registries are wired.
//
// Wiring deferred until the codec registry concurrency contract is
// finalized (tracked alongside the PostingsFormat wiring backlog).
func TestCodecLoadingDeadlock(t *testing.T) {
	t.Skip("stub: pending concurrent registry stress harness")
}
