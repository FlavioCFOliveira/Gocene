// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/index/TestIndexWriterMergePolicy.java
// (Apache Lucene 10.5.0). The @Nightly testMaxBufferedDocsChange lives in
// index_writer_merge_policy_monster_test.go; the two @AwaitsFix stress tests
// live in index_writer_merge_policy_awaitsfix_test.go.

package index_test

import (
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// Production members of IndexWriter that checkInvariants needs.
const (
	indexWriterWaitForMergesMissing           = "org.apache.lucene.index.IndexWriter#waitForMerges() is not ported"
	indexWriterGetNumBufferedDocumentsMissing = "org.apache.lucene.index.IndexWriter#getNumBufferedDocuments() is not ported"
)

// mockMergePolicy is the private MockMergePolicy: a less sophisticated version
// of LogDocMergePolicy, only for testing the interaction between IndexWriter
// and the MergePolicy.
type mockMergePolicy struct {
	*index.BaseMergePolicy
	mergeFactor int
}

func newMockMergePolicy() *mockMergePolicy {
	return &mockMergePolicy{BaseMergePolicy: index.NewBaseMergePolicy(), mergeFactor: 10}
}

func (m *mockMergePolicy) getMergeFactor() int { return m.mergeFactor }

func (m *mockMergePolicy) setMergeFactor(mergeFactor int) { m.mergeFactor = mergeFactor }

func (m *mockMergePolicy) FindMerges(_ index.MergeTrigger, segmentInfos *index.SegmentInfos, _ index.MergeContext) (*index.MergeSpecification, error) {
	var segments []*index.SegmentCommitInfo
	for sci := range segmentInfos.Iterator() {
		segments = append(segments, sci)
	}
	var spec *index.MergeSpecification
	for start := 0; start <= len(segments)-m.mergeFactor; {
		startDocCount := segments[start].SegmentInfo().MaxDoc()
		// Now search for the right-most segment that could be merged with the start segment
		end := start + 1
		for i := len(segments) - 1; i > start; i-- {
			docCount := segments[i].SegmentInfo().MaxDoc()
			if int64(docCount)*int64(m.mergeFactor) > int64(startDocCount) &&
				int64(docCount) < int64(m.mergeFactor)*int64(startDocCount) {
				end = i + 1
				break
			}
		}

		// Now record a merge if possible
		if start+m.mergeFactor <= end {
			if spec == nil {
				spec = index.NewMergeSpecification()
			}
			spec.Add(index.NewOneMerge(append([]*index.SegmentCommitInfo(nil), segments[start:start+m.mergeFactor]...)))
			start += m.mergeFactor
		} else {
			start++
		}
	}
	return spec, nil
}

func (m *mockMergePolicy) FindForcedMerges(*index.SegmentInfos, int, map[*index.SegmentCommitInfo]bool, index.MergeContext) (*index.MergeSpecification, error) {
	return nil, nil
}

func (m *mockMergePolicy) FindForcedDeletesMerges(*index.SegmentInfos, index.MergeContext) (*index.MergeSpecification, error) {
	return nil, nil
}

func newMergePolicyTestWriter(t *testing.T, dir store.Directory, maxBufferedDocs int, mp index.MergePolicy, ms index.MergeScheduler) *index.IndexWriter {
	t.Helper()
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetMaxBufferedDocs(maxBufferedDocs)
	conf.SetMergePolicy(mp)
	if ms != nil {
		conf.SetMergeScheduler(ms)
	}
	return mustNewIndexWriter(t, dir, conf)
}

// Test the normal case
func TestIndexWriterMergePolicyNormalCase(t *testing.T) {
	dir := newDirectory()

	writer := newMergePolicyTestWriter(t, dir, 10, newMockMergePolicy(), nil)

	for i := 0; i < 100; i++ {
		indexWriterMergePolicyAddDoc(t, writer)
		indexWriterMergePolicyCheckInvariants(t, writer)
	}

	mustClose(t, writer, dir)
}

// Test to see if there is over merge
func TestIndexWriterMergePolicyNoOverMerge(t *testing.T) {
	dir := newDirectory()

	writer := newMergePolicyTestWriter(t, dir, 10, newMockMergePolicy(), nil)

	for i := 0; i < 100; i++ {
		indexWriterMergePolicyAddDoc(t, writer)
		indexWriterMergePolicyCheckInvariants(t, writer)
	}
	t.Fatal(indexWriterGetNumBufferedDocumentsMissing)
}

// Test the case where flush is forced after every addDoc
func TestIndexWriterMergePolicyForceFlush(t *testing.T) {
	dir := newDirectory()

	mp := newMockMergePolicy()
	mp.setMergeFactor(10)
	writer := newMergePolicyTestWriter(t, dir, 10, mp, nil)

	for i := 0; i < 100; i++ {
		indexWriterMergePolicyAddDoc(t, writer)
		if err := writer.Flush(); err != nil {
			t.Fatalf("flush: %v", err)
		}
	}

	mustClose(t, writer, dir)
}

// Test the case where mergeFactor changes
func TestIndexWriterMergePolicyMergeFactorChange(t *testing.T) {
	dir := newDirectory()

	writer := newMergePolicyTestWriter(t, dir, 10, newMockMergePolicy(), index.NewSerialMergeScheduler())

	for i := 0; i < 250; i++ {
		indexWriterMergePolicyAddDoc(t, writer)
		indexWriterMergePolicyCheckInvariants(t, writer)
	}

	writer.GetConfig().GetMergePolicy().(*mockMergePolicy).setMergeFactor(5)

	// merge policy only fixes segments on levels where merges
	// have been triggered, so check invariants after all adds
	for i := 0; i < 10; i++ {
		indexWriterMergePolicyAddDoc(t, writer)
	}
	indexWriterMergePolicyCheckInvariants(t, writer)

	mustClose(t, writer, dir)
}

// Test the case where a merge results in no doc at all
func TestIndexWriterMergePolicyMergeDocCount0(t *testing.T) {
	dir := newDirectory()

	ldmp := newMockMergePolicy()
	ldmp.setMergeFactor(100)
	writer := newMergePolicyTestWriter(t, dir, 10, ldmp, nil)

	for i := 0; i < 250; i++ {
		indexWriterMergePolicyAddDoc(t, writer)
		indexWriterMergePolicyCheckInvariants(t, writer)
	}
	mustClose(t, writer)

	// delete some docs without merging
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetMergePolicy(index.NewNoMergePolicy())
	writer = mustNewIndexWriter(t, dir, conf)
	if _, err := writer.DeleteDocuments([]index.Term{*index.NewTerm("content", "aaa")}); err != nil {
		t.Fatalf("deleteDocuments: %v", err)
	}
	mustClose(t, writer)

	ldmp = newMockMergePolicy()
	ldmp.setMergeFactor(5)
	conf = newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetOpenMode(index.Append)
	conf.SetMaxBufferedDocs(10)
	conf.SetMergePolicy(ldmp)
	conf.SetMergeScheduler(index.NewConcurrentMergeScheduler())
	writer = mustNewIndexWriter(t, dir, conf)

	// merge factor is changed, so check invariants after all adds
	for i := 0; i < 10; i++ {
		indexWriterMergePolicyAddDoc(t, writer)
	}
	mustCommit(t, writer)
	t.Fatal(indexWriterWaitForMergesMissing)
}

func indexWriterMergePolicyAddDoc(t testing.TB, writer *index.IndexWriter) {
	t.Helper()
	doc := document.NewDocument()
	doc.Add(newTextField(t, "content", "aaa", false))
	mustAddDocument(t, writer, doc)
}

// indexWriterMergePolicyCheckInvariants renders the private
// checkInvariants(IndexWriter); its first statement is writer.waitForMerges(),
// and it also reads getNumBufferedDocuments() and maxDoc(int).
func indexWriterMergePolicyCheckInvariants(t testing.TB, writer *index.IndexWriter) {
	t.Helper()
	t.Fatal(indexWriterWaitForMergesMissing + "; " + indexWriterGetNumBufferedDocumentsMissing +
		"; org.apache.lucene.index.IndexWriter#maxDoc(int) is not ported")
}

const indexWriterMergePolicyEpsilon = 1e-14

// maxCFSSegmentSizeMBSetter is the part of MergePolicy assertSetters reads.
type maxCFSSegmentSizeMBSetter interface {
	SetMaxCFSSegmentSizeMB(float64)
	GetMaxCFSSegmentSizeMB() float64
}

func TestIndexWriterMergePolicySetters(t *testing.T) {
	indexWriterMergePolicyAssertSetters(t, index.NewLogByteSizeMergePolicy())
	indexWriterMergePolicyAssertSetters(t, newMockMergePolicy())
}

func assertFloatEquals(t testing.TB, expected, actual, delta float64) {
	t.Helper()
	if math.Abs(expected-actual) > delta {
		t.Fatalf("expected %v, got %v (delta %v)", expected, actual, delta)
	}
}

func indexWriterMergePolicyAssertSetters(t *testing.T, lmp maxCFSSegmentSizeMBSetter) {
	t.Helper()
	lmp.SetMaxCFSSegmentSizeMB(2.0)
	assertFloatEquals(t, 2.0, lmp.GetMaxCFSSegmentSizeMB(), indexWriterMergePolicyEpsilon)

	lmp.SetMaxCFSSegmentSizeMB(math.Inf(1))
	assertFloatEquals(t, float64(math.MaxInt64)/1024./1024., lmp.GetMaxCFSSegmentSizeMB(), indexWriterMergePolicyEpsilon*float64(math.MaxInt64))

	lmp.SetMaxCFSSegmentSizeMB(float64(math.MaxInt64) / 1024. / 1024.)
	assertFloatEquals(t, float64(math.MaxInt64)/1024./1024., lmp.GetMaxCFSSegmentSizeMB(), indexWriterMergePolicyEpsilon*float64(math.MaxInt64))

	// expectThrows(IllegalArgumentException.class, () -> lmp.setMaxCFSSegmentSizeMB(-2.0))
	t.Fatal("MergePolicy#setMaxCFSSegmentSizeMB(double) must throw IllegalArgumentException for -2.0; " +
		"the Go SetMaxCFSSegmentSizeMB has no error result and clamps negative values")
}

// fiveFlushedSegments renders the shared prologue of the merge-on-commit and
// merge-on-getReader tests: five single-document segments written under
// NoMergePolicy.
func fiveFlushedSegments(t *testing.T, dir store.Directory) {
	t.Helper()
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetMergePolicy(index.NewNoMergePolicy())
	firstWriter := mustNewIndexWriter(t, dir, conf)
	for i := 0; i < 5; i++ {
		testIndexWriterAddDoc(t, firstWriter)
		if err := firstWriter.Flush(); err != nil {
			t.Fatalf("flush: %v", err)
		}
	}
	firstReader := openReaderFromWriter(t, firstWriter)
	assertLeafCount(t, 5, firstReader)
	mustClose(t, firstReader)
	mustClose(t, firstWriter) // When this writer closes, it does not merge on commit.
}

func openReaderFromWriter(t testing.TB, w *index.IndexWriter) *index.DirectoryReader {
	t.Helper()
	r, err := index.OpenDirectoryReaderFromWriter(w)
	if err != nil {
		t.Fatalf("DirectoryReader.open(writer): %v", err)
	}
	return r
}

func assertLeafCount(t testing.TB, expected int, r index.IndexReaderInterface) {
	t.Helper()
	leaves, err := r.Leaves()
	if err != nil {
		t.Fatalf("leaves: %v", err)
	}
	if len(leaves) != expected {
		t.Fatalf("leaves().size(): expected %d, got %d", expected, len(leaves))
	}
}

func assertSegmentCount(t testing.TB, expected int, w *index.IndexWriter) {
	t.Helper()
	if got := w.GetSegmentCount(); got != expected {
		t.Fatalf("getSegmentCount(): expected %d, got %d", expected, got)
	}
}

// Test basic semantics of merge on commit
func TestIndexWriterMergePolicyMergeOnCommit(t *testing.T) {
	dir := newDirectory()
	fiveFlushedSegments(t, dir)

	iwc := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	iwc.SetMergePolicy(newMergeOnXMergePolicy(newMergePolicy(t), index.MergeTriggerCommit))
	iwc.SetMaxFullFlushMergeWaitMillis(math.MaxInt32)

	writerWithMergePolicy := mustNewIndexWriter(t, dir, iwc)

	// No changes. Refresh doesn't trigger a merge.
	unmergedReader := openReaderFromWriter(t, writerWithMergePolicy)
	assertLeafCount(t, 5, unmergedReader)
	mustClose(t, unmergedReader)

	mustCommit(t, writerWithMergePolicy) // Do merge on commit.
	assertSegmentCount(t, 1, writerWithMergePolicy)

	mergedReader := openReaderFromWriter(t, writerWithMergePolicy)
	assertLeafCount(t, 1, mergedReader)
	mustClose(t, mergedReader)

	reader := openReaderFromWriter(t, writerWithMergePolicy)
	searcher := search.NewIndexSearcher(reader)
	if got := reader.NumDocs(); got != 5 {
		t.Fatalf("numDocs: expected 5, got %d", got)
	}
	count, err := searcher.Count(search.NewMatchAllDocsQuery())
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 5 {
		t.Fatalf("count(MatchAllDocsQuery): expected 5, got %d", count)
	}
	mustClose(t, reader)

	mustClose(t, writerWithMergePolicy, dir)
}

// Test basic semantics of merge on commit and events recording invocation
func TestIndexWriterMergePolicyMergeOnCommitWithEventListener(t *testing.T) {
	dir := newDirectory()
	defer mustClose(t, dir)
	fiveFlushedSegments(t, dir)

	t.Fatal("org.apache.lucene.tests.index.MockIndexWriterEventListener is not ported")
}

func TestIndexWriterMergePolicyCarryOverNewDeletesOnCommit(t *testing.T) {
	directory := newDirectory()
	defer mustClose(t, directory)
	t.Fatal("overriding org.apache.lucene.index.IndexWriter#merge(MergePolicy.OneMerge) in an IndexWriter subclass is not ported")
}

// This test makes sure we release the merge readers on abort. MDW will fail if
// it can't close all files
func TestIndexWriterMergePolicyAbortMergeOnCommit(t *testing.T) {
	indexWriterMergePolicyAbortMergeOnX(t, false)
}

func TestIndexWriterMergePolicyAbortMergeOnGetReader(t *testing.T) {
	indexWriterMergePolicyAbortMergeOnX(t, true)
}

// blockingSerialMergeScheduler renders the anonymous SerialMergeScheduler
// subclasses whose merge(MergeSource, MergeTrigger) runs before() and then
// super.merge. synchronized mirrors a `synchronized` override.
type blockingSerialMergeScheduler struct {
	*index.SerialMergeScheduler
	synchronized bool
	mu           sync.Mutex
	before       func()
}

func (s *blockingSerialMergeScheduler) Merge(mergeSource index.MergeSource, trigger index.MergeTrigger) error {
	if s.synchronized {
		s.mu.Lock()
		defer s.mu.Unlock()
	}
	s.before()
	return s.SerialMergeScheduler.Merge(mergeSource, trigger)
}

func idDocument(t testing.TB, id string, stored bool) *document.Document {
	d := document.NewDocument()
	d.Add(newStringField(t, "id", id, stored))
	return d
}

func indexWriterMergePolicyAbortMergeOnX(t *testing.T, useGetReader bool) {
	directory := newDirectory()
	defer mustClose(t, directory)
	waitForMerge := newCountDownLatch()
	waitForDeleteAll := newCountDownLatch()
	trigger := index.MergeTriggerCommit
	if useGetReader {
		trigger = index.MergeTriggerGetReader
	}
	conf := newIndexWriterConfig()
	conf.SetMergePolicy(newMergeOnXMergePolicy(newMergePolicy(t), trigger))
	conf.SetMaxFullFlushMergeWaitMillis(30 * 1000)
	conf.SetMergeScheduler(&blockingSerialMergeScheduler{
		SerialMergeScheduler: index.NewSerialMergeScheduler(),
		synchronized:         true,
		before: func() {
			waitForMerge.countDown()
			waitForDeleteAll.awaitFromGoroutine(t, "waitForDeleteAll")
		},
	})
	writer := mustNewIndexWriter(t, directory, conf)
	defer mustClose(t, writer)

	d1 := idDocument(t, "1", false)
	d2 := idDocument(t, "2", false)
	mustAddDocument(t, writer, d1)
	if err := writer.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	mustAddDocument(t, writer, d2)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		success := false
		defer func() {
			if !success {
				waitForMerge.countDown()
			}
		}()
		if useGetReader {
			r, err := index.OpenDirectoryReaderFromWriter(writer)
			if err != nil {
				t.Errorf("DirectoryReader.open(writer): %v", err)
				return
			}
			if err := r.Close(); err != nil {
				t.Errorf("close: %v", err)
				return
			}
		} else if _, err := writer.Commit(); err != nil {
			t.Errorf("commit: %v", err)
			return
		}
		success = true
	}()
	waitForMerge.await(t, "waitForMerge")
	if _, err := writer.DeleteAll(); err != nil {
		t.Fatalf("deleteAll: %v", err)
	}
	waitForDeleteAll.countDown()
	wg.Wait()
}

