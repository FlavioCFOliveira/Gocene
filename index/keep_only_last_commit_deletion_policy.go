// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"sync"
)

// =============================================================================
// KeepOnlyLastCommitDeletionPolicy
// =============================================================================

// KeepOnlyLastCommitDeletionPolicy is the default deletion policy which
// keeps only the most recent commit and deletes all prior commits.
//
// This is the Go port of Lucene's org.apache.lucene.index.KeepOnlyLastCommitDeletionPolicy.
//
// This policy is appropriate for most use cases where only the current state
// of the index is needed. It provides:
//   - Minimal disk usage (only one commit kept)
//   - No need for manual commit management
//   - Automatic cleanup of old commits
//
// If you need to keep multiple commits (e.g., for backup, point-in-time recovery,
// or near-real-time search), use a different policy like:
//   - SnapshotDeletionPolicy: Keep commits with active snapshots
//   - KeepAllDeletionPolicy: Keep all commits
//   - Custom policy based on your needs
//
// Example:
//
//	policy := index.NewKeepOnlyLastCommitDeletionPolicy()
//	writer, err := index.NewIndexWriter(dir, config, policy)
//	if err != nil {
//	    // handle error
//	}
//	defer writer.Close()
type KeepOnlyLastCommitDeletionPolicy struct {
	*BaseIndexDeletionPolicy
}

// NewKeepOnlyLastCommitDeletionPolicy creates a new KeepOnlyLastCommitDeletionPolicy.
func NewKeepOnlyLastCommitDeletionPolicy() *KeepOnlyLastCommitDeletionPolicy {
	return &KeepOnlyLastCommitDeletionPolicy{
		BaseIndexDeletionPolicy: &BaseIndexDeletionPolicy{},
	}
}

// OnCommit is called each time a commit is made.
// It deletes all commits except the most recent one.
//
// Parameters:
//   - commits: A list of all current commits, sorted by age (oldest first)
//
// Returns an error if any deletion fails.
func (p *KeepOnlyLastCommitDeletionPolicy) OnCommit(commits []*IndexCommit) error {
	if len(commits) <= 1 {
		// Nothing to delete if we have 0 or 1 commits
		return nil
	}

	// Delete all commits except the last one (most recent)
	// Commits are sorted by age (oldest first), so we keep commits[len(commits)-1]
	for i := 0; i < len(commits)-1; i++ {
		if err := commits[i].Delete(); err != nil {
			return fmt.Errorf("failed to delete commit %d: %w", i, err)
		}
	}

	return nil
}

// OnInit is called when IndexWriter is being initialized.
// It cleans up old commits after a crash by keeping only the most recent one.
//
// Parameters:
//   - commits: A list of all current commits, sorted by age (oldest first)
//
// Returns an error if any deletion fails.
func (p *KeepOnlyLastCommitDeletionPolicy) OnInit(commits []*IndexCommit) error {
	// Same behavior as OnCommit - keep only the most recent commit
	return p.OnCommit(commits)
}

// Clone returns a clone of this policy.
func (p *KeepOnlyLastCommitDeletionPolicy) Clone() IndexDeletionPolicy {
	return NewKeepOnlyLastCommitDeletionPolicy()
}

// String returns a string representation of this policy.
func (p *KeepOnlyLastCommitDeletionPolicy) String() string {
	return "KeepOnlyLastCommitDeletionPolicy"
}

// =============================================================================
// KeepAllDeletionPolicy
// =============================================================================

// KeepAllDeletionPolicy keeps all commits and never deletes anything.
// This is useful for backup scenarios or when you want to manually manage commits.
//
// This is the Go port of Lucene's org.apache.lucene.index.KeepAllDeletionPolicy.
//
// This policy is useful for:
//   - Backup scenarios where you want to preserve all index states
//   - Scenarios where external processes need access to historical commits
//   - Testing and debugging purposes
//
// WARNING: This can consume significant disk space over time.
// Make sure to manually delete old commits if using this policy.
type KeepAllDeletionPolicy struct {
	*BaseIndexDeletionPolicy
}

// NewKeepAllDeletionPolicy creates a new KeepAllDeletionPolicy.
func NewKeepAllDeletionPolicy() *KeepAllDeletionPolicy {
	return &KeepAllDeletionPolicy{
		BaseIndexDeletionPolicy: &BaseIndexDeletionPolicy{},
	}
}

// OnCommit is called each time a commit is made.
// Does nothing - keeps all commits.
//
// Parameters:
//   - commits: A list of all current commits, sorted by age (oldest first)
//
// Always returns nil (no errors).
func (p *KeepAllDeletionPolicy) OnCommit(commits []*IndexCommit) error {
	// Keep all commits - do nothing
	return nil
}

