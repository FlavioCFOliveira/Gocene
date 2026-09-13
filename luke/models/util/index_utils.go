package util

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// GetFieldInfos returns field FieldInfos in the index.
//
// Mirrors org.apache.lucene.luke.models.util.IndexUtils#getFieldInfos.
func GetFieldInfos(reader index.IndexReader) (*index.FieldInfos, error) {
	if leaf, ok := reader.(index.LeafReader); ok {
		return leaf.GetFieldInfos(), nil
	}
	return index.FieldInfosGetMergedFieldInfos(reader)
}

// GetTerms returns the Terms for the specified field.
//
// Mirrors org.apache.lucene.luke.models.util.IndexUtils#getTerms.
func GetTerms(reader index.IndexReader, field string) (index.Terms, error) {
	if leaf, ok := reader.(index.LeafReader); ok {
		return leaf.Terms(field)
	}
	return index.MultiTermsGetTerms(reader, field)
}

// GetLiveDocs returns the Bits representing live documents in the index.
//
// Mirrors org.apache.lucene.luke.models.util.IndexUtils#getLiveDocs.
func GetLiveDocs(reader index.IndexReader) (util.Bits, error) {
	if leaf, ok := reader.(index.LeafReader); ok {
		return leaf.GetLiveDocs(), nil
	}
	return index.MultiBitsGetLiveDocs(reader)
}
