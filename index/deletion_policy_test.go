// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/index/TestDeletionPolicy.java
// (Apache Lucene 10.5.0). The @Nightly testExpirationTimeDeletionPolicy lives
// in deletion_policy_monster_test.go.

package index_test

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// deletionPolicyVerifyCommitOrder renders the private
// verifyCommitOrder(List<? extends IndexCommit>).
func deletionPolicyVerifyCommitOrder(commits []index.Commit) error {
	if len(commits) == 0 {
		return nil
	}
	firstCommit := commits[0]
	last := index.GenerationFromSegmentsFileName(firstCommit.GetSegmentsFileName())
	if last != firstCommit.GetGeneration() {
		return fmt.Errorf("generation: expected %d, got %d", last, firstCommit.GetGeneration())
	}
	for i := 1; i < len(commits); i++ {
		commit := commits[i]
		now := index.GenerationFromSegmentsFileName(commit.GetSegmentsFileName())
		if !(now > last) {
			return fmt.Errorf("SegmentInfos commits are out-of-order")
		}
		if now != commit.GetGeneration() {
			return fmt.Errorf("generation: expected %d, got %d", now, commit.GetGeneration())
		}
		last = now
	}
	return nil
}

// deletionPolicyKeepAll is the inner KeepAllDeletionPolicy.
type deletionPolicyKeepAll struct {
	numOnInit   int
	numOnCommit int
	dir         store.Directory
}

func (p *deletionPolicyKeepAll) OnInit(commits []index.Commit) error {
	if err := deletionPolicyVerifyCommitOrder(commits); err != nil {
		return err
	}
	p.numOnInit++
	return nil
}

func (p *deletionPolicyKeepAll) OnCommit(commits []index.Commit) error {
	lastCommit := commits[len(commits)-1]
	r, err := index.OpenDirectoryReader(p.dir)
	if err != nil {
		return err
	}
	leaves, err := r.Leaves()
	if err != nil {
		return err
	}
	if len(leaves) != lastCommit.GetSegmentCount() {
		return fmt.Errorf("lastCommit.segmentCount()=%d vs IndexReader.segmentCount=%d", lastCommit.GetSegmentCount(), len(leaves))
	}
	if err := r.Close(); err != nil {
		return err
	}
	if err := deletionPolicyVerifyCommitOrder(commits); err != nil {
		return err
	}
	p.numOnCommit++
	return nil
}

// Clone carries IndexDeletionPolicy's Go-only Clone member.
func (p *deletionPolicyKeepAll) Clone() index.IndexDeletionPolicy { return p }

// deletionPolicyKeepNoneOnInit is the inner KeepNoneOnInitDeletionPolicy:
// useful for adding to a big index when you know readers are not using it.
type deletionPolicyKeepNoneOnInit struct {
	numOnInit   int
	numOnCommit int
}

func (p *deletionPolicyKeepNoneOnInit) OnInit(commits []index.Commit) error {
	if err := deletionPolicyVerifyCommitOrder(commits); err != nil {
		return err
	}
	p.numOnInit++
	// On init, delete all commit points:
	for _, commit := range commits {
		if err := commit.Delete(); err != nil {
			return err
		}
		if !commit.IsDeleted() {
			return fmt.Errorf("assertTrue(commit.isDeleted())")
		}
	}
	return nil
}

func (p *deletionPolicyKeepNoneOnInit) OnCommit(commits []index.Commit) error {
	if err := deletionPolicyVerifyCommitOrder(commits); err != nil {
		return err
	}
	size := len(commits)
	// Delete all but last one:
	for i := 0; i < size-1; i++ {
		if err := commits[i].Delete(); err != nil {
			return err
		}
	}
	p.numOnCommit++
	return nil
}

func (p *deletionPolicyKeepNoneOnInit) Clone() index.IndexDeletionPolicy { return p }

