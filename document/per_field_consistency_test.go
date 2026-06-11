// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// Unit-test replacements for Lucene's TestPerFieldConsistency integration suite.
//
// The upstream tests verify that IndexWriter rejects documents whose per-field
// schema disagrees with previously indexed documents in the same segment.
// That requires a running IndexWriter + DirectoryReader — not yet available.
//
// These replacements exercise the FieldType validation and Document field
// coexistence logic that IS present in the document package today.

// TestPerFieldConsistency verifies that:
//   - A FieldType can be created with consistent indexing options
//   - Different field types (StringField, TextField, IntField, stored-only,
//     vector) coexist correctly in a single Document
//   - Document fields can be iterated and inspected
//   - Field names can be queried (Get, GetFieldNames, HasField)
func TestPerFieldConsistency(t *testing.T) {
	t.Run("consistent_field_type", func(t *testing.T) {
		ft := NewFieldType()
		ft.SetIndexed(true)
		ft.SetIndexOptions(index.IndexOptionsDocsAndFreqs)
		ft.SetStored(true)
		if err := ft.Validate(); err != nil {
			t.Fatalf("consistent FieldType should validate: %v", err)
		}
	})

	t.Run("mixed_field_types_coexist", func(t *testing.T) {
		doc := NewDocument()

		// StringField (indexed, not tokenized, stored)
		sf, err := NewStringField("id", "doc-1", true)
		if err != nil {
			t.Fatal(err)
		}
		doc.Add(sf)

		// TextField (indexed, tokenized, stored)
		tf, err := NewTextField("title", "Hello World", true)
		if err != nil {
			t.Fatal(err)
		}
		doc.Add(tf)

		// IntField (indexed via BKD, not tokenized, not stored)
		inf := NewIntPoint("count", 42)
		doc.Add(inf)

		// Stored-only field (not indexed)
		stof, err := NewStoredField("notes", "some notes")
		if err != nil {
			t.Fatal(err)
		}
		doc.Add(stof)

		// KnnFloatVectorField
		vf, err := NewKnnFloatVectorField("embedding", []float32{1, 2, 3}, index.VectorSimilarityFunctionCosine)
		if err != nil {
			t.Fatal(err)
		}
		doc.Add(vf)

		if doc.Size() != 5 {
			t.Fatalf("document has %d fields, want 5", doc.Size())
		}

		// Verify GetField returns each field by name
		for _, name := range []string{"id", "title", "count", "notes", "embedding"} {
			if doc.Get(name) == nil {
				t.Fatalf("Get(%q) returned nil", name)
			}
		}
	})

	t.Run("field_iteration", func(t *testing.T) {
		doc := NewDocument()
		doc.AddField("a", "1", StringFieldTypeStored)
		doc.AddField("b", "2", StringFieldTypeStored)
		doc.AddField("c", "3", StringFieldTypeStored)

		var names []string
		doc.Iterate(func(f IndexableField) bool {
			names = append(names, f.Name())
			return true
		})
		if len(names) != 3 || names[0] != "a" || names[2] != "c" {
			t.Fatalf("Iterate order wrong: %v", names)
		}

		// Early exit
		count := 0
		doc.Iterate(func(f IndexableField) bool {
			count++
			return f.Name() != "b"
		})
		if count != 2 {
			t.Fatalf("early-exit iteration visited %d fields, want 2", count)
		}
	})

	t.Run("field_name_queries", func(t *testing.T) {
		doc := NewDocument()
		doc.AddField("tag", "go", StringFieldTypeStored)
		doc.AddField("tag", "lucene", StringFieldTypeStored)
		doc.AddField("title", "doc", TextFieldTypeStored)

		if !doc.HasField("tag") {
			t.Fatal("HasField('tag') should be true")
		}
		if doc.HasField("missing") {
			t.Fatal("HasField('missing') should be false")
		}

		names := doc.GetFieldNames()
		if len(names) != 2 {
			t.Fatalf("GetFieldNames = %v, want [tag title]", names)
		}

		if doc.GetFieldCount("tag") != 2 {
			t.Fatalf("GetFieldCount('tag') = %d, want 2", doc.GetFieldCount("tag"))
		}
	})
}

// TestPerFieldConsistency_DocWithMissingSchemaOptionsThrowsError verifies that
// FieldType validation correctly rejects configurations where a required
// constituent property is absent:
//   - Indexed=true without IndexOptions (no point dimensions to compensate)
//   - Tokenized=true without Indexed=true
func TestPerFieldConsistency_DocWithMissingSchemaOptionsThrowsError(t *testing.T) {
	t.Run("indexed_without_index_options", func(t *testing.T) {
		ft := NewFieldType()
		ft.SetIndexed(true)
		// Intentionally omit SetIndexOptions — indexed with IndexOptionsNone
		// and no point dimensions should be rejected.
		if ft.Validate() == nil {
			t.Fatal("expected validation error: indexed field without IndexOptions")
		}
	})

	t.Run("indexed_without_index_options_point_dim_ok", func(t *testing.T) {
		// An indexed field with IndexOptionsNone IS valid when point
		// dimensions are configured (e.g. IntPoint).
		ft := NewFieldType()
		ft.SetIndexed(true)
		ft.SetDimensions(1, 4)
		if err := ft.Validate(); err != nil {
			t.Fatalf("indexed field with point dimensions should validate: %v", err)
		}
	})

	t.Run("tokenized_without_indexed", func(t *testing.T) {
		ft := NewFieldType()
		ft.SetTokenized(true)
		// Indexed defaults to false — tokenized without indexed should fail.
		if ft.Validate() == nil {
			t.Fatal("expected validation error: tokenized field must be indexed")
		}
	})
}

// TestPerFieldConsistency_DocWithExtraSchemaOptionsThrowsError verifies that
// FieldType validation correctly rejects configurations where a dependent
// indexing option is set without its required base option:
//   - StoreTermVectorPositions without StoreTermVectors
//   - StoreTermVectorOffsets without StoreTermVectors
//   - StoreTermVectorPayloads without StoreTermVectors
func TestPerFieldConsistency_DocWithExtraSchemaOptionsThrowsError(t *testing.T) {
	t.Run("positions_without_term_vectors", func(t *testing.T) {
		ft := NewFieldType()
		ft.SetStoreTermVectorPositions(true)
		if ft.Validate() == nil {
			t.Fatal("expected validation error: positions without term vectors")
		}
	})

	t.Run("offsets_without_term_vectors", func(t *testing.T) {
		ft := NewFieldType()
		ft.SetStoreTermVectorOffsets(true)
		if ft.Validate() == nil {
			t.Fatal("expected validation error: offsets without term vectors")
		}
	})

	t.Run("payloads_without_term_vectors", func(t *testing.T) {
		ft := NewFieldType()
		ft.SetStoreTermVectorPayloads(true)
		if ft.Validate() == nil {
			t.Fatal("expected validation error: payloads without term vectors")
		}
	})

	t.Run("all_three_without_term_vectors", func(t *testing.T) {
		ft := NewFieldType()
		ft.SetStoreTermVectorPositions(true)
		ft.SetStoreTermVectorOffsets(true)
		ft.SetStoreTermVectorPayloads(true)
		err := ft.Validate()
		if err == nil {
			t.Fatal("expected validation error for positions/offsets/payloads without term vectors")
		}
	})
}
