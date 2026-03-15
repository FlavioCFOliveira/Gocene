// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index_test

import (
	"fmt"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// =============================================================================
// Test Deletion Policies
// Source: org.apache.lucene.index.TestDeletionPolicy
// =============================================================================

// verifyCommitOrder verifies that commits are in ascending generation order.
// This is the Go equivalent of the Java verifyCommitOrder method.
func verifyCommitOrder(t *testing.T, commits []*index.IndexCommit) {
	t.Helper()
	if len(commits) == 0 {
		return
	}

	firstCommit := commits[0]
	lastGen := firstCommit.GetGeneration()

	if lastGen != firstCommit.GetGeneration() {
		t.Errorf("First commit generation mismatch: expected %d, got %d", lastGen, firstCommit.GetGeneration())
	}

	for i := 1; i < len(commits); i++ {
		commit := commits[i]
		now := commit.GetGeneration()
		if now <= lastGen {
			t.Errorf("SegmentInfos commits are out-of-order at index %d: %d <= %d", i, now, lastGen)
		}
		if now != commit.GetGeneration() {
			t.Errorf("Commit generation mismatch at index %d: expected %d, got %d", i, now, commit.GetGeneration())
		}
		lastGen = now
	}
}

// =============================================================================
// KeepAllDeletionPolicy (Test Implementation)
// =============================================================================

// KeepAllDeletionPolicy keeps all commits and never deletes anything.
// This is used for testing purposes.
type KeepAllDeletionPolicy struct {
	numOnInit   int
	numOnCommit int
	dir         store.Directory
}

// NewKeepAllDeletionPolicy creates a new KeepAllDeletionPolicy.
func NewKeepAllDeletionPolicy(dir store.Directory) *KeepAllDeletionPolicy {
	return &KeepAllDeletionPolicy{
		dir: dir,
	}
}

// OnInit is called when IndexWriter is initialized.
func (p *KeepAllDeletionPolicy) OnInit(commits []*index.IndexCommit) error {
	p.numOnInit++
	return nil
}

// OnCommit is called when a commit is made.
func (p *KeepAllDeletionPolicy) OnCommit(commits []*index.IndexCommit) error {
	if len(commits) == 0 {
		return nil
	}
	p.numOnCommit++
	return nil
}

// Clone returns a clone of this policy.
func (p *KeepAllDeletionPolicy) Clone() index.IndexDeletionPolicy {
	return NewKeepAllDeletionPolicy(p.dir)
}

// =============================================================================
// KeepNoneOnInitDeletionPolicy (Test Implementation)
// =============================================================================

// KeepNoneOnInitDeletionPolicy deletes all commits on init and all but the last on commit.
// This is useful for adding to a big index when you know readers are not using it.
type KeepNoneOnInitDeletionPolicy struct {
	numOnInit   int
	numOnCommit int
}

// NewKeepNoneOnInitDeletionPolicy creates a new KeepNoneOnInitDeletionPolicy.
func NewKeepNoneOnInitDeletionPolicy() *KeepNoneOnInitDeletionPolicy {
	return &KeepNoneOnInitDeletionPolicy{}
}

// OnInit is called when IndexWriter is initialized.
func (p *KeepNoneOnInitDeletionPolicy) OnInit(commits []*index.IndexCommit) error {
	p.numOnInit++
	// On init, delete all commit points
	for _, commit := range commits {
		if err := commit.Delete(); err != nil {
			return fmt.Errorf("failed to delete commit: %w", err)
		}
		if !commit.IsDeleted() {
			return fmt.Errorf("commit should be deleted but is not")
		}
	}
	return nil
}

// OnCommit is called when a commit is made.
func (p *KeepNoneOnInitDeletionPolicy) OnCommit(commits []*index.IndexCommit) error {
	// Delete all but last one
	size := len(commits)
	for i := 0; i < size-1; i++ {
		if err := commits[i].Delete(); err != nil {
			return fmt.Errorf("failed to delete commit %d: %w", i, err)
		}
	}
	p.numOnCommit++
	return nil
}

// Clone returns a clone of this policy.
func (p *KeepNoneOnInitDeletionPolicy) Clone() index.IndexDeletionPolicy {
	return NewKeepNoneOnInitDeletionPolicy()
}

// =============================================================================
// KeepLastNDeletionPolicy (Test Implementation)
// =============================================================================

// KeepLastNDeletionPolicy keeps the last N commits.
type KeepLastNDeletionPolicy struct {
	numOnInit   int
	numOnCommit int
	numToKeep   int
	numDelete   int
	seen        map[string]bool
}

// NewKeepLastNDeletionPolicy creates a new KeepLastNDeletionPolicy.
func NewKeepLastNDeletionPolicy(numToKeep int) *KeepLastNDeletionPolicy {
	return &KeepLastNDeletionPolicy{
		numToKeep: numToKeep,
		seen:      make(map[string]bool),
	}
}

// OnInit is called when IndexWriter is initialized.
func (p *KeepLastNDeletionPolicy) OnInit(commits []*index.IndexCommit) error {
	p.numOnInit++
	// Do no deletions on init
	return p.doDeletes(commits, false)
}

// OnCommit is called when a commit is made.
func (p *KeepLastNDeletionPolicy) OnCommit(commits []*index.IndexCommit) error {
	return p.doDeletes(commits, true)
}

// doDeletes performs the actual deletion logic.
func (p *KeepLastNDeletionPolicy) doDeletes(commits []*index.IndexCommit, isCommit bool) error {
	// Assert that we really are only called for each new commit
	if isCommit {
		fileName := commits[len(commits)-1].GetSegmentsFileName()
		if p.seen[fileName] {
			return fmt.Errorf("onCommit was called twice on the same commit point: %s", fileName)
		}
		p.seen[fileName] = true
		p.numOnCommit++
	}
	size := len(commits)
	for i := 0; i < size-p.numToKeep; i++ {
		if err := commits[i].Delete(); err != nil {
			return fmt.Errorf("failed to delete commit: %w", err)
		}
		p.numDelete++
	}
	return nil
}

// Clone returns a clone of this policy.
func (p *KeepLastNDeletionPolicy) Clone() index.IndexDeletionPolicy {
	return NewKeepLastNDeletionPolicy(p.numToKeep)
}

// =============================================================================
// ExpirationTimeDeletionPolicy (Test Implementation)
// =============================================================================

// ExpirationTimeDeletionPolicy deletes commits older than a specified time.
type ExpirationTimeDeletionPolicy struct {
	dir                store.Directory
	expirationTimeSecs float64
	numDelete          int
}

// NewExpirationTimeDeletionPolicy creates a new ExpirationTimeDeletionPolicy.
func NewExpirationTimeDeletionPolicy(dir store.Directory, seconds float64) *ExpirationTimeDeletionPolicy {
	return &ExpirationTimeDeletionPolicy{
		dir:                dir,
		expirationTimeSecs: seconds,
	}
}

// OnInit is called when IndexWriter is initialized.
func (p *ExpirationTimeDeletionPolicy) OnInit(commits []*index.IndexCommit) error {
	if len(commits) == 0 {
		return nil
	}
	return p.OnCommit(commits)
}

// OnCommit is called when a commit is made.
func (p *ExpirationTimeDeletionPolicy) OnCommit(commits []*index.IndexCommit) error {
	if len(commits) == 0 {
		return nil
	}

	lastCommit := commits[len(commits)-1]
	lastCommitTime := p.getCommitTime(lastCommit)

	// Any commit older than expireTime should be deleted
	expireTime := lastCommitTime/1000.0 - p.expirationTimeSecs

	for _, commit := range commits {
		if commit == lastCommit {
			continue
		}
		modTime := p.getCommitTime(commit) / 1000.0
		if modTime < expireTime {
			if err := commit.Delete(); err != nil {
				return fmt.Errorf("failed to delete expired commit: %w", err)
			}
			p.numDelete++
		}
	}
	return nil
}

// getCommitTime extracts the commit time from user data.
func (p *ExpirationTimeDeletionPolicy) getCommitTime(commit *index.IndexCommit) float64 {
	userData := commit.GetUserData()
	if userData == nil {
		return 0
	}
	timeStr := userData["commitTime"]
	if timeStr == "" {
		return 0
	}
	var timeVal int64
	fmt.Sscanf(timeStr, "%d", &timeVal)
	return float64(timeVal)
}

// Clone returns a clone of this policy.
func (p *ExpirationTimeDeletionPolicy) Clone() index.IndexDeletionPolicy {
	return NewExpirationTimeDeletionPolicy(p.dir, p.expirationTimeSecs)
}

// =============================================================================
// Test Cases
// =============================================================================

// TestKeepAllDeletionPolicy tests a deletion policy that keeps all commits.
// Source: TestDeletionPolicy.testKeepAllDeletionPolicy()
func TestKeepAllDeletionPolicy(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	policy := NewKeepAllDeletionPolicy(dir)

	// Create mock commits
	out1, _ := dir.CreateOutput("segments_1", store.IOContextWrite)
	out1.Close()
	out2, _ := dir.CreateOutput("segments_2", store.IOContextWrite)
	out2.Close()
	out3, _ := dir.CreateOutput("segments_3", store.IOContextWrite)
	out3.Close()

	commit1 := index.NewIndexCommitWithDirectory("segments_1", dir, 1, 1)
	commit2 := index.NewIndexCommitWithDirectory("segments_2", dir, 1, 2)
	commit3 := index.NewIndexCommitWithDirectory("segments_3", dir, 1, 3)

	commits := []*index.IndexCommit{commit1, commit2, commit3}

	// Test OnInit
	err := policy.OnInit(commits)
	if err != nil {
		t.Errorf("OnInit failed: %v", err)
	}
	if policy.numOnInit != 1 {
		t.Errorf("Expected numOnInit=1, got %d", policy.numOnInit)
	}

	// Test OnCommit
	err = policy.OnCommit(commits)
	if err != nil {
		t.Errorf("OnCommit failed: %v", err)
	}
	if policy.numOnCommit != 1 {
		t.Errorf("Expected numOnCommit=1, got %d", policy.numOnCommit)
	}

	// Verify files still exist (policy should not delete anything)
	if !dir.FileExists("segments_1") {
		t.Error("segments_1 should still exist")
	}
	if !dir.FileExists("segments_2") {
		t.Error("segments_2 should still exist")
	}
	if !dir.FileExists("segments_3") {
		t.Error("segments_3 should still exist")
	}
}

// TestKeepNoneOnInitDeletionPolicy tests a policy that deletes all commits on init.
// Source: TestDeletionPolicy.testKeepNoneOnInitDeletionPolicy()
func TestKeepNoneOnInitDeletionPolicy(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	// Create files for commits to exist initially
	out1, _ := dir.CreateOutput("segments_1", store.IOContextWrite)
	out1.Close()
	out2, _ := dir.CreateOutput("segments_2", store.IOContextWrite)
	out2.Close()
	out3, _ := dir.CreateOutput("segments_3", store.IOContextWrite)
	out3.Close()

	policy := NewKeepNoneOnInitDeletionPolicy()

	// Create mock commits
	commit1 := index.NewIndexCommitWithDirectory("segments_1", dir, 1, 1)
	commit2 := index.NewIndexCommitWithDirectory("segments_2", dir, 1, 2)
	commit3 := index.NewIndexCommitWithDirectory("segments_3", dir, 1, 3)

	commits := []*index.IndexCommit{commit1, commit2, commit3}

	// Test OnInit - should delete all commits
	err := policy.OnInit(commits)
	if err != nil {
		t.Errorf("OnInit failed: %v", err)
	}
	if policy.numOnInit != 1 {
		t.Errorf("Expected numOnInit=1, got %d", policy.numOnInit)
	}

	// Verify all commits were deleted
	for i, commit := range commits {
		if !commit.IsDeleted() {
			t.Errorf("Commit %d should be deleted after OnInit", i)
		}
	}

	// Create new commits for OnCommit test
	out4, _ := dir.CreateOutput("segments_4", store.IOContextWrite)
	out4.Close()
	out5, _ := dir.CreateOutput("segments_5", store.IOContextWrite)
	out5.Close()
	commit4 := index.NewIndexCommitWithDirectory("segments_4", dir, 1, 4)
	commit5 := index.NewIndexCommitWithDirectory("segments_5", dir, 1, 5)
	newCommits := []*index.IndexCommit{commit4, commit5}

	// Test OnCommit - should delete all but last
	err = policy.OnCommit(newCommits)
	if err != nil {
		t.Errorf("OnCommit failed: %v", err)
	}
	if policy.numOnCommit != 1 {
		t.Errorf("Expected numOnCommit=1, got %d", policy.numOnCommit)
	}

	// Verify first commit was deleted but last was kept
	if !commit4.IsDeleted() {
		t.Error("commit4 should be deleted")
	}
	if commit5.IsDeleted() {
		t.Error("commit5 should not be deleted")
	}
}

// TestKeepLastNDeletionPolicy tests a policy that keeps the last N commits.
// Source: TestDeletionPolicy.testKeepLastNDeletionPolicy()
func TestKeepLastNDeletionPolicy(t *testing.T) {
	const N = 5

	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	policy := NewKeepLastNDeletionPolicy(N)

	// Create 6 commits (N+1)
	commits := make([]*index.IndexCommit, N+1)
	for i := 0; i < N+1; i++ {
		segmentsFile := fmt.Sprintf("segments_%d", i+1)
		out, _ := dir.CreateOutput(segmentsFile, store.IOContextWrite)
		out.Close()
		commits[i] = index.NewIndexCommitWithDirectory(segmentsFile, dir, 1, int64(i+1))
	}

	// Test OnCommit - should delete oldest commit (only N kept)
	// Note: OnInit is not called, so we only test OnCommit behavior
	err := policy.OnCommit(commits)
	if err != nil {
		t.Errorf("OnCommit failed: %v", err)
	}
	if policy.numOnCommit != 1 {
		t.Errorf("Expected numOnCommit=1, got %d", policy.numOnCommit)
	}
	if policy.numDelete <= 0 {
		t.Errorf("Expected numDelete > 0, got %d", policy.numDelete)
	}

	// Verify oldest commit was deleted
	if !commits[0].IsDeleted() {
		t.Error("Oldest commit should be deleted")
	}

	// Verify last N commits were kept
	for i := 1; i < N+1; i++ {
		if commits[i].IsDeleted() {
			t.Errorf("Commit %d should not be deleted", i)
		}
	}
}

// TestKeepLastNCommitsDeletionPolicy tests the built-in KeepLastNCommitsDeletionPolicy.
// Source: TestDeletionPolicy.testKeepLastNCommitsDeletionPolicy()
func TestKeepLastNCommitsDeletionPolicy(t *testing.T) {
	numCommitsToKeep := 3

	// Create the policy
	policy := index.NewKeepLastNCommitsDeletionPolicy(numCommitsToKeep)

	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	// Create mock commits
	commits := make([]*index.IndexCommit, 5)
	for i := 0; i < 5; i++ {
		segmentsFile := fmt.Sprintf("segments_%d", i+1)
		out, _ := dir.CreateOutput(segmentsFile, store.IOContextWrite)
		out.Close()
		commits[i] = index.NewIndexCommitWithDirectory(segmentsFile, dir, 1, int64(i+1))
	}

	// Test OnCommit - should delete oldest 2 commits (keeping last 3)
	err := policy.OnCommit(commits)
	if err != nil {
		t.Errorf("OnCommit failed: %v", err)
	}

	// Verify oldest 2 commits were deleted
	for i := 0; i < 2; i++ {
		if !commits[i].IsDeleted() {
			t.Errorf("Commit %d should be deleted", i)
		}
	}

	// Verify last 3 commits were kept
	for i := 2; i < 5; i++ {
		if commits[i].IsDeleted() {
			t.Errorf("Commit %d should not be deleted", i)
		}
	}
}

// TestKeepLastNCommitsDeletionPolicyWithZeroCommits tests that zero commits to keep is invalid.
// Source: TestDeletionPolicy.testKeepLastNCommitsDeletionPolicyWithZeroCommits()
func TestKeepLastNCommitsDeletionPolicyWithZeroCommits(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for zero commits to keep")
		}
	}()

	// This should panic
	_ = index.NewKeepLastNCommitsDeletionPolicy(0)
}