// deletionPolicyKeepLastN is the inner KeepLastNDeletionPolicy.
type deletionPolicyKeepLastN struct {
	numOnInit   int
	numOnCommit int
	numToKeep   int
	numDelete   int
	seen        map[string]bool
}

func newDeletionPolicyKeepLastN(numToKeep int) *deletionPolicyKeepLastN {
	return &deletionPolicyKeepLastN{numToKeep: numToKeep, seen: map[string]bool{}}
}

func (p *deletionPolicyKeepLastN) OnInit(commits []index.Commit) error {
	if err := deletionPolicyVerifyCommitOrder(commits); err != nil {
		return err
	}
	p.numOnInit++
	// do no deletions on init
	return p.doDeletes(commits, false)
}

func (p *deletionPolicyKeepLastN) OnCommit(commits []index.Commit) error {
	if err := deletionPolicyVerifyCommitOrder(commits); err != nil {
		return err
	}
	return p.doDeletes(commits, true)
}

func (p *deletionPolicyKeepLastN) doDeletes(commits []index.Commit, isCommit bool) error {
	// Assert that we really are only called for each new
	// commit:
	if isCommit {
		fileName := commits[len(commits)-1].GetSegmentsFileName()
		if p.seen[fileName] {
			return fmt.Errorf("onCommit was called twice on the same commit point: %s", fileName)
		}
		p.seen[fileName] = true
		p.numOnCommit++
	}
	size := len(commits)
	for i := 0; i < size-p.numToKeep; i++ {
		if err := commits[i].Delete(); err != nil {
			return err
		}
		p.numDelete++
	}
	return nil
}

func (p *deletionPolicyKeepLastN) Clone() index.IndexDeletionPolicy { return p }

// getCommitTime renders the package-private static getCommitTime(IndexCommit).
func getCommitTime(commit index.Commit) (int64, error) {
	return strconv.ParseInt(commit.GetUserData()["commitTime"], 10, 64)
}

func setNoCFSRatio(conf *index.IndexWriterConfig, ratio float64) {
	conf.GetMergePolicy().(interface{ SetNoCFSRatio(float64) }).SetNoCFSRatio(ratio)
}

// lastCommitGeneration renders SegmentInfos.getLastCommitGeneration(Directory).
func lastCommitGeneration(t testing.TB, dir store.Directory) int64 {
	t.Helper()
	files, err := dir.ListAll()
	if err != nil {
		t.Fatalf("listAll: %v", err)
	}
	return spi.GetLastCommitGeneration(files)
}

func deleteSegmentsFile(t testing.TB, dir store.Directory, gen int64) {
	t.Helper()
	if err := dir.DeleteFile(index.FileNameFromGeneration(index.SegmentsPrefix, "", gen)); err != nil {
		t.Fatalf("deleteFile(segments gen %d): %v", gen, err)
	}
}

func mustListCommits(t testing.TB, dir store.Directory) index.IndexCommitList {
	t.Helper()
	commits, err := index.ListCommits(dir)
	if err != nil {
		t.Fatalf("DirectoryReader.listCommits: %v", err)
	}
	return commits
}

func assertCommitCount(t testing.TB, expected int, dir store.Directory) {
	t.Helper()
	if n := len(mustListCommits(t, dir)); n != expected {
		t.Fatalf("listCommits(dir).size(): expected %d, got %d", expected, n)
	}
}

func listAllCount(t testing.TB, dir store.Directory) int {
	t.Helper()
	files, err := dir.ListAll()
	if err != nil {
		t.Fatalf("listAll: %v", err)
	}
	return len(files)
}

