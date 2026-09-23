// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build gocene_monsters

// The @Nightly test ports of
// lucene/core/src/test/org/apache/lucene/index/TestIndexWriterCommit.java
// (Apache Lucene 10.5.0), built only with the gocene_monsters tag as Lucene
// runs them only when nightly tests are enabled.

package index_test

import (
	"fmt"
	"io"
	"math/rand"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	testanalysis "github.com/FlavioCFOliveira/Gocene/tests/analysis"
)

// Verify that a writer with "commit on close" indeed cleans up the temp
// segments created after opening that are not referenced by the starting
// segments file. We check this by using MockDirectoryWrapper to measure max
// temp disk space used.
// TODO: can this write less docs/indexes?
func TestIndexWriterCommitCommitOnCloseDiskUsage(t *testing.T) {
	// MemoryCodec, since it uses FST, is not necessarily
	// "additive", ie if you add up N small FSTs, then merge
	// them, the merged result can easily be larger than the
	// sum because the merged FST may use array encoding for
	// some arcs (which uses more space):

	dir := newDirectory()
	analyzer := analysis.NewAnalyzer(nil)
	if rand.Intn(2) == 0 {
		// no payloads
		analyzer.CreateComponents = func(string) *analysis.TokenStreamComponents {
			src := testanalysis.NewMockTokenizer(testanalysis.WHITESPACE, true, testanalysis.DefaultMaxTokenLength)
			return &analysis.TokenStreamComponents{
				Source: func(r io.Reader) error { src.SetReader(r); return nil },
				Sink:   src,
			}
		}
	} else {
		// fixed length payloads
		length := rand.Intn(200)
		analyzer.CreateComponents = func(string) *analysis.TokenStreamComponents {
			tokenizer := testanalysis.NewMockTokenizer(testanalysis.WHITESPACE, true, testanalysis.DefaultMaxTokenLength)
			return &analysis.TokenStreamComponents{
				Source: func(r io.Reader) error { tokenizer.SetReader(r); return nil },
				Sink:   testanalysis.NewMockFixedLengthPayloadFilter(tokenizer, length, rand.New(rand.NewSource(rand.Int63()))),
			}
		}
	}

	conf := newIndexWriterConfigWithAnalyzer(analyzer)
	conf.SetMaxBufferedDocs(10)
	conf.SetReaderPooling(false)
	conf.SetMergePolicy(newLogMergePolicyWithMergeFactor(10))
	writer := mustNewIndexWriter(t, dir, conf)
	for j := 0; j < 30; j++ {
		testIndexWriterAddDocWithIndex(t, writer, j)
	}
	mustClose(t, writer)
	if err := dir.ResetMaxUsedSizeInBytes(); err != nil {
		t.Fatalf("resetMaxUsedSizeInBytes: %v", err)
	}

	dir.SetTrackDiskUsage(true)
	startDiskUsage := dir.GetMaxUsedSizeInBytes()
	conf = newIndexWriterConfigWithAnalyzer(analyzer)
	conf.SetOpenMode(index.Append)
	conf.SetMaxBufferedDocs(10)
	conf.SetMergeScheduler(index.NewSerialMergeScheduler())
	conf.SetReaderPooling(false)
	conf.SetMergePolicy(newLogMergePolicyWithMergeFactor(10))
	writer = mustNewIndexWriter(t, dir, conf)

	for j := 0; j < 1470; j++ {
		testIndexWriterAddDocWithIndex(t, writer, j)
	}
	midDiskUsage := dir.GetMaxUsedSizeInBytes()
	if err := dir.ResetMaxUsedSizeInBytes(); err != nil {
		t.Fatalf("resetMaxUsedSizeInBytes: %v", err)
	}
	if err := writer.ForceMerge(1); err != nil {
		t.Fatalf("forceMerge: %v", err)
	}
	mustClose(t, writer)

	mustClose(t, mustOpenDirectoryReader(t, dir))

	endDiskUsage := dir.GetMaxUsedSizeInBytes()

	// Ending index is 50X as large as starting index; due
	// to 3X disk usage normally we allow 150X max
	// transient usage.  If something is wrong w/ deleter
	// and it doesn't delete intermediate segments then it
	// will exceed this 150X:
	if !(midDiskUsage < 150*startDiskUsage) {
		t.Fatalf("writer used too much space while adding documents: mid=%d start=%d end=%d max=%d",
			midDiskUsage, startDiskUsage, endDiskUsage, startDiskUsage*150)
	}
	if !(endDiskUsage < 150*startDiskUsage) {
		t.Fatalf("writer used too much space after close: endDiskUsage=%d startDiskUsage=%d max=%d",
			endDiskUsage, startDiskUsage, startDiskUsage*150)
	}
	mustClose(t, dir)
}

// LUCENE-2095: make sure with multiple threads commit
// doesn't return until all changes are in fact in the
// index
// TODO: incredibly slow (can we just use 2 threads?)
func TestIndexWriterCommitCommitThreadSafety(t *testing.T) {
	const numThreads = 5
	const maxIterations = 10
	dir := newDirectory()
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetMergePolicy(newLogMergePolicy())
	w := newRandomIndexWriterWithConfig(t, dir, conf)
	reduceOpenFiles(w.W)
	if _, err := w.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	var failed atomic.Bool
	var wg sync.WaitGroup
	for i := 0; i < numThreads; i++ {
		finalI := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			fail := func(err error) {
				failed.Store(true)
				t.Errorf("thread %d: %v", finalI, err)
			}
			doc := document.NewDocument()
			r, err := index.OpenDirectoryReader(dir)
			if err != nil {
				fail(err)
				return
			}
			f := newStringField(t, "f", "", false)
			doc.Add(f)
			iterations := 0
			count := 0
			for {
				if failed.Load() {
					break
				}
				for j := 0; j < 10; j++ {
					s := strconv.Itoa(finalI) + "_" + strconv.Itoa(count)
					count++
					f.SetStringValue(s)
					if _, err := w.AddDocument(doc); err != nil {
						fail(err)
						return
					}
					if _, err := w.Commit(); err != nil {
						fail(err)
						return
					}
					nr, err := index.OpenIfChanged(r)
					if err != nil {
						fail(err)
						return
					}
					if nr == nil {
						fail(fmt.Errorf("assertNotNull(r2)"))
						return
					}
					r2 := nr.(*index.DirectoryReader)
					if r2 == r {
						fail(fmt.Errorf("assertTrue(r2 != r)"))
						return
					}
					if err := r.Close(); err != nil {
						fail(err)
						return
					}
					r = r2
					fail(fmt.Errorf("org.apache.lucene.index.IndexReader#docFreq(Term) is not ported on DirectoryReader (term=f:%s)", s))
					return
				}
				iterations++
				if iterations >= maxIterations {
					break
				}
			}
			if err := r.Close(); err != nil {
				fail(err)
			}
		}()
	}
	wg.Wait()
	if failed.Load() {
		t.Fatal("assertFalse(failed.get())")
	}
	mustClose(t, w, dir)
}