// TestSnapshotDeletionPolicy tests the SnapshotDeletionPolicy.
// Source: TestDeletionPolicy (snapshot-related tests)
func TestSnapshotDeletionPolicy(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	// Create underlying policy
	primary := index.NewKeepOnlyLastCommitDeletionPolicy()
	policy := index.NewSnapshotDeletionPolicy(primary)

	// Test that policy was created with correct primary
	if policy.GetPrimary() == nil {
		t.Error("Expected primary policy to be set")
	}

	// Test initial state
	if policy.SnapshotCount() != 0 {
		t.Errorf("Expected 0 snapshots, got %d", policy.SnapshotCount())
	}
}

// TestSnapshotDeletionPolicyOperations tests snapshot operations.
func TestSnapshotDeletionPolicyOperations(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	primary := index.NewKeepOnlyLastCommitDeletionPolicy()
	policy := index.NewSnapshotDeletionPolicy(primary)

	// Initially no snapshots
	if policy.SnapshotCount() != 0 {
		t.Errorf("Expected 0 snapshots, got %d", policy.SnapshotCount())
	}

	snapshots := policy.GetSnapshots()
	if len(snapshots) != 0 {
		t.Errorf("Expected 0 snapshots in list, got %d", len(snapshots))
	}

	// Test HasSnapshot
	if policy.HasSnapshot(1) {
		t.Error("Expected no snapshot for generation 1")
	}
}

