package util

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// GetFieldInfos returns field FieldInfos in the index.
func GetFieldInfos(reader index.IndexReader) index.FieldInfos {
	if leaf, ok := reader.(*index.LeafReader); ok {
		return leaf.FieldInfos()
	}
	return index.FieldInfosGetMergedFieldInfos(reader)
}

// GetTerms returns the Terms for the specified field.
func GetTerms(reader index.IndexReader, field string) (index.Terms, error) {
	if leaf, ok := reader.(*index.LeafReader); ok {
		return leaf.Terms(field)
	}
	return index.MultiTermsGetTerms(reader, field)
}

// GetLiveDocs returns the Bits representing live documents in the index.
func GetLiveDocs(reader index.IndexReader) *util.Bits {
	if leaf, ok := reader.(*index.LeafReader); ok {
		return leaf.GetLiveDocs()
	}
	return index.MultiBitsGetLiveDocs(reader)
}