// OnInit is called when IndexWriter is being initialized.
// Does nothing - keeps all commits.
//
// Parameters:
//   - commits: A list of all current commits, sorted by age (oldest first)
//
// Always returns nil (no errors).
func (p *KeepAllDeletionPolicy) OnInit(commits []*IndexCommit) error {
	// Keep all commits - do nothing
	return nil
}

// Clone returns a clone of this policy.
func (p *KeepAllDeletionPolicy) Clone() IndexDeletionPolicy {
	return NewKeepAllDeletionPolicy()
}

// String returns a string representation of this policy.
func (p *KeepAllDeletionPolicy) String() string {
	return "KeepAllDeletionPolicy"
}

// =============================================================================
// SnapshotDeletionPolicy
// =============================================================================

// SnapshotDeletionPolicy keeps commits that have active snapshots.
// This is useful when you need to keep certain commits for point-in-time
// search or backup purposes.
//
// This is the Go port of Lucene's org.apache.lucene.index.SnapshotDeletionPolicy.
//
// A snapshot can be taken to "pin" a commit, preventing it from being deleted.
// When the snapshot is released, the commit becomes eligible for deletion.
//
// This policy is useful for:
//   - Creating point-in-time snapshots of the index
//   - Allowing external processes to read from old index commits
//   - Backup operations that need a stable view of the index
//
// Thread Safety:
//   - All snapshot operations are protected by a mutex
//   - Safe for concurrent use from multiple goroutines
type SnapshotDeletionPolicy struct {
	*BaseIndexDeletionPolicy

	// primary is the underlying deletion policy
	primary IndexDeletionPolicy

	// snapshots tracks active snapshots by their generation number
	// The value is the segments file name for reference
	snapshots map[int64]string

	// mu protects concurrent access to snapshots
	mu sync.RWMutex
}

// NewSnapshotDeletionPolicy creates a new SnapshotDeletionPolicy wrapping
// the given primary policy.
//
// Parameters:
//   - primary: The underlying deletion policy to wrap (defaults to KeepOnlyLastCommitDeletionPolicy if nil)
//
// Example:
//
//	// Use with default underlying policy
//	policy := index.NewSnapshotDeletionPolicy(nil)
//
//	// Use with custom underlying policy
//	keepAll := index.NewKeepAllDeletionPolicy()
//	policy := index.NewSnapshotDeletionPolicy(keepAll)
func NewSnapshotDeletionPolicy(primary IndexDeletionPolicy) *SnapshotDeletionPolicy {
	if primary == nil {
		primary = NewKeepOnlyLastCommitDeletionPolicy()
	}

	return &SnapshotDeletionPolicy{
		BaseIndexDeletionPolicy: &BaseIndexDeletionPolicy{},
		primary:                 primary,
		snapshots:               make(map[int64]string),
	}
}

// OnCommit is called each time a commit is made.
// It protects any snapshotted commits from deletion.
//
// Parameters:
//   - commits: A list of all current commits, sorted by age (oldest first)
//
// Returns an error if any deletion fails.
func (p *SnapshotDeletionPolicy) OnCommit(commits []*IndexCommit) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.onCommitLocked(commits)
}

// onCommitLocked implements OnCommit assuming the lock is already held.
func (p *SnapshotDeletionPolicy) onCommitLocked(commits []*IndexCommit) error {
	if len(commits) == 0 {
		return nil
	}

	// Build a set of generations that must be kept (snapshots + last commit)
	toKeep := make(map[int64]bool)

	// Add all snapshotted generations
	for gen := range p.snapshots {
		toKeep[gen] = true
	}

	// Always keep the last (most recent) commit
	lastCommit := commits[len(commits)-1]
	toKeep[lastCommit.GetGeneration()] = true

	// Find commits that can be deleted (not in toKeep)
	var toDelete []*IndexCommit
	for _, commit := range commits {
		if !toKeep[commit.GetGeneration()] {
			toDelete = append(toDelete, commit)
		}
	}

	// Delete the commits that can be removed
	for _, commit := range toDelete {
		if err := commit.Delete(); err != nil {
			return fmt.Errorf("failed to delete commit with generation %d: %w",
				commit.GetGeneration(), err)
		}
	}

	return nil
}

// OnInit is called when IndexWriter is being initialized.
// It protects any snapshotted commits and cleans up old commits.
//
// Parameters:
//   - commits: A list of all current commits, sorted by age (oldest first)
//
// Returns an error if any deletion fails.
func (p *SnapshotDeletionPolicy) OnInit(commits []*IndexCommit) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.onInitLocked(commits)
}

// onInitLocked implements OnInit assuming the lock is already held.
func (p *SnapshotDeletionPolicy) onInitLocked(commits []*IndexCommit) error {
	// Same behavior as OnCommit - protect snapshotted commits
	return p.onCommitLocked(commits)
}

