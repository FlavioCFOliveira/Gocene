// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facet

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// TaxonomyReader is used to read a taxonomy from an index.
//
// This is the Go port of Lucene's org.apache.lucene.facet.taxonomy.TaxonomyReader.
type TaxonomyReader struct {
	reader *index.IndexReader
	field  string
}

// NewTaxonomyReader creates a new TaxonomyReader.
func NewTaxonomyReader(reader *index.IndexReader, field string) *TaxonomyReader {
	return &TaxonomyReader{
		reader: reader,
		field:  field,
	}
}

// GetLabel returns the label for the given ID.
func (tr *TaxonomyReader) GetLabel(id int) (string, error) {
	// In Lucene, the ID is the document ID in the taxonomy index.
	// We retrieve the label from the field.
	doc := id
	if doc < 0 || doc >= tr.reader.MaxDoc() {
		return "", fmt.Errorf("invalid label ID %d", id)
	}

	// Use the reader to get the label.
	// This is a simplification; usually we'd use a specific field reader.
	val, err := tr.reader.GetFieldValue(doc, tr.field)
	if err != nil {
		return "", err
	}
	return val, nil
}

// GetParent returns the parent ID for the given label ID.
func (tr *TaxonomyReader) GetParent(id int) (int, error) {
	// In Lucene, the parent ID is stored in a separate field (e.g., "parent").
	// For now, we return -1.
	return -1, nil
}

// Close closes the taxonomy reader.
func (tr *TaxonomyReader) Close() error {
	return nil
}
