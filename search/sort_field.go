package search

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// SortField stores information about how to sort documents by terms in an
// individual field. Fields must be indexed in order to sort by them.
//
// Ported from org.apache.lucene.search.SortField. The struct itself lives in
// the spi package so that index/ and search/ can both refer to it without an
// import cycle; this alias keeps the Lucene name in the Lucene-named file.
//
// Because Go only allows a package to declare methods on a type it defines,
// the members of SortField.java are split by the types they need:
//
//   - spi/sort_field.go carries everything expressible without search/ or
//     index/ types: the Type enum, the constructors, getField/getType/
//     getReverse/getMissingValue/setMissingValue, getComparatorSource,
//     needsScores, toString, equals, hashCode and the optimizeSortWith*
//     accessors;
//   - this file carries the members that need search/ or index/ types:
//     rewrite(IndexSearcher) and the nested Provider, plus the FIELD_SCORE /
//     FIELD_DOC constants and the typed constructor façades.
type SortField = spi.SortField

var (
	// FieldScore represents sorting by document score (relevance).
	//
	// Mirrors the static field SortField.FIELD_SCORE, which Java builds as
	// new SortField(null, Type.SCORE). spi.SortField.Field is a string rather
	// than a pointer, so Java's null field name is rendered as the empty
	// string; SortField.validateField accepts exactly that for SCORE and DOC.
	FieldScore = NewSortField("", spi.SortFieldTypeScore)

	// FIELD_DOC represents sorting by document number (index order).
	//
	// Mirrors the static field SortField.FIELD_DOC, built by Java as
	// new SortField(null, Type.DOC); see FieldScore for the null field name.
	// It keeps Java's constant spelling because "FieldDoc" is taken by the
	// class org.apache.lucene.search.FieldDoc, ported in field_doc.go.
	FIELD_DOC = NewSortField("", spi.SortFieldTypeDoc)
)

// NewSortField creates a sort by terms in the given field with the type of term
// values explicitly given.
//
// Ported from SortField(String, Type).
func NewSortField(field string, sortType SortFieldType) *SortField {
	return spi.NewSortField(field, sortType)
}

// NewSortFieldWithReverse creates a sort, possibly in reverse, by terms in the
// given field with the type of term values explicitly given.
//
// Ported from SortField(String, Type, boolean).
func NewSortFieldWithReverse(field string, sortType SortFieldType, reverse bool) *SortField {
	return spi.NewSortFieldFull(field, sortType, reverse)
}

// NewSortFieldWithMissing creates a sort, possibly in reverse, by terms in the
// given field with the type of term values explicitly given and an initial
// missing-value sentinel.
//
// Ported from SortField(String, Type, boolean, Object).
func NewSortFieldWithMissing(field string, sortType SortFieldType, reverse bool, missingValue any) *SortField {
	return spi.NewSortFieldWithMissing(field, sortType, reverse, missingValue)
}

// NewSortFieldCustom creates a sort, possibly in reverse, with a custom
// comparison function.
//
// Ported from SortField(String, FieldComparatorSource, boolean). Java's
// two-argument SortField(String, FieldComparatorSource) is that same
// constructor with reverse = false, which Go cannot spell under the same name;
// callers write NewSortFieldCustom(field, comparator, false).
func NewSortFieldCustom(field string, comparator FieldComparatorSource, reverse bool) *SortField {
	return spi.NewSortFieldCustom(field, comparator, reverse)
}

// SortFieldComparatorSource returns the FieldComparatorSource a CUSTOM
// SortField was built with, or nil when the sort is not CUSTOM.
//
// spi.SortField.GetComparatorSource is typed any because spi must not import
// search; this is the typed view of it for search-side callers. It is not a
// separate Lucene member: it is SortField.getComparatorSource() restored to its
// Java return type at the package boundary.
func SortFieldComparatorSource(sf *SortField) FieldComparatorSource {
	src, _ := sf.GetComparatorSource().(FieldComparatorSource)
	return src
}

