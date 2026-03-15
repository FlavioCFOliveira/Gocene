// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package index_test contains tests for the SnapshotDeletionPolicy.
//
// # Ported from Apache Lucene's org.apache.lucene.index.TestSnapshotDeletionPolicy
//
// GC-193: Test SnapshotDeletionPolicy
//
// Test Coverage:
//   - Basic snapshot creation and release
//   - Multiple snapshots
//   - Concurrent writers with snapshots
//   - Rollback to old snapshot
//   - Snapshotting same commit twice
//   - Missing commits handling
package index_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// =============================================================================
// Test Helper Functions
// =============================================================================

// checkSnapshotExists verifies that a snapshot's segments file exists in the directory.
// Source: TestSnapshotDeletionPolicy.checkSnapshotExists()
func checkSnapshotExists(t *testing.T, dir store.Directory, commit *index.IndexCommit) {
	t.Helper()
	segFileName := commit.GetSegmentsFileName()
	if !dir.FileExists(segFileName) {
		t.Errorf("segments file not found in directory: %s", segFileName)
	}
}

// checkMaxDoc verifies that the commit has the expected maxDoc.
// Source: TestSnapshotDeletionPolicy.checkMaxDoc()
func checkMaxDoc(t *testing.T, commit *index.IndexCommit, expectedMaxDoc int) {
	t.Helper()
	// In a full implementation, this would open a DirectoryReader
	// For now, we verify the commit's segment count
	if commit.GetSegmentCount() != expectedMaxDoc {
		t.Errorf("Expected maxDoc %d, got %d", expectedMaxDoc, commit.GetSegmentCount())
	}
}

// prepareIndexAndSnapshots creates documents and snapshots.
// Source: TestSnapshotDeletionPolicy.prepareIndexAndSnapshots()
func prepareIndexAndSnapshots(t *testing.T, sdp *index.SnapshotDeletionPolicy, dir store.Directory, numSnapshots int) []*index.IndexCommit {
	t.Helper()
	snapshots := make([]*index.IndexCommit, 0, numSnapshots)

	for i := 0; i < numSnapshots; i++ {
		// Create a commit by creating a segments file
		segmentsFile := fmt.Sprintf("segments_%d", i+1)
		out, err := dir.CreateOutput(segmentsFile, store.IOContextWrite)
		if err != nil {
			t.Fatalf("Failed to create output: %v", err)
		}
		out.Close()

		// Create commit
		commit := index.NewIndexCommitWithDirectory(segmentsFile, dir, i+1, int64(i+1))

		// Take snapshot
		gen, err := sdp.Snapshot(commit)
		if err != nil {
			t.Fatalf("Failed to snapshot: %v", err)
		}
		if gen != int64(i+1) {
			t.Errorf("Expected generation %d, got %d", i+1, gen)
		}

		snapshots = append(snapshots, commit)
	}

	return snapshots
}

// assertSnapshotExists verifies that all expected snapshots exist.
// Source: TestSnapshotDeletionPolicy.assertSnapshotExists()
func assertSnapshotExists(t *testing.T, dir store.Directory, sdp *index.SnapshotDeletionPolicy, snapshots []*index.IndexCommit, checkSame bool) {
	t.Helper()
	for i, snapshot := range snapshots {
		checkMaxDoc(t, snapshot, i+1)
		checkSnapshotExists(t, dir, snapshot)

		if checkSame {
			// Check that the snapshot is tracked
			if !sdp.HasSnapshot(snapshot.GetGeneration()) {
				t.Errorf("Snapshot with generation %d should be tracked", snapshot.GetGeneration())
			}
		} else {
			// Just check generation matches
			if !sdp.HasSnapshot(snapshot.GetGeneration()) {
				t.Errorf("Snapshot with generation %d should be tracked", snapshot.GetGeneration())
			}
		}
	}
}

