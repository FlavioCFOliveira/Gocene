package index

import (
	"github.com/FlavioCFOliveira/Gocene/util"
)

// NumericDocValues is a per-document numeric value.
// This is the Go port of Lucene's org.apache.lucene.index.NumericDocValues.
type NumericDocValues interface {
	DocValuesIterator
	// LongValue returns the numeric value for the current document ID.
	// It is illegal to call this method after AdvanceExact(int) returned false.
	LongValue() (int64, error)
}

// LongValues performs bulk retrieval of numeric doc values.
// This is the Go port of NumericDocValues.longValues.
func LongValues(dv NumericDocValues, size int, docs []int, values []int64, defaultValue int64) error {
	return LongValuesWithOffsets(dv, size, docs, 0, values, 0, defaultValue)
}

// LongValuesWithOffsets is an offset-aware variant of LongValues.
// This is the Go port of NumericDocValues.longValues(int, int[], int, long[], int, long).
func LongValuesWithOffsets(dv NumericDocValues, size int, docs []int, docsOffset int, values []int64, valuesOffset int, defaultValue int64) error {
	for di, vi := docsOffset, valuesOffset; di < docsOffset+size; di++, vi++ {
		var value int64
		if ok, err := dv.AdvanceExact(docs[di]); err != nil {
			return err
		} else if ok {
			val, err := dv.LongValue()
			if err != nil {
				return err
			}
			value = val
		} else {
			value = defaultValue
		}
		values[vi] = value
	}
	return nil
}

// RangeIntoBitSet fills a FixedBitSet with the doc IDs in [fromDoc, toDoc)
// whose values are in [minValue, maxValue].
// This is the Go port of NumericDocValues.rangeIntoBitSet.
func RangeIntoBitSet(dv NumericDocValues, fromDoc, toDoc int, minValue, maxValue int64, bitSet *util.FixedBitSet, offset int) error {
	for d := fromDoc; d < toDoc; d++ {
		if ok, err := dv.AdvanceExact(d); err != nil {
			return err
		} else if ok {
			v, err := dv.LongValue()
			if err != nil {
				return err
			}
			if v >= minValue && v <= maxValue {
				bitSet.Set(d - offset)
			}
		}
	}
	return nil
}
