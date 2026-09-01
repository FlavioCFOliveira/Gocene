package documents

import (
	"fmt"
	"github.com/FlavioCFOliveira/Gocene/index"
)

// TermVectorsAdapter is an utility class to access to the term vectors.
type TermVectorsAdapter struct {
	reader index.IndexReader
}

func NewTermVectorsAdapter(reader index.IndexReader) *TermVectorsAdapter {
	return &TermVectorsAdapter{
		reader: reader,
	}
}

func (tva *TermVectorsAdapter) GetTermVector(docid int, field string) ([]*TermVectorEntry, error) {
	termVector, err := tva.reader.TermVectors().Get(docid, field)
	if err != nil {
		return nil, err
	}
	if termVector == nil {
		return []*TermVectorEntry{}, nil
	}

	var res []*TermVectorEntry
	te := termVector.Iterator()
	for te.Next() != nil {
		entry, err := NewTermVectorEntry(te)
		if err != nil {
			return nil, err
		}
		res = append(res, entry)
	}
	return res, nil
}
