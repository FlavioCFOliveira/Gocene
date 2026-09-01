// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
)

// KeepLastNCommitsDeletionPolicy keeps the last N commits and removes all prior commits
// after a new commit is done.
// Mirrors org.apache.lucene.index.KeepLastNCommitsDeletionPolicy from Apache Lucene 10.5.0.
type KeepLastNCommitsDeletionPolicy struct {
	numCommitsToKeep int
}

// NewKeepLastNCommitsDeletionPolicy constructs a KeepLastNCommitsDeletionPolicy.
func NewKeepLastNCommitsDeletionPolicy(numCommitsToKeep int) *KeepLastNCommitsDeletionPolicy {
	if numCommitsToKeep <= 0 {
		panic("number of recent commits to keep must be positive")
	}
	return &KeepLastNCommitsDeletionPolicy{
		numCommitsToKeep: numCommitsToKeep,
	}
}

func (p *KeepLastNCommitsDeletionPolicy) OnInit(commits []IndexCommit) error {
	return p.OnCommit(commits)
}

func (p *KeepLastNCommitsDeletionPolicy) OnCommit(commits []IndexCommit) error {
	// The commits list is already sorted from oldest to newest
	size := len(commits)
	for i := 0; i < size-p.numCommitsToKeep; i++ {
		commits[i].Delete()
	}
	return nil
}
