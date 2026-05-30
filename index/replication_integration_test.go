// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index_test

// Replication integration tests are skipped because:
//   - The package was originally declared as package index (internal test), causing an import
//     cycle: index -> document -> index.
//   - The code used document.Stored and document.Indexed constants that don't exist.

import "testing"

func TestReplicationEndToEnd(t *testing.T) {
	t.Fatal("Replication integration test — import cycle / unimplemented APIs, not yet fixed")
}

func TestReplicationConsistency(t *testing.T) {
	t.Fatal("Replication integration test — import cycle / unimplemented APIs, not yet fixed")
}

func TestReplicationIncremental(t *testing.T) {
	t.Fatal("Replication integration test — import cycle / unimplemented APIs, not yet fixed")
}

func TestReplicationFailureRecovery(t *testing.T) {
	t.Fatal("Replication integration test — import cycle / unimplemented APIs, not yet fixed")
}

func TestReplicationConcurrent(t *testing.T) {
	t.Fatal("Replication integration test — import cycle / unimplemented APIs, not yet fixed")
}

func TestReplicationWithDeletions(t *testing.T) {
	t.Fatal("Replication integration test — import cycle / unimplemented APIs, not yet fixed")
}

func BenchmarkReplication(b *testing.B) {
	b.Fatal("Replication integration benchmark — import cycle / unimplemented APIs, not yet fixed")
}

func TestReplicationSession(t *testing.T) {
	t.Fatal("Replication integration test — import cycle / unimplemented APIs, not yet fixed")
}

func TestReplicationSessionTimeout(t *testing.T) {
	t.Fatal("Replication integration test — import cycle / unimplemented APIs, not yet fixed")
}

func TestReplicationWithLargeFiles(t *testing.T) {
	t.Fatal("Replication integration test — import cycle / unimplemented APIs, not yet fixed")
}

func TestReplicationNetworkFailure(t *testing.T) {
	t.Fatal("Replication integration test — import cycle / unimplemented APIs, not yet fixed")
}

func TestReplicationChecksumVerification(t *testing.T) {
	t.Fatal("Replication integration test — import cycle / unimplemented APIs, not yet fixed")
}

func TestReplicationMetrics(t *testing.T) {
	t.Fatal("Replication integration test — import cycle / unimplemented APIs, not yet fixed")
}
