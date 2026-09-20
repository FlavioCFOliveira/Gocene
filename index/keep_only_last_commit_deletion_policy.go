// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
)

// KeepOnlyLastCommitDeletionPolicy keeps only the most recent commit and
// immediately removes all prior commits after a new commit is done.
// This is the default deletion policy.
type KeepOnlyLastCommitDeletionPolicy struct {
	*BaseIndexDeletionPolicy
}

// NewKeepOnlyLastCommitDeletionPolicy creates a new KeepOnlyLastCommitDeletionPolicy.
func NewKeepOnlyLastCommitDeletionPolicy() *KeepOnlyLastCommitDeletionPolicy {
	return &KeepOnlyLastCommitDeletionPolicy{
		BaseIndexDeletionPolicy: &BaseIndexDeletionPolicy{},
	}
}

// OnCommit deletes all commits except the most recent one.
func (p *KeepOnlyLastCommitDeletionPolicy) OnCommit(commits []Commit) error {
	if len(commits) <= 1 {
		return nil
	}
	for i := 0; i < len(commits)-1; i++ {
		if err := commits[i].Delete(); err != nil {
			return fmt.Errorf("failed to delete commit %d: %w", i, err)
		}
	}
	return nil
}

// OnInit deletes all commits except the most recent one.
func (p *KeepOnlyLastCommitDeletionPolicy) OnInit(commits []Commit) error {
	return p.OnCommit(commits)
}

// Clone returns a clone of this policy.
func (p *KeepOnlyLastCommitDeletionPolicy) Clone() IndexDeletionPolicy {
	return NewKeepOnlyLastCommitDeletionPolicy()
}

// String returns a string representation of the policy.
func (p *KeepOnlyLastCommitDeletionPolicy) String() string {
	return "KeepOnlyLastCommitDeletionPolicy"
}