func TestIndexWriterMergePolicyForceMergeWhileGetReader(t *testing.T) {
	directory := newDirectory()
	defer mustClose(t, directory)
	waitForMerge := newCountDownLatch()
	waitForForceMergeCalled := newCountDownLatch()
	conf := newIndexWriterConfig()
	conf.SetMergePolicy(newMergeOnXMergePolicy(newMergePolicy(t), index.MergeTriggerGetReader))
	conf.SetMaxFullFlushMergeWaitMillis(30 * 1000)
	conf.SetMergeScheduler(&blockingSerialMergeScheduler{
		SerialMergeScheduler: index.NewSerialMergeScheduler(),
		before: func() {
			waitForMerge.countDown()
			waitForForceMergeCalled.awaitFromGoroutine(t, "waitForForceMergeCalled")
		},
	})
	writer := mustNewIndexWriter(t, directory, conf)
	defer mustClose(t, writer)

	mustAddDocument(t, writer, idDocument(t, "1", false))
	if err := writer.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	mustAddDocument(t, writer, idDocument(t, "2", false))
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		reader, err := index.OpenDirectoryReaderFromWriter(writer)
		if err != nil {
			t.Errorf("DirectoryReader.open(writer): %v", err)
			return
		}
		defer func() {
			if err := reader.Close(); err != nil {
				t.Errorf("close: %v", err)
			}
		}()
		if got := reader.MaxDoc(); got != 2 {
			t.Errorf("maxDoc: expected 2, got %d", got)
		}
	}()
	waitForMerge.await(t, "waitForMerge")
	mustAddDocument(t, writer, idDocument(t, "3", false))
	waitForForceMergeCalled.countDown()
	if err := writer.ForceMerge(1); err != nil {
		t.Fatalf("forceMerge: %v", err)
	}
	wg.Wait()
}