// Test a silly deletion policy that keeps all commits around.
func TestDeletionPolicyKeepAllDeletionPolicy(t *testing.T) {
	for pass := 0; pass < 2; pass++ {
		useCompoundFile := pass%2 != 0

		dir := newDirectory()

		conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
		conf.SetIndexDeletionPolicy(&deletionPolicyKeepAll{dir: dir})
		conf.SetMaxBufferedDocs(10)
		conf.SetMergeScheduler(index.NewSerialMergeScheduler())
		setNoCFSRatio(conf, map[bool]float64{true: 1.0, false: 0.0}[useCompoundFile])
		writer := mustNewIndexWriter(t, dir, conf)
		policy := writer.GetConfig().GetIndexDeletionPolicy().(*deletionPolicyKeepAll)
		for i := 0; i < 107; i++ {
			deletionPolicyAddDoc(t, writer)
		}
		mustClose(t, writer)

		var needsMerging bool
		{
			r := mustOpenDirectoryReader(t, dir)
			leaves, err := r.Leaves()
			if err != nil {
				t.Fatalf("leaves: %v", err)
			}
			needsMerging = len(leaves) != 1
			mustClose(t, r)
		}
		if needsMerging {
			conf = newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
			conf.SetOpenMode(index.Append)
			conf.SetIndexDeletionPolicy(policy)
			setNoCFSRatio(conf, map[bool]float64{true: 1.0, false: 0.0}[useCompoundFile])
			writer = mustNewIndexWriter(t, dir, conf)
			policy = writer.GetConfig().GetIndexDeletionPolicy().(*deletionPolicyKeepAll)
			if err := writer.ForceMerge(1); err != nil {
				t.Fatalf("forceMerge: %v", err)
			}
			mustClose(t, writer)
		}

		expectedInits := 1
		if needsMerging {
			expectedInits = 2
		}
		if policy.numOnInit != expectedInits {
			t.Fatalf("numOnInit: expected %d, got %d", expectedInits, policy.numOnInit)
		}

		// If we are not auto committing then there should
		// be exactly 2 commits (one per close above):
		if policy.numOnCommit != expectedInits {
			t.Fatalf("numOnCommit: expected %d, got %d", expectedInits, policy.numOnCommit)
		}

		// Test listCommits
		commits := mustListCommits(t, dir)
		// 2 from closing writer
		if len(commits) != expectedInits {
			t.Fatalf("commits.size(): expected %d, got %d", expectedInits, len(commits))
		}

		// Make sure we can open a reader on each commit:
		for _, commit := range commits {
			r, err := index.OpenDirectoryReaderAtCommit(commit)
			if err != nil {
				t.Fatalf("DirectoryReader.open(commit): %v", err)
			}
			mustClose(t, r)
		}

		// Simplistic check: just verify all segments_N's still
		// exist, and, I can open a reader on each:
		gen := lastCommitGeneration(t, dir)
		for gen > 0 {
			mustClose(t, mustOpenDirectoryReader(t, dir))
			deleteSegmentsFile(t, dir, gen)
			gen--

			if gen > 0 {
				// Now that we've removed a commit point, which
				// should have orphan'd at least one index file.
				// Open & close a writer and assert that it
				// actually removed something:
				preCount := listAllCount(t, dir)
				conf = newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
				conf.SetOpenMode(index.Append)
				conf.SetIndexDeletionPolicy(policy)
				mustClose(t, mustNewIndexWriter(t, dir, conf))
				postCount := listAllCount(t, dir)
				if !(postCount < preCount) {
					t.Fatalf("assertTrue(postCount < preCount): %d, %d", postCount, preCount)
				}
			}
		}

		mustClose(t, dir)
	}
}

