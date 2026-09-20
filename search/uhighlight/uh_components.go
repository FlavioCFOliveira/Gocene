package uhighlight

import (
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// UHComponents is a parameter object to hold the components a
// FieldOffsetStrategy needs.
//
// This is the Go port of org.apache.lucene.search.uhighlight.UHComponents from
// Apache Lucene 10.5.0, which declares it as a record; the record components
// become exported struct fields here, in the same order.
type UHComponents struct {
	// Field is the field being highlighted.
	Field string
	// FieldMatcher renders the record component
	// Predicate<String> fieldMatcher.
	FieldMatcher func(string) bool
	// Query is the query being highlighted.
	Query search.Query
	// Terms holds all terms extracted from the query; some may be position
	// sensitive.
	Terms []*util.BytesRef
	// PhraseHelper carries the query's position-sensitive information.
	PhraseHelper *PhraseHelper
	// Automata holds the query's wildcards (i.e. multi-term query), which are
	// not position sensitive.
	Automata []*LabelledCharArrayMatcher
	// HasUnrecognizedQueryPart reports whether part of the query (other than
	// the extracted terms and automata) is a leaf Lucene does not know.
	HasUnrecognizedQueryPart bool
	// HighlightFlags is the set of flags in force for this field.
	HighlightFlags map[HighlightFlag]struct{}
}

// NewUHComponents renders the canonical constructor of the UHComponents
// record.
func NewUHComponents(
	field string,
	fieldMatcher func(string) bool,
	query search.Query,
	terms []*util.BytesRef,
	phraseHelper *PhraseHelper,
	automata []*LabelledCharArrayMatcher,
	hasUnrecognizedQueryPart bool,
	highlightFlags map[HighlightFlag]struct{},
) *UHComponents {
	return &UHComponents{
		Field:                    field,
		FieldMatcher:             fieldMatcher,
		Query:                    query,
		Terms:                    terms,
		PhraseHelper:             phraseHelper,
		Automata:                 automata,
		HasUnrecognizedQueryPart: hasUnrecognizedQueryPart,
		HighlightFlags:           highlightFlags,
	}
}
