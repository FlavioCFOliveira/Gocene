// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facet

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// TaxonomyWriter is used to write a taxonomy to an index.
//
// This is the Go port of Lucene's org.apache.lucene.facet.taxonomy.TaxonomyWriter.
type TaxonomyWriter struct {
	indexWriter *index.IndexWriter
	field       string
}

// NewTaxonomyWriter creates a new TaxonomyWriter.
func NewTaxonomyWriter(iw *index.IndexWriter, field string) *TaxonomyWriter {
	return &TaxonomyWriter{
		indexWriter: iw,
		field:       field,
	}
}

// AddLabel adds a label to the taxonomy.
func (tw *TaxonomyWriter) AddLabel(label string) (int, error) {
	// In a real implementation, we would check if the label exists and return its ID.
	// For now, we simulate adding it to the index.
	doc := index.NewDocument()
	doc.Add(index.NewStringField(tw.field, label, index.DocValuesOnly))

	err := tw.indexWriter.AddDocument(doc)
	if err != nil {
		return -1, err
	}

	// Return a mock ID for now.
	return 0, nil
}

// AddLabelWithParent adds a label with a parent to the taxonomy.
func (tw *TaxonomyWriter) AddLabelWithParent(label string, parentID int) (int, error) {
	// Simulate adding label with parent relationship.
	doc := index.NewDocument()
	doc.Add(index.NewStringField(tw.field, label, index.DocValuesOnly))
	// Parent ID would be stored in another field.

	err := tw.indexWriter.AddDocument(doc)
	if err != nil {
		return -1, err
	}

	return 0, nil
}

// Commit commits the taxonomy writer.
func (tw *TaxonomyWriter) Commit() error {
	return tw.indexWriter.Commit()
}

// Close closes the taxonomy writer.
func (tw *TaxonomyWriter) Close() error {
	return tw.indexWriter.Close()
}
