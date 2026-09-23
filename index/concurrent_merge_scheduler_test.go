// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/index/TestConcurrentMergeScheduler.java
// (Apache Lucene 10.5.0).
//
// Most Java tests subclass ConcurrentMergeScheduler and override one of its
// protected hooks (doMerge, maybeStall, doStall, getMergeThread,
// getIntraMergeExecutor). ConcurrentMergeScheduler calls those hooks on
// itself, so a Go embedder's method is never dispatched to; those tests fail
// at the override, naming the hook.

package index_test

import (
	"strconv"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
)

// cmsHookOverrideMissing names a protected ConcurrentMergeScheduler hook whose
// override is not dispatched to.
func cmsHookOverrideMissing(hook string) string {
	return "overriding the protected org.apache.lucene.index.ConcurrentMergeScheduler#" + hook +
		" in a ConcurrentMergeScheduler subclass is not ported"
}

// Make sure running BG merges still work fine even when
// we are hitting exceptions during flushing.
func TestConcurrentMergeSchedulerFlushExceptions(t *testing.T) {
	directory := newDirectory()
	defer mustClose(t, directory)
	t.Fatal("org.apache.lucene.tests.index.SuppressingConcurrentMergeScheduler and " +
		"MockDirectoryWrapper.Failure#callStackContainsAnyOf(String...) / isTestThread() are not ported")
}

// Test that deletes committed after a merge started and
// before it finishes, are correctly merged back:
func TestConcurrentMergeSchedulerDeleteMerging(t *testing.T) {
	directory := newDirectory()

	mp := index.NewLogDocMergePolicy()
	// Force degenerate merging so we can get a mix of
	// merging of segments with and without deletes at the
	// start:
	mp.SetMinMergeDocs(1000)
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetMergePolicy(mp)
	writer := mustNewIndexWriter(t, directory, conf)
	reduceOpenFiles(writer)

	doc := document.NewDocument()
	idField := newStringField(t, "id", "", true)
	doc.Add(idField)
	for i := 0; i < 10; i++ {
		for j := 0; j < 100; j++ {
			idField.SetStringValue(strconv.Itoa(i*100 + j))
			mustAddDocument(t, writer, doc)
		}

		delID := i
		for delID < 100*(1+i) {
			mustDeleteTerm(t, writer, "id", strconv.Itoa(delID))
			delID += 10
		}

		mustCommit(t, writer)
	}

	mustClose(t, writer)
	reader := mustOpenDirectoryReader(t, directory)
	// Verify that we did not lose any deletes...
	assertReaderNumDocs(t, 450, reader)
	mustClose(t, reader, directory)
}

func TestConcurrentMergeSchedulerNoExtraFiles(t *testing.T) {
	directory := newDirectory()
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetMaxBufferedDocs(2)
	writer := mustNewIndexWriter(t, directory, conf)

	for iter := 0; iter < 7; iter++ {
		for j := 0; j < 21; j++ {
			doc := document.NewDocument()
			doc.Add(newTextField(t, "content", "a b c", false))
			mustAddDocument(t, writer, doc)
		}

		mustClose(t, writer)
		assertNoUnreferencedFiles(t, directory, "testNoExtraFiles")

		// Reopen
		conf = newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
		conf.SetOpenMode(index.Append)
		conf.SetMaxBufferedDocs(2)
		writer = mustNewIndexWriter(t, directory, conf)
	}

	mustClose(t, writer, directory)
}

func TestConcurrentMergeSchedulerNoWaitClose(t *testing.T) {
	directory := newDirectory()
	defer mustClose(t, directory)
	iwc := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	// Force excessive merging:
	iwc.SetMaxBufferedDocs(2)
	iwc.SetMergePolicy(newLogMergePolicyWithMergeFactor(100))
	iwc.SetCommitOnClose(false)
	if _, ok := iwc.GetMergeScheduler().(*index.ConcurrentMergeScheduler); ok {
		t.Fatal(cmsHookOverrideMissing("getIntraMergeExecutor(MergePolicy.OneMerge)") +
			"; the protected field intraMergeExecutor it returns is not reachable")
	}
}

// LUCENE-4544
func TestConcurrentMergeSchedulerMaxMergeCount(t *testing.T) {
	dir := newDirectory()
	defer mustClose(t, dir)
	t.Fatal(cmsHookOverrideMissing("doMerge(MergeSource, MergePolicy.OneMerge)"))
}