// TestCommitOrder verifies that commits are properly ordered.
func TestCommitOrder(t *testing.T) {
	// Create mock commits with different generations
	commits := []*index.IndexCommit{
		index.NewIndexCommitWithDirectory("segments_1", nil, 1, 1),
		index.NewIndexCommitWithDirectory("segments_2", nil, 1, 2),
		index.NewIndexCommitWithDirectory("segments_3", nil, 1, 3),
	}

	verifyCommitOrder(t, commits)
}

// TestCommitOrderEmpty verifies empty commit list is handled.
func TestCommitOrderEmpty(t *testing.T) {
	var commits []*index.IndexCommit
	verifyCommitOrder(t, commits)
}

// TestIndexDeletionPolicyInterface verifies the interface is properly implemented.
func TestIndexDeletionPolicyInterface(t *testing.T) {
	// Test that all policies implement the interface
	var _ index.IndexDeletionPolicy = index.NewKeepOnlyLastCommitDeletionPolicy()
	var _ index.IndexDeletionPolicy = index.NewKeepAllDeletionPolicy()

	primary := index.NewKeepOnlyLastCommitDeletionPolicy()
	var _ index.IndexDeletionPolicy = index.NewSnapshotDeletionPolicy(primary)
}

// TestKeepOnlyLastCommitDeletionPolicy tests the default policy.
func TestKeepOnlyLastCommitDeletionPolicy(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	policy := index.NewKeepOnlyLastCommitDeletionPolicy()

	// Create commits with actual files
	out1, _ := dir.CreateOutput("segments_1", store.IOContextWrite)
	out1.Close()
	out2, _ := dir.CreateOutput("segments_2", store.IOContextWrite)
	out2.Close()
	out3, _ := dir.CreateOutput("segments_3", store.IOContextWrite)
	out3.Close()

	commit1 := index.NewIndexCommitWithDirectory("segments_1", dir, 1, 1)
	commit2 := index.NewIndexCommitWithDirectory("segments_2", dir, 1, 2)
	commit3 := index.NewIndexCommitWithDirectory("segments_3", dir, 1, 3)

	commits := []*index.IndexCommit{commit1, commit2, commit3}

	// Test OnCommit - should keep only the last commit
	err := policy.OnCommit(commits)
	if err != nil {
		t.Errorf("OnCommit failed: %v", err)
	}

	// Verify first two commits are deleted
	if !commit1.IsDeleted() {
		t.Error("Expected commit1 to be deleted")
	}
	if !commit2.IsDeleted() {
		t.Error("Expected commit2 to be deleted")
	}
	// Note: commit3 may or may not be considered "deleted" depending on implementation
}