// SortFieldGetComparator returns the [FieldComparator] to use for sorting sf.
//
// Ported from SortField.getComparator(int, Pruning). Go cannot declare a method
// on spi.SortField from this package — the comparator types it returns live
// here and spi must not import search — so the member is rendered as a
// function, exactly as [RewriteSortField] renders SortField.rewrite. A
// SortField subclass that overrides getComparator in Java (BinarySortField,
// FeatureSortField, LatLonPointSortField, XYPointSortField) declares its own
// GetComparator method on the Go type instead; a caller holding the base
// *SortField calls this function.
//
// numHits is the number of top hits the queue will store. pruning controls how
// the comparator may skip documents through
// LeafFieldComparator.CompetitiveIterator; no comparator in this package
// implements skipping yet, so the argument is recorded in the signature and
// currently changes nothing observable — Lucene's comparators with pruning
// disabled produce the identical ordering. The same holds for Java's `reverse`
// constructor argument, which its comparators use only to decide whether
// skipping is safe; the queue applies the direction itself through reverseMul.
//
// Java raises IllegalStateException for Type.REWRITEABLE and for an unknown
// type, and asserts that a CUSTOM sort carries a comparator source; all three
// are unchecked, and the Go rendering panics, as Gocene already renders the
// unchecked exceptions of SortField.validateField.
func SortFieldGetComparator(sf *SortField, numHits int, pruning Pruning) FieldComparator {
	var fieldComparator FieldComparator
	switch sf.Type {
	case spi.SortFieldTypeScore:
		fieldComparator = newRelevanceComparator(numHits)

	case spi.SortFieldTypeDoc:
		fieldComparator = newDocComparator(numHits)

	case spi.SortFieldTypeInt:
		fieldComparator = newIntComparator(numHits, sf.Field, missingInt32(sf))

	case spi.SortFieldTypeFloat:
		fieldComparator = newFloatComparator(numHits, sf.Field, missingFloat32(sf))

	case spi.SortFieldTypeLong:
		fieldComparator = newLongComparator(numHits, sf.Field, missingInt64(sf))

	case spi.SortFieldTypeDouble:
		fieldComparator = newDoubleComparator(numHits, sf.Field, missingFloat64(sf))

	case spi.SortFieldTypeCustom:
		// SortField.getComparatorSource() is typed any on spi.SortField, which
		// must not import search; SortFieldComparatorSource restores Java's
		// FieldComparatorSource return type at the package boundary.
		comparatorSource := SortFieldComparatorSource(sf)
		if comparatorSource == nil {
			panic(fmt.Sprintf("search: CUSTOM SortField %q has no FieldComparatorSource", sf.Field))
		}
		fieldComparator = comparatorSource.NewComparator(sf.Field, numHits, pruning, sf.Reverse)
		if fieldComparator == nil {
			panic(fmt.Sprintf("search: FieldComparatorSource for %q returned a nil comparator", sf.Field))
		}

	case spi.SortFieldTypeString:
		fieldComparator = newTermOrdValComparator(numHits, sf.Field, sf.MissingValue == STRING_LAST)

	case spi.SortFieldTypeStringVal:
		fieldComparator = NewBinaryFieldComparator(numHits, sf.Field, sf.MissingValue == STRING_LAST, nil)

	case spi.SortFieldTypeRewriteable:
		panic("SortField needs to be rewritten through Sort.rewrite(..) and SortField.rewrite(..)")

	default:
		panic(fmt.Sprintf("Illegal sort type: %s", sf.Type))
	}
	installSortFieldDocValuesSource(sf, fieldComparator)
	if !sf.GetOptimizeSortWithIndexedData() {
		fieldComparator.DisableSkipping()
	}
	return fieldComparator
}

// installSortFieldDocValuesSource wires the DocValues override a SortField
// carries into the comparator its type just selected.
//
// It is the Go rendering of the anonymous subclasses Apache Lucene 10.5.0
// builds inside a SortField subclass's getComparator: the comparator is the
// standard one for the sort type, with only its protected
// getSortedDocValues(LeafReaderContext, String) or
// getNumericDocValues(LeafReaderContext, String) hook overridden. Go has no
// method overriding, so the override is a value on the SortField and this
// function installs it. A source of the wrong shape for the sort type is a
// programming error and panics, as the other unchecked failures of
// SortFieldGetComparator do.
func installSortFieldDocValuesSource(sf *SortField, fieldComparator FieldComparator) {
	src := sf.GetDocValuesSource()
	if src == nil {
		return
	}
	switch c := fieldComparator.(type) {
	case *termOrdValComparator:
		sorted, ok := src.(SortedDocValuesSource)
		if !ok {
			panic(fmt.Sprintf("search: SortField %q of type %s needs a SortedDocValuesSource, got %T", sf.Field, sf.Type, src))
		}
		c.dvSource = sorted
	case *intComparator:
		c.dvSource = numericDocValuesSourceOf(sf, src)
	case *longComparator:
		c.dvSource = numericDocValuesSourceOf(sf, src)
	case *floatComparator:
		c.dvSource = numericDocValuesSourceOf(sf, src)
	case *doubleComparator:
		c.dvSource = numericDocValuesSourceOf(sf, src)
	default:
		panic(fmt.Sprintf("search: SortField %q of type %s cannot carry a DocValues source", sf.Field, sf.Type))
	}
}

