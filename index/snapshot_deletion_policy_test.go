// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/index/TestSnapshotDeletionPolicy.java
// (Apache Lucene 10.5.0).

package index_test

import (
	"sync"
	"testing"
	"time"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// indexWriterDeleteUnusedFilesMissing names the production member the Java
// tests call after releasing snapshots.
const indexWriterDeleteUnusedFilesMissing = "org.apache.lucene.index.IndexWriter#deleteUnusedFiles() is not ported"

// snapshotDeletionPolicyTest renders the instance state of
// TestSnapshotDeletionPolicy: the snapshots list and the read buffer.
type snapshotDeletionPolicyTest struct {
	t         *testing.T
	snapshots []index.Commit
	buffer    []byte
}

func newSnapshotDeletionPolicyTest(t *testing.T) *snapshotDeletionPolicyTest {
	return &snapshotDeletionPolicyTest{t: t, buffer: make([]byte, 4096)}
}

func (s *snapshotDeletionPolicyTest) getConfig(dp index.IndexDeletionPolicy) *index.IndexWriterConfig {
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	if dp != nil {
		conf.SetIndexDeletionPolicy(dp)
	}
	return conf
}

func (s *snapshotDeletionPolicyTest) checkSnapshotExists(dir store.Directory, c index.Commit) {
	s.t.Helper()
	segFileName := c.GetSegmentsFileName()
	if !slowFileExists(s.t, dir, segFileName) {
		s.t.Fatalf("segments file not found in directory: %s", segFileName)
	}
}

// openCommit renders DirectoryReader.open(IndexCommit) over the commit handed
// out by SnapshotDeletionPolicy.
func openCommit(t *testing.T, c index.Commit) *index.DirectoryReader {
	t.Helper()
	ic, ok := c.(*index.IndexCommit)
	if !ok {
		t.Fatalf("DirectoryReader.open(IndexCommit) is not ported for commit type %T", c)
	}
	reader, err := index.OpenDirectoryReaderAtCommit(ic)
	if err != nil {
		t.Fatalf("DirectoryReader.open(commit): %v", err)
	}
	return reader
}

func (s *snapshotDeletionPolicyTest) checkMaxDoc(commit index.Commit, expectedMaxDoc int) {
	s.t.Helper()
	reader := openCommit(s.t, commit)
	defer mustClose(s.t, reader)
	if got := reader.MaxDoc(); got != expectedMaxDoc {
		s.t.Fatalf("maxDoc: expected %d, got %d", expectedMaxDoc, got)
	}
}

func (s *snapshotDeletionPolicyTest) prepareIndexAndSnapshots(sdp *index.SnapshotDeletionPolicy, writer *index.IndexWriter, numSnapshots int) {
	s.t.Helper()
	for i := 0; i < numSnapshots; i++ {
		// create dummy document to trigger commit.
		mustAddDocument(s.t, writer, document.NewDocument())
		mustCommit(s.t, writer)
		snapshot, err := sdp.Snapshot()
		if err != nil {
			s.t.Fatalf("snapshot: %v", err)
		}
		s.snapshots = append(s.snapshots, snapshot)
	}
}

func (s *snapshotDeletionPolicyTest) getDeletionPolicy() *index.SnapshotDeletionPolicy {
	return index.NewSnapshotDeletionPolicy(index.NewKeepOnlyLastCommitDeletionPolicy())
}

func (s *snapshotDeletionPolicyTest) assertSnapshotExists(dir store.Directory, sdp *index.SnapshotDeletionPolicy, numSnapshots int, checkIndexCommitSame bool) {
	s.t.Helper()
	for i := 0; i < numSnapshots; i++ {
		snapshot := s.snapshots[i]
		s.checkMaxDoc(snapshot, i+1)
		s.checkSnapshotExists(dir, snapshot)
		if checkIndexCommitSame {
			if got := sdp.GetIndexCommit(snapshot.GetGeneration()); got != snapshot {
				s.t.Fatalf("assertSame(snapshot, sdp.getIndexCommit(gen)): %v != %v", snapshot, got)
			}
		} else {
			got := sdp.GetIndexCommit(snapshot.GetGeneration())
			if got == nil || got.GetGeneration() != snapshot.GetGeneration() {
				s.t.Fatalf("sdp.getIndexCommit(%d) = %v", snapshot.GetGeneration(), got)
			}
		}
	}
}

func snapshotPolicyOf(t *testing.T, writer *index.IndexWriter) *index.SnapshotDeletionPolicy {
	t.Helper()
	sdp, ok := writer.GetConfig().GetIndexDeletionPolicy().(*index.SnapshotDeletionPolicy)
	if !ok {
		t.Fatalf("(SnapshotDeletionPolicy) writer.getConfig().getIndexDeletionPolicy(): got %T",
			writer.GetConfig().GetIndexDeletionPolicy())
	}
	return sdp
}

func TestSnapshotDeletionPolicySnapshotDeletionPolicy(t *testing.T) {
	s := newSnapshotDeletionPolicyTest(t)
	fsDir := newDirectory()
	s.runTest(fsDir)
	mustClose(t, fsDir)
}

func snapshotContentDocument(t testing.TB) *document.Document {
	doc := document.NewDocument()
	customType := document.NewFieldTypeFrom(document.TextFieldTypeStored)
	customType.SetStoreTermVectors(true)
	customType.SetStoreTermVectorPositions(true)
	customType.SetStoreTermVectorOffsets(true)
	doc.Add(newField(t, "content", "aaa", customType))
	return doc
}

func (s *snapshotDeletionPolicyTest) runTest(dir store.Directory) {
	t := s.t
	maxIterations := 10
	if testNightly {
		maxIterations = 100
	}

	dp := s.getDeletionPolicy()
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetIndexDeletionPolicy(dp)
	conf.SetMaxBufferedDocs(2)
	writer := mustNewIndexWriter(t, dir, conf)

	// Verify we catch misuse:
	if _, err := dp.Snapshot(); err == nil {
		t.Fatal("expected IllegalStateException from snapshot() before the first commit")
	}

	mustCommit(t, writer)

	var wg sync.WaitGroup
	done := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(done)
		iterations := 0
		doc := snapshotContentDocument(t)
		for {
			for i := 0; i < 27; i++ {
				if _, err := writer.AddDocument(doc); err != nil {
					t.Errorf("addDocument failed: %v", err)
					return
				}
				if i%2 == 0 {
					if _, err := writer.Commit(); err != nil {
						t.Errorf("commit: %v", err)
						return
					}
				}
			}
			time.Sleep(time.Millisecond)
			iterations++
			if iterations >= maxIterations {
				return
			}
		}
	}()

	// While the above indexing thread is running, take many
	// backups:
	for alive := true; alive; {
		s.backupIndex(dir, dp)
		time.Sleep(20 * time.Millisecond)
		select {
		case <-done:
			alive = false
		default:
		}
	}

	wg.Wait()

	// Add one more document to force writer to commit a
	// final segment, so deletion policy has a chance to
	// delete again:
	mustAddDocument(t, writer, snapshotContentDocument(t))

	// Make sure we don't have any leftover files in the
	// directory:
	mustClose(t, writer)
	assertNoUnreferencedFiles(t, dir, "some files were not deleted but should have been")
}

