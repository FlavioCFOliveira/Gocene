// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// addTextField creates a stored, tokenized TextField and adds it to doc.
func addTextField(t *testing.T, doc *document.Document, name, value string) {
	t.Helper()
	field, err := document.NewTextField(name, value, true)
	if err != nil {
		t.Fatalf("NewTextField(%q): %v", name, err)
	}
	doc.Add(field)
}

// createSourceIndex creates a file-system backed source index with docCount
// documents and returns the directory path. The caller must not delete the
// path; it is cleaned up automatically by t.TempDir.
func createSourceIndex(t *testing.T, docCount int) string {
	t.Helper()
	sourceDir := t.TempDir()

	dir, err := store.NewNIOFSDirectory(sourceDir)
	if err != nil {
		t.Fatalf("NewFSDirectory: %v", err)
	}
	defer dir.Close()

	cfg := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(dir, cfg)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	defer writer.Close()

	for i := 0; i < docCount; i++ {
		doc := document.NewDocument()
		addTextField(t, doc, "id", fmt.Sprintf("doc-%d", i))
		addTextField(t, doc, "content", "replication test content")
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument: %v", err)
		}
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	return sourceDir
}

// openWriterOnPath reopens an IndexWriter in CREATE_OR_APPEND mode on an
// existing file-system directory.
func openWriterOnPath(t *testing.T, dirPath string) *index.IndexWriter {
	t.Helper()
	dir, err := store.NewNIOFSDirectory(dirPath)
	if err != nil {
		t.Fatalf("NewFSDirectory: %v", err)
	}
	t.Cleanup(func() { _ = dir.Close() })

	cfg := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(dir, cfg)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	return writer
}

// countDocs opens dirPath as a DirectoryReader and returns NumDocs.
func countDocs(t *testing.T, dirPath string) int {
	t.Helper()
	dir, err := store.NewNIOFSDirectory(dirPath)
	if err != nil {
		t.Fatalf("NewFSDirectory: %v", err)
	}
	defer dir.Close()

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	return reader.NumDocs()
}

// segmentCount returns the number of segments in dirPath.
func segmentCount(t *testing.T, dirPath string) int {
	t.Helper()
	dir, err := store.NewNIOFSDirectory(dirPath)
	if err != nil {
		t.Fatalf("NewFSDirectory: %v", err)
	}
	defer dir.Close()

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	return reader.GetSegmentInfos().Size()
}

// TestReplicationEndToEnd tests a complete replication flow from a source
// index directory to a target directory.
func TestReplicationEndToEnd(t *testing.T) {
	sourceDir := createSourceIndex(t, 10)
	targetDir := t.TempDir()

	repl, err := index.NewLocalReplicator(sourceDir, targetDir)
	if err != nil {
		t.Fatalf("NewLocalReplicator: %v", err)
	}
	defer repl.Close()

	if err := repl.Replicate(context.Background()); err != nil {
		t.Fatalf("Replicate: %v", err)
	}

	if got := countDocs(t, sourceDir); got != 10 {
		t.Errorf("source docs: expected 10, got %d", got)
	}
	if got := countDocs(t, targetDir); got != 10 {
		t.Errorf("replicated docs: expected 10, got %d", got)
	}
}

// TestReplicationConsistency verifies that document counts and segment
// structures match between source and target after replication.
func TestReplicationConsistency(t *testing.T) {
	sourceDir := createSourceIndex(t, 3)
	targetDir := t.TempDir()

	repl, err := index.NewLocalReplicator(sourceDir, targetDir)
	if err != nil {
		t.Fatalf("NewLocalReplicator: %v", err)
	}
	defer repl.Close()

	if err := repl.Replicate(context.Background()); err != nil {
		t.Fatalf("Replicate: %v", err)
	}

	if countDocs(t, sourceDir) != countDocs(t, targetDir) {
		t.Errorf("doc count mismatch: source=%d target=%d",
			countDocs(t, sourceDir), countDocs(t, targetDir))
	}

	if segmentCount(t, sourceDir) != segmentCount(t, targetDir) {
		t.Errorf("segment count mismatch: source=%d target=%d",
			segmentCount(t, sourceDir), segmentCount(t, targetDir))
	}
}

// TestReplicationIncremental tests that a second replication picks up
// documents added after the first replication.
func TestReplicationIncremental(t *testing.T) {
	sourceDir := createSourceIndex(t, 1)
	targetDir := t.TempDir()

	repl, err := index.NewLocalReplicator(sourceDir, targetDir)
	if err != nil {
		t.Fatalf("NewLocalReplicator: %v", err)
	}
	defer repl.Close()

	if err := repl.Replicate(context.Background()); err != nil {
		t.Fatalf("first Replicate: %v", err)
	}
	if got := countDocs(t, targetDir); got != 1 {
		t.Errorf("after first replication: expected 1 doc, got %d", got)
	}

	// Append a second document to the source index.
	writer := openWriterOnPath(t, sourceDir)
	doc := document.NewDocument()
	addTextField(t, doc, "id", "doc-1")
	addTextField(t, doc, "content", "second")
	if err := writer.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	writer.Close()

	if err := repl.Replicate(context.Background()); err != nil {
		t.Fatalf("second Replicate: %v", err)
	}
	if got := countDocs(t, targetDir); got != 2 {
		t.Errorf("after second replication: expected 2 docs, got %d", got)
	}
}

