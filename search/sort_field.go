package search

import (
	"fmt"
	"io"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// Type specifies the type of the terms to be sorted, or special types such as CUSTOM.
type Type string

const (
	TypeScore      Type = "SCORE"
	TypeDoc        Type = "DOC"
	TypeString     Type = "STRING"
	TypeInt        Type = "INT"
	TypeFloat      Type = "FLOAT"
	TypeLong       Type = "LONG"
	TypeDouble     Type = "DOUBLE"
	TypeCustom     Type = "CUSTOM"
	TypeStringVal  Type = "STRING_VAL"
	TypeRewriteable Type = "REWRITEABLE"
)

var (
	// FieldScore represents sorting by document score (relevance).
	FieldScore = NewSortField(nil, TypeScore)
	// FieldDoc represents sorting by document number (index order).
	FieldDoc = NewSortField(nil, TypeDoc)

	// StringFirst is passed to setMissingValue to have missing string values sort first.
	StringFirst = &stringFirst{}
	// StringLast is passed to setMissingValue to have missing string values sort last.
	StringLast = &stringLast{}
)

type stringFirst struct{}
type stringLast struct{}

func (s *stringFirst) String() string { return "SortField.STRING_FIRST" }
func (s *stringLast) String() string  { return "SortField.STRING_LAST" }

// SortField stores information about how to sort documents by terms in an individual field.
// Fields must be indexed in order to sort by them.
type SortField struct {
	field            string
	fieldType       Type
	reverse          bool
	comparatorSource FieldComparatorSource
	missingValue     any
	// optimizeSortWithIndexedData indicates if sort should be optimized with indexed data.
	// Deprecated: remove in Lucene 10.
	optimizeSortWithIndexedData bool
}

// NewSortField creates a sort by terms in the given field with the type of term values explicitly given.
func NewSortField(field string, fieldType Type) *SortField {
	return NewSortFieldWithReverse(field, fieldType, false, nil)
}

// NewSortFieldWithReverse creates a sort by terms in the given field with the type of term values explicitly given.
func NewSortFieldWithReverse(field string, fieldType Type, reverse bool) *SortField {
	return NewSortFieldWithMissing(field, fieldType, reverse, nil)
}

// NewSortFieldWithMissing creates a sort, possibly in reverse, by terms in the given field with the type of term values explicitly given.
func NewSortFieldWithMissing(field string, fieldType Type, reverse bool, missingValue any) *SortField {
	sf := &SortField{
		field:                      field,
		fieldType:                  fieldType,
		reverse:                    reverse,
		missingValue:               missingValue,
		optimizeSortWithIndexedData: true,
	}
	sf.validateField(field, fieldType, missingValue)
	return sf
}

// NewCustomSortField creates a sort with a custom comparison function.
func NewCustomSortField(field string, comparator FieldComparatorSource) *SortField {
	return NewCustomSortFieldWithReverse(field, comparator, false)
}

// NewCustomSortFieldWithReverse creates a sort, possibly in reverse, with a custom comparison function.
func NewCustomSortFieldWithReverse(field string, comparator FieldComparatorSource, reverse bool) *SortField {
	sf := &SortField{
		field:            field,
		fieldType:        TypeCustom,
		reverse:          reverse,
		comparatorSource: comparator,
		missingValue:    nil, // missingValue factored into comparator source
		optimizeSortWithIndexedData: true,
	}
	sf.validateField(field, TypeCustom, nil)
	return sf
}

func (sf *SortField) validateField(field string, fieldType Type, missingValue any) {
	if field == "" {
		if fieldType != TypeScore && fieldType != TypeDoc {
			panic("field can only be null when type is SCORE or DOC")
		}
	}
	if fieldType == TypeString {
		if missingValue != nil && missingValue != StringFirst && missingValue != StringLast {
			panic("for Type.STRING, missing value must be either STRING_FIRST or STRING_LAST")
		}
	}
}

func (sf *SortField) GetField() string {
	return sf.field
}

func (sf *SortField) GetType() Type {
	return sf.fieldType
}

func (sf *SortField) GetReverse() bool {
	return sf.reverse
}

func (sf *SortField) GetComparatorSource() FieldComparatorSource {
	return sf.comparatorSource
}

func (sf *SortField) GetMissingValue() any {
	return sf.missingValue
}

// SetMissingValue sets the value to use for documents that don't have a value.
// Deprecated: remove in Lucene 10.
func (sf *SortField) SetMissingValue(missingValue any) {
	if sf.fieldType == TypeString || sf.fieldType == TypeStringVal {
		if missingValue != nil && missingValue != StringFirst && missingValue != StringLast {
			panic("for STRING type, missing value must be either STRING_FIRST or STRING_LAST")
		}
	} else if sf.fieldType == TypeInt {
		if missingValue != nil {
			if _, ok := missingValue.(int); !ok {
				panic(fmt.Sprintf("missing values for Type.INT can only be of type int, but got %T", missingValue))
			}
		}
	} else if sf.fieldType == TypeLong {
		if missingValue != nil {
			if _, ok := missingValue.(int64); !ok {
				panic(fmt.Sprintf("missing values for Type.LONG can only be of type int64, but got %T", missingValue))
			}
		}
	} else if sf.fieldType == TypeFloat {
		if missingValue != nil {
			if _, ok := missingValue.(float32); !ok {
				panic(fmt.Sprintf("missing values for Type.FLOAT can only be of type float32, but got %T", missingValue))
			}
		}
	} else if sf.fieldType == TypeDouble {
		if missingValue != nil {
			if _, ok := missingValue.(float64); !ok {
				panic(fmt.Sprintf("missing values for Type.DOUBLE can only be of type float64, but got %T", missingValue))
			}
		}
	} else {
		panic("missing value only works for numeric or STRING types")
	}
	sf.missingValue = missingValue
}

func (sf *SortField) String() string {
	var buffer string
	switch sf.fieldType {
	case TypeScore:
		buffer = "<score>"
	case TypeDoc:
		buffer = "<doc>"
	case TypeString:
		buffer = fmt.Sprintf("<string: \"%s\">", sf.field)
	case TypeStringVal:
		buffer = fmt.Sprintf("<string_val: \"%s\">", sf.field)
	case TypeInt:
		buffer = fmt.Sprintf("<int: \"%s\">", sf.field)
	case TypeLong:
		buffer = fmt.Sprintf("<long: \"%s\">", sf.field)
	case TypeFloat:
		buffer = fmt.Sprintf("<float: \"%s\">", sf.field)
	case TypeDouble:
		buffer = fmt.Sprintf("<double: \"%s\">", sf.field)
	case TypeCustom:
		buffer = fmt.Sprintf("<custom:\"%s\": %v>", sf.field, sf.comparatorSource)
	case TypeRewriteable:
		buffer = fmt.Sprintf("<rewriteable: \"%s\">", sf.field)
	default:
		buffer = fmt.Sprintf("<???: \"%s\">", sf.field)
	}

	if sf.reverse {
		buffer += "!"
	}
	if sf.missingValue != nil {
		buffer += fmt.Sprintf(" missingValue=%v", sf.missingValue)
	}

	return buffer
}

func (sf *SortField) Equals(other *SortField) bool {
	if sf == other {
		return true
	}
	if other == nil {
		return false
	}
	return sf.field == other.field &&
		sf.fieldType == other.fieldType &&
		sf.reverse == other.reverse &&
		sf.comparatorSource == other.comparatorSource &&
		sf.missingValue == other.missingValue
}

func (sf *SortField) HashCode() int {
	// Simple hash combination for Go
	h := 17
	h = 31*h + len(sf.field)
	h = 31*h + int(len(sf.fieldType))
	if sf.reverse {
		h = 31*h + 1
	} else {
		h = 31*h + 0
	}
	// Note: in a real implementation, we'd want to use a proper hash for the objects
	return h
}

func (sf *SortField) GetComparator(numHits int, pruning Pruning) FieldComparator {
	var fieldComparator FieldComparator
	switch sf.fieldType {
	case TypeScore:
		// fieldComparator = NewRelevanceComparator(numHits)
		// TODO: implement RelevanceComparator
	case TypeDoc:
		// fieldComparator = NewDocComparator(numHits, sf.reverse, pruning)
		// TODO: implement DocComparator
	case TypeInt:
		// fieldComparator = NewIntComparator(numHits, sf.field, sf.missingValue.(int), sf.reverse, pruning)
		// TODO: implement IntComparator
	case TypeFloat:
		// fieldComparator = NewFloatComparator(numHits, sf.field, sf.missingValue.(float32), sf.reverse, pruning)
		// TODO: implement FloatComparator
	case TypeLong:
		// fieldComparator = NewLongComparator(numHits, sf.field, sf.missingValue.(int64), sf.reverse, pruning)
		// TODO: implement LongComparator
	case TypeDouble:
		// fieldComparator = NewDoubleComparator(numHits, sf.field, sf.missingValue.(float64), sf.reverse, pruning)
		// TODO: implement DoubleComparator
	case TypeCustom:
		if sf.comparatorSource == nil {
			panic("comparatorSource cannot be nil for TypeCustom")
		}
		fieldComparator = sf.comparatorSource.NewComparator(sf.field, numHits, pruning, sf.reverse)
	case TypeString:
		// fieldComparator = NewTermOrdValComparator(numHits, sf.field, sf.missingValue == StringLast, sf.reverse, pruning)
		// TODO: implement TermOrdValComparator
	case TypeStringVal:
		// fieldComparator = NewTermValComparator(numHits, sf.field, sf.missingValue == StringLast)
		// TODO: implement TermValComparator
	case TypeRewriteable:
		panic("SortField needs to be rewritten through Sort.rewrite(..) and SortField.rewrite(..)")
	default:
		panic(fmt.Sprintf("Illegal sort type: %s", sf.fieldType))
	}

	if !sf.getOptimizeSortWithIndexedData() {
		if fieldComparator != nil {
			fieldComparator.DisableSkipping()
		}
	}
	return fieldComparator
}

func (sf *SortField) Rewrite(searcher *IndexSearcher) (*SortField, error) {
	return sf, nil
}

func (sf *SortField) NeedsScores() bool {
	return sf.fieldType == TypeScore
}

func (sf *SortField) GetIndexSorter() *index.IndexSorter {
	switch sf.fieldType {
	case TypeString:
		// return index.NewStringSorter(ProviderName, sf.missingValue, sf.reverse, func(reader index.Reader) index.DocValues {
		// 	return index.GetSorted(reader, sf.field)
		// })
	case TypeInt:
		// return index.NewIntSorter(ProviderName, sf.missingValue.(int), sf.reverse, func(reader index.Reader) index.DocValues {
		// 	return index.GetNumeric(reader, sf.field)
		// })
	case TypeLong:
		// return index.NewLongSorter(ProviderName, sf.missingValue.(int64), sf.reverse, func(reader index.Reader) index.DocValues {
		// 	return index.GetNumeric(reader, sf.field)
		// })
	case TypeDouble:
		// return index.NewDoubleSorter(ProviderName, sf.missingValue.(float64), sf.reverse, func(reader index.Reader) index.DocValues {
		// 	return index.GetNumeric(reader, sf.field)
		// })
	case TypeFloat:
		// return index.NewFloatSorter(ProviderName, sf.missingValue.(float32), sf.reverse, func(reader index.Reader) index.DocValues {
		// 	return index.GetNumeric(reader, sf.field)
		// })
	}
	return nil
}

func (sf *SortField) setOptimizeSortWithIndexedData(optimize bool) {
	sf.optimizeSortWithIndexedData = optimize
}

func (sf *SortField) getOptimizeSortWithIndexedData() bool {
	return sf.optimizeSortWithIndexedData
}

func (sf *SortField) SetOptimizeSortWithPoints(optimize bool) {
	sf.setOptimizeSortWithIndexedData(optimize)
}

func (sf *SortField) GetOptimizeSortWithPoints() bool {
	return sf.getOptimizeSortWithIndexedData()
}

// Provider is a SortFieldProvider for field sorts.
type Provider struct {
	index.SortFieldProvider
}

const ProviderName = "SortField"

func NewProvider() *Provider {
	return &Provider{}
}

func (p *Provider) ReadSortField(in io.Reader) (*SortField, error) {
	// This requires DataInput-like reader.
	// In Gocene, this would use a specific binary reader.
	// For the sake of faithful translation, the logic is:
	// field := in.readString()
	// type := readType(in)
	// ...
	return nil, fmt.Errorf("ReadSortField not yet implemented: requires DataInput")
}

func (p *Provider) WriteSortField(sf *SortField, out io.Writer) error {
	return sf.serialize(out)
}

func readType(in io.Reader) (Type, error) {
	// typeStr := in.readString()
	// return Type(typeStr), nil
	return "", nil
}

func (sf *SortField) serialize(out io.Writer) error {
	// Logic from Java:
	// out.writeString(field)
	// out.writeString(type.toString())
	// out.writeInt(reverse ? 1 : 0)
	// ...
	return fmt.Errorf("serialize not yet implemented: requires DataOutput")
}