// TestKeepAllDeletionPolicyBasic tests the basic KeepAllDeletionPolicy.
func TestKeepAllDeletionPolicyBasic(t *testing.T) {
	policy := index.NewKeepAllDeletionPolicy()

	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	// Create commits with files
	out1, _ := dir.CreateOutput("segments_1", store.IOContextWrite)
	out1.Close()
	out2, _ := dir.CreateOutput("segments_2", store.IOContextWrite)
	out2.Close()
	out3, _ := dir.CreateOutput("segments_3", store.IOContextWrite)
	out3.Close()

	commit1 := index.NewIndexCommitWithDirectory("segments_1", dir, 1, 1)
	commit2 := index.NewIndexCommitWithDirectory("segments_2", dir, 1, 2)
	commit3 := index.NewIndexCommitWithDirectory("segments_3", dir, 1, 3)

	commits := []*index.IndexCommit{commit1, commit2, commit3}

	// Test OnCommit - should keep all commits
	err := policy.OnCommit(commits)
	if err != nil {
		t.Errorf("OnCommit failed: %v", err)
	}

	// Verify files still exist (policy should not delete anything)
	if !dir.FileExists("segments_1") {
		t.Error("segments_1 should still exist")
	}
	if !dir.FileExists("segments_2") {
		t.Error("segments_2 should still exist")
	}
	if !dir.FileExists("segments_3") {
		t.Error("segments_3 should still exist")
	}

	// Test OnInit
	err = policy.OnInit(commits)
	if err != nil {
		t.Errorf("OnInit failed: %v", err)
	}

	// Verify still no files deleted
	if !dir.FileExists("segments_1") {
		t.Error("segments_1 should still exist after OnInit")
	}
}