func TestIndexWriterMergePolicyFailAfterMergeCommitted(t *testing.T) {
	directory := newDirectory()
	defer mustClose(t, directory)
	t.Fatal("overriding org.apache.lucene.index.IndexWriter#doAfterFlush() in an IndexWriter subclass is not ported; " +
		"IndexWriter#hasPendingMerges() and IndexWriter#executeMerge(MergeTrigger) are not ported")
}

// Test basic semantics of merge on getReader
func TestIndexWriterMergePolicyMergeOnGetReader(t *testing.T) {
	dir := newDirectory()
	fiveFlushedSegments(t, dir)

	iwc := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	iwc.SetMergePolicy(newMergeOnXMergePolicy(newMergePolicy(t), index.MergeTriggerGetReader))
	iwc.SetMaxFullFlushMergeWaitMillis(math.MaxInt32)

	writerWithMergePolicy := mustNewIndexWriter(t, dir, iwc)

	unmergedReader := mustOpenDirectoryReader(t, dir) // No changes. GetReader doesn't trigger a merge.
	assertLeafCount(t, 5, unmergedReader)
	mustClose(t, unmergedReader)

	testIndexWriterAddDoc(t, writerWithMergePolicy)
	mergedReader := openReaderFromWriter(t, writerWithMergePolicy)
	// Doc added, do merge on getReader.
	assertLeafCount(t, 1, mergedReader)
	mustClose(t, mergedReader)

	mustClose(t, writerWithMergePolicy, dir)
}

