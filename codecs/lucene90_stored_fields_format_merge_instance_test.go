// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Source: lucene/core/src/test/org/apache/lucene/codecs/lucene90/TestLucene90StoredFieldsFormatMergeInstance.java
//
// In Lucene, TestLucene90StoredFieldsFormatMergeInstance extends
// TestLucene90StoredFieldsFormat and overrides shouldTestMergeInstance()
// to return true, causing the BaseIndexFileFormatTestCase harness to
// wrap every DirectoryReader with MergingDirectoryReaderWrapper so the
// inherited StoredFieldsReader tests exercise the merge-optimised
// instance returned by StoredFieldsReader.getMergeInstance().
//
// This file was updated in Sprint 116 T4640 after
// Lucene90CompressingStoredFieldsFormat.FieldsWriter was fully implemented
// (the stub "not-implemented" path is gone). The test now exercises the
// real writer lifecycle. GetMergeInstance is not yet exposed; once that
// hook lands, extend the test to call reader.GetMergeInstance().

package codecs_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs/lucene90"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestLucene90StoredFieldsFormatMergeInstance_StubContract verifies that
// the FieldsWriter correctly stamps the MODE_KEY segment attribute and
// opens without error when given a real directory and segment. It also
// asserts that writing and closing a zero-document segment succeeds,
// mirroring the merge-instance lifecycle in Lucene's
// TestLucene90StoredFieldsFormatMergeInstance.
//
// The previous "stub error" assertions were removed now that
// Lucene90CompressingStoredFieldsFormat.FieldsWriter is fully implemented.
// GetMergeInstance is not yet exposed; once that hook lands, extend this
// test to call reader.GetMergeInstance().CheckIntegrity() and assert
// VisitDocument parity.
func TestLucene90StoredFieldsFormatMergeInstance_StubContract(t *testing.T) {
	for _, mode := range []lucene90.Lucene90StoredFieldsMode{
		lucene90.Lucene90StoredFieldsBestSpeed,
		lucene90.Lucene90StoredFieldsBestCompression,
	} {
		mode := mode
		t.Run(mode.String(), func(t *testing.T) {
			t.Parallel()
			f := lucene90.NewLucene90StoredFieldsFormatWithMode(mode)
			dir, err := store.NewSimpleFSDirectory(t.TempDir())
			if err != nil {
				t.Fatalf("create dir: %v", err)
			}
			defer dir.Close()
			si := index.NewSegmentInfo("_0", 0, dir)
			if err := si.SetID(make([]byte, 16)); err != nil {
				t.Fatalf("set segment ID: %v", err)
			}

			// FieldsWriter stamps the MODE_KEY attribute before delegating.
			w, err := f.FieldsWriter(dir, si, store.IOContext{})
			if err != nil {
				t.Fatalf("FieldsWriter: %v", err)
			}

			// MODE_KEY must be stamped immediately.
			if got, want := si.GetAttribute(lucene90.Lucene90StoredFieldsModeKey), mode.String(); got != want {
				t.Fatalf("MODE_KEY = %q, want %q", got, want)
			}

			// Close should succeed even with zero documents.
			if err := w.Close(); err != nil {
				t.Fatalf("Close: %v", err)
			}
		})
	}
}
