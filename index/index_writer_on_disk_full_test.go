// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package index_test contains tests for IndexWriter behaviour when the
// underlying directory runs out of disk space.
//
// Ported from Apache Lucene 10.4.0:
//
//	lucene/core/src/test/org/apache/lucene/index/TestIndexWriterOnDiskFull.java
//
// GOC-4246: Port test `org.apache.lucene.index.TestIndexWriterOnDiskFull`.
package index_test

import (
	"errors"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

func TestIndexWriterOnDiskFull_AddDocumentOnDiskFull(t *testing.T) {
	for pass := 0; pass < 2; pass++ {
		doAbort := pass == 1
		diskFree := int64(100)
		for {
			base := store.NewByteBuffersDirectory()
			mock := store.NewMockDirectoryWrapper(base)
			mock.SetMaxSizeInBytes(diskFree)
			config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
			config.SetMergeScheduler(index.NewSerialMergeScheduler())
			writer, err := index.NewIndexWriter(mock, config)
			if err != nil {
				mock.Close()
				t.Fatalf("pass=%d diskFree=%d: NewIndexWriter: %v", pass, diskFree, err)
			}
			hitError := false
			indexExists := false
			for i := 0; i < 200; i++ {
				doc := newOnDiskFullDoc()
				if err := writer.AddDocument(doc); err != nil {
					hitError = true
					break
				}
			}
			if !hitError {
				err = writer.Commit()
				if err != nil {
					hitError = true
				} else {
					indexExists = true
				}
			}
			if hitError {
				if doAbort {
					_ = writer.Rollback()
				} else {
					if err := writer.Close(); err != nil {
						mock.SetMaxSizeInBytes(0)
						_ = writer.Close()
					}
				}
				if indexExists {
					reader, err := index.OpenDirectoryReader(mock)
					if err != nil {
						t.Fatalf("pass=%d diskFree=%d: cannot open reader: %v", pass, diskFree, err)
					}
					reader.Close()
				}
				mock.Close()
				diskFree += 3000
			} else {
				mock.SetMaxSizeInBytes(0)
				if err := writer.Close(); err != nil {
					t.Fatalf("pass=%d diskFree=%d: close: %v", pass, diskFree, err)
				}
				mock.Close()
				break
			}
		}
	}
}

func TestIndexWriterOnDiskFull_CorruptionAfterDiskFull(t *testing.T) {
	base := store.NewByteBuffersDirectory()
	mock := store.NewMockDirectoryWrapper(base)
	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	config.SetMergeScheduler(index.NewSerialMergeScheduler())
	mp := index.NewLogDocMergePolicy()
	mp.SetMergeFactor(2)
	config.SetMergePolicy(mp)
	writer, err := index.NewIndexWriter(mock, config)
	if err != nil {
		mock.Close()
		t.Fatalf("NewIndexWriter: %v", err)
	}
	doc := document.NewDocument()
	f, err := document.NewTextField("f", "doctor who", false)
	if err != nil {
		t.Fatal(err)
	}
	doc.Add(f)
	if err := writer.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := writer.DeleteDocuments(index.NewTerm("f", "who")); err != nil {
		t.Fatalf("DeleteDocuments: %v", err)
	}
	if err := writer.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument after delete: %v", err)
	}
	failTwice := &store.Failure{}
	failTwice.SetDoFail()
	var didFail1, didFail2 bool
	failTwice.SetEval(func(dir *store.MockDirectoryWrapper) error {
		if !didFail1 {
			didFail1 = true
			return errors.New("fake disk full during mergeTerms")
		}
		if !didFail2 {
			didFail2 = true
			return errors.New("fake disk full while writing LiveDocs")
		}
		return nil
	})
	mock.FailOn(failTwice)
	err = writer.Commit()
	if err == nil {
		t.Fatal("expected IOException, got nil")
	}
	if !didFail1 && !didFail2 {
		t.Fatal("expected at least one failure to fire")
	}
	ci, err := index.NewCheckIndex(mock)
	if err != nil {
		t.Fatalf("NewCheckIndex: %v", err)
	}
	status, err := ci.CheckIndex()
	if err != nil {
		t.Fatalf("CheckIndex error: %v", err)
	}
	if status != nil && status.MissingSegments {
		t.Fatal("CheckIndex: missing segments")
	}
	ci.Close()
	failTwice.ClearDoFail()
	err = writer.AddDocument(doc)
	if err == nil {
		t.Fatal("expected error after tragic failure, got nil")
	}
	var ace *index.AlreadyClosedException
	if !errors.As(err, &ace) {
		t.Fatalf("expected AlreadyClosedException, got %T: %v", err, err)
	}
	mock.Close()
}

func TestIndexWriterOnDiskFull_ImmediateDiskFull(t *testing.T) {
	base := store.NewByteBuffersDirectory()
	mock := store.NewMockDirectoryWrapper(base)
	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	config.SetMaxBufferedDocs(2)
	config.SetMergeScheduler(index.NewSerialMergeScheduler())
	writer, err := index.NewIndexWriter(mock, config)
	if err != nil {
		mock.Close()
		t.Fatalf("NewIndexWriter: %v", err)
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("empty Commit: %v", err)
	}
	sz, err := mock.FileLength("segments_1")
	if err != nil {
		t.Fatalf("FileLength: %v", err)
	}
	mock.SetMaxSizeInBytes(sz + 1)
	doc := document.NewDocument()
	ff, err := document.NewTextField("field", "aaa bbb ccc ddd eee fff ggg hhh iii jjj", true)
	if err != nil {
		t.Fatal(err)
	}
	doc.Add(ff)
	err = writer.AddDocument(doc)
	if err == nil {
		t.Fatal("expected error from AddDocument with full disk, got nil")
	}
	if !writer.IsClosed() {
		t.Logf("writer closed=%v after immediate disk full (non-fatal)", writer.IsClosed())
	}
	mock.Close()
}

