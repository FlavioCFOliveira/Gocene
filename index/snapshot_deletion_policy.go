// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"sync"
)

// SnapshotDeletionPolicy wraps any other IndexDeletionPolicy and adds the ability to hold
// and later release snapshots of an index.
// Mirrors org.apache.lucene.index.SnapshotDeletionPolicy from Apache Lucene 10.5.0.
type SnapshotDeletionPolicy struct {
	mu sync.Mutex

	// refCounts records how many snapshots are held against each commit generation
	refCounts map[int64]int

	// indexCommits maps gen to IndexCommit.
	indexCommits map[int64]IndexCommit

	// primary is the wrapped IndexDeletionPolicy
	primary IndexDeletionPolicy

	// lastCommit is the most recently committed IndexCommit.
	lastCommit IndexCommit

	// initCalled detects misuse
	initCalled bool
}

// NewSnapshotDeletionPolicy constructs a SnapshotDeletionPolicy.
func NewSnapshotDeletionPolicy(primary IndexDeletionPolicy) *SnapshotDeletionPolicy {
	return &SnapshotDeletionPolicy{
		primary:      primary,
		refCounts:    make(map[int64]int),
		indexCommits: make(map[int64]IndexCommit),
	}
}

func (p *SnapshotDeletionPolicy) OnCommit(commits []IndexCommit) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	wrapped := make([]IndexCommit, len(commits))
	for i, c := range commits {
		wrapped[i] = &snapshotCommitPoint{
			cp: c,
			p:  p,
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

func (p *SnapshotDeletionPolicy) OnInit(commits []IndexCommit) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.initCalled = true
	wrapped := make([]IndexCommit, len(commits))
	for i, c := range commits {
		wrapped[i] = &snapshotCommitPoint{
			cp: c,
			p:  p,
		}
	}

	if err := p.primary.OnInit(wrapped); err != nil {
		return err
	}

	for _, commit := range commits {
		if _, ok := p.refCounts[commit.GetGeneration()]; ok {
			p.indexCommits[commit.GetGeneration()] = commit
		}
	}

	if len(commits) > 0 {
		p.lastCommit = commits[len(commits)-1]
	}
	return nil
}

func (p *SnapshotDeletionPolicy) Release(commit IndexCommit) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	gen := commit.GetGeneration()
	return p.releaseGen(gen)
}

func (p *SnapshotDeletionPolicy) releaseGen(gen int64) error {
	if !p.initCalled {
		return fmt.Errorf("this instance is not being used by IndexWriter; be sure to use the instance returned from writer.getConfig().getIndexDeletionPolicy()")
	}

	refCount, ok := p.refCounts[gen]
	if !ok {
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

func (p *SnapshotDeletionPolicy) incRef(ic IndexCommit) {
	gen := ic.GetGeneration()
	refCount, ok := p.refCounts[gen]
	if !ok {
		p.indexCommits[gen] = p.lastCommit
		p.refCounts[gen] = 1
	} else {
		p.refCounts[gen] = refCount + 1
	}
}

func (p *SnapshotDeletionPolicy) Snapshot() (IndexCommit, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.initCalled {
		return nil, fmt.Errorf("this instance is not being used by IndexWriter; be sure to use the instance returned from writer.getConfig().getIndexDeletionPolicy()")
	}
	if p.lastCommit == nil {
		return nil, fmt.Errorf("No index commit to snapshot")
	}

	p.incRef(p.lastCommit)
	return p.lastCommit, nil
}

func (p *SnapshotDeletionPolicy) GetSnapshots() []IndexCommit {
	p.mu.Lock()
	defer p.mu.Unlock()

	snapshots := make([]IndexCommit, 0, len(p.indexCommits))
	for _, commit := range p.indexCommits {
		snapshots = append(snapshots, commit)
	}
	return snapshots
}

func (p *SnapshotDeletionPolicy) GetSnapshotCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()

	total := 0
	for _, refCount := range p.refCounts {
		total += refCount
	}
	return total
}

func (p *SnapshotDeletionPolicy) GetIndexCommit(gen int64) IndexCommit {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.indexCommits[gen]
}

type snapshotCommitPoint struct {
	cp IndexCommit
	p  *SnapshotDeletionPolicy
}

func (s *snapshotCommitPoint) Delete() {
	s.p.mu.Lock()
	defer s.p.mu.Unlock()

	if _, ok := s.p.refCounts[s.cp.GetGeneration()]; !ok {
		s.cp.Delete()
	}
}

func (s *snapshotCommitPoint) GetDirectory() store.Directory {
	return s.cp.GetDirectory()
}

func (s *snapshotCommitPoint) GetFileNames() ([]string, error) {
	return s.cp.GetFileNames()
}

func (s *snapshotCommitPoint) GetGeneration() int64 {
	return s.cp.GetGeneration()
}

func (s *snapshotCommitPoint) GetSegmentsFileName() string {
	return s.cp.GetSegmentsFileName()
}

func (s *snapshotCommitPoint) GetUserData() (map[string]string, error) {
	return s.cp.GetUserData()
}

func (s *snapshotCommitPoint) IsDeleted() bool {
	return s.cp.IsDeleted()
}

func (s *snapshotCommitPoint) GetSegmentCount() int {
	return s.cp.GetSegmentCount()
}

func (s *snapshotCommitPoint) String() string {
	return fmt.Sprintf("SnapshotDeletionPolicy.SnapshotCommitPoint(%v)", s.cp)
}