// mergeOnXMergePolicy is the private MergeOnXMergePolicy.
type mergeOnXMergePolicy struct {
	*index.FilterMergePolicy
	trigger index.MergeTrigger
}

func newMergeOnXMergePolicy(in index.MergePolicy, trigger index.MergeTrigger) *mergeOnXMergePolicy {
	return &mergeOnXMergePolicy{FilterMergePolicy: index.NewFilterMergePolicy(in), trigger: trigger}
}

func (p *mergeOnXMergePolicy) FindFullFlushMerges(mergeTrigger index.MergeTrigger, segmentInfos *index.SegmentInfos, mergeContext index.MergeContext) (*index.MergeSpecification, error) {
	// Optimize down to a single segment on commit
	if mergeTrigger == p.trigger && segmentInfos.Size() > 1 {
		var nonMergingSegments []*index.SegmentCommitInfo
		merging := mergeContext.GetMergingSegments()
		for sci := range segmentInfos.Iterator() {
			if !merging[sci] {
				nonMergingSegments = append(nonMergingSegments, sci)
			}
		}
		if len(nonMergingSegments) > 1 {
			mergeSpecification := index.NewMergeSpecification()
			mergeSpecification.Add(index.NewOneMerge(nonMergingSegments))
			return mergeSpecification, nil
		}
	}
	return nil, nil
}