// numericDocValuesSourceOf narrows a SortField's DocValues override to the
// numeric shape, panicking when it is not one. See
// installSortFieldDocValuesSource.
func numericDocValuesSourceOf(sf *SortField, src any) NumericDocValuesSource {
	numeric, ok := src.(NumericDocValuesSource)
	if !ok {
		panic(fmt.Sprintf("search: SortField %q of type %s needs a NumericDocValuesSource, got %T", sf.Field, sf.Type, src))
	}
	return numeric
}

// RewriteSortField rewrites sf, returning a new SortField if a change is made.
// The base implementation returns sf unchanged; SortFields of type REWRITEABLE
// are expected to be rewritten by the type that defines them.
//
// Ported from SortField.rewrite(IndexSearcher). Go cannot declare a method on
// spi.SortField from this package, so the member is rendered as a function.
func RewriteSortField(sf *SortField, searcher *IndexSearcher) (*SortField, error) {
	_ = searcher
	return sf, nil
}

// ProviderName is the name the SortField Provider is registered under.
//
// Mirrors SortField.Provider.NAME.
const ProviderName = "SortField"

// Provider is the SortFieldProvider for plain field sorts. It reads and writes
// the byte sequence SortField.Provider reads and writes, so segment-info files
// written here are read back by Apache Lucene 10.5.0 and vice versa.
//
// Ported from org.apache.lucene.search.SortField.Provider.
type Provider struct{}

// NewProvider creates a new Provider.
//
// Mirrors SortField.Provider().
func NewProvider() *Provider { return &Provider{} }

// Name returns the name this Provider is registered under.
func (p *Provider) Name() string { return ProviderName }

// ReadSortField reconstructs a SortField from in.
//
// Ported from SortField.Provider.readSortField(DataInput).
func (p *Provider) ReadSortField(in store.DataInput) (index.SortFieldValue, error) {
	field, err := in.ReadString()
	if err != nil {
		return nil, err
	}
	sortType, err := readType(in)
	if err != nil {
		return nil, err
	}
	reverseInt, err := in.ReadInt()
	if err != nil {
		return nil, err
	}
	reverse := reverseInt == 1
	hasMissing, err := in.ReadInt()
	if err != nil {
		return nil, err
	}
	if hasMissing == 1 {
		// missing object
		switch sortType {
		case spi.SortFieldTypeString:
			missingString, err := in.ReadInt()
			if err != nil {
				return nil, err
			}
			if missingString == 1 {
				return NewSortFieldWithMissing(field, sortType, reverse, STRING_FIRST), nil
			}
			return NewSortFieldWithMissing(field, sortType, reverse, STRING_LAST), nil
		case spi.SortFieldTypeInt:
			v, err := in.ReadInt()
			if err != nil {
				return nil, err
			}
			return NewSortFieldWithMissing(field, sortType, reverse, v), nil
		case spi.SortFieldTypeLong:
			v, err := in.ReadLong()
			if err != nil {
				return nil, err
			}
			return NewSortFieldWithMissing(field, sortType, reverse, v), nil
		case spi.SortFieldTypeFloat:
			v, err := in.ReadInt()
			if err != nil {
				return nil, err
			}
			return NewSortFieldWithMissing(field, sortType, reverse, util.SortableIntToFloat(v)), nil
		case spi.SortFieldTypeDouble:
			v, err := in.ReadLong()
			if err != nil {
				return nil, err
			}
			return NewSortFieldWithMissing(field, sortType, reverse, util.SortableLongToDouble(v)), nil
		default:
			return nil, fmt.Errorf("cannot deserialize sort of type %s", sortType)
		}
	}
	return NewSortFieldWithMissing(field, sortType, reverse, nil), nil
}

// WriteSortField writes sf to out using this provider's wire format.
//
// Ported from SortField.Provider.writeSortField(SortField, DataOutput).
func (p *Provider) WriteSortField(sf index.SortFieldValue, out store.DataOutput) error {
	field, ok := sf.(*SortField)
	if !ok {
		return fmt.Errorf("sort field is not a *search.SortField")
	}
	return serializeSortField(field, out)
}

