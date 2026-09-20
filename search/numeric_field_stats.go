// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"github.com/FlavioCFOliveira/Gocene/index"
)

// NumericFieldStats is a utility class for retrieving global numeric field statistics from index metadata structures,
// without accessing individual documents. It probes PointValues first and falls back to DocValuesSkipper.
// Returns nil when neither structure is available for the field.
//
// Mirrors org.apache.lucene.search.NumericFieldStats from Apache Lucene 10.5.0.
type NumericFieldStats struct{}

// Stats holds the global statistics for a numeric field.
type Stats struct {
	// Min is the global minimum value.
	Min int64
	// Max is the global maximum value.
	Max int64
	// DocCount is the total number of documents containing the field.
	DocCount int
}

// GetStats returns the global statistics for the given numeric field across all segments.
// Probes PointValues first; if unavailable, falls back to DocValuesSkipper.
// Returns nil if neither PointValues nor DocValuesSkipper are available for the field.
func GetStats(reader index.IndexReader, field string) (*Stats, error) {
	result, err := getStatsFromPoints(reader, field)
	if err != nil {
		return nil, err
	}
	if result != nil {
		return result, nil
	}
	return getStatsFromSkipper(reader, field)
}

func getStatsFromPoints(reader index.IndexReader, field string) (*Stats, error) {
	minPacked, err := index.PointValuesGetMinPackedValue(reader, field)
	if err != nil {
		return nil, err
	}
	maxPacked, err := index.PointValuesGetMaxPackedValue(reader, field)
	if err != nil {
		return nil, err
	}

	if minPacked == nil || maxPacked == nil || len(minPacked) > 8 || len(maxPacked) > 8 {
		return nil, nil
	}

	docCount, err := index.PointValuesGetDocCount(reader, field)
	if err != nil {
		return nil, err
	}
	return &Stats{
		Min:      decodeLong(minPacked),
		Max:      decodeLong(maxPacked),
		DocCount: docCount,
	}, nil
}

func getStatsFromSkipper(reader index.IndexReader, field string) (*Stats, error) {
	var min, max int64
	var docCount int
	initialized := false

	leaves, err := reader.Leaves()
	if err != nil {
		return nil, err
	}
	for _, ctx := range leaves {
		leafReader := ctx.LeafReader()
		if leafReader.GetFieldInfos().FieldInfo(field) == nil {
			continue
		}

		skipper, err := leafReader.GetDocValuesSkipper(field)
		if err != nil {
			return nil, err
		}
		if skipper == nil {
			return nil, nil
		}

		if !initialized {
			min = skipper.MinValue()
			max = skipper.MaxValue()
			initialized = true
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

	if !initialized {
		return nil, nil
	}

	return &Stats{
		Min:      min,
		Max:      max,
		DocCount: docCount,
	}, nil
}

// decodeLong decodes a packed []byte point value into an int64.
// Handles any packed value length from 1 to 8 bytes, covering HalfFloatPoint (2 bytes),
// IntField (4 bytes), LongField (8 bytes), and any other width that uses the
// standard sortable encoding (big-endian with sign bit flipped).
func decodeLong(packed []byte) int64 {
	if len(packed) == 0 || len(packed) > 8 {
		panic("packed value length must be between 1 and 8 bytes")
	}

	result := int64(int8(packed[0] ^ 0x80))
	for i := 1; i < len(packed); i++ {
		result = (result << 8) | int64(packed[i]&0xFF)
	}
	return result
}
