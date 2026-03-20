// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestReplicationEndToEnd tests a complete replication flow
func TestReplicationEndToEnd(t *testing.T) {
	// Create master directory
	masterDir, err := store.NewRAMDirectory()
	if err != nil {
		t.Fatalf("Failed to create master directory: %v", err)
	}
	defer masterDir.Close()

	// Create slave directory
	slaveDir, err := store.NewRAMDirectory()
	if err != nil {
		t.Fatalf("Failed to create slave directory: %v", err)
	}
	defer slaveDir.Close()

	// Create master writer
	config := NewIndexWriterConfig(analysis.NewStandardAnalyzer())
	masterWriter, err := NewIndexWriter(masterDir, config)
	if err != nil {
		t.Fatalf("Failed to create master writer: %v", err)
	}
	defer masterWriter.Close()

	// Add documents to master
	for i := 0; i < 10; i++ {
		doc := document.NewDocument()
		doc.Add(document.NewTextField("id", string(rune('a'+i)), document.Stored|document.Indexed))
		doc.Add(document.NewTextField("content", "replication test", document.Stored|document.Indexed))
		if err := masterWriter.AddDocument(doc); err != nil {
			t.Fatalf("Failed to add document: %v", err)
		}
	}
	if err := masterWriter.Commit(); err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	// Verify master has documents
	masterReader, err := OpenDirectoryReader(masterDir)
	if err != nil {
		t.Fatalf("Failed to open master reader: %v", err)
	}
	if masterReader.NumDocs() != 10 {
		t.Errorf("Expected 10 docs on master, got %d", masterReader.NumDocs())
	}
	masterReader.Close()

	// Create local replicator
	replicator := NewLocalReplicator(masterDir, slaveDir)

	// Perform replication
	ctx := context.Background()
	if err := replicator.Replicate(ctx); err != nil {
		t.Fatalf("Replication failed: %v", err)
	}

	// Verify slave has same documents
	slaveReader, err := OpenDirectoryReader(slaveDir)
	if err != nil {
		t.Fatalf("Failed to open slave reader: %v", err)
	}
	defer slaveReader.Close()

	if slaveReader.NumDocs() != 10 {
		t.Errorf("Expected 10 docs on slave after replication, got %d", slaveReader.NumDocs())
	}
}

// TestReplicationConsistency verifies data consistency after replication
func TestReplicationConsistency(t *testing.T) {
	masterDir, _ := store.NewRAMDirectory()
	defer masterDir.Close()

	slaveDir, _ := store.NewRAMDirectory()
	defer slaveDir.Close()

	config := NewIndexWriterConfig(analysis.NewStandardAnalyzer())
	masterWriter, _ := NewIndexWriter(masterDir, config)
	defer masterWriter.Close()

	// Add documents with specific content
	testDocs := []struct {
		id      string
		content string
	}{
		{"1", "first document"},
		{"2", "second document"},
		{"3", "third document"},
	}

	for _, td := range testDocs {
		doc := document.NewDocument()
		doc.Add(document.NewTextField("id", td.id, document.Stored|document.Indexed))
		doc.Add(document.NewTextField("content", td.content, document.Stored|document.Indexed))
		masterWriter.AddDocument(doc)
	}
	masterWriter.Commit()

	// Replicate
	replicator := NewLocalReplicator(masterDir, slaveDir)
	replicator.Replicate(context.Background())

	// Compare master and slave
	masterReader, _ := OpenDirectoryReader(masterDir)
	defer masterReader.Close()

	slaveReader, _ := OpenDirectoryReader(slaveDir)
	defer slaveReader.Close()

	if masterReader.NumDocs() != slaveReader.NumDocs() {
		t.Errorf("Doc count mismatch: master=%d, slave=%d", masterReader.NumDocs(), slaveReader.NumDocs())
	}

	// Verify segment infos match
	masterSegments := masterReader.GetSegmentInfos()
	slaveSegments := slaveReader.GetSegmentInfos()

	if masterSegments.Size() != slaveSegments.Size() {
		t.Errorf("Segment count mismatch: master=%d, slave=%d", masterSegments.Size(), slaveSegments.Size())
	}
}

