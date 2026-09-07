// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
)

// KeepLastNCommitsDeletionPolicy keeps the last N commits and removes all prior
// commits after a new commit is done. This policy can be useful for maintaining
// a history of recent commits while still managing index size.
type KeepLastNCommitsDeletionPolicy struct {
	*BaseIndexDeletionPolicy
	numCommitsToKeep int
}

// NewKeepLastNCommitsDeletionPolicy creates a new KeepLastNCommitsDeletionPolicy
// that retains the N most recent commits.
//
// It panics if numCommitsToKeep is not positive, matching the IllegalArgumentException
// thrown by the Lucene reference implementation.
func NewKeepLastNCommitsDeletionPolicy(numCommitsToKeep int) *KeepLastNCommitsDeletionPolicy {
	if numCommitsToKeep <= 0 {
		panic("number of recent commits to keep must be positive")
	}
	return &KeepLastNCommitsDeletionPolicy{
		BaseIndexDeletionPolicy: &BaseIndexDeletionPolicy{},
		numCommitsToKeep:        numCommitsToKeep,
	}
}

// OnCommit deletes all but the last N commits.
// The commits slice is assumed to be sorted from oldest to newest.
func (p *KeepLastNCommitsDeletionPolicy) OnCommit(commits []Commit) error {
	size := len(commits)
	for i := 0; i < size-p.numCommitsToKeep; i++ {
		if err := commits[i].Delete(); err != nil {
			return fmt.Errorf("failed to delete commit %d: %w", i, err)
		}
	}
	return nil
}

// OnInit is called when IndexWriter is being initialized.
func (p *KeepLastNCommitsDeletionPolicy) OnInit(commits []Commit) error {
	return p.OnCommit(commits)
}

// Clone returns a clone of this policy.
func (p *KeepLastNCommitsDeletionPolicy) Clone() IndexDeletionPolicy {
	return NewKeepLastNCommitsDeletionPolicy(p.numCommitsToKeep)
}

// String returns a string representation of the policy.
func (p *KeepLastNCommitsDeletionPolicy) String() string {
	return fmt.Sprintf("KeepLastNCommitsDeletionPolicy(numToKeep=%d)", p.numCommitsToKeep)
}