// Uses KeepAllDeletionPolicy to keep all commits around, then, opens a new
// IndexWriter on a previous commit point.
func TestDeletionPolicyOpenPriorSnapshot(t *testing.T) {
	dir := newDirectory()

	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetIndexDeletionPolicy(&deletionPolicyKeepAll{dir: dir})
	conf.SetMaxBufferedDocs(2)
	conf.SetMergePolicy(newLogMergePolicyWithMergeFactor(10))
	writer := mustNewIndexWriter(t, dir, conf)
	policy := writer.GetConfig().GetIndexDeletionPolicy().(*deletionPolicyKeepAll)
	for i := 0; i < 10; i++ {
		deletionPolicyAddDoc(t, writer)
		if (1+i)%2 == 0 {
			mustCommit(t, writer)
		}
	}
	mustClose(t, writer)

	commits := mustListCommits(t, dir)
	if len(commits) != 5 {
		t.Fatalf("commits.size(): expected 5, got %d", len(commits))
	}
	var lastCommit *index.IndexCommit
	for _, commit := range commits {
		if lastCommit == nil || commit.GetGeneration() > lastCommit.GetGeneration() {
			lastCommit = commit
		}
	}
	if lastCommit == nil {
		t.Fatal("assertTrue(lastCommit != null)")
	}

	// Now add 1 doc and merge
	conf = newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetIndexDeletionPolicy(policy)
	writer = mustNewIndexWriter(t, dir, conf)
	deletionPolicyAddDoc(t, writer)
	assertWriterDocStats(t, writer, -1, 11)
	if err := writer.ForceMerge(1); err != nil {
		t.Fatalf("forceMerge: %v", err)
	}
	mustClose(t, writer)

	assertCommitCount(t, 6, dir)

	priorCommitWriter := func() *index.IndexWriter {
		conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
		conf.SetIndexDeletionPolicy(policy)
		conf.SetIndexCommit(lastCommit)
		conf.SetMergePolicy(newLogMergePolicyWithMergeFactor(10))
		return mustNewIndexWriter(t, dir, conf)
	}

	// Now open writer on the commit just before merge:
	writer = priorCommitWriter()
	assertWriterDocStats(t, writer, -1, 10)

	// Should undo our rollback:
	if err := writer.Rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}

	assertLeavesAndDocs(t, dir, 1, 11)

	writer = priorCommitWriter()
	assertWriterDocStats(t, writer, -1, 10)
	// Commits the rollback:
	mustClose(t, writer)

	// Now 7 because we made another commit
	assertCommitCount(t, 7, dir)

	// Not fully merged because we rolled it back, and now only
	// 10 docs
	assertLeavesAndDocs(t, dir, -2, 10)

	// Re-merge
	conf = newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetIndexDeletionPolicy(policy)
	writer = mustNewIndexWriter(t, dir, conf)
	if err := writer.ForceMerge(1); err != nil {
		t.Fatalf("forceMerge: %v", err)
	}
	mustClose(t, writer)

	assertLeavesAndDocs(t, dir, 1, 10)

	// Now open writer on the commit just before merging,
	// but this time keeping only the last commit:
	conf = newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetIndexCommit(lastCommit)
	conf.SetMergePolicy(newLogMergePolicyWithMergeFactor(10))
	writer = mustNewIndexWriter(t, dir, conf)
	assertWriterDocStats(t, writer, -1, 10)

	// Reader still sees fully merged index, because writer
	// opened on the prior commit has not yet committed:
	assertLeavesAndDocs(t, dir, 1, 10)

	mustClose(t, writer)

	// Now reader sees not-fully-merged index:
	assertLeavesAndDocs(t, dir, -2, 10)

	mustClose(t, dir)
}

// assertLeavesAndDocs opens a reader on dir and asserts its leaf count
// (exactly, or > 1 when leaves is -2) and, unless numDocs is negative, its
// numDocs.
func assertLeavesAndDocs(t testing.TB, dir store.Directory, leaves, numDocs int) {
	t.Helper()
	r := mustOpenDirectoryReader(t, dir)
	defer mustClose(t, r)
	got, err := r.Leaves()
	if err != nil {
		t.Fatalf("leaves: %v", err)
	}
	if leaves == -2 {
		if !(len(got) > 1) {
			t.Fatalf("assertTrue(r.leaves().size() > 1): %d", len(got))
		}
	} else if len(got) != leaves {
		t.Fatalf("leaves().size(): expected %d, got %d", leaves, len(got))
	}
	if numDocs >= 0 && r.NumDocs() != numDocs {
		t.Fatalf("numDocs: expected %d, got %d", numDocs, r.NumDocs())
	}
}