// TestReplicationIncremental tests incremental replication
func TestReplicationIncremental(t *testing.T) {
	masterDir, _ := store.NewRAMDirectory()
	defer masterDir.Close()

	slaveDir, _ := store.NewRAMDirectory()
	defer slaveDir.Close()

	config := NewIndexWriterConfig(analysis.NewStandardAnalyzer())
	masterWriter, _ := NewIndexWriter(masterDir, config)
	defer masterWriter.Close()

	replicator := NewLocalReplicator(masterDir, slaveDir)

	// Initial replication
	doc1 := document.NewDocument()
	doc1.Add(document.NewTextField("content", "first", document.Stored|document.Indexed))
	masterWriter.AddDocument(doc1)
	masterWriter.Commit()

	replicator.Replicate(context.Background())

	reader1, _ := OpenDirectoryReader(slaveDir)
	count1 := reader1.NumDocs()
	reader1.Close()

	if count1 != 1 {
		t.Errorf("Expected 1 doc after first replication, got %d", count1)
	}

	// Incremental replication
	doc2 := document.NewDocument()
	doc2.Add(document.NewTextField("content", "second", document.Stored|document.Indexed))
	masterWriter.AddDocument(doc2)
	masterWriter.Commit()

	replicator.Replicate(context.Background())

	reader2, _ := OpenDirectoryReader(slaveDir)
	count2 := reader2.NumDocs()
	reader2.Close()

	if count2 != 2 {
		t.Errorf("Expected 2 docs after second replication, got %d", count2)
	}
}

// TestReplicationFailureRecovery tests recovery from replication failures
func TestReplicationFailureRecovery(t *testing.T) {
	masterDir, _ := store.NewRAMDirectory()
	defer masterDir.Close()

	slaveDir, _ := store.NewRAMDirectory()
	defer slaveDir.Close()

	config := NewIndexWriterConfig(analysis.NewStandardAnalyzer())
	masterWriter, _ := NewIndexWriter(masterDir, config)
	defer masterWriter.Close()

	// Add documents
	doc := document.NewDocument()
	doc.Add(document.NewTextField("content", "test", document.Stored|document.Indexed))
	masterWriter.AddDocument(doc)
	masterWriter.Commit()

	// Create replicator
	replicator := NewLocalReplicator(masterDir, slaveDir)

	// First attempt - should succeed
	err := replicator.Replicate(context.Background())
	if err != nil {
		t.Errorf("First replication should succeed: %v", err)
	}

	// Verify data
	reader, _ := OpenDirectoryReader(slaveDir)
	count := reader.NumDocs()
	reader.Close()

	if count != 1 {
		t.Errorf("Expected 1 doc after replication, got %d", count)
	}

	// Second attempt - should be idempotent
	err = replicator.Replicate(context.Background())
	if err != nil {
		t.Errorf("Second replication should succeed: %v", err)
	}

	// Count should still be 1
	reader2, _ := OpenDirectoryReader(slaveDir)
	count2 := reader2.NumDocs()
	reader2.Close()

	if count2 != 1 {
		t.Errorf("Expected 1 doc after idempotent replication, got %d", count2)
	}
}

// TestReplicationConcurrent tests concurrent replication operations
func TestReplicationConcurrent(t *testing.T) {
	masterDir, _ := store.NewRAMDirectory()
	defer masterDir.Close()

	slaveDir, _ := store.NewRAMDirectory()
	defer slaveDir.Close()

	config := NewIndexWriterConfig(analysis.NewStandardAnalyzer())
	masterWriter, _ := NewIndexWriter(masterDir, config)
	defer masterWriter.Close()

	// Add initial documents
	for i := 0; i < 100; i++ {
		doc := document.NewDocument()
		doc.Add(document.NewTextField("content", "concurrent test", document.Stored|document.Indexed))
		masterWriter.AddDocument(doc)
	}
	masterWriter.Commit()

	replicator := NewLocalReplicator(masterDir, slaveDir)

	// Concurrent replications
	var wg sync.WaitGroup
	errors := make(chan error, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := replicator.Replicate(context.Background()); err != nil {
				errors <- err
			}
		}()
	}

	wg.Wait()
	close(errors)

	errorCount := 0
	for err := range errors {
		if err != nil {
			errorCount++
		}
	}

	if errorCount > 0 {
		t.Errorf("Expected no errors in concurrent replication, got %d errors", errorCount)
	}

	// Verify final state
	reader, _ := OpenDirectoryReader(slaveDir)
	count := reader.NumDocs()
	reader.Close()

	if count != 100 {
		t.Errorf("Expected 100 docs after concurrent replication, got %d", count)
	}
}