func TestIndexWriterMergePolicySetDiagnostics(t *testing.T) {
	logMp := newLogMergePolicyWithMergeFactor(4)
	logMp.SetTargetSearchConcurrency(1)
	t.Fatal("overriding org.apache.lucene.index.MergePolicy.OneMerge#setMergeInfo(SegmentCommitInfo) " +
		"in a OneMerge subclass built with the OneMerge(OneMerge) constructor is not ported")
}

// mockAssertFileExistIndexInput is the private MockAssertFileExistIndexInput:
// every positioning or reading call first checks that the backing file still
// exists on disk.
type mockAssertFileExistIndexInput struct {
	spi.BaseDataInput
	resourceDescription string
	name                string
	delegate            store.IndexInput
	filePath            string
}

func newMockAssertFileExistIndexInput(name string, delegate store.IndexInput, filePath string) *mockAssertFileExistIndexInput {
	in := &mockAssertFileExistIndexInput{
		resourceDescription: fmt.Sprintf("MockAssertFileExistIndexInput(name=%s delegate=%v)", name, delegate),
		name:                name,
		delegate:            delegate,
		filePath:            filePath,
	}
	in.Core = in
	return in
}

func (in *mockAssertFileExistIndexInput) checkFileExist() error {
	if _, err := os.Stat(in.filePath); err != nil {
		return &fs.PathError{Op: "open", Path: in.filePath, Err: fs.ErrNotExist}
	}
	return nil
}

