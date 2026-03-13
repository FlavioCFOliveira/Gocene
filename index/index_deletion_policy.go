// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
)

// IndexDeletionPolicy is used to determine which index commits should be deleted.
// This is the Go port of Lucene's org.apache.lucene.index.IndexDeletionPolicy.
//
// IndexDeletionPolicy is invoked when commits are made or when the IndexWriter
// is initialized. The policy decides which commits to keep and which to delete.
//
// Implementations can implement different retention policies:
//   - KeepOnlyLastCommitDeletionPolicy: Keep only the most recent commit (default)
//   - KeepAllDeletionPolicy: Keep all commits (useful for backup scenarios)
//   - SnapshotDeletionPolicy: Keep commits that have active snapshots
//   - Custom policies based on age, count, or other criteria
//
// The policy is called in two situations:
//  1. OnCommit: When a new commit is being made
//  2. OnInit: When IndexWriter is being initialized
//
// IMPORTANT: The policy must ensure that at least one commit is kept,
// otherwise the index would be lost.
type IndexDeletionPolicy interface {
	// OnCommit is called each time a commit is made.
	// The policy should decide which commits to delete based on the new commit.
	//
	// Parameters:
	//   - commits: A list of all current commits, sorted by age (oldest first)
	//
	// The policy can call Delete() on any commits it wants to remove.
	OnCommit(commits []*IndexCommit) error

	// OnInit is called when IndexWriter is being initialized.
	// This gives the policy a chance to clean up old commits after a crash.
	//
	// Parameters:
	//   - commits: A list of all current commits, sorted by age (oldest first)
	//
	// The policy can call Delete() on any commits it wants to remove.
	OnInit(commits []*IndexCommit) error

	// Clone returns a clone of this policy.
	// This is called when opening a new IndexWriter.
	Clone() IndexDeletionPolicy
}

// BaseIndexDeletionPolicy provides common functionality for deletion policies.
type BaseIndexDeletionPolicy struct{}

// OnCommit is called when a commit is made (must be implemented by subclasses).
func (p *BaseIndexDeletionPolicy) OnCommit(commits []*IndexCommit) error {
	return fmt.Errorf("OnCommit not implemented")
}

// OnInit is called when IndexWriter is initialized (must be implemented by subclasses).
func (p *BaseIndexDeletionPolicy) OnInit(commits []*IndexCommit) error {
	return fmt.Errorf("OnInit not implemented")
}

// Clone returns a clone of this policy (must be implemented by subclasses).
func (p *BaseIndexDeletionPolicy) Clone() IndexDeletionPolicy {
	return nil
}

// EnsureCommitsValid checks that the commits list is valid.
func EnsureCommitsValid(commits []*IndexCommit) error {
	if commits == nil || len(commits) == 0 {
		return fmt.Errorf("commits list is empty")
	}

	// Ensure all commits have non-nil directories
	for i, commit := range commits {
		if commit == nil {
			return fmt.Errorf("commit at index %d is nil", i)
		}
		if commit.GetDirectory() == nil {
			return fmt.Errorf("commit at index %d has nil directory", i)
		}
	}

	return nil
}

// FindCommitByGeneration finds a commit by its generation.
func FindCommitByGeneration(commits []*IndexCommit, generation int64) *IndexCommit {
	for _, commit := range commits {
		if commit.GetGeneration() == generation {
			return commit
		}
	}
	return nil
}

// FilterDeletedCommits removes already deleted commits from the list.
func FilterDeletedCommits(commits []*IndexCommit) []*IndexCommit {
	result := make([]*IndexCommit, 0, len(commits))
	for _, commit := range commits {
		if !commit.IsDeleted() {
			result = append(result, commit)
		}
	}
	return result
}

// DeleteCommits deletes the specified commits, collecting errors.
func DeleteCommits(commits []*IndexCommit) error {
	var lastErr error
	for _, commit := range commits {
		if err := commit.Delete(); err != nil {
			lastErr = err
		}
	}
	return lastErr
}