// Test keeping NO commit points. This is a viable and useful case eg where
// you want to build a big index and you know there are no readers.
func TestDeletionPolicyKeepNoneOnInitDeletionPolicy(t *testing.T) {
	for pass := 0; pass < 2; pass++ {
		useCompoundFile := pass%2 != 0

		dir := newDirectory()

		conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
		conf.SetOpenMode(index.Create)
		conf.SetIndexDeletionPolicy(&deletionPolicyKeepNoneOnInit{})
		conf.SetMaxBufferedDocs(10)
		setNoCFSRatio(conf, map[bool]float64{true: 1.0, false: 0.0}[useCompoundFile])
		writer := mustNewIndexWriter(t, dir, conf)
		policy := writer.GetConfig().GetIndexDeletionPolicy().(*deletionPolicyKeepNoneOnInit)
		for i := 0; i < 107; i++ {
			deletionPolicyAddDoc(t, writer)
		}
		mustClose(t, writer)

		conf = newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
		conf.SetOpenMode(index.Append)
		conf.SetIndexDeletionPolicy(policy)
		setNoCFSRatio(conf, 1.0)
		writer = mustNewIndexWriter(t, dir, conf)
		policy = writer.GetConfig().GetIndexDeletionPolicy().(*deletionPolicyKeepNoneOnInit)
		if err := writer.ForceMerge(1); err != nil {
			t.Fatalf("forceMerge: %v", err)
		}
		mustClose(t, writer)

		if policy.numOnInit != 2 {
			t.Fatalf("numOnInit: expected 2, got %d", policy.numOnInit)
		}
		// If we are not auto committing then there should
		// be exactly 2 commits (one per close above):
		if policy.numOnCommit != 2 {
			t.Fatalf("numOnCommit: expected 2, got %d", policy.numOnCommit)
		}

		// Simplistic check: just verify the index is in fact
		// readable:
		mustClose(t, mustOpenDirectoryReader(t, dir))

		mustClose(t, dir)
	}
}

// Test a deletion policy that keeps last N commits.
func TestDeletionPolicyKeepLastNDeletionPolicy(t *testing.T) {
	const n = 5

	for pass := 0; pass < 2; pass++ {
		useCompoundFile := pass%2 != 0

		dir := newDirectory()

		policy := newDeletionPolicyKeepLastN(n)
		for j := 0; j < n+1; j++ {
			conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
			conf.SetOpenMode(index.Create)
			conf.SetIndexDeletionPolicy(policy)
			conf.SetMaxBufferedDocs(10)
			setNoCFSRatio(conf, map[bool]float64{true: 1.0, false: 0.0}[useCompoundFile])
			writer := mustNewIndexWriter(t, dir, conf)
			policy = writer.GetConfig().GetIndexDeletionPolicy().(*deletionPolicyKeepLastN)
			for i := 0; i < 17; i++ {
				deletionPolicyAddDoc(t, writer)
			}
			if err := writer.ForceMerge(1); err != nil {
				t.Fatalf("forceMerge: %v", err)
			}
			mustClose(t, writer)
		}

		if !(policy.numDelete > 0) {
			t.Fatal("assertTrue(policy.numDelete > 0)")
		}
		if policy.numOnInit != n+1 {
			t.Fatalf("numOnInit: expected %d, got %d", n+1, policy.numOnInit)
		}
		if policy.numOnCommit != n+1 {
			t.Fatalf("numOnCommit: expected %d, got %d", n+1, policy.numOnCommit)
		}

		// Simplistic check: just verify only the past N segments_N's still
		// exist, and, I can open a reader on each:
		gen := lastCommitGeneration(t, dir)
		for i := 0; i < n+1; i++ {
			reader, err := index.OpenDirectoryReader(dir)
			if err == nil {
				mustClose(t, reader)
				if i == n {
					t.Fatalf("should have failed on commits prior to last %d", n)
				}
			} else if i != n {
				t.Fatalf("DirectoryReader.open: %v", err)
			}
			if i < n {
				deleteSegmentsFile(t, dir, gen)
			}
			gen--
		}

		mustClose(t, dir)
	}
}

