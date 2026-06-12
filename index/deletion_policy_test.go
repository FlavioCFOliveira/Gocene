// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index_test

import (
	"strings"
	"sync"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// addPolicyDoc adds a single document with a "content" field to the writer.
// Uses StringField (non-tokenized) because the token-stream bridge is stubbed;
// non-tokenized fields are routed through the binary indexing path.
func addPolicyDoc(t *testing.T, w *index.IndexWriter) {
	t.Helper()
	doc := document.NewDocument()
	sf, err := document.NewStringField("content", "aaa", false)
	if err != nil {
		t.Fatalf("NewStringField: %v", err)
	}
	doc.Add(sf)
	if err := w.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
}

// expPolicy is a concrete deletion policy that expires commits whose
// generation is more than expireAfter behind the latest.
type expPolicy struct {
	expireAfter int64
}

func (p *expPolicy) OnInit(commits []*index.IndexCommit) error { return p.OnCommit(commits) }
func (p *expPolicy) OnCommit(commits []*index.IndexCommit) error {
	if len(commits) == 0 {
		return nil
	}
	latest := commits[len(commits)-1]
	latestGen := latest.GetGeneration()
	for _, c := range commits {
		if c.GetGeneration() <= latestGen-p.expireAfter {
			if err := c.Delete(); err != nil {
				return err
			}
		}
	}
	return nil
}
func (p *expPolicy) Clone() index.IndexDeletionPolicy { return &expPolicy{expireAfter: p.expireAfter} }

// TestDeletionPolicy_ExpirationTime tests an expiration-time deletion policy
// that deletes commits older than a given generation threshold.
func TestDeletionPolicy_ExpirationTime(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	writer, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	for i := 0; i < 5; i++ {
		addPolicyDoc(t, writer)
		if err := writer.Commit(); err != nil {
			t.Fatalf("Commit %d: %v", i, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	commits, err := index.ListCommits(dir)
	if err != nil {
		t.Fatalf("ListCommits: %v", err)
	}
	if len(commits) != 5 {
		t.Fatalf("expected 5 commits, got %d", len(commits))
	}

	// Apply expiration: delete commits whose generation is > 2 behind latest.
	latestGen := commits[len(commits)-1].GetGeneration()
	for _, c := range commits {
		if c.GetGeneration() <= latestGen-2 {
			if err := c.Delete(); err != nil {
				t.Fatalf("Delete(gen=%d): %v", c.GetGeneration(), err)
			}
		}
	}

	survivors, err := index.ListCommits(dir)
	if err != nil {
		t.Fatalf("ListCommits after delete: %v", err)
	}
	if want := 2; len(survivors) != want {
		t.Errorf("survivors = %d, want %d", len(survivors), want)
	}
	for i, c := range survivors {
		wantGen := latestGen - 1 + int64(i)
		if c.GetGeneration() != wantGen {
			t.Errorf("survivor[%d] gen = %d, want %d", i, c.GetGeneration(), wantGen)
		}
	}
}

// TestDeletionPolicy_KeepAll verifies that KeepAllDeletionPolicy never deletes
// any commit.
func TestDeletionPolicy_KeepAll(t *testing.T) {
	policy := index.NewKeepAllDeletionPolicy()

	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	for i := 0; i < 4; i++ {
		addPolicyDoc(t, writer)
		if err := writer.Commit(); err != nil {
			t.Fatalf("Commit %d: %v", i, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	commits, err := index.ListCommits(dir)
	if err != nil {
		t.Fatalf("ListCommits: %v", err)
	}

	if err := policy.OnCommit(commits); err != nil {
		t.Fatalf("policy.OnCommit: %v", err)
	}
	for _, c := range commits {
		if c.IsDeleted() {
			t.Errorf("commit gen=%d was deleted by KeepAll", c.GetGeneration())
		}
	}
	if err := policy.OnInit(commits); err != nil {
		t.Fatalf("policy.OnInit: %v", err)
	}
	for _, c := range commits {
		if c.IsDeleted() {
			t.Errorf("commit gen=%d was deleted by KeepAll OnInit", c.GetGeneration())
		}
	}
}

// TestDeletionPolicy_OpenPriorSnapshot tests SetIndexCommit validation and
// the ListCommits lifecycle.
func TestDeletionPolicy_OpenPriorSnapshot(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	for i := 0; i < 5; i++ {
		addPolicyDoc(t, writer)
		if i%2 == 1 {
			if err := writer.Commit(); err != nil {
				t.Fatalf("Commit %d: %v", i, err)
			}
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	commits, err := index.ListCommits(dir)
	if err != nil {
		t.Fatalf("ListCommits: %v", err)
	}
	if len(commits) == 0 {
		t.Fatal("expected at least 1 commit")
	}

	// SetIndexCommit with CREATE should be rejected.
	commit := commits[len(commits)-1]
	badConfig := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	badConfig.SetOpenMode(index.CREATE)
	badConfig.SetIndexCommit(commit)
	if _, err := index.NewIndexWriter(dir, badConfig); err == nil {
		t.Fatal("expected error when SetIndexCommit with OpenMode.CREATE")
	}

	// SetIndexCommit with APPEND on a non-empty index should work.
	goodConfig := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	goodConfig.SetOpenMode(index.APPEND)
	goodConfig.SetIndexCommit(commit)
	w2, err := index.NewIndexWriter(dir, goodConfig)
	if err != nil {
		t.Fatalf("NewIndexWriter with SetIndexCommit: %v", err)
	}
	if err := w2.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// TestDeletionPolicy_KeepNoneOnInit tests a policy whose OnInit deletes all
// existing commits except the latest.
func TestDeletionPolicy_KeepNoneOnInit(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	for i := 0; i < 3; i++ {
		addPolicyDoc(t, writer)
		if err := writer.Commit(); err != nil {
			t.Fatalf("Commit %d: %v", i, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	commits, err := index.ListCommits(dir)
	if err != nil {
		t.Fatalf("ListCommits: %v", err)
	}
	if len(commits) != 3 {
		t.Fatalf("expected 3 commits, got %d", len(commits))
	}

	policy := index.NewKeepOnlyLastCommitDeletionPolicy()
	if err := policy.OnInit(commits); err != nil {
		t.Fatalf("policy.OnInit: %v", err)
	}

	survivors, err := index.ListCommits(dir)
	if err != nil {
		t.Fatalf("ListCommits after policy: %v", err)
	}
	if len(survivors) != 1 {
		t.Errorf("expected 1 survivor after KeepOnlyLastCommit OnInit, got %d", len(survivors))
	}
}

// TestDeletionPolicy_KeepLastN tests keeping only the last N commits.
func TestDeletionPolicy_KeepLastN(t *testing.T) {
	const numToKeep = 3

	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	for i := 0; i < 6; i++ {
		addPolicyDoc(t, writer)
		if err := writer.Commit(); err != nil {
			t.Fatalf("Commit %d: %v", i, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	commits, err := index.ListCommits(dir)
	if err != nil {
		t.Fatalf("ListCommits: %v", err)
	}
	if len(commits) != 6 {
		t.Fatalf("expected 6 commits before deletion, got %d", len(commits))
	}

	// Keep last numToKeep.
	for i := 0; i < len(commits)-numToKeep; i++ {
		if err := commits[i].Delete(); err != nil {
			t.Fatalf("Delete commit %d: %v", i, err)
		}
	}

	survivors, err := index.ListCommits(dir)
	if err != nil {
		t.Fatalf("ListCommits after delete: %v", err)
	}
	if len(survivors) != numToKeep {
		t.Errorf("survivors = %d, want %d", len(survivors), numToKeep)
	}

	latestGen := commits[len(commits)-1].GetGeneration()
	for i, c := range survivors {
		wantGen := latestGen - int64(numToKeep-1) + int64(i)
		if c.GetGeneration() != wantGen {
			t.Errorf("survivor[%d] gen = %d, want %d", i, c.GetGeneration(), wantGen)
		}
	}
}

// TestDeletionPolicy_KeepLastNWithCreates tests keep-last-N across writer reopens.
func TestDeletionPolicy_KeepLastNWithCreates(t *testing.T) {
	const numToKeep = 5

	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	config.SetMaxBufferedDocs(10)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	for i := 0; i < 10; i++ {
		addPolicyDoc(t, writer)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	config2 := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	config2.SetOpenMode(index.APPEND)
	writer2, err := index.NewIndexWriter(dir, config2)
	if err != nil {
		t.Fatalf("NewIndexWriter (append): %v", err)
	}
	for i := 0; i < 7; i++ {
		addPolicyDoc(t, writer2)
		if i%2 == 0 {
			if err := writer2.Commit(); err != nil {
				t.Fatalf("Commit %d: %v", i, err)
			}
		}
	}
	if err := writer2.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	allCommits, err := index.ListCommits(dir)
	if err != nil {
		t.Fatalf("ListCommits: %v", err)
	}
	if len(allCommits) == 0 {
		t.Fatal("expected at least 1 commit")
	}

	// Keep last numToKeep.
	for i := 0; i < len(allCommits)-numToKeep; i++ {
		_ = allCommits[i].Delete()
	}

	survivors, err := index.ListCommits(dir)
	if err != nil {
		t.Fatalf("ListCommits after delete: %v", err)
	}
	if len(survivors) > numToKeep {
		t.Errorf("survivors = %d, want <= %d", len(survivors), numToKeep)
	}
}

// TestDeletionPolicy_KeepLastNCommits tests KeepLastNCommitsDeletionPolicy.
func TestDeletionPolicy_KeepLastNCommits(t *testing.T) {
	const numCommitsToKeep = 3

	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	for i := 0; i < 6; i++ {
		addPolicyDoc(t, writer)
		if err := writer.Commit(); err != nil {
			t.Fatalf("Commit %d: %v", i, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	allCommits, err := index.ListCommits(dir)
	if err != nil {
		t.Fatalf("ListCommits: %v", err)
	}
	if len(allCommits) != 6 {
		t.Fatalf("expected 6 commits, got %d", len(allCommits))
	}

	policy := index.NewKeepLastNCommitsDeletionPolicy(numCommitsToKeep)
	if err := policy.OnCommit(allCommits); err != nil {
		t.Fatalf("policy.OnCommit: %v", err)
	}

	remaining, err := index.ListCommits(dir)
	if err != nil {
		t.Fatalf("ListCommits after policy: %v", err)
	}
	if len(remaining) != numCommitsToKeep {
		t.Errorf("remaining commits = %d, want %d", len(remaining), numCommitsToKeep)
	}

	// Verify we can open a reader on each surviving commit.
	for _, c := range remaining {
		reader, err := index.OpenDirectoryReaderFromCommit(dir, c)
		if err != nil {
			t.Errorf("OpenDirectoryReaderFromCommit(seg=%s): %v", c.GetSegmentsFileName(), err)
			continue
		}
		reader.Close()
	}
}

// TestDeletionPolicy_KeepLastNCommitsZero verifies that constructing a
// KeepLastNCommitsDeletionPolicy with zero panics.
func TestDeletionPolicy_KeepLastNCommitsZero(t *testing.T) {
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Error("NewKeepLastNCommitsDeletionPolicy(0) should panic")
			}
		}()
		index.NewKeepLastNCommitsDeletionPolicy(0)
	}()

	p := index.NewKeepLastNCommitsDeletionPolicy(1)
	if p == nil {
		t.Error("NewKeepLastNCommitsDeletionPolicy(1) returned nil")
	}
}

// verifyCommitOrder checks that commits are sorted by generation ascending.
func verifyCommitOrder(t *testing.T, commits []*index.IndexCommit) {
	t.Helper()
	if len(commits) == 0 {
		return
	}
	last := commits[0].GetGeneration()
	for i, c := range commits[1:] {
		gen := c.GetGeneration()
		if gen <= last {
			t.Errorf("commits out of order at index %d: gen=%d <= prev=%d", i+1, gen, last)
		}
		last = gen
	}
}

// TestDeletionPolicy_SnapshotDeletionPolicy tests the SnapshotDeletionPolicy
// lifecycle.
func TestDeletionPolicy_SnapshotDeletionPolicy(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	for i := 0; i < 3; i++ {
		addPolicyDoc(t, writer)
		if err := writer.Commit(); err != nil {
			t.Fatalf("Commit %d: %v", i, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	commits, err := index.ListCommits(dir)
	if err != nil {
		t.Fatalf("ListCommits: %v", err)
	}
	if len(commits) < 2 {
		t.Fatalf("need at least 2 commits, got %d", len(commits))
	}

	snapPolicy := index.NewSnapshotDeletionPolicy(index.NewKeepOnlyLastCommitDeletionPolicy())
	firstCommit := commits[0]
	gen, err := snapPolicy.Snapshot(firstCommit)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if gen != firstCommit.GetGeneration() {
		t.Errorf("snapshot gen = %d, want %d", gen, firstCommit.GetGeneration())
	}

	if err := snapPolicy.OnCommit(commits); err != nil {
		t.Fatalf("OnCommit: %v", err)
	}

	released := snapPolicy.Release(gen)
	if !released {
		t.Error("Release returned false for existing snapshot")
	}
	if snapPolicy.HasSnapshot(gen) {
		t.Error("HasSnapshot true after Release")
	}
}

// TestDeletionPolicy_ConcurrentSnapshot tests concurrent access to
// SnapshotDeletionPolicy.
func TestDeletionPolicy_ConcurrentSnapshot(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	addPolicyDoc(t, writer)
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	commits, err := index.ListCommits(dir)
	if err != nil {
		t.Fatalf("ListCommits: %v", err)
	}
	if len(commits) == 0 {
		t.Fatal("no commits")
	}

	snapPolicy := index.NewSnapshotDeletionPolicy(index.NewKeepAllDeletionPolicy())
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			gen, err := snapPolicy.Snapshot(commits[0])
			if err != nil {
				return
			}
			snapPolicy.Release(gen)
		}()
	}
	wg.Wait()
}

// TestDeletionPolicy_ListCommits verifies the ListCommits function.
func TestDeletionPolicy_ListCommits(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	if _, err := index.ListCommits(dir); err == nil {
		t.Fatal("ListCommits should error on empty index")
	}

	for i := 0; i < 3; i++ {
		addPolicyDoc(t, writer)
		if err := writer.Commit(); err != nil {
			t.Fatalf("Commit %d: %v", i, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	commits, err := index.ListCommits(dir)
	if err != nil {
		t.Fatalf("ListCommits: %v", err)
	}
	if len(commits) != 3 {
		t.Fatalf("expected 3 commits, got %d", len(commits))
	}
	verifyCommitOrder(t, commits)

	for _, c := range commits {
		if c.GetSegmentsFileName() == "" {
			t.Error("commit has empty segments file name")
		}
		if c.GetGeneration() <= 0 {
			t.Errorf("commit gen=%d should be positive", c.GetGeneration())
		}
	}
}

// TestDeletionPolicy_DeleteOnEmpty verifies Delete fails without a directory.
func TestDeletionPolicy_DeleteOnEmpty(t *testing.T) {
	si := index.NewSegmentInfos()
	commit := index.NewIndexCommit(si)
	err := commit.Delete()
	if err == nil {
		t.Fatal("expected error deleting commit with nil directory")
	}
	if !strings.Contains(err.Error(), "directory not set") {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestDeletionPolicy_KeepAllClone verifies Clone works.
func TestDeletionPolicy_KeepAllClone(t *testing.T) {
	original := index.NewKeepAllDeletionPolicy()
	clone := original.Clone()
	if _, ok := clone.(*index.KeepAllDeletionPolicy); !ok {
		t.Errorf("Clone returned %T, want *index.KeepAllDeletionPolicy", clone)
	}
	if err := clone.OnCommit(nil); err != nil {
		t.Errorf("clone.OnCommit(nil): %v", err)
	}
}

// TestDeletionPolicy_IndexCommitUserData verifies user data round-trips.
func TestDeletionPolicy_IndexCommitUserData(t *testing.T) {
	si := index.NewSegmentInfos()
	ud := map[string]string{"key1": "val1", "key2": "val2"}
	si.SetUserData(ud)
	commit := index.NewIndexCommit(si)
	got := commit.GetUserData()
	if got["key1"] != "val1" || got["key2"] != "val2" {
		t.Errorf("GetUserData = %v, want %v", got, ud)
	}
}

// TestDeletionPolicy_IsDeleted verifies IsDeleted reflects file existence.
func TestDeletionPolicy_IsDeleted(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	addPolicyDoc(t, writer)
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	commits, err := index.ListCommits(dir)
	if err != nil {
		t.Fatalf("ListCommits: %v", err)
	}
	if len(commits) == 0 {
		t.Fatal("no commits")
	}

	c := commits[0]
	if c.IsDeleted() {
		t.Error("commit should not be deleted before Delete()")
	}
	if err := c.Delete(); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if !c.IsDeleted() {
		t.Error("commit should be deleted after Delete()")
	}
}

// TestDeletionPolicy_KeepLastNCommitsOnInit tests OnInit truncation.
func TestDeletionPolicy_KeepLastNCommitsOnInit(t *testing.T) {
	const numToKeep = 2

	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	for i := 0; i < 4; i++ {
		addPolicyDoc(t, writer)
		if err := writer.Commit(); err != nil {
			t.Fatalf("Commit %d: %v", i, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	policy := index.NewKeepLastNCommitsDeletionPolicy(numToKeep)
	allCommits, err := index.ListCommits(dir)
	if err != nil {
		t.Fatalf("ListCommits: %v", err)
	}
	if err := policy.OnInit(allCommits); err != nil {
		t.Fatalf("policy.OnInit: %v", err)
	}

	remaining, err := index.ListCommits(dir)
	if err != nil {
		t.Fatalf("ListCommits after OnInit: %v", err)
	}
	if len(remaining) != numToKeep {
		t.Errorf("OnInit survivors = %d, want %d", len(remaining), numToKeep)
	}
}

// TestDeletionPolicy_IndexCommitCloneEquality tests Equals.
func TestDeletionPolicy_IndexCommitCloneEquality(t *testing.T) {
	si := index.NewSegmentInfos()
	c1 := index.NewIndexCommit(si)
	c2 := index.NewIndexCommit(si)
	if !c1.Equals(c2) {
		t.Error("two commits from same SegmentInfos should be equal")
	}
	if c1.Equals(nil) {
		t.Error("Equals(nil) should return false")
	}
}

// TestDeletionPolicy_BaseOnCommitError verifies Base returns errors.
func TestDeletionPolicy_BaseOnCommitError(t *testing.T) {
	base := &index.BaseIndexDeletionPolicy{}
	if err := base.OnCommit(nil); err == nil {
		t.Error("BaseIndexDeletionPolicy.OnCommit should return error")
	}
	if err := base.OnInit(nil); err == nil {
		t.Error("BaseIndexDeletionPolicy.OnInit should return error")
	}
	if cloned := base.Clone(); cloned != nil {
		t.Error("BaseIndexDeletionPolicy.Clone should return nil")
	}
}

// TestDeletionPolicy_CommitGeneration verifies generation increases.
func TestDeletionPolicy_CommitGeneration(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	addPolicyDoc(t, writer)
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit 1: %v", err)
	}
	addPolicyDoc(t, writer)
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit 2: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	commits, err := index.ListCommits(dir)
	if err != nil {
		t.Fatalf("ListCommits: %v", err)
	}
	if len(commits) >= 2 && commits[1].GetGeneration() <= commits[0].GetGeneration() {
		t.Error("later commit should have higher generation")
	}
}

// TestDeletionPolicy_FilterDeletedCommits verifies the helper.
func TestDeletionPolicy_FilterDeletedCommits(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	for i := 0; i < 3; i++ {
		addPolicyDoc(t, writer)
		if err := writer.Commit(); err != nil {
			t.Fatalf("Commit %d: %v", i, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	allCommits, err := index.ListCommits(dir)
	if err != nil {
		t.Fatalf("ListCommits: %v", err)
	}
	if len(allCommits) < 2 {
		t.Fatalf("need >= 2 commits, got %d", len(allCommits))
	}
	if err := allCommits[0].Delete(); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	filtered := index.FilterDeletedCommits(allCommits)
	if len(filtered) != len(allCommits)-1 {
		t.Errorf("FilterDeletedCommits returned %d, want %d", len(filtered), len(allCommits)-1)
	}
}

// TestDeletionPolicy_DeleteCommits verifies the helper.
func TestDeletionPolicy_DeleteCommits(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	addPolicyDoc(t, writer)
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	allCommits, err := index.ListCommits(dir)
	if err != nil {
		t.Fatalf("ListCommits: %v", err)
	}
	if len(allCommits) == 0 {
		t.Fatal("no commits")
	}
	if err := index.DeleteCommits(allCommits); err != nil {
		t.Errorf("DeleteCommits: %v", err)
	}
}

// TestDeletionPolicy_OpenDirectoryReaderFromCommit verifies reader from commit.
func TestDeletionPolicy_OpenDirectoryReaderFromCommit(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	addPolicyDoc(t, writer)
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	addPolicyDoc(t, writer)
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit 2: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	commits, err := index.ListCommits(dir)
	if err != nil {
		t.Fatalf("ListCommits: %v", err)
	}
	if len(commits) == 0 {
		t.Fatal("no commits")
	}

	reader, err := index.OpenDirectoryReaderFromCommit(dir, commits[len(commits)-1])
	if err != nil {
		t.Fatalf("OpenDirectoryReaderFromCommit: %v", err)
	}
	reader.Close()
}

// TestDeletionPolicy_KeepLastNGetNumCommitsToKeep verifies the accessor.
func TestDeletionPolicy_KeepLastNGetNumCommitsToKeep(t *testing.T) {
	p := index.NewKeepLastNCommitsDeletionPolicy(5)
	if got := p.GetNumCommitsToKeep(); got != 5 {
		t.Errorf("GetNumCommitsToKeep = %d, want 5", got)
	}
}

// TestDeletionPolicy_String tests the String() method of concrete policy types.
func TestDeletionPolicy_String(t *testing.T) {
	if s := index.NewKeepAllDeletionPolicy().String(); s == "" {
		t.Error("KeepAllDeletionPolicy.String() returned empty")
	}
	if s := index.NewKeepOnlyLastCommitDeletionPolicy().String(); s == "" {
		t.Error("KeepOnlyLastCommitDeletionPolicy.String() returned empty")
	}
	if s := index.NewKeepLastNCommitsDeletionPolicy(5).String(); s == "" {
		t.Error("KeepLastNCommitsDeletionPolicy.String() returned empty")
	}
}

// TestDeletionPolicy_EmptyIndex verifies behavior on an empty index.
func TestDeletionPolicy_EmptyIndex(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	if _, err := index.ListCommits(dir); err == nil {
		t.Error("ListCommits on empty dir should error")
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader on empty dir: %v", err)
	}
	if n := reader.NumDocs(); n != 0 {
		t.Errorf("NumDocs = %d, want 0", n)
	}
	reader.Close()

	if _, err := index.OpenDirectoryReaderFromCommit(dir, nil); err == nil {
		t.Error("OpenDirectoryReaderFromCommit(nil) should error")
	}
}

// TestDeletionPolicy_FindCommitByGeneration tests the helper.
func TestDeletionPolicy_FindCommitByGeneration(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	addPolicyDoc(t, writer)
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	addPolicyDoc(t, writer)
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	commits, err := index.ListCommits(dir)
	if err != nil {
		t.Fatalf("ListCommits: %v", err)
	}
	for _, c := range commits {
		found := index.FindCommitByGeneration(commits, c.GetGeneration())
		if found == nil || found.GetGeneration() != c.GetGeneration() {
			t.Errorf("FindCommitByGeneration(%d) returned wrong commit", c.GetGeneration())
		}
	}
	if found := index.FindCommitByGeneration(commits, 99999); found != nil {
		t.Error("FindCommitByGeneration(99999) should return nil")
	}
}
