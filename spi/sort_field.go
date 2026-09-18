package spi

import "fmt"

// SortFieldType represents the type of a sort field.
type SortFieldType int

const (
	// SortFieldTypeScore sorts by relevance score.
	SortFieldTypeScore SortFieldType = iota
	// SortFieldTypeDoc sorts by document ID.
	SortFieldTypeDoc
	// SortFieldTypeString sorts by string value.
	SortFieldTypeString
	// SortFieldTypeInt sorts by integer value.
	SortFieldTypeInt
	// SortFieldTypeLong sorts by long value.
	SortFieldTypeLong
	// SortFieldTypeFloat sorts by float value.
	SortFieldTypeFloat
	// SortFieldTypeDouble sorts by double value.
	SortFieldTypeDouble
	// SortFieldTypeCustom sorts using a custom FieldComparatorSource.
	SortFieldTypeCustom
	// SortFieldTypeStringVal sorts using term values as strings, but comparing
	// by value rather than by ordinal. Mirrors SortField.Type.STRING_VAL.
	SortFieldTypeStringVal
	// SortFieldTypeRewriteable forces rewriting of the SortField through
	// SortField.rewrite before it can be used for sorting. Mirrors
	// SortField.Type.REWRITEABLE.
	SortFieldTypeRewriteable
)

// MissingValueStrategy defines how to handle missing values during sorting.
type MissingValueStrategy int

const (
	// MissingValueLast places documents with missing values at the end.
	MissingValueLast MissingValueStrategy = iota
	// MissingValueFirst places documents with missing values at the beginning.
	MissingValueFirst
	// MissingValueUseDefault uses a default value for missing documents.
	MissingValueUseDefault
)

// stringSentinel is the opaque type used for the STRING_FIRST / STRING_LAST
// sentinels. Identity comparison (pointer equality) is the only valid test.
type stringSentinel struct{ name string }

// String mirrors the toString() of the anonymous STRING_FIRST / STRING_LAST
// objects declared in SortField.java, which render as "SortField.STRING_FIRST"
// and "SortField.STRING_LAST".
func (s *stringSentinel) String() string { return "SortField." + s.name }

// STRING_FIRST is the missing-value sentinel that sorts missing values first.
var STRING_FIRST = &stringSentinel{"STRING_FIRST"}

// STRING_LAST is the missing-value sentinel that sorts missing values last.
var STRING_LAST = &stringSentinel{"STRING_LAST"}

// SortField defines how to sort documents by a specific field.
type SortField struct {
	// Field is the name of the field to sort by.
	Field string

	// Type is the type of the sort field.
	Type SortFieldType

	// Reverse sorts in descending order if true.
	Reverse bool

	// Missing is the strategy for handling missing values.
	Missing MissingValueStrategy

	// MissingValue is the value to use for missing documents (for numeric types).
	MissingValue interface{}

	// optimizeSortWithIndexedData mirrors SortField.optimizeSortWithIndexedData
	optimizeSortWithIndexedData bool

	// optimizeSet records whether optimizeSortWithIndexedData was explicitly set.
	optimizeSet bool

	// Selector is used for multi-valued fields (min, max, etc.).
	Selector string

	// comparatorSource holds the custom FieldComparatorSource for a
	// SortFieldTypeCustom sort.
	comparatorSource any // Use any to avoid importing search.FieldComparatorSource

	// docValuesSource holds the per-leaf DocValues override that a SortField
	// subclass installs on the comparator its type already selects.
	//
	// Apache Lucene 10.5.0 lets a SortField subclass keep a real Type
	// (STRING, INT, LONG, FLOAT, DOUBLE) and still change where the values
	// come from, by overriding getComparator to return the standard
	// comparator for that type with its protected getSortedDocValues /
	// getNumericDocValues hook overridden — that is exactly what
	// ToParentBlockJoinSortField does. Go has no subclassing and no method
	// overriding, so the override travels on the SortField itself and the
	// comparator factory installs it. It is typed any because spi must not
	// import search, which owns SortedDocValuesSource and
	// NumericDocValuesSource; see GetDocValuesSource.
	docValuesSource any
}

