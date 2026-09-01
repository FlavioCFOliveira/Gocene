package documents

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/luke/models/util"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// DocValuesAdapter is an utility class to access to the doc values.
type DocValuesAdapter struct {
	reader index.IndexReader
}

func NewDocValuesAdapter(reader index.IndexReader) *DocValuesAdapter {
	return &DocValuesAdapter{
		reader: reader,
	}
}

func (dva *DocValuesAdapter) GetDocValues(docid int, field string) (*DocValues, error) {
	finfo := util.GetFieldInfo(dva.reader, field)
	dvType := finfo.DocValuesType()

	switch dvType {
	case index.DocValuesTypeBinary:
		return dva.createBinaryDocValues(docid, field, index.DocValuesTypeBinary)
	case index.DocValuesTypeNumeric:
		return dva.createNumericDocValues(docid, field, index.DocValuesTypeNumeric)
	case index.DocValuesTypeSortedNumeric:
		return dva.createSortedNumericDocValues(docid, field, index.DocValuesTypeSortedNumeric)
	case index.DocValuesTypeSorted:
		return dva.createSortedDocValues(docid, field, index.DocValuesTypeSorted)
	case index.DocValuesTypeSortedSet:
		return dva.createSortedSetDocValues(docid, field, index.DocValuesTypeSortedSet)
	default:
		return nil, nil
	}
}

func (dva *DocValuesAdapter) createBinaryDocValues(docid int, field string, dvType index.DocValuesType) (*DocValues, error) {
	bvalues := util.GetBinaryDocValues(dva.reader, field)
	if bvalues.AdvanceExact(docid) {
		return NewDocValues(
			dvType,
			[]*util.BytesRef{util.BytesRefDeepCopyOf(bvalues.BinaryValue())},
			nil), nil
	}
	return nil, nil
}

func (dva *DocValuesAdapter) createNumericDocValues(docid int, field string, dvType index.DocValuesType) (*DocValues, error) {
	nvalues := util.GetNumericDocValues(dva.reader, field)
	if nvalues.AdvanceExact(docid) {
		return NewDocValues(
			dvType,
			nil,
			[]int64{nvalues.LongValue()}), nil
	}
	return nil, nil
}

func (dva *DocValuesAdapter) createSortedNumericDocValues(docid int, field string, dvType index.DocValuesType) (*DocValues, error) {
	snvalues := util.GetSortedNumericDocValues(dva.reader, field)
	if snvalues.AdvanceExact(docid) {
		var numericValues []int64
		dvCount := snvalues.DocValueCount()
		for i := 0; i < dvCount; i++ {
			numericValues = append(numericValues, snvalues.NextValue())
		}
		return NewDocValues(dvType, nil, numericValues), nil
	}
	return nil, nil
}

func (dva *DocValuesAdapter) createSortedDocValues(docid int, field string, dvType index.DocValuesType) (*DocValues, error) {
	svalues := util.GetSortedDocValues(dva.reader, field)
	if svalues.AdvanceExact(docid) {
		return NewDocValues(
			dvType,
			[]*util.BytesRef{util.BytesRefDeepCopyOf(svalues.LookupOrd(svalues.OrdValue()))},
			nil), nil
	}
	return nil, nil
}

func (dva *DocValuesAdapter) createSortedSetDocValues(docid int, field string, dvType index.DocValuesType) (*DocValues, error) {
	ssvalues := util.GetSortedSetDocvalues(dva.reader, field)
	if ssvalues.AdvanceExact(docid) {
		var values []*util.BytesRef
		for i := 0; i < ssvalues.DocValueCount(); i++ {
			values = append(values, util.BytesRefDeepCopyOf(ssvalues.LookupOrd(ssvalues.NextOrd())))
		}
		return NewDocValues(dvType, values, nil), nil
	}
	return nil, nil
}