// backupIndex is the example showing how to use the SnapshotDeletionPolicy to
// take a backup. It does not really do a backup; instead, it reads every byte
// of every file just to test that the files indeed exist and are readable
// even while the index is changing.
func (s *snapshotDeletionPolicyTest) backupIndex(dir store.Directory, dp *index.SnapshotDeletionPolicy) {
	// To backup an index we first take a snapshot:
	snapshot, err := dp.Snapshot()
	if err != nil {
		s.t.Fatalf("snapshot: %v", err)
	}
	defer func() {
		// Make sure to release the snapshot, otherwise these
		// files will never be deleted during this IndexWriter
		// session:
		if err := dp.Release(snapshot); err != nil {
			s.t.Fatalf("release: %v", err)
		}
	}()
	s.copyFiles(dir, snapshot)
}

func (s *snapshotDeletionPolicyTest) copyFiles(dir store.Directory, cp index.Commit) {
	// While we hold the snapshot, and nomatter how long
	// we take to do the backup, the IndexWriter will
	// never delete the files in the snapshot:
	files, err := cp.GetFileNames()
	if err != nil {
		s.t.Fatalf("getFileNames: %v", err)
	}
	for _, fileName := range files {
		// NOTE: in a real backup you would not use
		// readFile; you would need to use something else
		// that copies the file to a backup location.
		s.readFile(dir, fileName)
	}
}