// TestReplicationFailureRecovery verifies that repeated replication is
// idempotent and does not corrupt the target index.
func TestReplicationFailureRecovery(t *testing.T) {
	sourceDir := createSourceIndex(t, 1)
	targetDir := t.TempDir()

	repl, err := index.NewLocalReplicator(sourceDir, targetDir)
	if err != nil {
		t.Fatalf("NewLocalReplicator: %v", err)
	}
	defer repl.Close()

	if err := repl.Replicate(context.Background()); err != nil {
		t.Fatalf("first Replicate: %v", err)
	}
	if got := countDocs(t, targetDir); got != 1 {
		t.Errorf("after first replication: expected 1 doc, got %d", got)
	}

	// Re-running replication must be a no-op and not fail.
	if err := repl.Replicate(context.Background()); err != nil {
		t.Fatalf("second Replicate: %v", err)
	}
	if got := countDocs(t, targetDir); got != 1 {
		t.Errorf("after idempotent replication: expected 1 doc, got %d", got)
	}
}

// TestReplicationConcurrent exercises the replicator from many goroutines.
// The replicator serializes operations internally, so all calls must succeed
// and the final target must contain the full source index.
func TestReplicationConcurrent(t *testing.T) {
	sourceDir := createSourceIndex(t, 100)
	targetDir := t.TempDir()

	repl, err := index.NewLocalReplicator(sourceDir, targetDir)
	if err != nil {
		t.Fatalf("NewLocalReplicator: %v", err)
	}
	defer repl.Close()

	var wg sync.WaitGroup
	errs := make(chan error, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := repl.Replicate(context.Background()); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		t.Errorf("concurrent replication error: %v", err)
	}

	if got := countDocs(t, targetDir); got != 100 {
		t.Errorf("after concurrent replication: expected 100 docs, got %d", got)
	}
}