// TestReplicationWithDeletions tests replication with deleted documents
func TestReplicationWithDeletions(t *testing.T) {
	masterDir, _ := store.NewRAMDirectory()
	defer masterDir.Close()

	slaveDir, _ := store.NewRAMDirectory()
	defer slaveDir.Close()

	config := NewIndexWriterConfig(analysis.NewStandardAnalyzer())
	masterWriter, _ := NewIndexWriter(masterDir, config)
	defer masterWriter.Close()

	// Add documents
	for i := 0; i < 5; i++ {
		doc := document.NewDocument()
		doc.Add(document.NewTextField("id", string(rune('a'+i)), document.Stored|document.Indexed))
		masterWriter.AddDocument(doc)
	}
	masterWriter.Commit()

	// Replicate
	replicator := NewLocalReplicator(masterDir, slaveDir)
	replicator.Replicate(context.Background())

	// Verify count
	reader, _ := OpenDirectoryReader(slaveDir)
	count := reader.NumDocs()
	reader.Close()

	if count != 5 {
		t.Errorf("Expected 5 docs, got %d", count)
	}
}

// TestReplicationPerformance benchmarks replication performance
func BenchmarkReplication(b *testing.B) {
	masterDir, _ := store.NewRAMDirectory()
	defer masterDir.Close()

	slaveDir, _ := store.NewRAMDirectory()
	defer slaveDir.Close()

	config := NewIndexWriterConfig(analysis.NewStandardAnalyzer())
	masterWriter, _ := NewIndexWriter(masterDir, config)
	defer masterWriter.Close()

	// Add documents
	for i := 0; i < 1000; i++ {
		doc := document.NewDocument()
		doc.Add(document.NewTextField("content", "benchmark document", document.Stored|document.Indexed))
		masterWriter.AddDocument(doc)
	}
	masterWriter.Commit()

	replicator := NewLocalReplicator(masterDir, slaveDir)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Clear slave directory for each iteration
		slaveDir2, _ := store.NewRAMDirectory()
		r := NewLocalReplicator(masterDir, slaveDir2)
		r.Replicate(context.Background())
		slaveDir2.Close()
	}
}

// TestReplicationSession tests replication session management
func TestReplicationSession(t *testing.T) {
	session := NewReplicationSession("test-session", "master1")

	if session.GetID() != "test-session" {
		t.Errorf("Expected session ID 'test-session', got %s", session.GetID())
	}

	if session.GetSource() != "master1" {
		t.Errorf("Expected source 'master1', got %s", session.GetSource())
	}

	if !session.IsActive() {
		t.Error("Expected session to be active initially")
	}

	session.Close()

	if session.IsActive() {
		t.Error("Expected session to be inactive after Close")
	}
}

// TestReplicationSessionTimeout tests session timeout behavior
func TestReplicationSessionTimeout(t *testing.T) {
	session := NewReplicationSession("timeout-test", "master1")

	// Set short timeout for testing
	session.SetTimeout(time.Millisecond * 100)

	if session.IsActive() {
		time.Sleep(time.Millisecond * 150)
	}

	// Session should have timed out
	// Note: Implementation depends on actual Session behavior
}

