// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index_test

// NRT search tests are skipped because:
//   - The package was originally declared as package index (internal test), causing an import
//     cycle: index -> document -> index.
//   - The code used document.Stored and document.Indexed constants that don't exist.
//   - document.NewTextField 3-arg form doesn't match the actual signature.
//   - writer.GetReader() is not implemented.
//   - reader.Reopen() is not implemented.

import "testing"

func TestNRTSearchBasic(t *testing.T) {
	t.Fatal("NRT search test — import cycle / unimplemented APIs, not yet fixed")
}

func TestNRTSearchAfterReopen(t *testing.T) {
	t.Fatal("NRT search test — import cycle / unimplemented APIs, not yet fixed")
}

func TestNRTSearchConsistency(t *testing.T) {
	t.Fatal("NRT search test — import cycle / unimplemented APIs, not yet fixed")
}

func TestNRTSearchConcurrentReadWrite(t *testing.T) {
	t.Fatal("NRT search test — import cycle / unimplemented APIs, not yet fixed")
}

func TestNRTSearchAfterDelete(t *testing.T) {
	t.Fatal("NRT search test — import cycle / unimplemented APIs, not yet fixed")
}

func TestNRTSearchMultipleFields(t *testing.T) {
	t.Fatal("NRT search test — import cycle / unimplemented APIs, not yet fixed")
}

func BenchmarkNRTSearchLatency(b *testing.B) {
	b.Fatal("NRT search benchmark — import cycle / unimplemented APIs, not yet fixed")
}

func BenchmarkNRTSearchThroughput(b *testing.B) {
	b.Fatal("NRT search benchmark — import cycle / unimplemented APIs, not yet fixed")
}