// Test a deletion policy that keeps last N commits around, through creates.
func TestDeletionPolicyKeepLastNDeletionPolicyWithCreates(t *testing.T) {
	const n = 10

	for pass := 0; pass < 2; pass++ {
		useCompoundFile := pass%2 != 0

		dir := newDirectory()
		conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
		conf.SetOpenMode(index.Create)
		conf.SetIndexDeletionPolicy(newDeletionPolicyKeepLastN(n))
		conf.SetMaxBufferedDocs(10)
		setNoCFSRatio(conf, map[bool]float64{true: 1.0, false: 0.0}[useCompoundFile])
		writer := mustNewIndexWriter(t, dir, conf)
		policy := writer.GetConfig().GetIndexDeletionPolicy().(*deletionPolicyKeepLastN)
		mustClose(t, writer)
		searchTerm := index.NewTerm("content", "aaa")
		query := search.NewTermQuery(searchTerm)

		for i := 0; i < n+1; i++ {
			conf = newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
			conf.SetOpenMode(index.Append)
			conf.SetIndexDeletionPolicy(policy)
			conf.SetMaxBufferedDocs(10)
			setNoCFSRatio(conf, map[bool]float64{true: 1.0, false: 0.0}[useCompoundFile])
			writer = mustNewIndexWriter(t, dir, conf)
			policy = writer.GetConfig().GetIndexDeletionPolicy().(*deletionPolicyKeepLastN)
			for j := 0; j < 17; j++ {
				deletionPolicyAddDocWithID(t, writer, i*(n+1)+j)
			}
			// this is a commit
			mustClose(t, writer)
			conf = index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer())
			conf.SetIndexDeletionPolicy(policy)
			conf.SetMergePolicy(index.NewNoMergePolicy())
			writer = mustNewIndexWriter(t, dir, conf)
			policy = writer.GetConfig().GetIndexDeletionPolicy().(*deletionPolicyKeepLastN)
			mustDeleteTerm(t, writer, "id", strconv.Itoa(i*(n+1)+3))
			// this is a commit
			mustClose(t, writer)
			reader := mustOpenDirectoryReader(t, dir)
			searcher := newSearcher(t, reader)
			assertHitCount(t, searcher, query, 16)
			mustClose(t, reader)

			conf = newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
			conf.SetOpenMode(index.Create)
			conf.SetIndexDeletionPolicy(policy)
			writer = mustNewIndexWriter(t, dir, conf)
			policy = writer.GetConfig().GetIndexDeletionPolicy().(*deletionPolicyKeepLastN)
			// This will not commit: there are no changes
			// pending because we opened for "create":
			mustClose(t, writer)
		}

		if policy.numOnInit != 3*(n+1)+1 {
			t.Fatalf("numOnInit: expected %d, got %d", 3*(n+1)+1, policy.numOnInit)
		}
		if policy.numOnCommit != 3*(n+1)+1 {
			t.Fatalf("numOnCommit: expected %d, got %d", 3*(n+1)+1, policy.numOnCommit)
		}

		rwReader := mustOpenDirectoryReader(t, dir)
		searcher := newSearcher(t, rwReader)
		assertHitCount(t, searcher, query, 0)

		// Simplistic check: just verify only the past N segments_N's still
		// exist, and, I can open a reader on each:
		gen := lastCommitGeneration(t, dir)

		expectedCount := 0

		mustClose(t, rwReader)

		for i := 0; i < n+1; i++ {
			reader, err := index.OpenDirectoryReader(dir)
			if err == nil {
				// Work backwards in commits on what the expected
				// count should be.
				searcher = newSearcher(t, reader)
				assertHitCount(t, searcher, query, expectedCount)
				if expectedCount == 0 {
					expectedCount = 16
				} else if expectedCount == 16 {
					expectedCount = 17
				} else if expectedCount == 17 {
					expectedCount = 0
				}
				mustClose(t, reader)
				if i == n {
					t.Fatalf("should have failed on commits before last %d", n)
				}
			} else if i != n {
				t.Fatalf("DirectoryReader.open: %v", err)
			}
			if i < n {
				deleteSegmentsFile(t, dir, gen)
			}
			gen--
		}

		mustClose(t, dir)
	}
}