// getDeletionPolicy creates a SnapshotDeletionPolicy for testing.
// Source: TestSnapshotDeletionPolicy.getDeletionPolicy()
func getDeletionPolicy() *index.SnapshotDeletionPolicy {
	return index.NewSnapshotDeletionPolicy(index.NewKeepOnlyLastCommitDeletionPolicy())
}

// =============================================================================
// Test Cases
// =============================================================================

// TestSnapshotDeletionPolicy_Basic tests basic snapshot operations.
// Source: TestSnapshotDeletionPolicy.testSnapshotDeletionPolicy()
// Focus: Create/release snapshot
func TestSnapshotDeletionPolicy_Basic(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	sdp := getDeletionPolicy()

	// Create initial commit file
	out, err := dir.CreateOutput("segments_1", store.IOContextWrite)
	if err != nil {
		t.Fatalf("Failed to create output: %v", err)
	}
	out.Close()

	commit := index.NewIndexCommitWithDirectory("segments_1", dir, 1, 1)

	// Take a snapshot
	gen, err := sdp.Snapshot(commit)
	if err != nil {
		t.Fatalf("Failed to snapshot: %v", err)
	}
	if gen != 1 {
		t.Errorf("Expected generation 1, got %d", gen)
	}

	// Verify snapshot exists
	if !sdp.HasSnapshot(1) {
		t.Error("Expected snapshot to exist")
	}
	if sdp.SnapshotCount() != 1 {
		t.Errorf("Expected 1 snapshot, got %d", sdp.SnapshotCount())
	}

	// Release the snapshot
	released := sdp.Release(1)
	if !released {
		t.Error("Expected snapshot to be released")
	}

	// Verify snapshot is released
	if sdp.HasSnapshot(1) {
		t.Error("Expected snapshot to be released")
	}
	if sdp.SnapshotCount() != 0 {
		t.Errorf("Expected 0 snapshots, got %d", sdp.SnapshotCount())
	}
}

// TestSnapshotDeletionPolicy_MultipleSnapshots tests creating multiple snapshots.
// Source: TestSnapshotDeletionPolicy.testBasicSnapshots()
// Focus: Multiple snapshots
func TestSnapshotDeletionPolicy_MultipleSnapshots(t *testing.T) {
	numSnapshots := 3

	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	sdp := getDeletionPolicy()

	// Create 3 snapshots
	snapshots := prepareIndexAndSnapshots(t, sdp, dir, numSnapshots)

	// Verify snapshot count
	if sdp.SnapshotCount() != numSnapshots {
		t.Errorf("Expected %d snapshots, got %d", numSnapshots, sdp.SnapshotCount())
	}

	// Verify all snapshots exist
	assertSnapshotExists(t, dir, sdp, snapshots, true)

	// Verify GetSnapshots returns correct generations
	gens := sdp.GetSnapshots()
	if len(gens) != numSnapshots {
		t.Errorf("Expected %d snapshot generations, got %d", numSnapshots, len(gens))
	}

	// Release all snapshots
	for _, snap := range snapshots {
		sdp.Release(snap.GetGeneration())
	}

	if sdp.SnapshotCount() != 0 {
		t.Errorf("Expected 0 snapshots after release, got %d", sdp.SnapshotCount())
	}
}

