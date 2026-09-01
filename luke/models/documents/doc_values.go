package documents

import (
	"fmt"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// DocValues is a holder for doc values.
type DocValues struct {
	dvType         index.DocValuesType
	values         []*util.BytesRef
	numericValues []int64
}

func NewDocValues(dvType index.DocValuesType, values []*util.BytesRef, numericValues []int64) *DocValues {
	return &DocValues{
		dvType:         dvType,
		values:         values,
		numericValues: numericValues,
	}
}

func (dv *DocValues) DocValuesType() index.DocValuesType {
	return dv.dvType
}

func (dv *DocValues) Values() []*util.BytesRef {
	return dv.values
}

func (dv *DocValues) NumericValues() []int64 {
	return dv.numericValues
}

func (dv *DocValues) String() string {
	var numValuesStr string
	for i, v := range dv.numericValues {
		if i > 0 {
			numValuesStr += ","
		}
		numValuesStr += fmt.Sprintf("%d", v)
	}
	return fmt.Sprintf("DocValues{dvType=%v, values=%v, numericValues=[%s]}",
		dv.dvType, dv.values, numValuesStr)
}