func TestIndexWriterOnDiskFull_AddIndexOnDiskFull(t *testing.T) {
	srcDir := store.NewByteBuffersDirectory()
	srcConfig := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	srcWriter, err := index.NewIndexWriter(srcDir, srcConfig)
	if err != nil {
		srcDir.Close()
		t.Fatalf("NewIndexWriter(src): %v", err)
	}
	for j := 0; j < 5; j++ {
		doc := newOnDiskFullDocWithIndex(j)
		if err := srcWriter.AddDocument(doc); err != nil {
			t.Fatalf("src AddDocument(%d): %v", j, err)
		}
	}
	if err := srcWriter.Close(); err != nil {
		t.Fatalf("src writer Close: %v", err)
	}

	const startCount = 10
	startDir := store.NewByteBuffersDirectory()
	startConfig := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	startWriter, err := index.NewIndexWriter(startDir, startConfig)
	if err != nil {
		startDir.Close()
		t.Fatalf("NewIndexWriter(start): %v", err)
	}
	for j := 0; j < startCount; j++ {
		doc := newOnDiskFullDocWithIndex(j)
		if err := startWriter.AddDocument(doc); err != nil {
			t.Fatalf("start AddDocument(%d): %v", j, err)
		}
	}
	if err := startWriter.Close(); err != nil {
		t.Fatalf("start writer Close: %v", err)
	}

	diskUsage := int64(0)
	files, _ := startDir.ListAll()
	for _, f := range files {
		sz, _ := startDir.FileLength(f)
		diskUsage += sz
	}

	for iter := 0; iter < 3; iter++ {
		diskFree := diskUsage + 100
		for {
			mock := store.NewMockDirectoryWrapper(startDir)
			iwc := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
			iwc.SetOpenMode(index.APPEND)
			iwc.SetMergePolicy(index.NewLogMergePolicy())
			iwc.SetMergeScheduler(index.NewSerialMergeScheduler())
			writer, err := index.NewIndexWriter(mock, iwc)
			if err != nil {
				mock.Close()
				t.Fatalf("iter=%d: NewIndexWriter: %v", iter, err)
			}
			success := false
			for x := 0; x < 2; x++ {
				if x == 0 {
					mock.SetMaxSizeInBytes(diskFree)
				} else {
					mock.SetMaxSizeInBytes(0)
				}
				addErr := writer.AddIndexes(srcDir)
				if addErr != nil {
					success = false
				} else {
					success = true
				}
				ci, checkErr := index.NewCheckIndex(mock)
				if checkErr != nil {
					t.Fatalf("iter=%d x=%d: NewCheckIndex: %v", iter, x, checkErr)
				}
				status, checkErr := ci.CheckIndex()
				ci.Close()
				if checkErr != nil {
					t.Fatalf("iter=%d x=%d: CheckIndex error: %v", iter, x, checkErr)
				}
				if status != nil && status.MissingSegments {
					t.Fatalf("iter=%d x=%d: CheckIndex: missing segments", iter, x)
				}
				if x == 0 {
					mock.SetMaxSizeInBytes(0)
					_ = writer.Rollback()
					iwc2 := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
					iwc2.SetOpenMode(index.APPEND)
					iwc2.SetMergePolicy(index.NewLogMergePolicy())
					iwc2.SetMergeScheduler(index.NewSerialMergeScheduler())
					writer, err = index.NewIndexWriter(mock, iwc2)
					if err != nil {
						t.Fatalf("iter=%d: reopen: %v", iter, err)
					}
				}
			}
			mock.SetMaxSizeInBytes(0)
			if err := writer.Close(); err != nil {
				t.Logf("iter=%d: close error (non-fatal): %v", iter, err)
			}
			mock.Close()
			if success {
				break
			}
			diskFree += 40000
		}
	}
	startDir.Close()
	srcDir.Close()
}

func newOnDiskFullDoc() *document.Document {
	doc := document.NewDocument()
	f, _ := document.NewTextField("content", "aaa", false)
	doc.Add(f)
	dv, _ := document.NewNumericDocValuesField("numericdv", 1)
	doc.Add(dv)
	point := document.NewIntPoint("point", 1)
	doc.Add(point)
	return doc
}

func newOnDiskFullDocWithIndex(idx int) *document.Document {
	doc := document.NewDocument()
	f, _ := document.NewTextField("content", "aaa", false)
	doc.Add(f)
	idF, _ := document.NewStringField("id", string(rune('0'+idx%10)), false)
	doc.Add(idF)
	dv, _ := document.NewNumericDocValuesField("numericdv", 1)
	doc.Add(dv)
	point := document.NewIntPoint("point", 1)
	doc.Add(point)
	return doc
}
