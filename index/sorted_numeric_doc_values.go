package index

import (
	"github.com/FlavioCFOliveira/Gocene/util"
)

// SortedNumericDocValues defines an interface for per-document numeric values,
// sorted according to Long.compare.
// This is the Go port of Lucene's org.apache.lucene.index.SortedNumericDocValues.
type SortedNumericDocValues interface {
	NumericDocValues
	// NextValue iterates to the next value in the current document.
	// Do not call this more than DocValueCount() times for the document.
	NextValue() (int64, error)
	// DocValueCount retrieves the number of values for the current document.
	// This must always be greater than zero.
	DocValueCount() (int, error)
	// OrdValue returns the ordinal for the current docID.
	OrdValue() (int, error)
	// LookupOrd retrieves the value for the specified ordinal.
	LookupOrd(ord int) (int64, error)
	// GetValueCount returns the number of unique values.
	GetValueCount() (int, error)
}

// SortedRangeIntoBitSet fills a FixedBitSet with the doc IDs in [fromDoc, toDoc)
// whose sorted numeric values contain at least one value in [minValue, maxValue].
// This is the Go port of SortedNumericDocValues.rangeIntoBitSet.
func SortedRangeIntoBitSet(dv SortedNumericDocValues, fromDoc, toDoc int, minValue, maxValue int64, bitSet *util.FixedBitSet, offset int) error {
	for doc := fromDoc; doc < toDoc; doc++ {
		if ok, err := dv.AdvanceExact(doc); err != nil {
			return err
		} else if ok {
			count, err := dv.DocValueCount()
			if err != nil {
				return err
			}
			for i := 0; i < count; i++ {
				val, err := dv.NextValue()
				if err != nil {
					return err
				}
				if val >= minValue {
					if val <= maxValue {
						bitSet.Set(doc - offset)
					}
					break
				}
			}
		}
	}
	return nil
}
