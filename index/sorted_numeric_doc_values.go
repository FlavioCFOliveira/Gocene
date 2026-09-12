package index

import (
	"github.com/FlavioCFOliveira/Gocene/util"
)


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
