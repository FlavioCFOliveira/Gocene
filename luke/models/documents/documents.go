package documents

import (
	"github.com/FlavioCFOliveira/Gocene/index"
)

// Documents is a dedicated interface for Luke's Documents tab.
type Documents interface {
	// GetMaxDoc returns one greater than the largest possible document number.
	GetMaxDoc() int

	// GetFieldNames returns field names in this index.
	GetFieldNames() []string

	// IsLive returns true if the document with the specified docid is not deleted, otherwise false.
	IsLive(docid int) bool

	// GetDocumentFields returns the list of field information and field data for the specified document.
	GetDocumentFields(docid int) ([]*DocumentField, error)

	// GetCurrentField returns the current target field name.
	GetCurrentField() string

	// FirstTerm returns the first indexed term in the specified field.
	FirstTerm(field string) (*index.Term, error)

	// NextTerm increments the terms iterator and returns the next indexed term for the target field.
	NextTerm() (*index.Term, error)

	// SeekTerm seeks to the specified term, if it exists, or to the next (ceiling) term.
	SeekTerm(termText string) (*index.Term, error)

	// FirstTermDoc returns the first document id (posting) associated with the current term.
	FirstTermDoc() (*int, error)

	// NextTermDoc increments the postings iterator and returns the next document id (posting) for the current term.
	NextTermDoc() (*int, error)

	// GetTermPositions returns the list of the position information for the current posting.
	GetTermPositions() ([]*TermPosting, error)

	// GetDocFreq returns the document frequency for the current term.
	GetDocFreq() (*int, error)

	// GetTermVectors returns the term vectors for the specified field in the specified document.
	GetTermVectors(docid int, field string) ([]*TermVectorEntry, error)

	// GetDocValues returns the doc values for the specified field in the specified document.
	GetDocValues(docid int, field string) (*DocValues, error)
}
