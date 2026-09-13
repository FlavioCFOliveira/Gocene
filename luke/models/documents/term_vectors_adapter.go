package documents

import (
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
	termVectors, err := tva.reader.TermVectors()
	if err != nil {
		return nil, err
	}
	// Java: reader.termVectors().get(docid, field) == get(docid).terms(field).
	fields, err := termVectors.Get(docid)
	if err != nil {
		return nil, err
	}
	if fields == nil {
		return []*TermVectorEntry{}, nil
	}
	termVector, err := fields.Terms(field)
	if err != nil {
		return nil, err
	}
	if termVector == nil {
		return []*TermVectorEntry{}, nil
	}

	var res []*TermVectorEntry
	te, err := termVector.Iterator()
	if err != nil {
		return nil, err
	}
	for {
		next, err := te.Next()
		if err != nil {
			return nil, err
		}
		if next == nil {
			break
		}
		entry, err := NewTermVectorEntry(te)
		if err != nil {
			return nil, err
		}
		res = append(res, entry)
	}
	return res, nil
}