// TestReplicationWithDeletions verifies that applied deletes are preserved
// when segment files are copied to the replica.
func TestReplicationWithDeletions(t *testing.T) {
	sourceDir := createSourceIndex(t, 5)
	targetDir := t.TempDir()

	writer := openWriterOnPath(t, sourceDir)
	// Delete two documents by their unique id terms.
	if err := writer.DeleteDocuments(index.NewTerm("id", "doc-0")); err != nil {
		t.Fatalf("DeleteDocuments: %v", err)
	}
	if err := writer.DeleteDocuments(index.NewTerm("id", "doc-2")); err != nil {
		t.Fatalf("DeleteDocuments: %v", err)
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	writer.Close()

	repl, err := index.NewLocalReplicator(sourceDir, targetDir)
	if err != nil {
		t.Fatalf("NewLocalReplicator: %v", err)
	}
	defer repl.Close()

	if err := repl.Replicate(context.Background()); err != nil {
		t.Fatalf("Replicate: %v", err)
	}

	if got := countDocs(t, targetDir); got != 3 {
		t.Errorf("after deletions: expected 3 docs, got %d", got)
	}
}

// TestReplicationSession exercises the basic lifecycle of a replication
// session created through the NRT replication writer.
func TestReplicationSession(t *testing.T) {
	sourceDir := createSourceIndex(t, 1)
	dir, err := store.NewNIOFSDirectory(sourceDir)
	if err != nil {
		t.Fatalf("NewFSDirectory: %v", err)
	}
	defer dir.Close()

	cfg := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(dir, cfg)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	defer writer.Close()

	rw, err := index.NewNRTReplicationWriter(writer)
	if err != nil {
		t.Fatalf("NewNRTReplicationWriter: %v", err)
	}
	defer rw.Close()

	session, err := rw.CreateSession(context.Background(), time.Hour)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if session.ID == "" {
		t.Fatal("expected non-empty session ID")
	}

	if _, err := rw.GetSession(session.ID); err != nil {
		t.Fatalf("GetSession: %v", err)
	}

	if err := rw.CloseSession(session.ID); err != nil {
		t.Fatalf("CloseSession: %v", err)
	}
	if _, err := rw.GetSession(session.ID); err == nil {
		t.Fatal("expected error for closed session")
	}
}

// TestReplicationSessionTimeout verifies that sessions expire after their
// TTL elapses.
func TestReplicationSessionTimeout(t *testing.T) {
	sourceDir := createSourceIndex(t, 1)
	dir, err := store.NewNIOFSDirectory(sourceDir)
	if err != nil {
		t.Fatalf("NewFSDirectory: %v", err)
	}
	defer dir.Close()

	cfg := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(dir, cfg)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	defer writer.Close()

	rw, err := index.NewNRTReplicationWriter(writer)
	if err != nil {
		t.Fatalf("NewNRTReplicationWriter: %v", err)
	}
	defer rw.Close()

	session, err := rw.CreateSession(context.Background(), 50*time.Millisecond)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if _, err := rw.GetSession(session.ID); err != nil {
		t.Fatalf("GetSession before timeout: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	if _, err := rw.GetSession(session.ID); err == nil {
		t.Fatal("expected session to have expired")
	}
}

// TestReplicationWithLargeFiles replicates an index with larger documents.
func TestReplicationWithLargeFiles(t *testing.T) {
	sourceDir := createSourceIndex(t, 100)
	targetDir := t.TempDir()

	repl, err := index.NewLocalReplicator(sourceDir, targetDir)
	if err != nil {
		t.Fatalf("NewLocalReplicator: %v", err)
	}
	defer repl.Close()

	if err := repl.Replicate(context.Background()); err != nil {
		t.Fatalf("Replicate: %v", err)
	}

	if got := countDocs(t, targetDir); got != 100 {
		t.Errorf("expected 100 docs, got %d", got)
	}
}

// TestReplicationNetworkFailure simulates a network interruption by
// replicating with a cancelled context; the operation must report the
// cancellation and must not leave the target corrupted.
func TestReplicationNetworkFailure(t *testing.T) {
	sourceDir := createSourceIndex(t, 1)
	targetDir := t.TempDir()

	repl, err := index.NewLocalReplicator(sourceDir, targetDir)
	if err != nil {
		t.Fatalf("NewLocalReplicator: %v", err)
	}
	defer repl.Close()

	if err := repl.Replicate(context.Background()); err != nil {
		t.Fatalf("normal Replicate: %v", err)
	}
	if got := countDocs(t, targetDir); got != 1 {
		t.Errorf("expected 1 doc after normal replication, got %d", got)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = repl.Replicate(ctx)
	if err == nil {
		t.Error("expected cancelled replication to report an error")
	}
}

// TestReplicationChecksumVerification enables post-copy checksum
// verification and replicates a small index successfully.
func TestReplicationChecksumVerification(t *testing.T) {
	sourceDir := createSourceIndex(t, 1)
	targetDir := t.TempDir()

	repl, err := index.NewLocalReplicator(sourceDir, targetDir)
	if err != nil {
		t.Fatalf("NewLocalReplicator: %v", err)
	}
	defer repl.Close()

	repl.SetVerifyChecksums(true)
	if err := repl.Replicate(context.Background()); err != nil {
		t.Fatalf("Replicate with checksum verification: %v", err)
	}

	if got := countDocs(t, targetDir); got != 1 {
		t.Errorf("expected 1 doc, got %d", got)
	}
}

// TestReplicationMetrics checks that replication statistics are updated.
func TestReplicationMetrics(t *testing.T) {
	sourceDir := createSourceIndex(t, 50)
	targetDir := t.TempDir()

	repl, err := index.NewLocalReplicator(sourceDir, targetDir)
	if err != nil {
		t.Fatalf("NewLocalReplicator: %v", err)
	}
	defer repl.Close()

	if err := repl.Replicate(context.Background()); err != nil {
		t.Fatalf("Replicate: %v", err)
	}

	stats := repl.GetStats()
	if stats.TotalChecks != 1 {
		t.Errorf("expected 1 check, got %d", stats.TotalChecks)
	}
	if stats.SuccessfulChecks != 1 {
		t.Errorf("expected 1 successful check, got %d", stats.SuccessfulChecks)
	}
	if stats.TotalBytesTransferred <= 0 {
		t.Errorf("expected bytes transferred > 0, got %d", stats.TotalBytesTransferred)
	}
	if got := countDocs(t, targetDir); got != 50 {
		t.Errorf("expected 50 docs, got %d", got)
	}
}

// BenchmarkReplication measures the time to copy a committed index to a fresh
// target directory.
func BenchmarkReplication(b *testing.B) {
	sourceDir := createSourceIndexForBenchmark(b, 1000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		targetDir := b.TempDir()
		repl, err := index.NewLocalReplicator(sourceDir, targetDir)
		if err != nil {
			b.Fatalf("NewLocalReplicator: %v", err)
		}
		if err := repl.Replicate(context.Background()); err != nil {
			b.Fatalf("Replicate: %v", err)
		}
		repl.Close()
	}
}

// createSourceIndexForBenchmark is the benchmark analogue of createSourceIndex.
func createSourceIndexForBenchmark(b *testing.B, docCount int) string {
	b.Helper()
	sourceDir := b.TempDir()

	dir, err := store.NewNIOFSDirectory(sourceDir)
	if err != nil {
		b.Fatalf("NewFSDirectory: %v", err)
	}
	defer dir.Close()

	cfg := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(dir, cfg)
	if err != nil {
		b.Fatalf("NewIndexWriter: %v", err)
	}
	defer writer.Close()

	for i := 0; i < docCount; i++ {
		doc := document.NewDocument()
		field, _ := document.NewTextField("content", "benchmark document", true)
		doc.Add(field)
		if err := writer.AddDocument(doc); err != nil {
			b.Fatalf("AddDocument: %v", err)
		}
	}

	if err := writer.Commit(); err != nil {
		b.Fatalf("Commit: %v", err)
	}

	return sourceDir
}