func (in *mockAssertFileExistIndexInput) String() string { return in.resourceDescription }

func (in *mockAssertFileExistIndexInput) Close() error { return in.delegate.Close() }

func (in *mockAssertFileExistIndexInput) Clone() store.IndexInput {
	return newMockAssertFileExistIndexInput(in.name, in.delegate.Clone(), in.filePath)
}

func (in *mockAssertFileExistIndexInput) Slice(sliceDescription string, offset, length int64) (store.IndexInput, error) {
	if err := in.checkFileExist(); err != nil {
		return nil, err
	}
	slice, err := in.delegate.Slice(sliceDescription, offset, length)
	if err != nil {
		return nil, err
	}
	return newMockAssertFileExistIndexInput(sliceDescription, slice, in.filePath), nil
}

func (in *mockAssertFileExistIndexInput) GetFilePointer() int64 { return in.delegate.GetFilePointer() }

func (in *mockAssertFileExistIndexInput) SetPosition(pos int64) error {
	if err := in.checkFileExist(); err != nil {
		return err
	}
	return in.delegate.SetPosition(pos)
}

func (in *mockAssertFileExistIndexInput) Length() int64 { return in.delegate.Length() }

func (in *mockAssertFileExistIndexInput) ReadByte() (byte, error) {
	if err := in.checkFileExist(); err != nil {
		return 0, err
	}
	return in.delegate.ReadByte()
}

func (in *mockAssertFileExistIndexInput) ReadBytes(b []byte, offset, length int) error {
	if err := in.checkFileExist(); err != nil {
		return err
	}
	return in.delegate.ReadBytes(b, offset, length)
}

// SkipBytes carries IndexInput#skipBytes(long): seek(getFilePointer() + numBytes).
func (in *mockAssertFileExistIndexInput) SkipBytes(numBytes int64) error {
	if numBytes < 0 {
		return fmt.Errorf("numBytes must be >= 0, got %d", numBytes)
	}
	return in.SetPosition(in.GetFilePointer() + numBytes)
}

// ReadBytesN is the Go-only IndexInput convenience, built on ReadBytes.
func (in *mockAssertFileExistIndexInput) ReadBytesN(n int) ([]byte, error) {
	b := make([]byte, n)
	if err := in.ReadBytes(b, 0, n); err != nil {
		return nil, err
	}
	return b, nil
}

// assertFileExistDirectory renders the anonymous FilterDirectory subclass
// wrapping every opened input in a mockAssertFileExistIndexInput.
type assertFileExistDirectory struct {
	*store.FilterDirectory
	path string
}

func newAssertFileExistDirectory(t testing.TB, path string) *assertFileExistDirectory {
	return &assertFileExistDirectory{FilterDirectory: store.NewFilterDirectory(newFSDirectoryAt(t, path)), path: path}
}

func (d *assertFileExistDirectory) OpenInput(name string, context store.IOContext) (store.IndexInput, error) {
	indexInput, err := d.FilterDirectory.OpenInput(name, context)
	if err != nil {
		return nil, err
	}
	return newMockAssertFileExistIndexInput(name, indexInput, filepath.Join(d.path, name)), nil
}

func idVersionDocument(t testing.TB, id, version string) *document.Document {
	doc := document.NewDocument()
	doc.Add(newStringField(t, "id", id, true))
	doc.Add(newStringField(t, "version", version, true))
	return doc
}

func softDeleteField(t testing.TB) []*document.Field {
	field, err := document.NewNumericDocValuesField("soft_delete", 1)
	if err != nil {
		t.Fatalf("NumericDocValuesField: %v", err)
	}
	return []*document.Field{field.Field}
}

func mustSoftUpdateDocument(t testing.TB, w *index.IndexWriter, term *index.Term, doc *document.Document, softDeletes []*document.Field) {
	t.Helper()
	if _, err := w.SoftUpdateDocument(term, doc, softDeletes); err != nil {
		t.Fatalf("softUpdateDocument: %v", err)
	}
}

