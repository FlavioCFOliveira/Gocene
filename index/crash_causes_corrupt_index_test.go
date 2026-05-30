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
	"testing"

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

// TestCrashCorruptsIndexing ports TestCrashCausesCorruptIndex#testCrashCorruptsIndexing
// (LUCENE-3627).
//
// SKIPPED: this end-to-end scenario drives IndexWriter.Commit through a forced
// failure during pending_segments_2 creation, then reopens the index and
// re-runs DirectoryReader.Open + IndexSearcher.Search. The crash-recovery path
// in IndexWriter (cleanup of a created-but-empty segments_2) and the
// DirectoryReader/IndexSearcher search stack required by searchForFleas are
// not yet wired up in Gocene, so the assertions cannot be exercised. The
// CrashAfterCreateOutput / CrashingException helpers above are kept ready so
// the body can be filled in once those dependencies land.
func TestCrashCorruptsIndexing(t *testing.T) {
	t.Fatal("GOC-4165: IndexWriter crash-recovery and DirectoryReader/IndexSearcher search path not yet available in Gocene")
}