var _ index.SortFieldProvider = (*Provider)(nil)

// readType reads the enum constant name written by serializeSortField and
// resolves it back to a SortFieldType.
//
// Ported from the protected static SortField.readType(DataInput).
func readType(in store.DataInput) (SortFieldType, error) {
	name, err := in.ReadString()
	if err != nil {
		return 0, err
	}
	return spi.ParseSortFieldType(name)
}

// serializeSortField writes sf in the byte layout SortField.Provider reads.
//
// Ported from the private SortField.serialize(DataOutput). Go cannot declare a
// method on spi.SortField from this package, so the member is rendered as a
// function next to the Provider that is its only caller, mirroring Java's
// nesting of Provider inside SortField.
func serializeSortField(sf *SortField, out store.DataOutput) error {
	if err := out.WriteString(sf.Field); err != nil {
		return err
	}
	if err := out.WriteString(sf.Type.String()); err != nil {
		return err
	}
	reverseInt := int32(0)
	if sf.Reverse {
		reverseInt = 1
	}
	if err := out.WriteInt(reverseInt); err != nil {
		return err
	}
	if sf.MissingValue == nil {
		return out.WriteInt(0)
	}
	if err := out.WriteInt(1); err != nil {
		return err
	}
	switch sf.Type {
	case spi.SortFieldTypeString:
		switch sf.MissingValue {
		case STRING_LAST:
			return out.WriteInt(0)
		case STRING_FIRST:
			return out.WriteInt(1)
		default:
			return fmt.Errorf("cannot serialize missing value of %v for type STRING", sf.MissingValue)
		}
	case spi.SortFieldTypeInt:
		v, err := sortFieldMissingInt32(sf.MissingValue)
		if err != nil {
			return err
		}
		return out.WriteInt(v)
	case spi.SortFieldTypeLong:
		v, err := sortFieldMissingInt64(sf.MissingValue)
		if err != nil {
			return err
		}
		return out.WriteLong(v)
	case spi.SortFieldTypeFloat:
		v, err := sortFieldMissingFloat32(sf.MissingValue)
		if err != nil {
			return err
		}
		return out.WriteInt(util.FloatToSortableInt(v))
	case spi.SortFieldTypeDouble:
		v, err := sortFieldMissingFloat64(sf.MissingValue)
		if err != nil {
			return err
		}
		return out.WriteLong(util.DoubleToSortableLong(v))
	default:
		return fmt.Errorf("cannot serialize SortField of type %s", sf.Type)
	}
}

// sortFieldMissingInt32 narrows a Type.INT missing value to the 32-bit width
// Java writes. Java casts the boxed Integer directly and raises
// ClassCastException on anything else; the Go rendering accepts the integer
// spellings the comparator factory already accepts (see missingInt32) and
// reports the same refusal as an error.
func sortFieldMissingInt32(v any) (int32, error) {
	switch n := v.(type) {
	case int32:
		return n, nil
	case int:
		return int32(n), nil
	case int64:
		return int32(n), nil
	}
	return 0, fmt.Errorf("missing values for Type.INT can only be integers, but got %T", v)
}

// sortFieldMissingInt64 narrows a Type.LONG missing value; see
// sortFieldMissingInt32 for the rendering of Java's cast.
func sortFieldMissingInt64(v any) (int64, error) {
	switch n := v.(type) {
	case int64:
		return n, nil
	case int:
		return int64(n), nil
	case int32:
		return int64(n), nil
	}
	return 0, fmt.Errorf("missing values for Type.LONG can only be integers, but got %T", v)
}

// sortFieldMissingFloat32 narrows a Type.FLOAT missing value; see
// sortFieldMissingInt32 for the rendering of Java's cast.
func sortFieldMissingFloat32(v any) (float32, error) {
	switch n := v.(type) {
	case float32:
		return n, nil
	case float64:
		return float32(n), nil
	}
	return 0, fmt.Errorf("missing values for Type.FLOAT can only be floats, but got %T", v)
}

// sortFieldMissingFloat64 narrows a Type.DOUBLE missing value; see
// sortFieldMissingInt32 for the rendering of Java's cast.
func sortFieldMissingFloat64(v any) (float64, error) {
	switch n := v.(type) {
	case float64:
		return n, nil
	case float32:
		return float64(n), nil
	}
	return 0, fmt.Errorf("missing values for Type.DOUBLE can only be floats, but got %T", v)
}
