// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index_test

// NRT indexing tests are skipped because:
//   - The package was originally declared as package index (internal test), causing an import
//     cycle: index -> document -> index.
//   - The code used document.Stored and document.Indexed constants that don't exist.
//   - document.NewTextField 3-arg form doesn't match the actual signature.
//   - writer.GetReader() is not implemented.
//   - reader.Reopen() and reader.IsCurrent() are not implemented.

import "testing"

func TestNRTBasicIndexing(t *testing.T) {
	t.Fatal("NRT indexing test — import cycle / unimplemented APIs, not yet fixed")
}

func TestNRTReopen(t *testing.T) {
	t.Fatal("NRT indexing test — import cycle / unimplemented APIs, not yet fixed")
}

func TestNRTDocumentVisibility(t *testing.T) {
	t.Fatal("NRT indexing test — import cycle / unimplemented APIs, not yet fixed")
}

func TestNRTDeleteOperations(t *testing.T) {
	t.Fatal("NRT indexing test — import cycle / unimplemented APIs, not yet fixed")
}

func TestNRTUpdateOperations(t *testing.T) {
	t.Fatal("NRT indexing test — import cycle / unimplemented APIs, not yet fixed")
}

func TestNRTLargeDocumentSet(t *testing.T) {
	t.Fatal("NRT indexing test — import cycle / unimplemented APIs, not yet fixed")
}

func TestNRTMultipleReopens(t *testing.T) {
	t.Fatal("NRT indexing test — import cycle / unimplemented APIs, not yet fixed")
}

func TestNRTReopenWithDeletes(t *testing.T) {
	t.Fatal("NRT indexing test — import cycle / unimplemented APIs, not yet fixed")
}

func TestNRTContextCancellation(t *testing.T) {
	t.Fatal("NRT indexing test — import cycle / unimplemented APIs, not yet fixed")
}

func TestNRTReopenPerformance(t *testing.T) {
	t.Fatal("NRT indexing test — import cycle / unimplemented APIs, not yet fixed")
}

func TestNRTConcurrentAccess(t *testing.T) {
	t.Fatal("NRT indexing test — import cycle / unimplemented APIs, not yet fixed")
}

func TestNRTIsCurrent(t *testing.T) {
	t.Fatal("NRT indexing test — import cycle / unimplemented APIs, not yet fixed")
}

func TestNRTWithDifferentAnalyzers(t *testing.T) {
	t.Fatal("NRT indexing test — import cycle / unimplemented APIs, not yet fixed")
}
