// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package webapi exposes a small JSON HTTP API that demonstrates how to use
// the Gocene module end-to-end: opening an on-disk index, writing documents,
// running paginated queries through the classic Lucene query parser, and
// rebuilding the index from a golden corpus shipped with the binary.
package webapi

import (
	"errors"
	"strconv"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/document"
)

// Book is the demo domain object exposed over HTTP and stored in the index.
type Book struct {
	ID      string   `json:"id"`
	Title   string   `json:"title"`
	Author  string   `json:"author"`
	Year    int      `json:"year"`
	Tags    []string `json:"tags"`
	Summary string   `json:"summary"`
}

// Indexed field names. These are the only fields accepted by the search
// endpoint's `field` parameter.
const (
	FieldID      = "id"
	FieldTitle   = "title"
	FieldAuthor  = "author"
	FieldYear    = "year"
	FieldTags    = "tags"
	FieldSummary = "summary"
)

// SearchableFields lists the fields targeted when no `field` parameter is
// supplied on the search endpoint. ID and year are exact-match fields and are
// excluded from the default full-text fan-out.
var SearchableFields = []string{FieldTitle, FieldAuthor, FieldTags, FieldSummary}

// AllFields enumerates every field name accepted by the `field` query parameter.
var AllFields = []string{FieldID, FieldTitle, FieldAuthor, FieldYear, FieldTags, FieldSummary}

// IsValidField reports whether name is one of the known indexed fields.
func IsValidField(name string) bool {
	for _, f := range AllFields {
		if f == name {
			return true
		}
	}
	return false
}

// Validate checks the required Book fields. ID is optional on create (the
// store will generate one when empty) but mandatory on update.
func (b *Book) Validate(requireID bool) error {
	if requireID && strings.TrimSpace(b.ID) == "" {
		return errors.New("id is required")
	}
	if strings.TrimSpace(b.Title) == "" {
		return errors.New("title is required")
	}
	if strings.TrimSpace(b.Author) == "" {
		return errors.New("author is required")
	}
	if b.Year < 0 {
		return errors.New("year must be non-negative")
	}
	return nil
}

// toDocument builds a *document.Document with the indexing layout used by
// this example. The id field is a non-tokenised StringField so it can drive
// UpdateDocument/DeleteDocuments by Term. Tags are stored as one TextField
// instance per tag (multi-valued) so they round-trip through GetValues.
func (b *Book) toDocument() (*document.Document, error) {
	doc := document.NewDocument()

	idField, err := document.NewStringField(FieldID, b.ID, true)
	if err != nil {
		return nil, err
	}
	doc.Add(idField)

	titleField, err := document.NewTextField(FieldTitle, b.Title, true)
	if err != nil {
		return nil, err
	}
	doc.Add(titleField)

	authorField, err := document.NewTextField(FieldAuthor, b.Author, true)
	if err != nil {
		return nil, err
	}
	doc.Add(authorField)

	yearField, err := document.NewStringField(FieldYear, strconv.Itoa(b.Year), true)
	if err != nil {
		return nil, err
	}
	doc.Add(yearField)

	for _, tag := range b.Tags {
		tagField, err := document.NewTextField(FieldTags, tag, true)
		if err != nil {
			return nil, err
		}
		doc.Add(tagField)
	}

	summaryField, err := document.NewTextField(FieldSummary, b.Summary, true)
	if err != nil {
		return nil, err
	}
	doc.Add(summaryField)

	return doc, nil
}

// bookFromDocument reconstructs a Book from a stored document retrieved via
// IndexSearcher.Doc(docID). Multi-valued fields use Document.GetValues.
func bookFromDocument(doc *document.Document) Book {
	b := Book{
		ID:      firstValue(doc.GetValues(FieldID)),
		Title:   firstValue(doc.GetValues(FieldTitle)),
		Author:  firstValue(doc.GetValues(FieldAuthor)),
		Tags:    doc.GetValues(FieldTags),
		Summary: firstValue(doc.GetValues(FieldSummary)),
	}
	if y, err := strconv.Atoi(firstValue(doc.GetValues(FieldYear))); err == nil {
		b.Year = y
	}
	return b
}

func firstValue(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}