func TestConcurrentMergeSchedulerSmallMergesDonNotGetThreads(t *testing.T) {
	dir := newDirectory()
	defer mustClose(t, dir)
	t.Fatal(cmsHookOverrideMissing("doMerge(MergeSource, MergePolicy.OneMerge)"))
}

func TestConcurrentMergeSchedulerIntraMergeThreadPoolIsLimitedByMaxThreads(t *testing.T) {
	t.Fatal("org.apache.lucene.index.ConcurrentMergeScheduler.MergeThread, the fields mergeThreads and " +
		"intraMergeExecutor, and ConcurrentMergeScheduler#sync() are not ported")
}

func TestConcurrentMergeSchedulerTotalBytesSize(t *testing.T) {
	d := newDirectory()
	defer mustClose(t, d)
	t.Fatal(cmsHookOverrideMissing("doMerge(MergeSource, MergePolicy.OneMerge)") + " (TrackingCMS)")
}

func TestConcurrentMergeSchedulerInvalidMaxMergeCountAndThreads(t *testing.T) {
	cms := index.NewConcurrentMergeScheduler()
	if err := cms.SetMaxMergesAndThreads(index.AutoDetectMergesAndThreads, 3); err == nil {
		t.Fatal("expected IllegalArgumentException from setMaxMergesAndThreads(AUTO_DETECT, 3)")
	}
	if err := cms.SetMaxMergesAndThreads(3, index.AutoDetectMergesAndThreads); err == nil {
		t.Fatal("expected IllegalArgumentException from setMaxMergesAndThreads(3, AUTO_DETECT)")
	}
}

func TestConcurrentMergeSchedulerLiveMaxMergeCount(t *testing.T) {
	d := newDirectory()
	defer mustClose(t, d)
	t.Fatal(cmsHookOverrideMissing("doMerge(MergeSource, MergePolicy.OneMerge)"))
}

// LUCENE-6063
func TestConcurrentMergeSchedulerMaybeStallCalled(t *testing.T) {
	dir := newDirectory()
	defer mustClose(t, dir)
	t.Fatal(cmsHookOverrideMissing("maybeStall(MergeSource)"))
}

// LUCENE-6094
func TestConcurrentMergeSchedulerHangDuringRollback(t *testing.T) {
	dir := newDirectory()
	defer mustClose(t, dir)
	t.Fatal(cmsHookOverrideMissing("doMerge(MergeSource, MergePolicy.OneMerge)"))
}

// LUCENE-10118 : Verify the basic log output from MergeThreads
func TestConcurrentMergeSchedulerMergeThreadMessages(t *testing.T) {
	dir := newDirectory()
	defer mustClose(t, dir)
	t.Fatal(cmsHookOverrideMissing("getMergeThread(MergeSource, MergePolicy.OneMerge)"))
}

func assertCMSCounts(t testing.TB, cms *index.ConcurrentMergeScheduler, maxMergeCount, maxThreadCount int) {
	t.Helper()
	if got := cms.MaxMergeCount(); got != maxMergeCount {
		t.Fatalf("getMaxMergeCount(): expected %d, got %d", maxMergeCount, got)
	}
	if got := cms.MaxThreadCount(); got != maxThreadCount {
		t.Fatalf("getMaxThreadCount(): expected %d, got %d", maxThreadCount, got)
	}
}

func TestConcurrentMergeSchedulerDynamicDefaults(t *testing.T) {
	dir := newDirectory()
	iwc := index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	cms := index.NewConcurrentMergeScheduler()
	assertCMSCounts(t, cms, index.AutoDetectMergesAndThreads, index.AutoDetectMergesAndThreads)
	iwc.SetMergeScheduler(cms)
	iwc.SetMaxBufferedDocs(2)
	lmp := newLogMergePolicy()
	lmp.SetMergeFactor(2)
	iwc.SetMergePolicy(lmp)

	w := mustNewIndexWriter(t, dir, iwc)
	mustAddDocument(t, w, document.NewDocument())
	mustAddDocument(t, w, document.NewDocument())
	// flush

	mustAddDocument(t, w, document.NewDocument())
	mustAddDocument(t, w, document.NewDocument())
	// flush + merge

	// CMS should have now set true values:
	if cms.MaxMergeCount() == index.AutoDetectMergesAndThreads {
		t.Fatal("assertTrue(cms.getMaxMergeCount() != AUTO_DETECT_MERGES_AND_THREADS)")
	}
	if cms.MaxThreadCount() == index.AutoDetectMergesAndThreads {
		t.Fatal("assertTrue(cms.getMaxThreadCount() != AUTO_DETECT_MERGES_AND_THREADS)")
	}
	mustClose(t, w, dir)
}