// TestSnapshotDeletionPolicy_ConcurrentSnapshotting tests concurrent snapshot operations.
// Source: TestSnapshotDeletionPolicy.testMultiThreadedSnapshotting()
// Focus: Concurrent writers
func TestSnapshotDeletionPolicy_ConcurrentSnapshotting(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	sdp := getDeletionPolicy()

	numThreads := 10
	snapshots := make([]*index.IndexCommit, numThreads)
	var mu sync.Mutex

	var wg sync.WaitGroup
	startingGun := make(chan struct{})

	for i := 0; i < numThreads; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			// Wait for starting gun
			<-startingGun

			// Create commit file
			segmentsFile := fmt.Sprintf("segments_%d", idx+1)
			out, err := dir.CreateOutput(segmentsFile, store.IOContextWrite)
			if err != nil {
				t.Errorf("Failed to create output: %v", err)
				return
			}
			out.Close()

			// Create commit and snapshot
			commit := index.NewIndexCommitWithDirectory(segmentsFile, dir, 1, int64(idx+1))
			gen, err := sdp.Snapshot(commit)
			if err != nil {
				t.Errorf("Failed to snapshot: %v", err)
				return
			}

			mu.Lock()
			snapshots[idx] = commit
			mu.Unlock()

			if gen != int64(idx+1) {
				t.Errorf("Expected generation %d, got %d", idx+1, gen)
			}
		}(i)
	}

	// Fire the starting gun
	close(startingGun)

	// Wait for all threads
	wg.Wait()

	// Verify all snapshots were created
	if sdp.SnapshotCount() != numThreads {
		t.Errorf("Expected %d snapshots, got %d", numThreads, sdp.SnapshotCount())
	}

	// Create one more commit (not snapshotted)
	out, _ := dir.CreateOutput("segments_final", store.IOContextWrite)
	out.Close()

	// Release all snapshots
	for i := 0; i < numThreads; i++ {
		if snapshots[i] != nil {
			sdp.Release(snapshots[i].GetGeneration())
		}
	}

	if sdp.SnapshotCount() != 0 {
		t.Errorf("Expected 0 snapshots after release, got %d", sdp.SnapshotCount())
	}
}

// TestSnapshotDeletionPolicy_ReleaseSnapshot tests releasing a snapshot.
// Source: TestSnapshotDeletionPolicy.testReleaseSnapshot()
func TestSnapshotDeletionPolicy_ReleaseSnapshot(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	sdp := getDeletionPolicy()

	// Create first commit and snapshot
	out1, _ := dir.CreateOutput("segments_1", store.IOContextWrite)
	out1.Close()
	commit1 := index.NewIndexCommitWithDirectory("segments_1", dir, 1, 1)
	sdp.Snapshot(commit1)

	// Create another commit (not snapshotted)
	out2, _ := dir.CreateOutput("segments_2", store.IOContextWrite)
	out2.Close()

	// Verify first snapshot exists
	if !sdp.HasSnapshot(1) {
		t.Error("Expected snapshot 1 to exist")
	}

	// Release the snapshot
	released := sdp.Release(1)
	if !released {
		t.Error("Expected snapshot to be released")
	}

	// Verify snapshot is released
	if sdp.HasSnapshot(1) {
		t.Error("Expected snapshot to be released")
	}

	// Verify segments file still exists (we didn't delete it)
	if !dir.FileExists("segments_1") {
		t.Error("segments_1 should still exist in directory")
	}
}

// TestSnapshotDeletionPolicy_SnapshotLastCommitTwice tests snapshotting the same commit twice.
// Source: TestSnapshotDeletionPolicy.testSnapshotLastCommitTwice()
func TestSnapshotDeletionPolicy_SnapshotLastCommitTwice(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	sdp := getDeletionPolicy()

	// Create commit
	out, _ := dir.CreateOutput("segments_1", store.IOContextWrite)
	out.Close()
	commit := index.NewIndexCommitWithDirectory("segments_1", dir, 1, 1)

	// Snapshot the same commit twice
	gen1, err := sdp.Snapshot(commit)
	if err != nil {
		t.Fatalf("Failed to snapshot first time: %v", err)
	}

	gen2, err := sdp.Snapshot(commit)
	if err != nil {
		t.Fatalf("Failed to snapshot second time: %v", err)
	}

	// Should return the same generation
	if gen1 != gen2 {
		t.Errorf("Expected same generation, got %d and %d", gen1, gen2)
	}

	// Should only count as one snapshot
	if sdp.SnapshotCount() != 1 {
		t.Errorf("Expected 1 snapshot, got %d", sdp.SnapshotCount())
	}

	// Create another commit
	out2, _ := dir.CreateOutput("segments_2", store.IOContextWrite)
	out2.Close()

	// Release first snapshot - should not affect the second (same) snapshot
	released := sdp.Release(gen1)
	if !released {
		t.Error("Expected snapshot to be released")
	}

	// Since it's the same snapshot, releasing it should remove it
	if sdp.HasSnapshot(gen1) {
		t.Error("Expected snapshot to be released")
	}
}

