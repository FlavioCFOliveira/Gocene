// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/index/TestIndexWriterForceMerge.java
// (Apache Lucene 10.5.0). The @AwaitsFix testMergePerField lives in
// index_writer_force_merge_awaitsfix_test.go.

package index_test

import (
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	testanalysis "github.com/FlavioCFOliveira/Gocene/tests/analysis"
)

func TestIndexWriterForceMergePartialMerge(t *testing.T) {
	dir := newDirectory()

	doc := document.NewDocument()
	doc.Add(newStringField(t, "content", "aaa", false))
	incrMin := 40
	if testNightly {
		incrMin = 15
	}
	for numDocs := 10; numDocs < 500; numDocs += nextInt(incrMin, 5*incrMin) {
		ldmp := index.NewLogDocMergePolicy()
		ldmp.SetMinMergeDocs(1)
		ldmp.SetMergeFactor(5)
		conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
		conf.SetOpenMode(index.Create)
		conf.SetMaxBufferedDocs(2)
		conf.SetMergePolicy(ldmp)
		writer := mustNewIndexWriter(t, dir, conf)
		for j := 0; j < numDocs; j++ {
			mustAddDocument(t, writer, doc)
		}
		mustClose(t, writer)

		sis := mustReadLatestCommit(t, dir)
		segCount := sis.Size()

		ldmp = index.NewLogDocMergePolicy()
		ldmp.SetMergeFactor(5)
		conf = newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
		conf.SetMergePolicy(ldmp)
		writer = mustNewIndexWriter(t, dir, conf)
		if err := writer.ForceMerge(3); err != nil {
			t.Fatalf("forceMerge(3): %v", err)
		}
		mustClose(t, writer)

		sis = mustReadLatestCommit(t, dir)
		optSegCount := sis.Size()

		if segCount < 3 {
			if segCount != optSegCount {
				t.Fatalf("optSegCount: expected %d, got %d", segCount, optSegCount)
			}
		} else if optSegCount != 3 {
			t.Fatalf("optSegCount: expected 3, got %d", optSegCount)
		}
	}
	mustClose(t, dir)
}

func TestIndexWriterForceMergeMaxNumSegments2(t *testing.T) {
	dir := newDirectory()

	doc := document.NewDocument()
	doc.Add(newStringField(t, "content", "aaa", false))

	ldmp := index.NewLogDocMergePolicy()
	ldmp.SetMinMergeDocs(1)
	ldmp.SetMergeFactor(4)
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetMaxBufferedDocs(2)
	conf.SetMergePolicy(ldmp)
	conf.SetMergeScheduler(index.NewConcurrentMergeScheduler())
	writer := mustNewIndexWriter(t, dir, conf)
	defer mustClose(t, writer, dir)

	for i := 0; i < 19; i++ {
		mustAddDocument(t, writer, doc)
	}

	mustCommit(t, writer)
	t.Fatal(indexWriterWaitForMergesMissing)
}

// forceMergeTempSpaceAnalyzer renders the anonymous Analyzer of
// testForceMergeTempSpaceUsage: a lower-casing whitespace MockTokenizer.
func forceMergeTempSpaceAnalyzer() analysis.Analyzer {
	a := analysis.NewAnalyzer(nil)
	a.CreateComponents = func(string) *analysis.TokenStreamComponents {
		src := testanalysis.NewMockTokenizer(testanalysis.WHITESPACE, true, testanalysis.DefaultMaxTokenLength)
		return &analysis.TokenStreamComponents{
			Source: func(r io.Reader) error {
				src.SetReader(r)
				return nil
			},
			Sink: src,
		}
	}
	return a
}

func sumFileLengths(t testing.TB, dir store.Directory) int64 {
	t.Helper()
	files, err := dir.ListAll()
	if err != nil {
		t.Fatalf("listAll: %v", err)
	}
	var total int64
	for _, f := range files {
		n, err := dir.FileLength(f)
		if err != nil {
			t.Fatalf("fileLength(%s): %v", f, err)
		}
		total += n
	}
	return total
}

