// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene90

// The Sprint 48 lucene90 port surfaces the top-level format types as
// typed stubs alongside the pre-existing concrete helpers
// (indexed_disi.go, the LZ4/Deflate compression-mode pairs). Concrete
// behaviour ports land progressively in follow-up deep-port sprints.

// Lucene90StoredFieldsFormat mirrors
// org.apache.lucene.codecs.lucene90.Lucene90StoredFieldsFormat.
type Lucene90StoredFieldsFormat struct{}

// NewLucene90StoredFieldsFormat builds a Lucene90StoredFieldsFormat.
func NewLucene90StoredFieldsFormat() *Lucene90StoredFieldsFormat {
	return &Lucene90StoredFieldsFormat{}
}

// Lucene90TermVectorsFormat mirrors
// org.apache.lucene.codecs.lucene90.Lucene90TermVectorsFormat.
type Lucene90TermVectorsFormat struct{}

// NewLucene90TermVectorsFormat builds a Lucene90TermVectorsFormat.
func NewLucene90TermVectorsFormat() *Lucene90TermVectorsFormat {
	return &Lucene90TermVectorsFormat{}
}
