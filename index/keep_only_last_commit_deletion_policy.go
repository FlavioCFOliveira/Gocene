// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
)

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

	// minCommitCountToKeep ensures we keep at least this many commits
	minCommitCountToKeep int
}

// NewKeepOnlyLastCommitDeletionPolicy creates a new KeepOnlyLastCommitDeletionPolicy.
func NewKeepOnlyLastCommitDeletionPolicy() *KeepOnlyLastCommitDeletionPolicy {
	return &KeepOnlyLastCommitDeletionPolicy{
		BaseIndexDeletionPolicy: &BaseIndexDeletionPolicy{},
		minCommitCountToKeep:    1,
	}
}

// OnCommit is called each time a commit is made.
// Deletes all but the most recent commit.
func (p *KeepOnlyLastCommitDeletionPolicy) OnCommit(commits []*IndexCommit) error {
	return p.cleanupOldCommits(commits)
}

// OnInit is called when IndexWriter is being initialized.
// Deletes all but the most recent commit.
func (p *KeepOnlyLastCommitDeletionPolicy) OnInit(commits []*IndexCommit) error {
	return p.cleanupOldCommits(commits)
}

// cleanupOldCommits deletes all but the most recent commit(s).
func (p *KeepOnlyLastCommitDeletionPolicy) cleanupOldCommits(commits []*IndexCommit) error {
	// Validate commits
	if err := EnsureCommitsValid(commits); err != nil {
		return fmt.Errorf("invalid commits: %w", err)
	}

	// Need at least one commit
	if len(commits) <= p.minCommitCountToKeep {
		return nil
	}

	// Sort by generation to ensure we keep the newest
	IndexCommitList(commits).SortByGenerationDesc()

	// Delete all but the last commit(s)
	var lastErr error
	for i := p.minCommitCountToKeep; i < len(commits); i++ {
		commit := commits[i]

		// Skip if already deleted
		if commit.IsDeleted() {
			continue
		}

		if err := commit.Delete(); err != nil {
			// Log the error but continue trying to delete other commits
			lastErr = err
		}
	}

	return lastErr
}

// Clone returns a clone of this policy.
func (p *KeepOnlyLastCommitDeletionPolicy) Clone() IndexDeletionPolicy {
	return NewKeepOnlyLastCommitDeletionPolicy()
}

// String returns a string representation of this policy.
func (p *KeepOnlyLastCommitDeletionPolicy) String() string {
	return fmt.Sprintf("KeepOnlyLastCommitDeletionPolicy(minCommitCountToKeep=%d)",
		p.minCommitCountToKeep)
}

// KeepAllDeletionPolicy keeps all commits and never deletes anything.
// This is useful for backup scenarios or when you want to manually manage commits.
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
func (p *KeepAllDeletionPolicy) OnCommit(commits []*IndexCommit) error {
	// Keep all commits - do nothing
	return nil
}

// OnInit is called when IndexWriter is being initialized.
// Does nothing - keeps all commits.
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

// SnapshotDeletionPolicy keeps commits that have active snapshots.
// This is useful when you need to keep certain commits for point-in-time
// search or backup purposes.
//
// A snapshot can be taken to "pin" a commit, preventing it from being deleted.
// When the snapshot is released, the commit becomes eligible for deletion.
type SnapshotDeletionPolicy struct {
	*BaseIndexDeletionPolicy

	// parent is the underlying deletion policy
	parent IndexDeletionPolicy

	// snapshots maps generation to snapshot count
	snapshots map[int64]int

	// mu protects snapshots
	// Note: In a full implementation, this would need proper synchronization
}

// NewSnapshotDeletionPolicy creates a new SnapshotDeletionPolicy.
func NewSnapshotDeletionPolicy(parent IndexDeletionPolicy) *SnapshotDeletionPolicy {
	if parent == nil {
		parent = NewKeepOnlyLastCommitDeletionPolicy()
	}
	return &SnapshotDeletionPolicy{
		BaseIndexDeletionPolicy: &BaseIndexDeletionPolicy{},
		parent:                  parent,
		snapshots:               make(map[int64]int),
	}
}

// Snapshot takes a snapshot of the given commit.
// This prevents the commit from being deleted until ReleaseSnapshot is called.
func (p *SnapshotDeletionPolicy) Snapshot(commit *IndexCommit) error {
	if commit == nil {
		return fmt.Errorf("commit cannot be nil")
	}

	gen := commit.GetGeneration()
	p.snapshots[gen]++
	return nil
}

// ReleaseSnapshot releases a snapshot of the given commit.
// After all snapshots are released, the commit becomes eligible for deletion.
func (p *SnapshotDeletionPolicy) ReleaseSnapshot(commit *IndexCommit) error {
	if commit == nil {
		return fmt.Errorf("commit cannot be nil")
	}

	gen := commit.GetGeneration()
	if p.snapshots[gen] > 0 {
		p.snapshots[gen]--
		if p.snapshots[gen] == 0 {
			delete(p.snapshots, gen)
		}
	}
	return nil
}

// IsSnapshotted returns true if the given commit has an active snapshot.
func (p *SnapshotDeletionPolicy) IsSnapshotted(commit *IndexCommit) bool {
	if commit == nil {
		return false
	}
	return p.snapshots[commit.GetGeneration()] > 0
}

// GetSnapshotCount returns the number of snapshots for the given commit.
func (p *SnapshotDeletionPolicy) GetSnapshotCount(commit *IndexCommit) int {
	if commit == nil {
		return 0
	}
	return p.snapshots[commit.GetGeneration()]
}

// OnCommit is called each time a commit is made.
func (p *SnapshotDeletionPolicy) OnCommit(commits []*IndexCommit) error {
	// Filter out snapshotted commits
	commitsToDelete := p.filterSnapshottedCommits(commits)

	// Delegate to parent policy
	return p.parent.OnCommit(commitsToDelete)
}

// OnInit is called when IndexWriter is being initialized.
func (p *SnapshotDeletionPolicy) OnInit(commits []*IndexCommit) error {
	// Filter out snapshotted commits
	commitsToDelete := p.filterSnapshottedCommits(commits)

	// Delegate to parent policy
	return p.parent.OnInit(commitsToDelete)
}

// filterSnapshottedCommits removes snapshotted commits from the list.
func (p *SnapshotDeletionPolicy) filterSnapshottedCommits(commits []*IndexCommit) []*IndexCommit {
	result := make(IndexCommitList, 0, len(commits))
	for _, commit := range commits {
		if !p.IsSnapshotted(commit) {
			result = append(result, commit)
		}
	}
	return result
}

// Clone returns a clone of this policy.
func (p *SnapshotDeletionPolicy) Clone() IndexDeletionPolicy {
	// Clone the parent policy
	parentClone := p.parent.Clone()
	if parentClone == nil {
		parentClone = NewKeepOnlyLastCommitDeletionPolicy()
	}

	return &SnapshotDeletionPolicy{
		BaseIndexDeletionPolicy: &BaseIndexDeletionPolicy{},
		parent:                  parentClone,
		snapshots:               make(map[int64]int),
	}
}

// String returns a string representation of this policy.
func (p *SnapshotDeletionPolicy) String() string {
	totalSnapshots := 0
	for _, count := range p.snapshots {
		totalSnapshots += count
	}
	return fmt.Sprintf("SnapshotDeletionPolicy(parent=%v, snapshots=%d)",
		p.parent, totalSnapshots)
}