// Make sure forceMerge doesn't use any more than 1X starting index size as its
// temporary free space required.
func TestIndexWriterForceMergeForceMergeTempSpaceUsage(t *testing.T) {
	dir := newDirectory()
	// don't use MockAnalyzer, variable length payloads can cause merge to make
	// things bigger, since things are optimized for fixed length case. this is a
	// problem for MemoryPF's encoding. (it might have other problems too)
	analyzer := forceMergeTempSpaceAnalyzer()
	conf := newIndexWriterConfigWithAnalyzer(analyzer)
	conf.SetMaxBufferedDocs(10)
	conf.SetMergePolicy(newLogMergePolicy())
	writer := mustNewIndexWriter(t, dir, conf)

	for j := 0; j < 500; j++ {
		testIndexWriterAddDocWithIndex(t, writer, j)
	}
	// force one extra segment w/ different doc store so
	// we see the doc stores get merged
	mustCommit(t, writer)
	testIndexWriterAddDocWithIndex(t, writer, 500)
	mustClose(t, writer)

	startDiskUsage := sumFileLengths(t, dir)
	startListing := forceMergeListFiles(t, dir)

	if err := dir.ResetMaxUsedSizeInBytes(); err != nil {
		t.Fatalf("resetMaxUsedSizeInBytes: %v", err)
	}
	dir.SetTrackDiskUsage(true)

	conf = newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetOpenMode(index.Append)
	conf.SetMergePolicy(newLogMergePolicy())
	writer = mustNewIndexWriter(t, dir, conf)

	if err := writer.ForceMerge(1); err != nil {
		t.Fatalf("forceMerge(1): %v", err)
	}
	mustClose(t, writer)

	finalDiskUsage := sumFileLengths(t, dir)

	// The result of the merged index is often smaller, but sometimes it could
	// be bigger (compression slightly changes, Codec changes etc.). Therefore
	// we compare the temp space used to the max of the initial and final index
	// size
	maxStartFinalDiskUsage := max(startDiskUsage, finalDiskUsage)
	maxDiskUsage := dir.GetMaxUsedSizeInBytes()
	if !(maxDiskUsage <= 4*maxStartFinalDiskUsage) {
		t.Fatalf("forceMerge used too much temporary space: starting usage was %d bytes; final usage was %d bytes; "+
			"max temp usage was %d but should have been at most %d (= 4X starting usage), BEFORE=%sAFTER=%s",
			startDiskUsage, finalDiskUsage, maxDiskUsage, 4*maxStartFinalDiskUsage, startListing, forceMergeListFiles(t, dir))
	}
	mustClose(t, dir)
}

// forceMergeListFiles renders the private listFiles(Directory): a listing of
// files and sizes, recursing into CFS to debug nested files there.
func forceMergeListFiles(t testing.TB, dir store.Directory) string {
	t.Helper()
	infos := mustReadLatestCommit(t, dir)
	var sb strings.Builder
	sb.WriteString("\n")
	for info := range infos.Iterator() {
		for _, file := range info.Files() {
			n, err := dir.FileLength(file)
			if err != nil {
				t.Fatalf("fileLength(%s): %v", file, err)
			}
			fmt.Fprintf(&sb, "%-20s%d\n", file, n)
		}
		if info.SegmentInfo().IsCompoundFile() {
			cfs, err := info.SegmentInfo().Codec().CompoundFormat().GetCompoundReader(dir, info.SegmentInfo())
			if err != nil {
				t.Fatalf("getCompoundReader: %v", err)
			}
			cfsFiles, err := cfs.ListAll()
			if err != nil {
				t.Fatalf("cfs.listAll: %v", err)
			}
			for _, file := range cfsFiles {
				n, err := cfs.FileLength(file)
				if err != nil {
					t.Fatalf("cfs.fileLength(%s): %v", file, err)
				}
				fmt.Fprintf(&sb, " |- (inside compound file) %-20s%d\n", file, n)
			}
			if err := cfs.Close(); err != nil {
				t.Fatalf("cfs.close: %v", err)
			}
		}
	}
	sb.WriteString("\n")
	return sb.String()
}

// Test calling forceMerge(1, false) whereby forceMerge is kicked
// off but we don't wait for it to finish (but
// writer.close()) does wait
func TestIndexWriterForceMergeBackgroundForceMerge(t *testing.T) {
	dir := newDirectory()
	for pass := 0; pass < 2; pass++ {
		conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
		conf.SetOpenMode(index.Create)
		conf.SetMaxBufferedDocs(2)
		conf.SetMergePolicy(newLogMergePolicyWithMergeFactor(51))
		writer := mustNewIndexWriter(t, dir, conf)
		doc := document.NewDocument()
		doc.Add(newStringField(t, "field", "aaa", false))
		for i := 0; i < 100; i++ {
			mustAddDocument(t, writer, doc)
		}
		if _, err := writer.ForceMergeWithObserver(1, false); err != nil {
			t.Fatalf("forceMerge(1, false): %v", err)
		}

		if 0 == pass {
			mustClose(t, writer)
			reader := mustOpenDirectoryReader(t, dir)
			assertLeafCount(t, 1, reader)
			mustClose(t, reader)
		} else {
			// Get another segment to flush so we can verify it is
			// NOT included in the merging
			mustAddDocument(t, writer, doc)
			mustAddDocument(t, writer, doc)
			mustClose(t, writer)

			reader := mustOpenDirectoryReader(t, dir)
			leaves, err := reader.Leaves()
			if err != nil {
				t.Fatalf("leaves: %v", err)
			}
			if !(len(leaves) > 1) {
				t.Fatalf("assertTrue(reader.leaves().size() > 1): %d", len(leaves))
			}
			mustClose(t, reader)

			infos := mustReadLatestCommit(t, dir)
			assertSegmentInfosSize(t, 2, infos)
		}
	}

	mustClose(t, dir)
}