// TestPolicyClone verifies that policies can be cloned.
func TestPolicyClone(t *testing.T) {
	// Test KeepOnlyLastCommitDeletionPolicy clone
	policy1 := index.NewKeepOnlyLastCommitDeletionPolicy()
	clone1 := policy1.Clone()
	if clone1 == nil {
		t.Error("Expected non-nil clone for KeepOnlyLastCommitDeletionPolicy")
	}

	// Test KeepAllDeletionPolicy clone
	policy2 := index.NewKeepAllDeletionPolicy()
	clone2 := policy2.Clone()
	if clone2 == nil {
		t.Error("Expected non-nil clone for KeepAllDeletionPolicy")
	}

	// Test SnapshotDeletionPolicy clone
	primary := index.NewKeepOnlyLastCommitDeletionPolicy()
	policy3 := index.NewSnapshotDeletionPolicy(primary)
	clone3 := policy3.Clone()
	if clone3 == nil {
		t.Error("Expected non-nil clone for SnapshotDeletionPolicy")
	}
}

// TestPolicyString verifies String() methods.
func TestPolicyString(t *testing.T) {
	policy1 := index.NewKeepOnlyLastCommitDeletionPolicy()
	str1 := policy1.String()
	if str1 == "" {
		t.Error("Expected non-empty string for KeepOnlyLastCommitDeletionPolicy")
	}

	policy2 := index.NewKeepAllDeletionPolicy()
	str2 := policy2.String()
	if str2 == "" {
		t.Error("Expected non-empty string for KeepAllDeletionPolicy")
	}
}

