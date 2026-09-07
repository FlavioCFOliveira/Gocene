// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// SnapshotDeletionPolicy wraps any other IndexDeletionPolicy and adds the
// ability to hold and later release snapshots of an index. While a snapshot
// is held, the IndexWriter will not remove any files associated with it
// even if the index is otherwise being actively, arbitrarily changed.
// Because we wrap another arbitrary IndexDeletionPolicy, this gives you
// the freedom to continue using whatever IndexDeletionPolicy you would
// normally want to use with your index.
//
// This class maintains all snapshots in-memory, and so the information is not
// persisted and not protected against system failures. If persistence is
// important, you can use PersistentSnapshotDeletionPolicy.
type SnapshotDeletionPolicy struct {
	mu sync.RWMutex

	// primary is the wrapped IndexDeletionPolicy
	primary IndexDeletionPolicy

	// refCounts records how many snapshots are held against each commit generation
	refCounts map[int64]int

	// indexCommits maps generation to the commit point
	indexCommits map[int64]Commit

	// lastCommit is the most recently committed commit
	lastCommit Commit

	// initCalled tracks if OnInit has been called
	initCalled bool
}

// NewSnapshotDeletionPolicy creates a new SnapshotDeletionPolicy wrapping the provided primary policy.
func NewSnapshotDeletionPolicy(primary IndexDeletionPolicy) *SnapshotDeletionPolicy {
	if primary == nil {
		primary = NewKeepOnlyLastCommitDeletionPolicy()
	}
	return &SnapshotDeletionPolicy{
		primary:      primary,
		refCounts:    make(map[int64]int),
		indexCommits: make(map[int64]Commit),
	}
}

func (p *SnapshotDeletionPolicy) OnCommit(commits []Commit) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	wrapped := make([]Commit, len(commits))
	for i, c := range commits {
		wrapped[i] = &snapshotCommitPoint{
			policy: p,
			commit: c,
		}
	}

	if err := p.primary.OnCommit(wrapped); err != nil {
		return err
	}

	if len(commits) > 0 {
		p.lastCommit = commits[len(commits)-1]
	}
	return nil
}

func (p *SnapshotDeletionPolicy) OnInit(commits []Commit) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.initCalled = true

	wrapped := make([]Commit, len(commits))
	for i, c := range commits {
		wrapped[i] = &snapshotCommitPoint{
			policy: p,
			commit: c,
		}
	}

	if err := p.primary.OnInit(wrapped); err != nil {
		return err
	}

	for _, commit := range commits {
		gen := commit.GetGeneration()
		if _, exists := p.refCounts[gen]; exists {
			p.indexCommits[gen] = commit
		}
	}

	if len(commits) > 0 {
		p.lastCommit = commits[len(commits)-1]
	}
	return nil
}

// Snapshot snapshots the last commit and returns it. Once a commit is 'snapshotted,'
// it is protected from deletion. The snapshot can be removed by calling
// Release(commit).
func (p *SnapshotDeletionPolicy) Snapshot() (Commit, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.initCalled {
		return nil, fmt.Errorf("this instance is not being used by IndexWriter; be sure to use the instance returned from writer.getConfig().getIndexDeletionPolicy()")
	}
	if p.lastCommit == nil {
		return nil, fmt.Errorf("no index commit to snapshot")
	}

	p.incRef(p.lastCommit)
	return p.lastCommit, nil
}

// Release releases a snapshotted commit.
func (p *SnapshotDeletionPolicy) Release(commit Commit) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	gen := commit.GetGeneration()
	return p.releaseGen(gen)
}

// ReleaseGen releases a snapshot by generation.
func (p *SnapshotDeletionPolicy) ReleaseGen(gen int64) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.releaseGen(gen)
}

func (p *SnapshotDeletionPolicy) releaseGen(gen int64) error {
	if !p.initCalled {
		return fmt.Errorf("this instance is not being used by IndexWriter; be sure to use the instance returned from writer.getConfig().getIndexDeletionPolicy()")
	}

	refCount, exists := p.refCounts[gen]
	if !exists {
		return fmt.Errorf("commit gen=%d is not currently snapshotted", gen)
	}

	refCount--
	if refCount == 0 {
		delete(p.refCounts, gen)
		delete(p.indexCommits, gen)
	} else {
		p.refCounts[gen] = refCount
	}
	return nil
}

func (p *SnapshotDeletionPolicy) incRef(ic Commit) {
	gen := ic.GetGeneration()
	refCount := p.refCounts[gen]
	p.indexCommits[gen] = p.lastCommit
	p.refCounts[gen] = refCount + 1
}

// GetSnapshots returns all commits held by at least one snapshot.
func (p *SnapshotDeletionPolicy) GetSnapshots() []Commit {
	p.mu.RLock()
	defer p.mu.RUnlock()

	snapshots := make([]Commit, 0, len(p.indexCommits))
	for _, c := range p.indexCommits {
		snapshots = append(snapshots, c)
	}
	return snapshots
}

// GetSnapshotCount returns the total number of snapshots currently held.
func (p *SnapshotDeletionPolicy) GetSnapshotCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()

	total := 0
	for _, refCount := range p.refCounts {
		total += refCount
	}
	return total
}

// GetIndexCommit retrieves a commit from its generation; returns nil if not snapshotted.
func (p *SnapshotDeletionPolicy) GetIndexCommit(gen int64) Commit {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.indexCommits[gen]
}

func (p *SnapshotDeletionPolicy) Clone() IndexDeletionPolicy {
	p.mu.RLock()
	primaryClone := p.primary.Clone()
	p.mu.RUnlock()
	return NewSnapshotDeletionPolicy(primaryClone)
}

func (p *SnapshotDeletionPolicy) String() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return fmt.Sprintf("SnapshotDeletionPolicy(primary=%v, snapshotCount=%d)",
		p.primary, len(p.refCounts))
}

type snapshotCommitPoint struct {
	policy *SnapshotDeletionPolicy
	commit Commit
}

func (s *snapshotCommitPoint) Delete() error {
	s.policy.mu.Lock()
	defer s.policy.mu.Unlock()

	if _, snapshotted := s.policy.refCounts[s.commit.GetGeneration()]; !snapshotted {
		return s.commit.Delete()
	}
	return nil
}

func (s *snapshotCommitPoint) GetGeneration() int64 { return s.commit.GetGeneration() }
func (s *snapshotCommitPoint) GetDirectory() store.Directory { return s.commit.GetDirectory() }
func (s *snapshotCommitPoint) GetFileNames() ([]string, error) { return s.commit.GetFileNames() }
func (s *snapshotCommitPoint) GetSegmentsFileName() string { return s.commit.GetSegmentsFileName() }
func (s *snapshotCommitPoint) GetUserData() map[string]string { return s.commit.GetUserData() }
func (s *snapshotCommitPoint) IsDeleted() bool { return s.commit.IsDeleted() }
func (s *snapshotCommitPoint) GetSegmentCount() int { return s.commit.GetSegmentCount() }
