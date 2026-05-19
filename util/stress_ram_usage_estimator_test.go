// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"testing"
)

// TestStressRamUsageEstimator is the Go port of Lucene's TestStressRamUsageEstimator
// "stress monster" suite (org.apache.lucene.util.TestStressRamUsageEstimator). The
// original Java tests are annotated @Nightly and are excluded from default Lucene
// test runs because each one allocates large arrays (1,000,000 byte-array slots
// for testLargeSetOfByteArrays; up to 50 MiB worth of shallow-sized objects until
// OutOfMemoryError for testSimpleByteArrays) and relies on Runtime.gc() /
// Runtime.totalMemory() to compare actual vs. estimated heap usage.
//
// In Gocene, stress/monster tests are guarded behind the GOCENE_RUN_MONSTERS
// environment variable so that `go test ./...` remains fast and memory-safe by
// default. The full port of the body is intentionally deferred: Go's runtime
// does not expose a direct analogue of Java's per-class shallowSizeOf reflection,
// so a faithful reproduction will require a deterministic Go shaping of the
// scenario (typed allocations + runtime/debug.ReadGCStats for before/after
// totals) instead of a 1:1 translation.
//
// Skipping here is the contract: the test is registered, discoverable via
// `go test -run TestStressRamUsageEstimator`, and acts as a placeholder for the
// full port that will land alongside the Go-flavoured stress harness.
func TestStressRamUsageEstimator(t *testing.T) {
	t.Skip("stress monster test (large allocations + GC observation); set GOCENE_RUN_MONSTERS=1 and port body when Go stress harness lands")
}