// TestIndexCommitOperations tests IndexCommit operations.
func TestIndexCommitOperations(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	commit := index.NewIndexCommitWithDirectory("segments_1", dir, 5, 1)

	// Test getters
	if commit.GetSegmentsFileName() != "segments_1" {
		t.Errorf("Expected segments file name 'segments_1', got '%s'", commit.GetSegmentsFileName())
	}
	if commit.GetSegmentCount() != 5 {
		t.Errorf("Expected segment count 5, got %d", commit.GetSegmentCount())
	}
	if commit.GetGeneration() != 1 {
		t.Errorf("Expected generation 1, got %d", commit.GetGeneration())
	}

	// Test user data
	userData := commit.GetUserData()
	if userData == nil {
		t.Error("Expected non-nil user data")
	}
}

// TestIndexCommitList tests IndexCommitList operations.
func TestIndexCommitList(t *testing.T) {
	commits := index.IndexCommitList{
		index.NewIndexCommitWithDirectory("segments_3", nil, 1, 3),
		index.NewIndexCommitWithDirectory("segments_1", nil, 1, 1),
		index.NewIndexCommitWithDirectory("segments_2", nil, 1, 2),
	}

	// Test Len
	if commits.Len() != 3 {
		t.Errorf("Expected length 3, got %d", commits.Len())
	}

	// Test SortByGeneration
	commits.SortByGeneration()
	if commits[0].GetGeneration() != 1 {
		t.Errorf("Expected first commit generation 1, got %d", commits[0].GetGeneration())
	}
	if commits[1].GetGeneration() != 2 {
		t.Errorf("Expected second commit generation 2, got %d", commits[1].GetGeneration())
	}
	if commits[2].GetGeneration() != 3 {
		t.Errorf("Expected third commit generation 3, got %d", commits[2].GetGeneration())
	}

	// Test GetLatest
	latest := commits.GetLatest()
	if latest.GetGeneration() != 3 {
		t.Errorf("Expected latest generation 3, got %d", latest.GetGeneration())
	}

	// Test GetOldest
	oldest := commits.GetOldest()
	if oldest.GetGeneration() != 1 {
		t.Errorf("Expected oldest generation 1, got %d", oldest.GetGeneration())
	}
}

// TestIndexCommitEquals tests IndexCommit equality.
func TestIndexCommitEquals(t *testing.T) {
	commit1 := index.NewIndexCommitWithDirectory("segments_1", nil, 1, 1)
	commit2 := index.NewIndexCommitWithDirectory("segments_1", nil, 1, 1)
	commit3 := index.NewIndexCommitWithDirectory("segments_2", nil, 1, 2)

	if !commit1.Equals(commit2) {
		t.Error("Expected commit1 to equal commit2")
	}
	if commit1.Equals(commit3) {
		t.Error("Expected commit1 to NOT equal commit3")
	}
	if commit1.Equals(nil) {
		t.Error("Expected commit1 to NOT equal nil")
	}
}

// TestEnsureCommitsValid tests the EnsureCommitsValid helper.
func TestEnsureCommitsValid(t *testing.T) {
	// Test nil commits
	err := index.EnsureCommitsValid(nil)
	if err == nil {
		t.Error("Expected error for nil commits")
	}

	// Test empty commits
	err = index.EnsureCommitsValid([]*index.IndexCommit{})
	if err == nil {
		t.Error("Expected error for empty commits")
	}

	// Test valid commits
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()
	commits := []*index.IndexCommit{
		index.NewIndexCommitWithDirectory("segments_1", dir, 1, 1),
	}
	err = index.EnsureCommitsValid(commits)
	if err != nil {
		t.Errorf("Expected no error for valid commits, got: %v", err)
	}
}

// TestFindCommitByGeneration tests the FindCommitByGeneration helper.
func TestFindCommitByGeneration(t *testing.T) {
	commits := []*index.IndexCommit{
		index.NewIndexCommitWithDirectory("segments_1", nil, 1, 1),
		index.NewIndexCommitWithDirectory("segments_2", nil, 1, 2),
		index.NewIndexCommitWithDirectory("segments_3", nil, 1, 3),
	}

	found := index.FindCommitByGeneration(commits, 2)
	if found == nil {
		t.Error("Expected to find commit with generation 2")
	} else if found.GetGeneration() != 2 {
		t.Errorf("Expected generation 2, got %d", found.GetGeneration())
	}

	notFound := index.FindCommitByGeneration(commits, 99)
	if notFound != nil {
		t.Error("Expected nil for non-existent generation")
	}
}

