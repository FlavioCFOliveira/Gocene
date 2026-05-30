// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index_test

// NRT benchmarks are skipped because:
//   - The package was originally declared as package index (internal test), causing an import
//     cycle: index -> document -> index.
//   - The code used document.Stored and document.Indexed constants that don't exist.
//   - document.NewTextField 3-arg form doesn't match the actual signature.

import "testing"

func BenchmarkNRTIndexing(b *testing.B) {
	b.Fatal("NRT benchmark import cycle — not yet fixed")
}

func BenchmarkNRTReopen(b *testing.B) {
	b.Fatal("NRT benchmark import cycle — not yet fixed")
}

func BenchmarkNRTReopenWithChanges(b *testing.B) {
	b.Fatal("NRT benchmark import cycle — not yet fixed")
}

func BenchmarkNRTConcurrentIndexing(b *testing.B) {
	b.Fatal("NRT benchmark import cycle — not yet fixed")
}

func BenchmarkNRTReaderCreation(b *testing.B) {
	b.Fatal("NRT benchmark import cycle — not yet fixed")
}

func BenchmarkNRTDocumentThroughput(b *testing.B) {
	b.Fatal("NRT benchmark import cycle — not yet fixed")
}

func BenchmarkNRTReopenLatency(b *testing.B) {
	b.Fatal("NRT benchmark import cycle — not yet fixed")
}

func BenchmarkNRTVsNonNRT(b *testing.B) {
	b.Fatal("NRT benchmark import cycle — not yet fixed")
}

func BenchmarkNRTMemoryUsage(b *testing.B) {
	b.Fatal("NRT benchmark import cycle — not yet fixed")
}