func mustFlush(t testing.TB, w *index.IndexWriter) {
	t.Helper()
	if err := w.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}
}

func TestIndexWriterMergePolicyForceMergeDVUpdateFileWithConcurrentFlush(t *testing.T) {
	path := filepath.Join(t.TempDir(), "testForceMergeDVUpdateFileWithConcurrentFlush")
	mockDirectory := newAssertFileExistDirectory(t, path)
	defer mustClose(t, mockDirectory)
	t.Fatal("org.apache.lucene.index.SoftDeletesRetentionMergePolicy(String, Supplier<Query>, MergePolicy) is not ported " +
		"(the Go constructor takes no retention query); overriding MergePolicy.OneMerge#initMergeReaders(IOFunction) " +
		"and OneMerge#wrapForMerge(CodecReader) in a OneMerge subclass is not ported")
}

// concurrentFlushSegments renders the shared prologue of the two
// DV-update-file-with-concurrent-flush tests: two segments, the second
// carrying a soft update of id:2.
func concurrentFlushSegments(t *testing.T, mockDirectory store.Directory) {
	t.Helper()
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetMergePolicy(index.NewNoMergePolicy())
	firstWriter := mustNewIndexWriter(t, mockDirectory, conf)

	mustAddDocument(t, firstWriter, idVersionDocument(t, "1", "1"))
	mustFlush(t, firstWriter)
	mustAddDocument(t, firstWriter, idVersionDocument(t, "2", "1"))
	mustSoftUpdateDocument(t, firstWriter, index.NewTerm("id", "2"), idVersionDocument(t, "2", "2"), softDeleteField(t))
	mustFlush(t, firstWriter)
	firstReader := openReaderFromWriter(t, firstWriter)
	assertLeafCount(t, 2, firstReader)
	mustClose(t, firstReader, firstWriter)
}

// blockingConcurrentMergeScheduler renders the anonymous
// ConcurrentMergeScheduler subclasses whose merge(MergeSource, MergeTrigger)
// runs before() and then super.merge.
type blockingConcurrentMergeScheduler struct {
	*index.ConcurrentMergeScheduler
	before func()
}

func (s *blockingConcurrentMergeScheduler) Merge(mergeSource index.MergeSource, trigger index.MergeTrigger) error {
	s.before()
	return s.ConcurrentMergeScheduler.Merge(mergeSource, trigger)
}

// concurrentSoftUpdate renders the thread body of the two
// DV-update-file-with-concurrent-flush tests.
func concurrentSoftUpdate(t *testing.T, writer *index.IndexWriter, waitForInitMergeReader, waitForDVUpdate *countDownLatch) {
	if !waitForInitMergeReader.awaitFromGoroutine(t, "waitForInitMergeReader") {
		return
	}
	if _, err := writer.SoftUpdateDocument(index.NewTerm("id", "2"), idVersionDocument(t, "2", "3"), softDeleteField(t)); err != nil {
		t.Errorf("softUpdateDocument: %v", err)
		return
	}
	reader, err := index.OpenDirectoryReaderFromWriterWithOptions(writer, true, false)
	if err != nil {
		t.Errorf("DirectoryReader.open(writer, true, false): %v", err)
		return
	}
	if err := reader.Close(); err != nil {
		t.Errorf("close: %v", err)
		return
	}
	waitForDVUpdate.countDown()
}

func newMergeOnXWriterWithBlockingCMS(t *testing.T, dir store.Directory, trigger index.MergeTrigger, waitForInitMergeReader, waitForDVUpdate *countDownLatch) *index.IndexWriter {
	t.Helper()
	mockConcurrentMergeScheduler := &blockingConcurrentMergeScheduler{
		ConcurrentMergeScheduler: index.NewConcurrentMergeScheduler(),
		before: func() {
			waitForInitMergeReader.countDown()
			waitForDVUpdate.awaitFromGoroutine(t, "waitForDVUpdate")
		},
	}
	iwc := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	iwc.SetMergePolicy(newMergeOnXMergePolicy(newMergePolicy(t), trigger))
	iwc.SetMaxFullFlushMergeWaitMillis(math.MaxInt32)
	iwc.SetMergeScheduler(mockConcurrentMergeScheduler)
	return mustNewIndexWriter(t, dir, iwc)
}

// The Java thread of the next two tests is never joined; the Go goroutine is
// joined before the test returns because a goroutine may not report to t
// after the test has completed.

