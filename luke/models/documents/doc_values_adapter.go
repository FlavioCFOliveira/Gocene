package documents

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/luke/models/util"
	coreutil "github.com/FlavioCFOliveira/Gocene/util"
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
	finfo, err := util.GetFieldInfo(dva.reader, field)
	if err != nil {
		return nil, err
	}
	if finfo == nil {
		return nil, nil
	}
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
	bvalues, err := util.GetBinaryDocValues(dva.reader, field)
	if err != nil || bvalues == nil {
		return nil, err
	}
	ok, err := bvalues.AdvanceExact(docid)
	if err != nil || !ok {
		return nil, err
	}
	b, err := bvalues.BinaryValue()
	if err != nil {
		return nil, err
	}
	return NewDocValues(
		dvType,
		[]*coreutil.BytesRef{coreutil.BytesRefDeepCopyOf(coreutil.NewBytesRef(b))},
		nil), nil
}

func (dva *DocValuesAdapter) createNumericDocValues(docid int, field string, dvType index.DocValuesType) (*DocValues, error) {
	nvalues, err := util.GetNumericDocValues(dva.reader, field)
	if err != nil || nvalues == nil {
		return nil, err
	}
	ok, err := nvalues.AdvanceExact(docid)
	if err != nil || !ok {
		return nil, err
	}
	v, err := nvalues.LongValue()
	if err != nil {
		return nil, err
	}
	return NewDocValues(dvType, nil, []int64{v}), nil
}

func (dva *DocValuesAdapter) createSortedNumericDocValues(docid int, field string, dvType index.DocValuesType) (*DocValues, error) {
	snvalues, err := util.GetSortedNumericDocValues(dva.reader, field)
	if err != nil || snvalues == nil {
		return nil, err
	}
	ok, err := snvalues.AdvanceExact(docid)
	if err != nil || !ok {
		return nil, err
	}
	dvCount, err := snvalues.DocValueCount()
	if err != nil {
		return nil, err
	}
	var numericValues []int64
	for i := 0; i < dvCount; i++ {
		v, err := snvalues.NextValue()
		if err != nil {
			return nil, err
		}
		numericValues = append(numericValues, v)
	}
	return NewDocValues(dvType, nil, numericValues), nil
}

func (dva *DocValuesAdapter) createSortedDocValues(docid int, field string, dvType index.DocValuesType) (*DocValues, error) {
	svalues, err := util.GetSortedDocValues(dva.reader, field)
	if err != nil || svalues == nil {
		return nil, err
	}
	ok, err := svalues.AdvanceExact(docid)
	if err != nil || !ok {
		return nil, err
	}
	ord, err := svalues.OrdValue()
	if err != nil {
		return nil, err
	}
	b, err := svalues.LookupOrd(ord)
	if err != nil {
		return nil, err
	}
	return NewDocValues(
		dvType,
		[]*coreutil.BytesRef{coreutil.BytesRefDeepCopyOf(coreutil.NewBytesRef(b))},
		nil), nil
}

func (dva *DocValuesAdapter) createSortedSetDocValues(docid int, field string, dvType index.DocValuesType) (*DocValues, error) {
	ssvalues, err := util.GetSortedSetDocvalues(dva.reader, field)
	if err != nil || ssvalues == nil {
		return nil, err
	}
	ok, err := ssvalues.AdvanceExact(docid)
	if err != nil || !ok {
		return nil, err
	}
	var values []*coreutil.BytesRef
	for i := 0; i < ssvalues.DocValueCount(); i++ {
		ord, err := ssvalues.NextOrd()
		if err != nil {
			return nil, err
		}
		b, err := ssvalues.LookupOrd(ord)
		if err != nil {
			return nil, err
		}
		values = append(values, coreutil.BytesRefDeepCopyOf(coreutil.NewBytesRef(b)))
	}
	return NewDocValues(dvType, values, nil), nil
}