func (s *snapshotDeletionPolicyTest) readFile(dir store.Directory, name string) {
	input, err := dir.OpenInput(name, store.IOContextReadOnce)
	if err != nil {
		s.t.Fatalf("openInput(%s): %v", name, err)
	}
	defer func() {
		if err := input.Close(); err != nil {
			s.t.Fatalf("close(%s): %v", name, err)
		}
	}()
	size, err := dir.FileLength(name)
	if err != nil {
		s.t.Fatalf("fileLength(%s): %v", name, err)
	}
	bytesLeft := size
	for bytesLeft > 0 {
		var numToRead int
		if bytesLeft < int64(len(s.buffer)) {
			numToRead = int(bytesLeft)
		} else {
			numToRead = len(s.buffer)
		}
		if err := input.ReadBytes(s.buffer, 0, numToRead); err != nil {
			s.t.Fatalf("readBytes(%s): %v", name, err)
		}
		bytesLeft -= int64(numToRead)
	}
	// Don't do this in your real backups!  This is just
	// to force a backup to take a somewhat long time, to
	// make sure we are exercising the fact that the
	// IndexWriter should not delete this file even when I
	// take my time reading it.
	time.Sleep(time.Millisecond)
}

func TestSnapshotDeletionPolicyBasicSnapshots(t *testing.T) {
	s := newSnapshotDeletionPolicyTest(t)
	numSnapshots := 3

	// Create 3 snapshots: snapshot0, snapshot1, snapshot2
	dir := newDirectory()
	writer := mustNewIndexWriter(t, dir, s.getConfig(s.getDeletionPolicy()))
	sdp := snapshotPolicyOf(t, writer)
	s.prepareIndexAndSnapshots(sdp, writer, numSnapshots)
	mustClose(t, writer)

	if got := len(sdp.GetSnapshots()); got != numSnapshots {
		t.Fatalf("getSnapshots().size(): expected %d, got %d", numSnapshots, got)
	}
	if got := sdp.GetSnapshotCount(); got != numSnapshots {
		t.Fatalf("getSnapshotCount(): expected %d, got %d", numSnapshots, got)
	}
	s.assertSnapshotExists(dir, sdp, numSnapshots, true)

	// open a reader on a snapshot - should succeed.
	mustClose(t, openCommit(t, s.snapshots[0]))

	// open a new IndexWriter w/ no snapshots to keep and assert that all snapshots are gone.
	sdp = s.getDeletionPolicy()
	writer = mustNewIndexWriter(t, dir, s.getConfig(sdp))
	defer mustClose(t, writer, dir)
	t.Fatal(indexWriterDeleteUnusedFilesMissing)
}

func TestSnapshotDeletionPolicyMultiThreadedSnapshotting(t *testing.T) {
	s := newSnapshotDeletionPolicyTest(t)
	dir := newDirectory()

	writer := mustNewIndexWriter(t, dir, s.getConfig(s.getDeletionPolicy()))
	sdp := snapshotPolicyOf(t, writer)

	const numThreads = 10
	snapshots := make([]index.Commit, numThreads)
	startingGun := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < numThreads; i++ {
		finalI := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-startingGun
			if _, err := writer.AddDocument(document.NewDocument()); err != nil {
				t.Errorf("t%d addDocument: %v", finalI, err)
				return
			}
			if _, err := writer.Commit(); err != nil {
				t.Errorf("t%d commit: %v", finalI, err)
				return
			}
			snapshot, err := sdp.Snapshot()
			if err != nil {
				t.Errorf("t%d snapshot: %v", finalI, err)
				return
			}
			snapshots[finalI] = snapshot
		}()
	}

	close(startingGun)
	wg.Wait()
	if t.Failed() {
		t.FailNow()
	}

	// Do one last commit, so that after we release all snapshots, we stay w/ one commit
	mustAddDocument(t, writer, document.NewDocument())
	mustCommit(t, writer)

	if err := sdp.Release(snapshots[0]); err != nil {
		t.Fatalf("release: %v", err)
	}
	defer mustClose(t, writer, dir)
	t.Fatal(indexWriterDeleteUnusedFilesMissing)
}

