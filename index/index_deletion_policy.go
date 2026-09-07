// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// Commit defines the interface for an index commit point.
// This allows IndexDeletionPolicy implementations (like SnapshotDeletionPolicy)
// to wrap commit points and intercept deletion requests.
type Commit interface {
	Delete() error
	GetGeneration() int64
	GetDirectory() store.Directory
	GetFileNames() ([]string, error)
	GetSegmentsFileName() string
	GetUserData() map[string]string
	IsDeleted() bool
	GetSegmentCount() int
}

// IndexDeletionPolicy is used to determine which index commits should be deleted.
// This is the Go port of Lucene's org.apache.lucene.index.IndexDeletionPolicy.
type IndexDeletionPolicy interface {
	// OnCommit is called each time a commit is made.
	OnCommit(commits []Commit) error

	// OnInit is called when IndexWriter is being initialized.
	OnInit(commits []Commit) error

	// Clone returns a clone of this policy.
	Clone() IndexDeletionPolicy
}

// BaseIndexDeletionPolicy provides common functionality for deletion policies.
type BaseIndexDeletionPolicy struct{}

func (p *BaseIndexDeletionPolicy) OnCommit(commits []Commit) error {
	return fmt.Errorf("OnCommit not implemented")
}

func (p *BaseIndexDeletionPolicy) OnInit(commits []Commit) error {
	return fmt.Errorf("OnInit not implemented")
}

func (p *BaseIndexDeletionPolicy) Clone() IndexDeletionPolicy {
	return nil
}

// KeepAllDeletionPolicy keeps all commits and never deletes anything.
type KeepAllDeletionPolicy struct {
	*BaseIndexDeletionPolicy
}

func NewKeepAllDeletionPolicy() *KeepAllDeletionPolicy {
	return &KeepAllDeletionPolicy{
		BaseIndexDeletionPolicy: &BaseIndexDeletionPolicy{},
	}
}

func (p *KeepAllDeletionPolicy) OnCommit(commits []Commit) error {
	return nil
}

func (p *KeepAllDeletionPolicy) OnInit(commits []Commit) error {
	return nil
}

func (p *KeepAllDeletionPolicy) Clone() IndexDeletionPolicy {
	return NewKeepAllDeletionPolicy()
}

func (p *KeepAllDeletionPolicy) String() string {
	return "KeepAllDeletionPolicy"
}

// KeepLastNCommitsDeletionPolicy keeps the last N commits and removes all prior
// commits after a new commit is done.
type KeepLastNCommitsDeletionPolicy struct {
	*BaseIndexDeletionPolicy
	numCommitsToKeep int
}

func NewKeepLastNCommitsDeletionPolicy(numCommitsToKeep int) *KeepLastNCommitsDeletionPolicy {
	if numCommitsToKeep <= 0 {
		panic("number of recent commits to keep must be positive")
	}
	return &KeepLastNCommitsDeletionPolicy{
		BaseIndexDeletionPolicy: &BaseIndexDeletionPolicy{},
		numCommitsToKeep:        numCommitsToKeep,
	}
}

func (p *KeepLastNCommitsDeletionPolicy) OnCommit(commits []Commit) error {
	size := len(commits)
	for i := 0; i < size-p.numCommitsToKeep; i++ {
		if err := commits[i].Delete(); err != nil {
			return fmt.Errorf("failed to delete commit %d: %w", i, err)
		}
	}
	return nil
}

func (p *KeepLastNCommitsDeletionPolicy) OnInit(commits []Commit) error {
	return p.OnCommit(commits)
}

func (p *KeepLastNCommitsDeletionPolicy) Clone() IndexDeletionPolicy {
	return NewKeepLastNCommitsDeletionPolicy(p.numCommitsToKeep)
}

func (p *KeepLastNCommitsDeletionPolicy) String() string {
	return fmt.Sprintf("KeepLastNCommitsDeletionPolicy(numToKeep=%d)", p.numCommitsToKeep)
}

// SnapshotDeletionPolicy wraps any other IndexDeletionPolicy and adds the
// ability to hold and later release snapshots of an index.
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

func (p *SnapshotDeletionPolicy) Release(commit Commit) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	gen := commit.GetGeneration()
	return p.releaseGen(gen)
}

// ReleaseGen releases a snapshot by generation. Mirrors the protected
// SnapshotDeletionPolicy.releaseGen, which exists so that
// PersistentSnapshotDeletionPolicy.release(long) can reach it.
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

func (p *SnapshotDeletionPolicy) GetSnapshots() []Commit {
	p.mu.RLock()
	defer p.mu.RUnlock()

	snapshots := make([]Commit, 0, len(p.indexCommits))
	for _, c := range p.indexCommits {
		snapshots = append(snapshots, c)
	}
	return snapshots
}

// GetSnapshotCount returns the total number of snapshots currently held,
// counting every reference taken on every snapshotted generation. Mirrors
// SnapshotDeletionPolicy.getSnapshotCount.
func (p *SnapshotDeletionPolicy) GetSnapshotCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()

	total := 0
	for _, refCount := range p.refCounts {
		total += refCount
	}
	return total
}

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

// EnsureCommitsValid checks that the commits list is valid.
func EnsureCommitsValid(commits []Commit) error {
	if commits == nil || len(commits) == 0 {
		return fmt.Errorf("commits list is empty")
	}
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

func FindCommitByGeneration(commits []Commit, generation int64) Commit {
	for _, commit := range commits {
		if commit.GetGeneration() == generation {
			return commit
		}
	}
	return nil
}

func FilterDeletedCommits(commits []Commit) []Commit {
	result := make([]Commit, 0, len(commits))
	for _, commit := range commits {
		if !commit.IsDeleted() {
			result = append(result, commit)
		}
	}
	return result
}

func DeleteCommits(commits []Commit) error {
	var lastErr error
	for _, commit := range commits {
		if err := commit.Delete(); err != nil {
			lastErr = err
		}
	}
	return lastErr
}
