package vectorhighlight

import (
	"github.com/FlavioCFOliveira/Gocene/highlight"
	"github.com/FlavioCFOliveira/Gocene/index"
)

// FragmentsBuilder is an interface for fragments (snippets) builder classes.
// A FragmentsBuilder class can be plugged in to FastVectorHighlighter.
//
// Mirrors org.apache.lucene.search.vectorhighlight.FragmentsBuilder of Apache
// Lucene 10.5.0.
type FragmentsBuilder interface {
	// CreateFragment creates a fragment.
	CreateFragment(reader index.IndexReader, docID int, fieldName string, fieldFragList FieldFragList) (string, error)

	// CreateFragments creates multiple fragments.
	CreateFragments(reader index.IndexReader, docID int, fieldName string, fieldFragList FieldFragList, maxNumFragments int) ([]string, error)

	// CreateFragmentWithTags creates a fragment with specified tags and encoder.
	CreateFragmentWithTags(reader index.IndexReader, docID int, fieldName string, fieldFragList FieldFragList, preTags, postTags []string, encoder highlight.Encoder) (string, error)

	// CreateFragmentsWithTags creates multiple fragments with specified tags and encoder.
	CreateFragmentsWithTags(reader index.IndexReader, docID int, fieldName string, fieldFragList FieldFragList, maxNumFragments int, preTags, postTags []string, encoder highlight.Encoder) ([]string, error)
}
