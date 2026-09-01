package blockterms

import (
	"io"

	"github.com/FlavioCFOliveira/Gocene/internal/codecs"
	"github.com/FlavioCFOliveira/Gocene/internal/index"
)

// TermsIndexWriterBase is the base class for terms index implementations.
type TermsIndexWriterBase interface {
	io.Closer
	AddField(fieldInfo *index.FieldInfo, termsFilePointer int64) (FieldWriter, error)
}

// FieldWriter is the API for indexing terms for a single field.
type FieldWriter interface {
	CheckIndexTerm(text []byte, stats codecs.TermStats) (bool, error)
	Add(text []byte, stats codecs.TermStats, termsFilePointer int64) error
	Finish(termsFilePointer int64) error
}