func (sf *SortField) GetField() string              { return sf.Field }
func (sf *SortField) GetReverse() bool              { return sf.Reverse }
func (sf *SortField) SetMissingValue(v interface{}) { sf.MissingValue = v }
func (sf *SortField) SetOptimizeSortWithIndexedData(v bool) {
	sf.optimizeSortWithIndexedData = v
	sf.optimizeSet = true
}
func (sf *SortField) GetOptimizeSortWithIndexedData() bool {
	if !sf.optimizeSet {
		return true
	}
	return sf.optimizeSortWithIndexedData
}

// Descending returns true if this sort field sorts in descending order.
func (sf *SortField) Descending() bool {
	return sf.Reverse
}

// SetReverse sets whether this sort field sorts in descending order.
func (sf *SortField) SetReverse(reverse bool) {
	sf.Reverse = reverse
}

// -----------------------------------------------------------------------------
// SortField.java members
//
// The declarations below complete the port of
// org.apache.lucene.search.SortField (Apache Lucene 10.5.0) on the type that
// actually carries the state. They live in spi rather than in search because Go
// only allows a package to declare methods on a type it defines, and
// search.SortField is an alias of this type.
//
// Members of SortField.java that cannot be rendered here — getComparator(int,
// Pruning), rewrite(IndexSearcher), getIndexSorter() and the nested Provider —
// need types owned by search/ and index/, which spi must not import; they are
// rendered in package search.
// -----------------------------------------------------------------------------

// String returns the name of the enum constant, mirroring Java's
// Enum.toString() for SortField.Type. The spelling is part of the binary
// contract: SortField.serialize writes out.writeString(type.toString()), so
// these are the exact bytes Apache Lucene 10.5.0 reads back through
// SortField.readType.
func (t SortFieldType) String() string {
	switch t {
	case SortFieldTypeScore:
		return "SCORE"
	case SortFieldTypeDoc:
		return "DOC"
	case SortFieldTypeString:
		return "STRING"
	case SortFieldTypeInt:
		return "INT"
	case SortFieldTypeFloat:
		return "FLOAT"
	case SortFieldTypeLong:
		return "LONG"
	case SortFieldTypeDouble:
		return "DOUBLE"
	case SortFieldTypeCustom:
		return "CUSTOM"
	case SortFieldTypeStringVal:
		return "STRING_VAL"
	case SortFieldTypeRewriteable:
		return "REWRITEABLE"
	default:
		return fmt.Sprintf("SortFieldType(%d)", int(t))
	}
}

// ParseSortFieldType resolves the enum constant named name, mirroring Java's
// SortField.Type.valueOf(String). An unknown name is rejected, as
// SortField.readType rejects it.
func ParseSortFieldType(name string) (SortFieldType, error) {
	switch name {
	case "SCORE":
		return SortFieldTypeScore, nil
	case "DOC":
		return SortFieldTypeDoc, nil
	case "STRING":
		return SortFieldTypeString, nil
	case "INT":
		return SortFieldTypeInt, nil
	case "FLOAT":
		return SortFieldTypeFloat, nil
	case "LONG":
		return SortFieldTypeLong, nil
	case "DOUBLE":
		return SortFieldTypeDouble, nil
	case "CUSTOM":
		return SortFieldTypeCustom, nil
	case "STRING_VAL":
		return SortFieldTypeStringVal, nil
	case "REWRITEABLE":
		return SortFieldTypeRewriteable, nil
	default:
		return 0, fmt.Errorf("can't deserialize SortField - unknown type %s", name)
	}
}

