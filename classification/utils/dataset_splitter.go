// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package utils

// DatasetSplitter splits a Lucene index into training, test, and
// cross-validation sub-indexes.
//
// Port of org.apache.lucene.classification.utils.DatasetSplitter.
//
// Deviation: Full implementation deferred to backlog #2693 (requires
// IndexReader, IndexWriter, GroupingSearch, and Directory).
type DatasetSplitter struct {
	crossValidationRatio float64
	testRatio            float64
}

// NewDatasetSplitter creates a DatasetSplitter with the given ratios.
//
//   - testRatio: fraction of the original index to use for the test index
//     (0.0 – 1.0)
//   - crossValidationRatio: fraction to use for cross-validation (0.0 – 1.0)
func NewDatasetSplitter(testRatio, crossValidationRatio float64) *DatasetSplitter {
	return &DatasetSplitter{
		testRatio:            testRatio,
		crossValidationRatio: crossValidationRatio,
	}
}

// Split divides originalIndex into three indexes.
//
// All parameters are interface{} placeholders until the corresponding Gocene
// types (IndexReader, Directory, Analyzer) are stable.
//
// Returns nil — deferred to #2693.
func (s *DatasetSplitter) Split(
	_ interface{}, // originalIndex IndexReader
	_ interface{}, // trainingIndex Directory
	_ interface{}, // testIndex Directory
	_ interface{}, // crossValidationIndex Directory
	_ interface{}, // analyzer Analyzer
	_ bool, // termVectors
	_ string, // classFieldName
	_ ...string, // fieldNames
) error {
	return nil
}