func TestConcurrentMergeSchedulerResetToAutoDefault(t *testing.T) {
	cms := index.NewConcurrentMergeScheduler()
	assertCMSCounts(t, cms, index.AutoDetectMergesAndThreads, index.AutoDetectMergesAndThreads)
	if err := cms.SetMaxMergesAndThreads(4, 3); err != nil {
		t.Fatalf("setMaxMergesAndThreads(4, 3): %v", err)
	}
	assertCMSCounts(t, cms, 4, 3)

	if err := cms.SetMaxMergesAndThreads(index.AutoDetectMergesAndThreads, 4); err == nil {
		t.Fatal("expected IllegalArgumentException from setMaxMergesAndThreads(AUTO_DETECT, 4)")
	}

	if err := cms.SetMaxMergesAndThreads(4, index.AutoDetectMergesAndThreads); err == nil {
		t.Fatal("expected IllegalArgumentException from setMaxMergesAndThreads(4, AUTO_DETECT)")
	}

	if err := cms.SetMaxMergesAndThreads(index.AutoDetectMergesAndThreads, index.AutoDetectMergesAndThreads); err != nil {
		t.Fatalf("setMaxMergesAndThreads(AUTO_DETECT, AUTO_DETECT): %v", err)
	}
	assertCMSCounts(t, cms, index.AutoDetectMergesAndThreads, index.AutoDetectMergesAndThreads)
}

func TestConcurrentMergeSchedulerSpinningDefaults(t *testing.T) {
	cms := index.NewConcurrentMergeScheduler()
	cms.SetDefaultMaxMergesAndThreads(true)
	if got := cms.MaxThreadCount(); got != 1 {
		t.Fatalf("getMaxThreadCount(): expected 1, got %d", got)
	}
	if got := cms.MaxMergeCount(); got != 6 {
		t.Fatalf("getMaxMergeCount(): expected 6, got %d", got)
	}
}

func TestConcurrentMergeSchedulerAutoIOThrottleGetter(t *testing.T) {
	cms := index.NewConcurrentMergeScheduler()
	if cms.GetAutoIOThrottle() {
		t.Fatal("assertFalse(cms.getAutoIOThrottle())")
	}
	t.Fatal("org.apache.lucene.index.ConcurrentMergeScheduler#enableAutoIOThrottle() and " +
		"#disableAutoIOThrottle() are not ported (Gocene has SetAutoIOThrottle(bool))")
}

func TestConcurrentMergeSchedulerNonSpinningDefaults(t *testing.T) {
	cms := index.NewConcurrentMergeScheduler()
	cms.SetDefaultMaxMergesAndThreads(false)
	threadCount := cms.MaxThreadCount()
	if !(threadCount >= 1) {
		t.Fatalf("assertTrue(threadCount >= 1): %d", threadCount)
	}
	if !(threadCount <= 4) {
		t.Fatalf("assertTrue(threadCount <= 4): %d", threadCount)
	}
	if got := cms.MaxMergeCount(); got != 5+threadCount {
		t.Fatalf("getMaxMergeCount(): expected %d, got %d", 5+threadCount, got)
	}
}

// LUCENE-6197
func TestConcurrentMergeSchedulerNoStallMergeThreads(t *testing.T) {
	dir := newDirectory()

	iwc := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	iwc.SetMergePolicy(index.NewNoMergePolicy())
	iwc.SetMaxBufferedDocs(2)
	iwc.SetUseCompoundFile(true) // reduce open files
	w := mustNewIndexWriter(t, dir, iwc)
	numDocs := 100
	if testNightly {
		numDocs = 1000
	}
	for i := 0; i < numDocs; i++ {
		doc := document.NewDocument()
		doc.Add(newStringField(t, "field", strconv.Itoa(i), true))
		mustAddDocument(t, w, doc)
	}
	mustClose(t, w)
	defer mustClose(t, dir)

	t.Fatal(cmsHookOverrideMissing("doStall()"))
}

// This test tries to produce 2 merges running concurrently with 2 segments per
// merge. While these merges run we kick off a forceMerge that puts a pending
// merge in the queue but waits for things to happen. While we do this we
// reduce maxMergeCount to 1. If concurrency in CMS is not right the
// forceMerge will wait forever since none of the currently running merges
// picks up the pending merge. This test fails every time.
func TestConcurrentMergeSchedulerChangeMaxMergeCountyWhileForceMerge(t *testing.T) {
	t.Fatal("overriding org.apache.lucene.index.IndexWriter#isEnableTestPoints() in an IndexWriter subclass is not ported")
}