// validateField reproduces the private SortField.validateField(String, Type,
// Object) called from every SortField constructor. Java raises
// IllegalArgumentException; the Go rendering panics, matching the rendering
// already used for the same class of constructor precondition in
// NewBinarySortFieldCustom.
//
// A null Java field name is rendered as the empty string, because
// spi.SortField.Field is a string and not a pointer; FIELD_SCORE and FIELD_DOC
// are therefore built with an empty name, exactly the case Java permits.
func validateField(field string, sortType SortFieldType, missingValue any) {
	if field == "" {
		if sortType != SortFieldTypeScore && sortType != SortFieldTypeDoc {
			panic("field can only be null when type is SCORE or DOC")
		}
	}
	if sortType == SortFieldTypeString {
		if missingValue != nil && missingValue != STRING_FIRST && missingValue != STRING_LAST {
			panic("for Type.STRING, missing value must be either STRING_FIRST or STRING_LAST")
		}
	}
}

// NewSortFieldWithMissing creates a sort, possibly in reverse, by terms in the
// given field with the type of term values explicitly given and an initial
// missing-value sentinel.
//
// Ported from SortField(String, Type, boolean, Object).
func NewSortFieldWithMissing(field string, sortType SortFieldType, reverse bool, missingValue any) *SortField {
	validateField(field, sortType, missingValue)
	return &SortField{
		Field:        field,
		Type:         sortType,
		Reverse:      reverse,
		MissingValue: missingValue,
	}
}

// NewSortFieldCustom creates a sort, possibly in reverse, with a custom
// comparison function. The comparator is held as any because spi must not
// import search, which owns FieldComparatorSource; see GetComparatorSource.
//
// Ported from SortField(String, FieldComparatorSource, boolean). Java's
// two-argument SortField(String, FieldComparatorSource) is the same
// constructor with reverse = false; Go cannot overload the name, so callers
// spell it NewSortFieldCustom(field, comparator, false).
func NewSortFieldCustom(field string, comparator any, reverse bool) *SortField {
	validateField(field, SortFieldTypeCustom, nil)
	return &SortField{
		Field:   field,
		Type:    SortFieldTypeCustom,
		Reverse: reverse,
		// missingValue is factored into the comparator source.
		MissingValue:     nil,
		comparatorSource: comparator,
	}
}

// GetType returns the type of contents in the field.
//
// Ported from SortField.getType().
func (sf *SortField) GetType() SortFieldType { return sf.Type }

// GetMissingValue returns the value to use for documents that don't have a
// value. A nil value indicates that the default should be used.
//
// Ported from SortField.getMissingValue().
func (sf *SortField) GetMissingValue() any { return sf.MissingValue }

// GetComparatorSource returns the FieldComparatorSource used for custom
// sorting, or nil when this is not a CUSTOM sort. The value is returned as any
// because spi must not import search; callers in search type-assert it back to
// search.FieldComparatorSource.
//
// Ported from SortField.getComparatorSource().
func (sf *SortField) GetComparatorSource() any { return sf.comparatorSource }

// SetDocValuesSource installs the per-leaf DocValues override the comparator
// for this sort type must read through: a search.SortedDocValuesSource for
// SortFieldTypeString and a search.NumericDocValuesSource for the numeric
// types. It carries the getSortedDocValues / getNumericDocValues override that
// a SortField subclass performs in Apache Lucene 10.5.0; see docValuesSource.
func (sf *SortField) SetDocValuesSource(src any) { sf.docValuesSource = src }

// GetDocValuesSource returns the DocValues override installed by
// SetDocValuesSource, or nil when the comparator reads the field's own values.
// The value is returned as any because spi must not import search; callers in
// search type-assert it back to SortedDocValuesSource / NumericDocValuesSource.
func (sf *SortField) GetDocValuesSource() any { return sf.docValuesSource }

// NeedsScores reports whether the relevance score is needed to sort documents.
//
// Ported from SortField.needsScores().
func (sf *SortField) NeedsScores() bool { return sf.Type == SortFieldTypeScore }