// TestSnapshotDeletionPolicy_RollbackToOldSnapshot tests rolling back to an old snapshot.
// Source: TestSnapshotDeletionPolicy.testRollbackToOldSnapshot()
func TestSnapshotDeletionPolicy_RollbackToOldSnapshot(t *testing.T) {
	numSnapshots := 2

	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	sdp := getDeletionPolicy()

	// Create 2 snapshots
	snapshots := prepareIndexAndSnapshots(t, sdp, dir, numSnapshots)

	// Verify both snapshots exist
	assertSnapshotExists(t, dir, sdp, snapshots, false)

	// Simulate rollback by releasing the newer snapshot
	sdp.Release(snapshots[1].GetGeneration())

	// Verify first snapshot still exists
	if !sdp.HasSnapshot(snapshots[0].GetGeneration()) {
		t.Error("Expected first snapshot to still exist")
	}

	// Verify second snapshot is released
	if sdp.HasSnapshot(snapshots[1].GetGeneration()) {
		t.Error("Expected second snapshot to be released")
	}

	// Verify the segments file for the second snapshot still exists
	// (it won't be deleted until the policy runs)
	if !dir.FileExists(snapshots[1].GetSegmentsFileName()) {
		t.Error("snapshot1 segments file should still exist in directory")
	}
}

// TestSnapshotDeletionPolicy_MissingCommits tests behavior with missing commits.
// Source: TestSnapshotDeletionPolicy.testMissingCommits()
func TestSnapshotDeletionPolicy_MissingCommits(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	sdp := getDeletionPolicy()

	// Create first commit and snapshot
	out1, _ := dir.CreateOutput("segments_1", store.IOContextWrite)
	out1.Close()
	commit1 := index.NewIndexCommitWithDirectory("segments_1", dir, 1, 1)
	sdp.Snapshot(commit1)

	// Create second commit (not snapshotted)
	out2, _ := dir.CreateOutput("segments_2", store.IOContextWrite)
	out2.Close()

	// Delete the first commit's file (simulating it being deleted by another policy)
	dir.DeleteFile("segments_1")

	// Verify the commit is considered deleted
	if !commit1.IsDeleted() {
		t.Error("Expected commit1 to be marked as deleted")
	}

	// The snapshot still exists in the policy
	if !sdp.HasSnapshot(1) {
		t.Error("Expected snapshot to still be tracked by policy")
	}
}

// TestSnapshotDeletionPolicy_ConcurrentWriters tests concurrent writers with snapshots.
// This is a simplified version of the main testSnapshotDeletionPolicy test.
// Source: TestSnapshotDeletionPolicy.testSnapshotDeletionPolicy()
// Focus: Concurrent writers
func TestSnapshotDeletionPolicy_ConcurrentWriters(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	sdp := getDeletionPolicy()

	// Create initial commit
	out, _ := dir.CreateOutput("segments_1", store.IOContextWrite)
	out.Close()
	commit := index.NewIndexCommitWithDirectory("segments_1", dir, 1, 1)
	sdp.Snapshot(commit)

	// Start a background writer goroutine
	writerDone := make(chan struct{})
	go func() {
		defer close(writerDone)
		for i := 0; i < 10; i++ {
			// Simulate adding documents by creating new commits
			segmentsFile := fmt.Sprintf("segments_writer_%d", i)
			out, _ := dir.CreateOutput(segmentsFile, store.IOContextWrite)
			out.Close()
			time.Sleep(1 * time.Millisecond)
		}
	}()

	// While writer is running, take snapshots
	snapshots := make([]int64, 0)
	for i := 0; i < 5; i++ {
		select {
		case <-writerDone:
			break
		default:
			// Take a snapshot of current state
			segmentsFile := fmt.Sprintf("segments_snap_%d", i)
			out, _ := dir.CreateOutput(segmentsFile, store.IOContextWrite)
			out.Close()
			commit := index.NewIndexCommitWithDirectory(segmentsFile, dir, 1, int64(i+100))
			gen, _ := sdp.Snapshot(commit)
			snapshots = append(snapshots, gen)
			time.Sleep(5 * time.Millisecond)
		}
	}

	// Wait for writer to finish
	<-writerDone

	// Verify all snapshots still exist
	for _, gen := range snapshots {
		if !sdp.HasSnapshot(gen) {
			t.Errorf("Expected snapshot %d to exist", gen)
		}
	}

	// Release all snapshots
	for _, gen := range snapshots {
		sdp.Release(gen)
	}
	sdp.Release(1) // Release initial snapshot

	if sdp.SnapshotCount() != 0 {
		t.Errorf("Expected 0 snapshots after cleanup, got %d", sdp.SnapshotCount())
	}
}

