package search

import (
	"bytes"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// BinarySortField is a sort field for BinaryDocValues, usable as an index sort.
type BinarySortField struct {
	SortField
	providerName string
}

func NewBinarySortField(field string, reverse bool) *BinarySortField {
	return NewBinarySortFieldWithMissing(field, reverse, nil)
}

func NewBinarySortFieldWithMissing(field string, reverse bool, missing interface{}) *BinarySortField {
	sf := NewSortField(field, SortFieldTypeCustom)
	sf.Reverse = reverse
	sf.SetMissingValue(missing)
	return &BinarySortField{
		SortField:    *sf,
		providerName: "BinarySortField",
	}
}

// GetSortKeyDocValues returns the per-document sort key as BinaryDocValues.
func (bsf *BinarySortField) GetSortKeyDocValues(reader index.LeafReader) (index.BinaryDocValues, error) {
	return reader.GetBinaryDocValues(bsf.Field)
}

// Compare compares two documents based on the unsigned byte order of the field's value.
func (bsf *BinarySortField) Compare(reader index.LeafReader, doc1, doc2 int) int {
	dv, err := bsf.GetSortKeyDocValues(reader)
	if err != nil {
		return 0
	}
	
	// This is a simplified linear scan.
	// In a real implementation, we would advance the DV to the documents.
	
	val1, _ := dv.BinaryValueAt(doc1)
	val2, _ := dv.BinaryValueAt(doc2)
	
	res := bytes.Compare(val1, val2)
	if bsf.Reverse {
		return -res
	}
	return res
}

func (bsf *BinarySortField) ToString() string {
	return fmt.Sprintf("<binary: \"%s\">", bsf.Field)
}
