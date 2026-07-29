// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build gocene_monsters

// Package index_test contains the @Monster / @Nightly test ports from Apache
// Lucene's org.apache.lucene.index.TestIndexWriterDelete.
//
// Source: lucene/core/src/test/org/apache/lucene/index/TestIndexWriterDelete.java
// Reference tag: releases/lucene/10.4.0 (commit 9983b7c)
//
// These tests are excluded from the standard suite because Lucene annotates
// them @Monster or @Nightly. They require the same helper imports as the main
// delete test file and share newDeleteTestAnalyzer, addDoc, updateDoc, getHitCount
// and copyDirectory which are kept in index_writer_delete_test.go.
package index_test

import (
	"fmt"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// ---------------------------------------------------------------------------
// testDeleteAllRepeated (@Monster in Lucene)
// ---------------------------------------------------------------------------

// TestIndexWriterDelete_DeleteAllRepeated ports testDeleteAllRepeated.
//
// Skipped: the Lucene test is annotated @Monster ("Takes 1-2 minutes but
// writes tons of files to disk"); it stress-allocates 50M field numbers to
// provoke OOME. It is not suitable for the standard suite.
func TestIndexWriterDelete_DeleteAllRepeated(t *testing.T) {
	t.Fatal("@Monster in Lucene: 50M-field OOME stress test, excluded from standard suite")
}

// ---------------------------------------------------------------------------
// testDeletesOnDiskFull / testUpdatesOnDiskFull (@Nightly in Lucene)
// ---------------------------------------------------------------------------

// doTestOperationsOnDiskFull ports Lucene's private helper. It builds an
// index with START_COUNT docs, then iterates with ever-increasing free disk
// space applying deletes (or updates) through a MockDirectoryWrapper whose
// size is capped and random I/O exceptions are enabled. The test verifies that
// either all operations succeed (index ends with END_COUNT hits) or the whole
// batch is rolled back transactionally (index still has START_COUNT hits), but
// never a partially-corrupt middle state.
func doTestOperationsOnDiskFull(t *testing.T, updates bool) {
	t.Helper()

	const startCount = 157
	const endCount = 144
	searchTerm := index.NewTerm("content", "aaa")

	startDir := store.NewMockDirectoryWrapper(store.NewByteBuffersDirectory())
	defer startDir.Close()

	config := index.NewIndexWriterConfig(newDeleteTestAnalyzer())
	writer, err := index.NewIndexWriter(startDir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter startDir: %v", err)
	}
	for i := 0; i < startCount; i++ {
		doc := document.NewDocument()
		idField, err := document.NewStringField("id", fmt.Sprintf("%d", i), true)
		if err != nil {
			t.Fatalf("NewStringField: %v", err)
		}
		doc.Add(idField)
		contentField, err := document.NewTextField("content", fmt.Sprintf("aaa %d", i), false)
		if err != nil {
			t.Fatalf("NewTextField: %v", err)
		}
		doc.Add(contentField)
		dvField, err := document.NewNumericDocValuesField("dv", int64(i))
		if err != nil {
			t.Fatalf("NewNumericDocValuesField: %v", err)
		}
		doc.Add(dvField)
		if _, err := writer.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument %d: %v", i, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close startDir writer: %v", err)
	}

	diskUsage, err := startDir.SizeInBytes()
	if err != nil {
		t.Fatalf("startDir.SizeInBytes: %v", err)
	}
	diskFree := diskUsage + 10

	done := false
	var lastErr error
	for !done {
		base := store.NewByteBuffersDirectory()
		copyDirectory(t, startDir, base)
		dir := store.NewMockDirectoryWrapper(base)

		conf := index.NewIndexWriterConfig(newDeleteTestAnalyzer())
		conf.SetMaxBufferedDocs(1000)
		cms := index.NewConcurrentMergeScheduler()
		conf.SetMergeScheduler(cms)
		modifier, err := index.NewIndexWriter(dir, conf)
		if err != nil {
			dir.Close()
			t.Fatalf("NewIndexWriter modifier: %v", err)
		}
		// Enable merge-exception suppression before fault injection starts.
		if scheduler, ok := modifier.GetConfig().GetMergeScheduler().(*index.ConcurrentMergeScheduler); ok {
			scheduler.SetSuppressExceptions()
		}

		success := false
		for x := 0; x < 2; x++ {
			var thisDiskFree int64
			var rate float64
			testName := "disk full during close"
			if x == 0 {
				thisDiskFree = diskFree
				diskRatio := float64(diskFree) / float64(diskUsage)
				rate = 0.1
				if diskRatio >= 2.0 {
					rate /= 2
				}
				if diskRatio >= 4.0 {
					rate /= 2
				}
				if diskRatio >= 6.0 {
					rate = 0.0
				}
				// Small random open failure rate, matching the Java test spirit.
				dir.SetRandomIOExceptionRateOnOpen(0.005)
			} else {
				thisDiskFree = 0
				rate = 0.0
				dir.SetRandomIOExceptionRateOnOpen(0.0)
			}
			dir.SetMaxSizeInBytes(thisDiskFree)
			dir.SetRandomIOExceptionRate(rate)

			if x == 0 {
				docID := 12
				for i := 0; i < 13; i++ {
					if updates {
						doc := document.NewDocument()
						idField, err := document.NewStringField("id", fmt.Sprintf("%d", i), true)
						if err != nil {
							t.Fatalf("NewStringField: %v", err)
						}
						doc.Add(idField)
						contentField, err := document.NewTextField("content", fmt.Sprintf("bbb %d", i), false)
						if err != nil {
							t.Fatalf("NewTextField: %v", err)
						}
						doc.Add(contentField)
						dvField, err := document.NewNumericDocValuesField("dv", int64(i))
						if err != nil {
							t.Fatalf("NewNumericDocValuesField: %v", err)
						}
						doc.Add(dvField)
						if _, err := modifier.UpdateDocument(index.NewTerm("id", fmt.Sprintf("%d", docID)), doc); err != nil {
							lastErr = err
							break
						}
					} else {
						if _, err := modifier.DeleteDocuments(index.NewTerm("id", fmt.Sprintf("%d", docID))); err != nil {
							lastErr = err
							break
						}
					}
					docID += 12
				}
				if lastErr == nil {
					if err := modifier.Close(); err != nil {
						lastErr = err
					}
				}
			}

			if lastErr == nil {
				success = true
				if x == 0 {
					done = true
				}
			} else {
				if x == 1 {
					t.Fatalf("%s: hit IOException after disk space was freed up: %v", testName, lastErr)
				}
			}

			// Disable injection before recovery/verification.
			savedRate := dir.GetRandomIOExceptionRate()
			savedMax := dir.GetMaxSizeInBytes()
			dir.SetRandomIOExceptionRate(0.0)
			dir.SetRandomIOExceptionRateOnOpen(0.0)
			dir.SetMaxSizeInBytes(0)

			if !success {
				_ = modifier.Rollback()
			}

			if success {
				ci, err := index.NewCheckIndex(dir)
				if err != nil {
					t.Fatalf("NewCheckIndex: %v", err)
				}
				_, err = ci.CheckIndex()
				ci.Close()
				if err != nil {
					t.Fatalf("CheckIndex: %v", err)
				}
			}

			result := getHitCount(t, dir, searchTerm)
			if success {
				if x == 0 && result != endCount {
					t.Fatalf("%s: expected %d hits, got %d", testName, endCount, result)
				}
				if x == 1 && result != startCount && result != endCount {
					t.Fatalf("%s: expected %d or %d hits, got %d", testName, startCount, endCount, result)
				}
			} else {
				if result != startCount && result != endCount {
					t.Fatalf("%s: threw but expected %d or %d hits, got %d", testName, startCount, endCount, result)
				}
			}

			if result == endCount {
				dir.Close()
				break
			}

			dir.SetRandomIOExceptionRate(savedRate)
			dir.SetMaxSizeInBytes(savedMax)
			lastErr = nil
		}

		if err := dir.Close(); err != nil {
			t.Fatalf("Close dir: %v", err)
		}
		diskFree += max(10, diskFree>>3)
	}
}