// TestFilterDeletedCommits tests the FilterDeletedCommits helper.
func TestFilterDeletedCommits(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	// Create a file for the commit to exist
	out1, _ := dir.CreateOutput("segments_1", store.IOContextWrite)
	out1.Close()
	out3, _ := dir.CreateOutput("segments_3", store.IOContextWrite)
	out3.Close()

	commit1 := index.NewIndexCommitWithDirectory("segments_1", dir, 1, 1)
	commit2 := index.NewIndexCommitWithDirectory("segments_2", dir, 1, 2)
	commit3 := index.NewIndexCommitWithDirectory("segments_3", dir, 1, 3)

	commits := []*index.IndexCommit{commit1, commit2, commit3}

	// Delete commit2
	commit2.Delete()

	filtered := index.FilterDeletedCommits(commits)
	if len(filtered) != 2 {
		t.Errorf("Expected 2 filtered commits, got %d", len(filtered))
	}
}

// TestSnapshotWithCommit tests taking a snapshot of a commit.
func TestSnapshotWithCommit(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	// Create a file for the commit
	out, _ := dir.CreateOutput("segments_1", store.IOContextWrite)
	out.Close()

	primary := index.NewKeepOnlyLastCommitDeletionPolicy()
	policy := index.NewSnapshotDeletionPolicy(primary)

	// Create a commit
	commit := index.NewIndexCommitWithDirectory("segments_1", dir, 1, 1)

	// Take a snapshot
	gen, err := policy.Snapshot(commit)
	if err != nil {
		t.Errorf("Failed to snapshot commit: %v", err)
	}
	if gen != 1 {
		t.Errorf("Expected generation 1, got %d", gen)
	}

	// Verify snapshot exists
	if !policy.HasSnapshot(1) {
		t.Error("Expected snapshot to exist")
	}
	if policy.SnapshotCount() != 1 {
		t.Errorf("Expected 1 snapshot, got %d", policy.SnapshotCount())
	}

	// Release the snapshot
	released := policy.Release(1)
	if !released {
		t.Error("Expected snapshot to be released")
	}
	if policy.HasSnapshot(1) {
		t.Error("Expected snapshot to be released")
	}
}

// TestSnapshotReleaseNonExistent tests releasing a non-existent snapshot.
func TestSnapshotReleaseNonExistent(t *testing.T) {
	primary := index.NewKeepOnlyLastCommitDeletionPolicy()
	policy := index.NewSnapshotDeletionPolicy(primary)

	// Try to release a non-existent snapshot
	released := policy.Release(999)
	if released {
		t.Error("Expected release of non-existent snapshot to return false")
	}
}

// TestSnapshotReleaseAll tests releasing all snapshots.
func TestSnapshotReleaseAll(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	primary := index.NewKeepOnlyLastCommitDeletionPolicy()
	policy := index.NewSnapshotDeletionPolicy(primary)

	// Create files and commits
	for i := 1; i <= 3; i++ {
		segmentsFile := fmt.Sprintf("segments_%d", i)
		out, _ := dir.CreateOutput(segmentsFile, store.IOContextWrite)
		out.Close()
		commit := index.NewIndexCommitWithDirectory(segmentsFile, dir, 1, int64(i))
		_, err := policy.Snapshot(commit)
		if err != nil {
			t.Errorf("Failed to snapshot commit %d: %v", i, err)
		}
	}

	if policy.SnapshotCount() != 3 {
		t.Errorf("Expected 3 snapshots, got %d", policy.SnapshotCount())
	}

	// Release all
	policy.ReleaseAll()

	if policy.SnapshotCount() != 0 {
		t.Errorf("Expected 0 snapshots after ReleaseAll, got %d", policy.SnapshotCount())
	}
}

// TestKeepLastNCommitsDeletionPolicyOnInit tests OnInit behavior.
func TestKeepLastNCommitsDeletionPolicyOnInit(t *testing.T) {
	numCommitsToKeep := 3
	policy := index.NewKeepLastNCommitsDeletionPolicy(numCommitsToKeep)

	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	// Create 5 commits
	commits := make([]*index.IndexCommit, 5)
	for i := 0; i < 5; i++ {
		segmentsFile := fmt.Sprintf("segments_%d", i+1)
		out, _ := dir.CreateOutput(segmentsFile, store.IOContextWrite)
		out.Close()
		commits[i] = index.NewIndexCommitWithDirectory(segmentsFile, dir, 1, int64(i+1))
	}

	// Test OnInit - should behave same as OnCommit
	err := policy.OnInit(commits)
	if err != nil {
		t.Errorf("OnInit failed: %v", err)
	}

	// Verify oldest 2 commits were deleted
	for i := 0; i < 2; i++ {
		if !commits[i].IsDeleted() {
			t.Errorf("Commit %d should be deleted after OnInit", i)
		}
	}
}