// Clone returns a clone of this policy.
// Note: Snapshots are not transferred to the clone.
func (p *SnapshotDeletionPolicy) Clone() IndexDeletionPolicy {
	p.mu.RLock()
	primaryClone := p.primary.Clone()
	p.mu.RUnlock()

	return NewSnapshotDeletionPolicy(primaryClone)
}

// Snapshot creates a snapshot of the given commit and returns its generation.
// The commit will be protected from deletion until Release is called.
//
// Parameters:
//   - commit: The commit to snapshot (must not be nil)
//
// Returns the generation number of the snapshotted commit.
// Returns an error if the commit is nil or already deleted.
//
// Example:
//
//	gen, err := policy.Snapshot(commit)
//	if err != nil {
//	    // handle error
//	}
//	// ... use the snapshot ...
//	policy.Release(gen)
func (p *SnapshotDeletionPolicy) Snapshot(commit *IndexCommit) (int64, error) {
	if commit == nil {
		return 0, fmt.Errorf("cannot snapshot nil commit")
	}

	if commit.IsDeleted() {
		return 0, fmt.Errorf("cannot snapshot deleted commit")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	gen := commit.GetGeneration()
	segmentsFile := commit.GetSegmentsFileName()

	// Check if already snapshotted
	if _, exists := p.snapshots[gen]; exists {
		return gen, nil // Already snapshotted, just return the generation
	}

	p.snapshots[gen] = segmentsFile
	return gen, nil
}

// SnapshotGeneration creates a snapshot of a commit identified by generation.
// This is useful when you only have the generation number, not the full commit.
//
// Parameters:
//   - commits: List of available commits
//   - generation: The generation number to snapshot
//
// Returns an error if the generation is not found in the commits list.
func (p *SnapshotDeletionPolicy) SnapshotGeneration(commits []*IndexCommit, generation int64) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Find the commit with this generation
	var targetCommit *IndexCommit
	for _, commit := range commits {
		if commit.GetGeneration() == generation {
			targetCommit = commit
			break
		}
	}

	if targetCommit == nil {
		return fmt.Errorf("commit with generation %d not found", generation)
	}

	if targetCommit.IsDeleted() {
		return fmt.Errorf("cannot snapshot deleted commit")
	}

	p.snapshots[generation] = targetCommit.GetSegmentsFileName()
	return nil
}

// Release removes a snapshot, allowing the commit to be deleted by future
// OnCommit calls if the underlying policy decides to delete it.
//
// Parameters:
//   - generation: The generation number of the snapshot to release
//
// Returns true if a snapshot was released, false if no snapshot existed.
//
// Example:
//
//	gen, _ := policy.Snapshot(commit)
//	// ... use the snapshot ...
//	released := policy.Release(gen)
//	if !released {
//	    // snapshot was not found
//	}
func (p *SnapshotDeletionPolicy) Release(generation int64) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	_, exists := p.snapshots[generation]
	if exists {
		delete(p.snapshots, generation)
	}
	return exists
}

// ReleaseAll releases all snapshots, allowing all commits to be deleted
// by the underlying policy in future OnCommit calls.
func (p *SnapshotDeletionPolicy) ReleaseAll() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.snapshots = make(map[int64]string)
}

// GetSnapshots returns the generations of all active snapshots.
// The returned slice is sorted in ascending order.
func (p *SnapshotDeletionPolicy) GetSnapshots() []int64 {
	p.mu.RLock()
	defer p.mu.RUnlock()

	generations := make([]int64, 0, len(p.snapshots))
	for gen := range p.snapshots {
		generations = append(generations, gen)
	}

	// Sort for consistent ordering
	sortGenerations(generations)
	return generations
}

// HasSnapshot returns true if there is an active snapshot for the given generation.
func (p *SnapshotDeletionPolicy) HasSnapshot(generation int64) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	_, exists := p.snapshots[generation]
	return exists
}

// SnapshotCount returns the number of active snapshots.
func (p *SnapshotDeletionPolicy) SnapshotCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return len(p.snapshots)
}

// GetPrimary returns the underlying deletion policy.
func (p *SnapshotDeletionPolicy) GetPrimary() IndexDeletionPolicy {
	return p.primary
}

// String returns a string representation of this policy.
func (p *SnapshotDeletionPolicy) String() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return fmt.Sprintf("SnapshotDeletionPolicy(primary=%v, snapshotCount=%d)",
		p.primary, len(p.snapshots))
}

// =============================================================================
// Helper functions
// =============================================================================

// sortGenerations sorts a slice of generation numbers in ascending order.
// This is implemented as a simple insertion sort for small slices,
// which is typically sufficient for the number of snapshots we expect.
func sortGenerations(generations []int64) {
	// Simple insertion sort - good for small slices
	for i := 1; i < len(generations); i++ {
		for j := i; j > 0 && generations[j-1] > generations[j]; j-- {
			generations[j-1], generations[j] = generations[j], generations[j-1]
		}
	}
}