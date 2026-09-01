package search

import (
	"github.com/FlavioCFOliveira/Gocene/index"
)

// NumericFieldStats provides global numeric field statistics from index metadata.
type NumericFieldStats struct{}

// Stats holds the global statistics for a numeric field.
type Stats struct {
	Min      int64
	Max      int64
	DocCount int
}

// GetStats returns the global statistics for the given numeric field across all segments.
func GetStats(reader index.IndexReader, field string) *Stats {
	if result := getStatsFromPoints(reader, field); result != nil {
		return result
	}
	return getStatsFromSkipper(reader, field)
}

func getStatsFromPoints(reader index.IndexReader, field string) *Stats {
	// PointValues utility functions are expected to be in the index package.
	// If they are not implemented, this will return nil.
	minPacked := index.PointValuesGetMinPackedValue(reader, field)
	maxPacked := index.PointValuesGetMaxPackedValue(reader, field)
	if minPacked == nil || maxPacked == nil || len(minPacked) > 8 || len(maxPacked) > 8 {
		return nil
	}
	docCount := index.PointValuesGetDocCount(reader, field)
	return &Stats{
		Min:      decodeLong(minPacked),
		Max:      decodeLong(maxPacked),
		DocCount: docCount,
	}
}

func getStatsFromSkipper(reader index.IndexReader, field string) *Stats {
	var min, max int64
	var minSet, maxSet bool
	docCount := 0
	for _, ctx := range reader.Leaves() {
		leaf := ctx.Reader()
		if leaf.GetFieldInfos().FieldInfo(field) == nil {
			continue
		}
		skipper, err := leaf.GetDocValuesSkipper(field)
		if err != nil || skipper == nil {
			return nil
		}
		if !minSet {
			min = skipper.MinValue()
			max = skipper.MaxValue()
			minSet = true
			maxSet = true
		} else {
			if skipper.MinValue() < min {
				min = skipper.MinValue()
			}
			if skipper.MaxValue() > max {
				max = skipper.MaxValue()
			}
		}
		docCount += skipper.DocCount()
	}
	if !minSet || !maxSet {
		return nil
	}
	return &Stats{
		Min:      min,
		Max:      max,
		DocCount: docCount,
	}
}

func decodeLong(packed []byte) int64 {
	if len(packed) == 0 {
		return 0
	}
	result := int64(byte(packed[0]) ^ 0x80)
	for i := 1; i < len(packed); i++ {
		result = (result << 8) | int64(packed[i]&0xFF)
	}
	return result
}