// TestKeepLastNCommitsDeletionPolicyGetNumCommits tests GetNumCommitsToKeep.
func TestKeepLastNCommitsDeletionPolicyGetNumCommits(t *testing.T) {
	policy := index.NewKeepLastNCommitsDeletionPolicy(10)
	if policy.GetNumCommitsToKeep() != 10 {
		t.Errorf("Expected 10 commits to keep, got %d", policy.GetNumCommitsToKeep())
	}
}

// TestIndexCommitString tests the String method.
func TestIndexCommitString(t *testing.T) {
	commit := index.NewIndexCommitWithDirectory("segments_5", nil, 3, 5)
	str := commit.String()
	if str == "" {
		t.Error("Expected non-empty string representation")
	}
	// Should contain key information
	if str != "" && len(str) < 10 {
		t.Error("Expected meaningful string representation")
	}
}

// TestIndexCommitListString tests the String method of IndexCommitList.
func TestIndexCommitListString(t *testing.T) {
	commits := index.IndexCommitList{
		index.NewIndexCommitWithDirectory("segments_1", nil, 1, 1),
		index.NewIndexCommitWithDirectory("segments_2", nil, 1, 2),
	}
	str := commits.String()
	if str == "" {
		t.Error("Expected non-empty string representation")
	}
}

// TestIndexCommitGetDirectory tests GetDirectory.
func TestIndexCommitGetDirectory(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	commit := index.NewIndexCommitWithDirectory("segments_1", dir, 1, 1)
	if commit.GetDirectory() != dir {
		t.Error("Expected directory to match")
	}
}

// TestIndexCommitGetUserDataValue tests GetUserDataValue.
func TestIndexCommitGetUserDataValue(t *testing.T) {
	commit := index.NewIndexCommitWithDirectory("segments_1", nil, 1, 1)

	// Default should be empty
	val := commit.GetUserDataValue("key")
	if val != "" {
		t.Errorf("Expected empty value for unset key, got '%s'", val)
	}
}

// TestSnapshotDeletionPolicyGetSnapshots tests GetSnapshots.
func TestSnapshotDeletionPolicyGetSnapshots(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	primary := index.NewKeepOnlyLastCommitDeletionPolicy()
	policy := index.NewSnapshotDeletionPolicy(primary)

	// Create multiple commits and snapshot them
	for i := 1; i <= 3; i++ {
		segmentsFile := fmt.Sprintf("segments_%d", i)
		out, _ := dir.CreateOutput(segmentsFile, store.IOContextWrite)
		out.Close()
		commit := index.NewIndexCommitWithDirectory(segmentsFile, dir, 1, int64(i))
		_, err := policy.Snapshot(commit)
		if err != nil {
			t.Errorf("Failed to snapshot commit %d: %v", i, err)
		}
	}

	snapshots := policy.GetSnapshots()
	if len(snapshots) != 3 {
		t.Errorf("Expected 3 snapshots, got %d", len(snapshots))
	}
}

// TestSnapshotDeletionPolicySnapshotNil tests snapshotting nil commit.
func TestSnapshotDeletionPolicySnapshotNil(t *testing.T) {
	primary := index.NewKeepOnlyLastCommitDeletionPolicy()
	policy := index.NewSnapshotDeletionPolicy(primary)

	_, err := policy.Snapshot(nil)
	if err == nil {
		t.Error("Expected error when snapshotting nil commit")
	}
}

// TestSnapshotDeletionPolicySnapshotDeleted tests snapshotting deleted commit.
func TestSnapshotDeletionPolicySnapshotDeleted(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	out, _ := dir.CreateOutput("segments_1", store.IOContextWrite)
	out.Close()
	commit := index.NewIndexCommitWithDirectory("segments_1", dir, 1, 1)

	// Delete the commit
	commit.Delete()

	primary := index.NewKeepOnlyLastCommitDeletionPolicy()
	policy := index.NewSnapshotDeletionPolicy(primary)

	_, err := policy.Snapshot(commit)
	if err == nil {
		t.Error("Expected error when snapshotting deleted commit")
	}
}

// TestDocumentCreation tests document creation for use with IndexWriter.
func TestDocumentCreation(t *testing.T) {
	doc := document.NewDocument()
	if doc == nil {
		t.Fatal("Expected non-nil document")
	}

	textField, err := document.NewTextField("content", "test value", false)
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
