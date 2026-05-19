// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"testing"
)

// TestPerFieldConsistency ports Lucene's
// org.apache.lucene.document.TestPerFieldConsistency. The original verifies
// that, within a single segment and across segments of the same index,
// IndexWriter rejects documents whose fields disagree with the per-field
// schema previously established for a given field name (missing or extra
// indexing options, doc values, points, or KNN vectors).
//
// In Gocene the canonical IndexWriter / IndexWriterConfig / NoMergePolicy /
// DirectoryReader surface required to drive this scenario is not yet
// available, so the port is staged as a skipping placeholder under
// Sprint 55 option (c). The stub lives in document/ because the test
// exercises consistency between heterogeneous fields declared on the same
// Document, not search-side behaviour.
//
// Tracking: GOC-4013.
func TestPerFieldConsistency(t *testing.T) {
	t.Skip("TestPerFieldConsistency requires IndexWriter/DirectoryReader; deferred (GOC-4013, Sprint 55 option c)")
}

// TestPerFieldConsistency_DocWithMissingSchemaOptionsThrowsError mirrors
// Lucene's testDocWithMissingSchemaOptionsThrowsError. See [TestPerFieldConsistency]
// for the deferral rationale.
func TestPerFieldConsistency_DocWithMissingSchemaOptionsThrowsError(t *testing.T) {
	t.Skip("requires IndexWriter/DirectoryReader; deferred (GOC-4013, Sprint 55 option c)")
}

// TestPerFieldConsistency_DocWithExtraSchemaOptionsThrowsError mirrors
// Lucene's testDocWithExtraSchemaOptionsThrowsError. See [TestPerFieldConsistency]
// for the deferral rationale.
func TestPerFieldConsistency_DocWithExtraSchemaOptionsThrowsError(t *testing.T) {
	t.Skip("requires IndexWriter/DirectoryReader; deferred (GOC-4013, Sprint 55 option c)")
}