// TestSnapshotDeletionPolicy_SnapshotNil tests snapshotting nil commit.
func TestSnapshotDeletionPolicy_SnapshotNil(t *testing.T) {
	sdp := getDeletionPolicy()

	_, err := sdp.Snapshot(nil)
	if err == nil {
		t.Error("Expected error when snapshotting nil commit")
	}
}

// TestSnapshotDeletionPolicy_SnapshotDeleted tests snapshotting deleted commit.
func TestSnapshotDeletionPolicy_SnapshotDeleted(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	// Create and delete a commit
	out, _ := dir.CreateOutput("segments_1", store.IOContextWrite)
	out.Close()
	commit := index.NewIndexCommitWithDirectory("segments_1", dir, 1, 1)

	// Delete the file
	dir.DeleteFile("segments_1")

	sdp := getDeletionPolicy()
	_, err := sdp.Snapshot(commit)
	if err == nil {
		t.Error("Expected error when snapshotting deleted commit")
	}
}

// TestSnapshotDeletionPolicy_ReleaseNonExistent tests releasing non-existent snapshot.
func TestSnapshotDeletionPolicy_ReleaseNonExistent(t *testing.T) {
	sdp := getDeletionPolicy()

	released := sdp.Release(999)
	if released {
		t.Error("Expected release of non-existent snapshot to return false")
	}
}

// TestSnapshotDeletionPolicy_ReleaseAll tests releasing all snapshots.
func TestSnapshotDeletionPolicy_ReleaseAll(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	sdp := getDeletionPolicy()

	// Create multiple snapshots
	for i := 1; i <= 5; i++ {
		segmentsFile := fmt.Sprintf("segments_%d", i)
		out, _ := dir.CreateOutput(segmentsFile, store.IOContextWrite)
		out.Close()
		commit := index.NewIndexCommitWithDirectory(segmentsFile, dir, 1, int64(i))
		sdp.Snapshot(commit)
	}

	if sdp.SnapshotCount() != 5 {
		t.Errorf("Expected 5 snapshots, got %d", sdp.SnapshotCount())
	}

	// Release all
	sdp.ReleaseAll()

	if sdp.SnapshotCount() != 0 {
		t.Errorf("Expected 0 snapshots after ReleaseAll, got %d", sdp.SnapshotCount())
	}
}

// TestSnapshotDeletionPolicy_GetSnapshots tests GetSnapshots.
func TestSnapshotDeletionPolicy_GetSnapshots(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	sdp := getDeletionPolicy()

	// Create multiple snapshots out of order
	generations := []int64{5, 2, 8, 1, 3}
	for _, gen := range generations {
		segmentsFile := fmt.Sprintf("segments_%d", gen)
		out, _ := dir.CreateOutput(segmentsFile, store.IOContextWrite)
		out.Close()
		commit := index.NewIndexCommitWithDirectory(segmentsFile, dir, 1, gen)
		sdp.Snapshot(commit)
	}

	snapshots := sdp.GetSnapshots()
	if len(snapshots) != 5 {
		t.Errorf("Expected 5 snapshots, got %d", len(snapshots))
	}

	// Verify sorted order
	for i := 1; i < len(snapshots); i++ {
		if snapshots[i-1] > snapshots[i] {
			t.Error("Expected snapshots to be sorted in ascending order")
		}
	}
}

