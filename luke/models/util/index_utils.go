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

// GetFieldInfo returns the FieldInfo referenced by the field.
//
// Mirrors org.apache.lucene.luke.models.util.IndexUtils#getFieldInfo.
func GetFieldInfo(reader index.IndexReader, fieldName string) (*index.FieldInfo, error) {
	infos, err := GetFieldInfos(reader)
	if err != nil || infos == nil {
		return nil, err
	}
	return infos.FieldInfoByName(fieldName), nil
}

// GetBinaryDocValues returns the BinaryDocValues for the specified field.
//
// Mirrors org.apache.lucene.luke.models.util.IndexUtils#getBinaryDocValues.
func GetBinaryDocValues(reader index.IndexReader, field string) (index.BinaryDocValues, error) {
	if leaf, ok := reader.(index.LeafReader); ok {
		return leaf.GetBinaryDocValues(field)
	}
	return index.MultiDocValuesGetBinaryValues(reader, field)
}

// GetNumericDocValues returns the NumericDocValues for the specified field.
//
// Mirrors org.apache.lucene.luke.models.util.IndexUtils#getNumericDocValues.
func GetNumericDocValues(reader index.IndexReader, field string) (index.NumericDocValues, error) {
	if leaf, ok := reader.(index.LeafReader); ok {
		return leaf.GetNumericDocValues(field)
	}
	return index.MultiDocValuesGetNumericValues(reader, field)
}

// GetSortedNumericDocValues returns the SortedNumericDocValues for the field.
//
// Mirrors IndexUtils#getSortedNumericDocValues.
func GetSortedNumericDocValues(reader index.IndexReader, field string) (index.SortedNumericDocValues, error) {
	if leaf, ok := reader.(index.LeafReader); ok {
		return leaf.GetSortedNumericDocValues(field)
	}
	return index.MultiDocValuesGetSortedNumericValues(reader, field)
}

// GetSortedDocValues returns the SortedDocValues for the specified field.
//
// Mirrors org.apache.lucene.luke.models.util.IndexUtils#getSortedDocValues.
func GetSortedDocValues(reader index.IndexReader, field string) (index.SortedDocValues, error) {
	if leaf, ok := reader.(index.LeafReader); ok {
		return leaf.GetSortedDocValues(field)
	}
	return index.MultiDocValuesGetSortedValues(reader, field)
}

// GetSortedSetDocvalues returns the SortedSetDocValues for the specified field.
//
// The lower-case "v" in "Docvalues" mirrors Lucene's own method name
// (IndexUtils#getSortedSetDocvalues), which is spelled that way upstream.
func GetSortedSetDocvalues(reader index.IndexReader, field string) (index.SortedSetDocValues, error) {
	if leaf, ok := reader.(index.LeafReader); ok {
		return leaf.GetSortedSetDocValues(field)
	}
	return index.MultiDocValuesGetSortedSetValues(reader, field)
}