func TestIndexWriterMergePolicyMergeDVUpdateFileOnGetReaderWithConcurrentFlush(t *testing.T) {
	waitForInitMergeReader := newCountDownLatch()
	waitForDVUpdate := newCountDownLatch()

	path := filepath.Join(t.TempDir(), "testMergeDVUpdateFileOnGetReaderWithConcurrentFlush")
	mockDirectory := newAssertFileExistDirectory(t, path)
	concurrentFlushSegments(t, mockDirectory)

	writerWithMergePolicy := newMergeOnXWriterWithBlockingCMS(t, mockDirectory, index.MergeTriggerGetReader, waitForInitMergeReader, waitForDVUpdate)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		concurrentSoftUpdate(t, writerWithMergePolicy, waitForInitMergeReader, waitForDVUpdate)
	}()

	mergedReader := openReaderFromWriter(t, writerWithMergePolicy)
	assertLeafCount(t, 1, mergedReader)
	mustClose(t, mergedReader)

	mustClose(t, writerWithMergePolicy, mockDirectory)
	wg.Wait()
}

func TestIndexWriterMergePolicyMergeDVUpdateFileOnCommitWithConcurrentFlush(t *testing.T) {
	waitForInitMergeReader := newCountDownLatch()
	waitForDVUpdate := newCountDownLatch()

	path := filepath.Join(t.TempDir(), "testMergeDVUpdateFileOnCommitWithConcurrentFlush")
	mockDirectory := newAssertFileExistDirectory(t, path)
	concurrentFlushSegments(t, mockDirectory)

	writerWithMergePolicy := newMergeOnXWriterWithBlockingCMS(t, mockDirectory, index.MergeTriggerCommit, waitForInitMergeReader, waitForDVUpdate)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		concurrentSoftUpdate(t, writerWithMergePolicy, waitForInitMergeReader, waitForDVUpdate)
	}()

	mustCommit(t, writerWithMergePolicy)
	assertSegmentCount(t, 2, writerWithMergePolicy)

	mustClose(t, writerWithMergePolicy, mockDirectory)
	wg.Wait()
}

// onlyForceMergeTieredMergePolicy renders the anonymous TieredMergePolicy
// subclass whose findMerges returns null: only allow force merge.
type onlyForceMergeTieredMergePolicy struct {
	*index.TieredMergePolicy
}

func (p *onlyForceMergeTieredMergePolicy) FindMerges(index.MergeTrigger, *index.SegmentInfos, index.MergeContext) (*index.MergeSpecification, error) {
	return nil, nil
}

func TestIndexWriterMergePolicyForceMergeWithPendingHardAndSoftDeleteFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "testForceMergeWithPendingHardAndSoftDeleteFile")
	mockDirectory := newAssertFileExistDirectory(t, path)

	mockMergePolicy := index.NewOneMergeWrappingMergePolicy(
		&onlyForceMergeTieredMergePolicy{TieredMergePolicy: index.NewTieredMergePolicy()},
		func(merge *index.OneMerge) *index.OneMerge { return index.NewOneMerge(merge.Segments) })

	conf := newIndexWriterConfig()
	conf.SetMergePolicy(mockMergePolicy)
	writer := mustNewIndexWriter(t, mockDirectory, conf)

	mustAddDocument(t, writer, idVersionDocument(t, "1", "1"))
	mustCommit(t, writer)

	mustAddDocument(t, writer, idVersionDocument(t, "2", "1"))
	mustAddDocument(t, writer, idVersionDocument(t, "3", "1"))
	mustAddDocument(t, writer, idVersionDocument(t, "4", "1"))
	mustAddDocument(t, writer, idVersionDocument(t, "5", "1"))
	mustCommit(t, writer)

	if _, err := writer.UpdateDocument(index.NewTerm("id", "2"), idVersionDocument(t, "2", "2")); err != nil {
		t.Fatalf("updateDocument: %v", err)
	}
	mustCommit(t, writer)

	if _, err := writer.UpdateDocument(index.NewTerm("id", "3"), idVersionDocument(t, "3", "2")); err != nil {
		t.Fatalf("updateDocument: %v", err)
	}

	mustSoftUpdateDocument(t, writer, index.NewTerm("id", "4"), idVersionDocument(t, "4", "2"), softDeleteField(t))

	reader, err := writer.GetReader(true, false)
	if err != nil {
		t.Fatalf("getReader(true, false): %v", err)
	}
	mustClose(t, reader)
	mustCommit(t, writer)

	if err := writer.ForceMerge(1); err != nil {
		t.Fatalf("forceMerge: %v", err)
	}

	mustClose(t, writer, mockDirectory)
}