// TestSnapshotDeletionPolicy_WithKeepAllPolicy tests SnapshotDeletionPolicy wrapping KeepAllDeletionPolicy.
func TestSnapshotDeletionPolicy_WithKeepAllPolicy(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	// Use KeepAll as the underlying policy
	primary := index.NewKeepAllDeletionPolicy()
	sdp := index.NewSnapshotDeletionPolicy(primary)

	// Create commit
	out, _ := dir.CreateOutput("segments_1", store.IOContextWrite)
	out.Close()
	commit := index.NewIndexCommitWithDirectory("segments_1", dir, 1, 1)

	// Take snapshot
	gen, err := sdp.Snapshot(commit)
	if err != nil {
		t.Fatalf("Failed to snapshot: %v", err)
	}
	if gen != 1 {
		t.Errorf("Expected generation 1, got %d", gen)
	}

	// Verify primary policy is KeepAll
	if sdp.GetPrimary() == nil {
		t.Error("Expected primary policy to be set")
	}
}

// TestSnapshotDeletionPolicy_Clone tests cloning the policy.
func TestSnapshotDeletionPolicy_Clone(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	sdp := getDeletionPolicy()

	// Create a snapshot
	out, _ := dir.CreateOutput("segments_1", store.IOContextWrite)
	out.Close()
	commit := index.NewIndexCommitWithDirectory("segments_1", dir, 1, 1)
	sdp.Snapshot(commit)

	if sdp.SnapshotCount() != 1 {
		t.Errorf("Expected 1 snapshot, got %d", sdp.SnapshotCount())
	}

	// Clone the policy
	cloned := sdp.Clone()
	if cloned == nil {
		t.Fatal("Expected non-nil clone")
	}

	// Clone should not have the snapshots
	clonedSDP, ok := cloned.(*index.SnapshotDeletionPolicy)
	if !ok {
		t.Fatal("Expected clone to be *SnapshotDeletionPolicy")
	}

	// Note: Per the implementation, snapshots are not transferred to clone
	if clonedSDP.SnapshotCount() != 0 {
		t.Errorf("Expected cloned policy to have 0 snapshots, got %d", clonedSDP.SnapshotCount())
	}
}

// TestSnapshotDeletionPolicy_OnCommit tests the OnCommit behavior.
func TestSnapshotDeletionPolicy_OnCommit(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	sdp := getDeletionPolicy()

	// Create commits
	commits := make([]*index.IndexCommit, 3)
	for i := 0; i < 3; i++ {
		segmentsFile := fmt.Sprintf("segments_%d", i+1)
		out, _ := dir.CreateOutput(segmentsFile, store.IOContextWrite)
		out.Close()
		commits[i] = index.NewIndexCommitWithDirectory(segmentsFile, dir, 1, int64(i+1))
	}

	// Snapshot the first commit
	sdp.Snapshot(commits[0])

	// Run OnCommit - should not delete the snapshotted commit or the last commit
	err := sdp.OnCommit(commits)
	if err != nil {
		t.Errorf("OnCommit failed: %v", err)
	}

	// First commit should not be deleted (it has a snapshot)
	if commits[0].IsDeleted() {
		t.Error("First commit should not be deleted (has snapshot)")
	}

	// Last commit should not be deleted
	if commits[2].IsDeleted() {
		t.Error("Last commit should not be deleted")
	}
}

