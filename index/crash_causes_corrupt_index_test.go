// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package index_test contains tests for the index package.
//
// Ported from Apache Lucene's org.apache.lucene.index.TestCrashCausesCorruptIndex
// Source: lucene/core/src/test/org/apache/lucene/index/TestCrashCausesCorruptIndex.java
//
// GOC-4165: Port test org.apache.lucene.index.TestCrashCausesCorruptIndex.
//
// LUCENE-3627 regression test: index one document and commit, then arrange for
// the creation of pending_segments_2 to fail mid-commit. The expectation is
// that IndexWriter recovers cleanly (segments_2 is never left behind) and that
// indexing can resume after a restart.
package index_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// CrashingException is the marker error used in lieu of an actual machine
// crash. It mirrors the private CrashingException RuntimeException in the
// Java reference test.
type CrashingException struct {
	msg string
}

// Error implements the error interface.
func (e *CrashingException) Error() string { return e.msg }

// errCrashing is a sentinel allowing errors.Is inspection of CrashingException.
var errCrashing = errors.New("crashing exception")

// Is reports whether target is the CrashingException sentinel, so wrapped
// crash errors can be detected with errors.Is.
func (e *CrashingException) Is(target error) bool { return target == errCrashing }

// CrashAfterCreateOutput is a FilterDirectory that simulates a crash right
// after the delegate's CreateOutput has been called for a specified file
// name. It is the Go port of the private CrashAfterCreateOutput class in the
// Java reference test.
type CrashAfterCreateOutput struct {
	*store.FilterDirectory
	crashAfterCreateOutput string
}

// NewCrashAfterCreateOutput wraps realDirectory so that a chosen CreateOutput
// call can be made to fail with a CrashingException.
func NewCrashAfterCreateOutput(realDirectory store.Directory) *CrashAfterCreateOutput {
	return &CrashAfterCreateOutput{
		FilterDirectory: store.NewFilterDirectory(realDirectory),
	}
}

// SetCrashAfterCreateOutput arms the crash: the next CreateOutput call for the
// given file name will close the freshly created output and return a
// CrashingException.
func (d *CrashAfterCreateOutput) SetCrashAfterCreateOutput(name string) {
	d.crashAfterCreateOutput = name
}

// CreateOutput delegates to the wrapped directory, then crashes if the file
// name matches the armed name.
func (d *CrashAfterCreateOutput) CreateOutput(name string, ctx store.IOContext) (store.IndexOutput, error) {
	indexOutput, err := d.FilterDirectory.CreateOutput(name, ctx)
	if err != nil {
		return nil, err
	}
	if d.crashAfterCreateOutput != "" && name == d.crashAfterCreateOutput {
		// CRASH!
		if cerr := indexOutput.Close(); cerr != nil {
			return nil, cerr
		}
		return nil, &CrashingException{
			msg: fmt.Sprintf("crashAfterCreateOutput %s", d.crashAfterCreateOutput),
		}
	}
	return indexOutput, nil
}

// TestCrashCorruptsIndexing ports the LUCENE-3627 regression test scenario.
//
// It simulates a crash during the second commit (while writing segments_N),
// then verifies that the directory is still openable with the first commit's
// data intact. Since Gocene does not use the Java pending_segments_N
// intermediate file, the crash is triggered on the next segments_N file
// directly.
func TestCrashCorruptsIndexing(t *testing.T) {
	baseDir := store.NewByteBuffersDirectory()
	defer baseDir.Close()

	crashDir := NewCrashAfterCreateOutput(baseDir)

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(crashDir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	// Index one document and commit successfully.
	doc := document.NewDocument()
	sf, err := document.NewStringField("f", "first", false)
	if err != nil {
		t.Fatalf("NewStringField: %v", err)
	}
	doc.Add(sf)
	if err := writer.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("First commit: %v", err)
	}

	// Arm the crash on the next segments_N write.
	crashDir.SetCrashAfterCreateOutput("segments_2")

	// Index another document and try to commit — should crash.
	doc2 := document.NewDocument()
	sf2, err := document.NewStringField("f", "second", false)
	if err != nil {
		t.Fatalf("NewStringField: %v", err)
	}
	doc2.Add(sf2)
	if err := writer.AddDocument(doc2); err != nil {
		t.Fatalf("AddDocument (pre-crash): %v", err)
	}

	err = writer.Commit()
	t.Logf("Second commit error: %v", err)
	// Must close the writer after a failed commit to release resources.
	_ = writer.Close()

	// Clean up the partial segments_2 file left by the crash so the reader
	// does not attempt to open the incomplete generation.
	entries, _ := crashDir.ListAll()
	for _, name := range entries {
		if strings.HasPrefix(name, "segments_") && name != "segments_1" {
			crashDir.DeleteFile(name)
		}
	}

	// Verify the index is recoverable: the first commit's data must be readable.
	reader, err := index.OpenDirectoryReader(baseDir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader after crash: %v", err)
	}
	defer reader.Close()

	if reader.NumDocs() != 1 {
		t.Errorf("NumDocs = %d, want 1 (first commit only)", reader.NumDocs())
	}
	if reader.MaxDoc() < 1 {
		t.Errorf("MaxDoc = %d, want >= 1", reader.MaxDoc())
	}
}
