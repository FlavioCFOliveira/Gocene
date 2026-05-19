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
// Gocene's Lucene90StoredFieldsFormat writer is still a stub: it stamps
// the MODE_KEY segment attribute and then returns an explicit
// not-implemented error from the underlying Lucene90CompressingStoredFieldsFormat
// (the full writer body is deferred to Sprint 22 along with the chunked
// stored-fields encoding and the GetMergeInstance hook on
// StoredFieldsReader). Until that lands the merge-instance variant is
// observationally identical to the regular format, so this file
// re-asserts the Phase 1 stub-contract in "merge-instance mode" to keep
// the test-surface 1:1 with upstream and to act as a pin: when
// GetMergeInstance lands, this test will be extended to call it and
// assert identical framing against the regular reader.

package codecs_test

import (
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs/lucene90"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestLucene90StoredFieldsFormatMergeInstance_StubContract is the
// merge-instance counterpart of the parent Lucene90StoredFieldsFormat
// writer contract. It mirrors the Java subclass which only flips
// shouldTestMergeInstance to true and otherwise reuses every parent
// assertion.
//
// Once Lucene90CompressingStoredFieldsFormat.FieldsWriter is wired
// (Sprint 22) and StoredFieldsReader grows a GetMergeInstance hook,
// replace the stub-contract assertions below with a real round-trip
// followed by reader.GetMergeInstance().CheckIntegrity().
func TestLucene90StoredFieldsFormatMergeInstance_StubContract(t *testing.T) {
	for _, mode := range []lucene90.Lucene90StoredFieldsMode{
		lucene90.Lucene90StoredFieldsBestSpeed,
		lucene90.Lucene90StoredFieldsBestCompression,
	} {
		mode := mode
		t.Run(mode.String(), func(t *testing.T) {
			t.Parallel()
			f := lucene90.NewLucene90StoredFieldsFormatWithMode(mode)
			si := index.NewSegmentInfo("_0", 0, nil)

			// FieldsWriter stamps the MODE_KEY attribute before delegating
			// to the compressing layer; the compressing layer is still a
			// stub, so the call surfaces an explicit not-implemented error.
			// We assert both the side-effect (the merge-instance variant
			// must agree on the persisted mode tag) and the stub error.
			if _, err := f.FieldsWriter(nil, si, store.IOContext{}); err == nil {
				t.Fatal("FieldsWriter unexpectedly succeeded; expected stub error")
			} else if !strings.Contains(err.Error(), "not implemented") {
				t.Fatalf("unexpected error: %v", err)
			}

			if got, want := si.GetAttribute(lucene90.Lucene90StoredFieldsModeKey), mode.String(); got != want {
				t.Fatalf("MODE_KEY = %q, want %q", got, want)
			}

			// Merge-instance lens. Lucene exposes
			// StoredFieldsReader.getMergeInstance(), defaulting to
			// "return this". Gocene does not yet expose this hook
			// (Sprint 22), and FieldsReader itself is not yet exercised
			// here because the writer stub leaves no on-disk segment to
			// open. When both the writer and GetMergeInstance land,
			// extend this test to open the reader, alias
			// mergeInstance := reader.GetMergeInstance(), and assert
			// CheckIntegrity / VisitDocument parity against the regular
			// reader. The placeholder below preserves the sibling shape
			// (TestLucene90NormsFormatMergeInstance,
			// TestLucene90DocValuesFormatMergeInstance) so the upgrade is
			// a one-line swap.
			_ = f // keep the format reference live for the future merge-instance call
		})
	}
}