// assertHitCount renders assertEquals(expected, searcher.search(query,
// 1000).scoreDocs.length).
func assertHitCount(t testing.TB, searcher *search.IndexSearcher, query search.Query, expected int) {
	t.Helper()
	hits, err := searcher.Search(query, 1000)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(hits.ScoreDocs) != expected {
		t.Fatalf("hits.length: expected %d, got %d", expected, len(hits.ScoreDocs))
	}
}

func TestDeletionPolicyKeepLastNCommitsDeletionPolicy(t *testing.T) {
	numCommitsToKeep := 3
	conf := index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetIndexDeletionPolicy(index.NewKeepLastNCommitsDeletionPolicy(numCommitsToKeep))

	if _, ok := conf.GetIndexDeletionPolicy().(*index.KeepLastNCommitsDeletionPolicy); !ok {
		t.Fatalf("getIndexDeletionPolicy().getClass(): got %T", conf.GetIndexDeletionPolicy())
	}

	// Create an index and make several commits
	dir := newDirectory()
	writer := mustNewIndexWriter(t, dir, conf)

	for i := 0; i < 5; i++ {
		deletionPolicyAddDoc(t, writer)
		mustCommit(t, writer)
	}

	mustClose(t, writer)

	// Check that only the last N commits are kept
	commits := mustListCommits(t, dir)
	if len(commits) != numCommitsToKeep {
		t.Fatalf("commits.size(): expected %d, got %d", numCommitsToKeep, len(commits))
	}

	// Verify that we can open and read from each of the remaining commits
	for _, commit := range commits {
		reader, err := index.OpenDirectoryReaderAtCommit(commit)
		if err != nil {
			t.Fatalf("DirectoryReader.open(commit): %v", err)
		}
		if !(reader.NumDocs() > 0) {
			t.Fatalf("assertTrue(reader.numDocs() > 0): %d", reader.NumDocs())
		}
		mustClose(t, reader)
	}

	// Check that the retained commits are the most recent ones
	latestGen := commits[len(commits)-1].GetGeneration()
	for i := 0; i < numCommitsToKeep; i++ {
		if got := commits[len(commits)-1-i].GetGeneration(); got != latestGen-int64(i) {
			t.Fatalf("generation: expected %d, got %d", latestGen-int64(i), got)
		}
	}

	mustClose(t, dir)
}

func TestDeletionPolicyKeepLastNCommitsDeletionPolicyWithZeroCommits(t *testing.T) {
	numCommitsToKeep := 0
	conf := index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	var message string
	func() {
		defer func() {
			if r := recover(); r != nil {
				message = fmt.Sprint(r)
			}
		}()
		conf.SetIndexDeletionPolicy(index.NewKeepLastNCommitsDeletionPolicy(numCommitsToKeep))
	}()
	if message == "" {
		t.Fatal("expected IllegalArgumentException")
	}
	if !strings.Contains(message, "number of recent commits to keep must be positive") {
		t.Fatalf("message: %q", message)
	}
}

func deletionPolicyAddDocWithID(t testing.TB, writer *index.IndexWriter, id int) {
	t.Helper()
	doc := document.NewDocument()
	doc.Add(newTextField(t, "content", "aaa", false))
	doc.Add(newStringField(t, "id", strconv.Itoa(id), false))
	mustAddDocument(t, writer, doc)
}

func deletionPolicyAddDoc(t testing.TB, writer *index.IndexWriter) {
	t.Helper()
	doc := document.NewDocument()
	doc.Add(newTextField(t, "content", "aaa", false))
	mustAddDocument(t, writer, doc)
}
