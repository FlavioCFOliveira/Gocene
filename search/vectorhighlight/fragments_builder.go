package vectorhighlight

import (
	"io"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/highlight"
)

// FragmentsBuilder is an interface for fragments (snippets) builder classes.
// A FragmentsBuilder class can be plugged in to FastVectorHighlighter.
type FragmentsBuilder interface {
	// CreateFragment creates a fragment.
	CreateFragment(reader index.IndexReader, docID int, fieldName string, fieldFragList *FieldFragList) (string, error)

	// CreateFragments creates multiple fragments.
	CreateFragments(reader index.IndexReader, docID int, fieldName string, fieldFragList *FieldFragList, maxNumFragments int) ([]string, error)

	// CreateFragmentWithTags creates a fragment with specified tags and encoder.
	CreateFragmentWithTags(reader index.IndexReader, docID int, fieldName string, fieldFragList *FieldFragList, preTags, postTags []string, encoder highlight.Encoder) (string, error)

	// CreateFragmentsWithTags creates multiple fragments with specified tags and encoder.
	CreateFragmentsWithTags(reader index.IndexReader, docID int, fieldName string, fieldFragList *FieldFragList, maxNumFragments int, preTags, postTags []string, encoder highlight.Encoder) ([]string, error)
}
