package index

import (
	"github.com/FlavioCFOliveira/Gocene/search"
)

// NumericDocValues represents a per-document numeric value.
type NumericDocValues interface {
	DocValuesIterator
	// LongValue returns the numeric value for the current document ID.
	// It is illegal to call this method after AdvanceExact(target) returned false.
	LongValue() (int64, error)
	// LongValues is a bulk retrieval of numeric doc values.
	// The docs array is required to be sorted in ascending order with no duplicates.
	LongValues(size int, docs []int32, values []int64, defaultValue int64) error
	// LongValuesOffset is an offset-aware variant of LongValues.
	LongValuesOffset(size int, docs []int32, docsOffset int, values []int64, valuesOffset int, defaultValue int64) error
	// RangeIntoBitSet fills a BitSet with the doc IDs in [fromDoc, toDoc)
	// whose values are in [minValue, maxValue].
	RangeIntoBitSet(fromDoc, toDoc int, minValue, maxValue int64, bitSet search.BitSet, offset int) error
}

// BaseNumericDocValues provides default implementations for NumericDocValues.
// Concrete implementations should embed this and set the Impl field to themselves.
type BaseNumericDocValues struct {
	Impl NumericDocValues
}

// LongValues is a bulk retrieval of numeric doc values.
func (b *BaseNumericDocValues) LongValues(size int, docs []int32, values []int64, defaultValue int64) error {
	return b.LongValuesOffset(size, docs, 0, values, 0, defaultValue)
}

// LongValuesOffset is an offset-aware variant of LongValues.
func (b *BaseNumericDocValues) LongValuesOffset(size int, docs []int32, docsOffset int, values []int64, valuesOffset int, defaultValue int64) error {
	for di := docsOffset, vi := valuesOffset, end := docsOffset + size; di < end; di++, vi++ {
		if ok, err := b.Impl.AdvanceExact(int(docs[di])); ok && err == nil {
			val, err := b.Impl.LongValue()
			if err != nil {
				return err
			}
			values[vi] = val
		} else if err != nil {
			return err
		} else {
			values[vi] = defaultValue
		}
	}
	return nil
}

// RangeIntoBitSet fills a BitSet with the doc IDs in [fromDoc, toDoc)
// whose values are in [minValue, maxValue].
func (b *BaseNumericDocValues) RangeIntoBitSet(fromDoc, toDoc int, minValue, maxValue int64, bitSet search.BitSet, offset int) error {
	for d := fromDoc; d < toDoc; d++ {
		if ok, err := b.Impl.AdvanceExact(d); ok && err == nil {
			v, err := b.Impl.LongValue()
			if err != nil {
				return err
			}
			if v >= minValue && v <= maxValue {
				bitSet.Set(d-offset, toDoc-offset)
			}
		} else if err != nil {
			return err
		}
	}
	return nil
}
