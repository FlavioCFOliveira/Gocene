package spi

import (
	"testing"
)

func TestPostingsFormat_Name(t *testing.T) {
	// PostingsFormat interface requires Name() method
	var _ PostingsFormat = nil // verify interface exists
}

func TestDocValuesFormat_Name(t *testing.T) {
	var _ DocValuesFormat = nil
}

func TestNormsFormat_Name(t *testing.T) {
	var _ NormsFormat = nil
}

func TestPointsFormat_Name(t *testing.T) {
	var _ PointsFormat = nil
}

func TestStoredFieldsFormat_Name(t *testing.T) {
	var _ StoredFieldsFormat = nil
}

func TestTermVectorsFormat_Name(t *testing.T) {
	var _ TermVectorsFormat = nil
}

func TestCompoundFormat_Name(t *testing.T) {
	var _ CompoundFormat = nil
}

func TestKnnVectorsFormat_Name(t *testing.T) {
	var _ KnnVectorsFormat = nil
}

func TestSegmentInfoFormat_Name(t *testing.T) {
	var _ SegmentInfoFormat = nil
}

func TestSegmentInfosFormat_Name(t *testing.T) {
	var _ SegmentInfosFormat = nil
}

func TestFieldInfosFormat_Name(t *testing.T) {
	var _ FieldInfosFormat = nil
}