func TestSnapshotDeletionPolicyRollbackToOldSnapshot(t *testing.T) {
	s := newSnapshotDeletionPolicyTest(t)
	numSnapshots := 2
	dir := newDirectory()

	sdp := s.getDeletionPolicy()
	writer := mustNewIndexWriter(t, dir, s.getConfig(sdp))
	s.prepareIndexAndSnapshots(sdp, writer, numSnapshots)
	mustClose(t, writer)

	// now open the writer on "snapshot0" - make sure it succeeds
	snapshot0, ok := s.snapshots[0].(*index.IndexCommit)
	if !ok {
		t.Fatalf("IndexWriterConfig.setIndexCommit(IndexCommit) is not ported for commit type %T", s.snapshots[0])
	}
	writer = mustNewIndexWriter(t, dir, s.getConfig(sdp).SetIndexCommit(snapshot0))
	// this does the actual rollback
	mustCommit(t, writer)
	defer mustClose(t, writer, dir)
	t.Fatal(indexWriterDeleteUnusedFilesMissing)
}

func TestSnapshotDeletionPolicyReleaseSnapshot(t *testing.T) {
	s := newSnapshotDeletionPolicyTest(t)
	dir := newDirectory()
	writer := mustNewIndexWriter(t, dir, s.getConfig(s.getDeletionPolicy()))
	sdp := snapshotPolicyOf(t, writer)
	s.prepareIndexAndSnapshots(sdp, writer, 1)

	// Create another commit - we must do that, because otherwise the "snapshot"
	// files will still remain in the index, since it's the last commit.
	mustAddDocument(t, writer, document.NewDocument())
	mustCommit(t, writer)

	// Release
	if err := sdp.Release(s.snapshots[0]); err != nil {
		t.Fatalf("release: %v", err)
	}
	defer mustClose(t, writer, dir)
	t.Fatal(indexWriterDeleteUnusedFilesMissing)
}

func TestSnapshotDeletionPolicySnapshotLastCommitTwice(t *testing.T) {
	s := newSnapshotDeletionPolicyTest(t)
	dir := newDirectory()

	writer := mustNewIndexWriter(t, dir, s.getConfig(s.getDeletionPolicy()))
	sdp := snapshotPolicyOf(t, writer)
	mustAddDocument(t, writer, document.NewDocument())
	mustCommit(t, writer)

	s1, err := sdp.Snapshot()
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	s2, err := sdp.Snapshot()
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if s1 != s2 { // should be the same instance
		t.Fatalf("assertSame(s1, s2): %v != %v", s1, s2)
	}

	// create another commit
	mustAddDocument(t, writer, document.NewDocument())
	mustCommit(t, writer)

	// release "s1" should not delete "s2"
	if err := sdp.Release(s1); err != nil {
		t.Fatalf("release: %v", err)
	}
	defer mustClose(t, writer, dir)
	t.Fatal(indexWriterDeleteUnusedFilesMissing)
}

func TestSnapshotDeletionPolicyMissingCommits(t *testing.T) {
	s := newSnapshotDeletionPolicyTest(t)
	// Tests the behavior of SDP when commits that are given at ctor are missing
	// on onInit().
	dir := newDirectory()
	writer := mustNewIndexWriter(t, dir, s.getConfig(s.getDeletionPolicy()))
	sdp := snapshotPolicyOf(t, writer)
	mustAddDocument(t, writer, document.NewDocument())
	mustCommit(t, writer)
	s1, err := sdp.Snapshot()
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}

	// create another commit, not snapshotted.
	mustAddDocument(t, writer, document.NewDocument())
	mustClose(t, writer)

	// open a new writer w/ KeepOnlyLastCommit policy, so it will delete "s1"
	// commit.
	mustClose(t, mustNewIndexWriter(t, dir, s.getConfig(nil)))

	if slowFileExists(t, dir, s1.GetSegmentsFileName()) {
		t.Fatal("snapshotted commit should not exist")
	}
	mustClose(t, dir)
}