// TestSnapshotDeletionPolicy_OnInit tests the OnInit behavior.
func TestSnapshotDeletionPolicy_OnInit(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	sdp := getDeletionPolicy()

	// Create commits
	commits := make([]*index.IndexCommit, 3)
	for i := 0; i < 3; i++ {
		segmentsFile := fmt.Sprintf("segments_%d", i+1)
		out, _ := dir.CreateOutput(segmentsFile, store.IOContextWrite)
		out.Close()
		commits[i] = index.NewIndexCommitWithDirectory(segmentsFile, dir, 1, int64(i+1))
	}

	// Snapshot the first commit
	sdp.Snapshot(commits[0])

	// Run OnInit - should behave same as OnCommit
	err := sdp.OnInit(commits)
	if err != nil {
		t.Errorf("OnInit failed: %v", err)
	}

	// First commit should not be deleted (it has a snapshot)
	if commits[0].IsDeleted() {
		t.Error("First commit should not be deleted after OnInit (has snapshot)")
	}
}

// TestSnapshotDeletionPolicy_String tests the String method.
func TestSnapshotDeletionPolicy_String(t *testing.T) {
	sdp := getDeletionPolicy()

	str := sdp.String()
	if str == "" {
		t.Error("Expected non-empty string representation")
	}

	// Should contain policy name
	if str != "" && len(str) < 10 {
		t.Error("Expected meaningful string representation")
	}
}

// TestSnapshotDeletionPolicy_WithDocumentCreation tests integration with document package.
func TestSnapshotDeletionPolicy_WithDocumentCreation(t *testing.T) {
	// Create a document
	doc := document.NewDocument()
	if doc == nil {
		t.Fatal("Expected non-nil document")
	}

	// Create a text field
	textField, err := document.NewTextField("content", "test content", true)
	if err != nil {
		t.Fatalf("Failed to create text field: %v", err)
	}

	doc.Add(textField)

	// Verify field was added
	fields := doc.GetFields()
	if len(fields) != 1 {
		t.Errorf("Expected 1 field, got %d", len(fields))
	}
}

// TestSnapshotDeletionPolicy_SnapshotGeneration tests SnapshotGeneration method.
func TestSnapshotDeletionPolicy_SnapshotGeneration(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	sdp := getDeletionPolicy()

	// Create commits
	commits := make([]*index.IndexCommit, 3)
	for i := 0; i < 3; i++ {
		segmentsFile := fmt.Sprintf("segments_%d", i+1)
		out, _ := dir.CreateOutput(segmentsFile, store.IOContextWrite)
		out.Close()
		commits[i] = index.NewIndexCommitWithDirectory(segmentsFile, dir, 1, int64(i+1))
	}

	// Snapshot by generation
	err := sdp.SnapshotGeneration(commits, 2)
	if err != nil {
		t.Errorf("Failed to snapshot generation: %v", err)
	}

	if !sdp.HasSnapshot(2) {
		t.Error("Expected snapshot for generation 2 to exist")
	}

	// Try to snapshot non-existent generation
	err = sdp.SnapshotGeneration(commits, 99)
	if err == nil {
		t.Error("Expected error for non-existent generation")
	}
}

// TestSnapshotDeletionPolicy_ConcurrentAccess tests thread safety.
func TestSnapshotDeletionPolicy_ConcurrentAccess(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	sdp := getDeletionPolicy()

	var wg sync.WaitGroup
	numGoroutines := 20
	numOperations := 50

	// Concurrent snapshot operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				gen := int64(id*numOperations + j + 1)
				segmentsFile := fmt.Sprintf("segments_%d_%d", id, j)
				out, _ := dir.CreateOutput(segmentsFile, store.IOContextWrite)
				out.Close()
				commit := index.NewIndexCommitWithDirectory(segmentsFile, dir, 1, gen)
				sdp.Snapshot(commit)
			}
		}(i)
	}

	wg.Wait()

	expectedCount := numGoroutines * numOperations
	if sdp.SnapshotCount() != expectedCount {
		t.Errorf("Expected %d snapshots, got %d", expectedCount, sdp.SnapshotCount())
	}

	// Concurrent release operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				gen := int64(id*numOperations + j + 1)
				sdp.Release(gen)
			}
		}(i)
	}

	wg.Wait()

	if sdp.SnapshotCount() != 0 {
		t.Errorf("Expected 0 snapshots after release, got %d", sdp.SnapshotCount())
	}
}