// TestIndexWriterDelete_DeletesOnDiskFull ports testDeletesOnDiskFull.
func TestIndexWriterDelete_DeletesOnDiskFull(t *testing.T) {
	doTestOperationsOnDiskFull(t, false)
}

// TestIndexWriterDelete_UpdatesOnDiskFull ports testUpdatesOnDiskFull.
func TestIndexWriterDelete_UpdatesOnDiskFull(t *testing.T) {
	doTestOperationsOnDiskFull(t, true)
}

// ---------------------------------------------------------------------------
// testIndexingThenDeleting (@Nightly in Lucene)
// ---------------------------------------------------------------------------

// TestIndexWriterDelete_IndexingThenDeleting ports testIndexingThenDeleting.
//
// Skipped: @Nightly in Lucene; it loops on IndexWriter.getFlushCount() until a
// RAM-triggered flush occurs and asserts thousands of operations happened
// first. The RAM-buffer-driven flush counting is timing-sensitive and the
// test is explicitly excluded from the standard suite upstream.
func TestIndexWriterDelete_IndexingThenDeleting(t *testing.T) {
	t.Fatal("@Nightly in Lucene: RAM-buffer flush-count stress test, excluded from standard suite")
}

// ---------------------------------------------------------------------------
// testApplyDeletesOnFlush (@Nightly in Lucene)
// ---------------------------------------------------------------------------

// TestIndexWriterDelete_ApplyDeletesOnFlush ports testApplyDeletesOnFlush.
//
// Skipped: @Nightly in Lucene. It also subclasses IndexWriter to override
// doAfterFlush() and polls slowFileExists for live-docs side files; neither an
// IndexWriter doAfterFlush hook nor slowFileExists is available in Gocene.
func TestIndexWriterDelete_ApplyDeletesOnFlush(t *testing.T) {
	t.Fatal("@Nightly in Lucene; also needs IndexWriter.doAfterFlush override and slowFileExists")
}
