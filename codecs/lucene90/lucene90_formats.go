// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene90

// The Sprint 48 lucene90 port surfaces the top-level format types as
// typed stubs alongside the pre-existing concrete helpers
// (indexed_disi.go, the LZ4/Deflate compression-mode pairs). Concrete
// behaviour ports land progressively in follow-up deep-port sprints.
//
// Lucene90StoredFieldsFormat lives in lucene90_stored_fields_format.go.

// Lucene90TermVectorsFormat mirrors
// org.apache.lucene.codecs.lucene90.Lucene90TermVectorsFormat.
type Lucene90TermVectorsFormat struct{}

// NewLucene90TermVectorsFormat builds a Lucene90TermVectorsFormat.
func NewLucene90TermVectorsFormat() *Lucene90TermVectorsFormat {
	return &Lucene90TermVectorsFormat{}
}