// String renders the sort field the way SortField.toString() does.
func (sf *SortField) String() string {
	var buffer string
	switch sf.Type {
	case SortFieldTypeScore:
		buffer = "<score>"
	case SortFieldTypeDoc:
		buffer = "<doc>"
	case SortFieldTypeString:
		buffer = fmt.Sprintf("<string: %q>", sf.Field)
	case SortFieldTypeStringVal:
		buffer = fmt.Sprintf("<string_val: %q>", sf.Field)
	case SortFieldTypeInt:
		buffer = fmt.Sprintf("<int: %q>", sf.Field)
	case SortFieldTypeLong:
		buffer = fmt.Sprintf("<long: %q>", sf.Field)
	case SortFieldTypeFloat:
		buffer = fmt.Sprintf("<float: %q>", sf.Field)
	case SortFieldTypeDouble:
		buffer = fmt.Sprintf("<double: %q>", sf.Field)
	case SortFieldTypeCustom:
		buffer = fmt.Sprintf("<custom:%q: %v>", sf.Field, sf.comparatorSource)
	case SortFieldTypeRewriteable:
		buffer = fmt.Sprintf("<rewriteable: %q>", sf.Field)
	default:
		buffer = fmt.Sprintf("<???: %q>", sf.Field)
	}

	if sf.Reverse {
		buffer += "!"
	}
	if sf.MissingValue != nil {
		buffer += fmt.Sprintf(" missingValue=%v", sf.MissingValue)
	}
	return buffer
}

// Equals reports whether other describes the same sort. If a
// FieldComparatorSource was provided it must be comparable (Java requires it to
// implement equals properly, unless a singleton is always used).
//
// Ported from SortField.equals(Object).
func (sf *SortField) Equals(other *SortField) bool {
	if sf == other {
		return true
	}
	if sf == nil || other == nil {
		return false
	}
	return other.Field == sf.Field &&
		other.Type == sf.Type &&
		other.Reverse == sf.Reverse &&
		sf.comparatorSource == other.comparatorSource &&
		sf.MissingValue == other.MissingValue
}

// HashCode returns a hash code consistent with Equals.
//
// Ported from SortField.hashCode(), which is Objects.hash(field, type, reverse,
// comparatorSource, missingValue) — an Arrays.hashCode fold with the 31
// multiplier and 0 for a null element. Java's value is not reproducible in Go
// (enum and Object hash codes are JVM identity hashes, and Lucene never
// serialises this number), so the fold uses the enum ordinal for the type and
// Java's String/Boolean hash codes for the field and the reverse flag.
func (sf *SortField) HashCode() int {
	h := 1
	fieldHash := 0
	for i := 0; i < len(sf.Field); i++ {
		fieldHash = 31*fieldHash + int(sf.Field[i])
	}
	h = 31*h + fieldHash
	h = 31*h + int(sf.Type)
	if sf.Reverse {
		h = 31*h + 1231
	} else {
		h = 31*h + 1237
	}
	h = 31*h + anyHashCode(sf.comparatorSource)
	h = 31*h + anyHashCode(sf.MissingValue)
	return h
}

// anyHashCode folds an arbitrary sort-field attachment into the SortField hash.
// Java calls Object.hashCode(), with 0 for null; Go has no universal hash, so
// values are folded through their fmt rendering, which is stable for the
// sentinels and the numeric missing values Lucene allows here.
func anyHashCode(v any) int {
	if v == nil {
		return 0
	}
	h := 0
	for _, b := range []byte(fmt.Sprintf("%v", v)) {
		h = 31*h + int(b)
	}
	return h
}

// SetOptimizeSortWithPoints enables or disables the numeric sort optimization
// that uses the points index. It is a duplicate of
// SetOptimizeSortWithIndexedData, exactly as in Java.
//
// Ported from SortField.setOptimizeSortWithPoints(boolean) (deprecated in
// Lucene 10, kept because the port must not drop members).
func (sf *SortField) SetOptimizeSortWithPoints(optimizeSortWithPoints bool) {
	sf.SetOptimizeSortWithIndexedData(optimizeSortWithPoints)
}

// GetOptimizeSortWithPoints reports whether sort optimization should use the
// points index. It is a duplicate of GetOptimizeSortWithIndexedData, exactly as
// in Java.
//
// Ported from SortField.getOptimizeSortWithPoints() (deprecated in Lucene 10).
func (sf *SortField) GetOptimizeSortWithPoints() bool {
	return sf.GetOptimizeSortWithIndexedData()
}