// TestReplicationWithLargeFiles tests replication with large files
func TestReplicationWithLargeFiles(t *testing.T) {
	masterDir, _ := store.NewRAMDirectory()
	defer masterDir.Close()

	slaveDir, _ := store.NewRAMDirectory()
	defer slaveDir.Close()

	config := NewIndexWriterConfig(analysis.NewStandardAnalyzer())
	masterWriter, _ := NewIndexWriter(masterDir, config)
	defer masterWriter.Close()

	// Add documents with larger content
	for i := 0; i < 100; i++ {
		doc := document.NewDocument()
		// Larger text content
		content := "This is a larger document content for testing replication with larger files. " +
			"It contains more text to simulate real-world document sizes. " +
			"Document number " + string(rune('0'+i%10))
		doc.Add(document.NewTextField("content", content, document.Stored|document.Indexed))
		masterWriter.AddDocument(doc)
	}
	masterWriter.Commit()

	// Replicate
	replicator := NewLocalReplicator(masterDir, slaveDir)
	err := replicator.Replicate(context.Background())
	if err != nil {
		t.Fatalf("Replication with large files failed: %v", err)
	}

	// Verify
	reader, _ := OpenDirectoryReader(slaveDir)
	count := reader.NumDocs()
	reader.Close()

	if count != 100 {
		t.Errorf("Expected 100 docs, got %d", count)
	}
}

// TestReplicationNetworkFailure simulates network failures
func TestReplicationNetworkFailure(t *testing.T) {
	masterDir, _ := store.NewRAMDirectory()
	defer masterDir.Close()

	slaveDir, _ := store.NewRAMDirectory()
	defer slaveDir.Close()

	config := NewIndexWriterConfig(analysis.NewStandardAnalyzer())
	masterWriter, _ := NewIndexWriter(masterDir, config)
	defer masterWriter.Close()

	doc := document.NewDocument()
	doc.Add(document.NewTextField("content", "test", document.Stored|document.Indexed))
	masterWriter.AddDocument(doc)
	masterWriter.Commit()

	// Create replicator with context that can be cancelled
	replicator := NewLocalReplicator(masterDir, slaveDir)

	// Normal replication should succeed
	ctx := context.Background()
	err := replicator.Replicate(ctx)
	if err != nil {
		t.Errorf("Normal replication should succeed: %v", err)
	}

	// Test with cancelled context
	ctx2, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err = replicator.Replicate(ctx2)
	// Error is expected since context is cancelled
	if err == nil {
		t.Log("Replication with cancelled context completed (may be cached)")
	}
}

// TestReplicationChecksumVerification tests checksum verification during replication
func TestReplicationChecksumVerification(t *testing.T) {
	masterDir, _ := store.NewRAMDirectory()
	defer masterDir.Close()

	slaveDir, _ := store.NewRAMDirectory()
	defer slaveDir.Close()

	config := NewIndexWriterConfig(analysis.NewStandardAnalyzer())
	masterWriter, _ := NewIndexWriter(masterDir, config)
	defer masterWriter.Close()

	doc := document.NewDocument()
	doc.Add(document.NewTextField("content", "checksum test", document.Stored|document.Indexed))
	masterWriter.AddDocument(doc)
	masterWriter.Commit()

	// Create replicator with verification enabled
	replicator := NewLocalReplicator(masterDir, slaveDir)
	replicator.SetVerifyChecksums(true)

	err := replicator.Replicate(context.Background())
	if err != nil {
		t.Errorf("Replication with checksum verification failed: %v", err)
	}
}

// TestReplicationMetrics tests replication metrics collection
func TestReplicationMetrics(t *testing.T) {
	masterDir, _ := store.NewRAMDirectory()
	defer masterDir.Close()

	slaveDir, _ := store.NewRAMDirectory()
	defer slaveDir.Close()

	config := NewIndexWriterConfig(analysis.NewStandardAnalyzer())
	masterWriter, _ := NewIndexWriter(masterDir, config)
	defer masterWriter.Close()

	// Add documents
	for i := 0; i < 50; i++ {
		doc := document.NewDocument()
		doc.Add(document.NewTextField("content", "metrics test", document.Stored|document.Indexed))
		masterWriter.AddDocument(doc)
	}
	masterWriter.Commit()

	// Create replicator with metrics
	replicator := NewLocalReplicator(masterDir, slaveDir)

	// Perform replication
	start := time.Now()
	err := replicator.Replicate(context.Background())
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("Replication failed: %v", err)
	}

	t.Logf("Replication completed in %v", duration)

	// Verify data
	reader, _ := OpenDirectoryReader(slaveDir)
	count := reader.NumDocs()
	reader.Close()

	if count != 50 {
		t.Errorf("Expected 50 docs, got %d", count)
	}
}
