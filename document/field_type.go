// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"fmt"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// FieldType describes the properties of a field.
//
// This is the Go port of Lucene's org.apache.lucene.document.FieldType
// (Apache Lucene 10.4.0). The struct preserves the public contract of the
// Java original while remaining idiomatic Go.
//
// Divergences from Java (documented for back-compat with already-shipped
// Gocene code):
//   - Public fields are exposed alongside Lucene-style getter methods. The
//     Lucene-style getters (Stored(), Tokenized(), IndexOptions(), ...) are
//     the recommended way to read FieldType state; the public fields remain
//     accessible for back-compat with code shipped in earlier sprints.
//   - The original Tokenized default in Lucene is `true`; the pre-existing
//     Gocene NewFieldType used `false` and many shipped consumers
//     (BinaryDocValuesField, NumericDocValuesField, ...) rely on that
//     default for fields that are neither indexed nor tokenized. To avoid
//     a wide-blast regression at sprint scope, NewFieldType() preserves
//     the Gocene default (Tokenized=false). Callers that need
//     Lucene-canonical defaults can construct a struct literal or call
//     NewLuceneFieldType(). This divergence is documented in
//     project-gocene-fieldtype-defaults memory.
type FieldType struct {
	// Indexed determines whether the field is indexed for searching.
	// If false, the field is not searchable.
	//
	// NOTE: this field has no peer in Lucene's FieldType; in Lucene the
	// "indexed" property is derived from IndexOptions != NONE. It is
	// preserved here for back-compat with already-shipped Gocene code.
	Indexed bool

	// Stored determines whether the field value is stored in the index.
	// Stored fields can be retrieved in search results.
	Stored bool

	// Tokenized determines whether the field value should be tokenized.
	// If true, the field value is analyzed and tokens are indexed.
	// If false, the field value is indexed as a single term.
	Tokenized bool

	// StoreTermVectors determines whether term vectors are stored.
	// Term vectors include term frequencies and positions per document.
	StoreTermVectors bool

	// StoreTermVectorPositions determines whether term vector positions are stored.
	// Requires StoreTermVectors to be true.
	StoreTermVectorPositions bool

	// StoreTermVectorOffsets determines whether term vector offsets are stored.
	// Requires StoreTermVectors to be true.
	StoreTermVectorOffsets bool

	// StoreTermVectorPayloads determines whether term vector payloads are stored.
	// Requires StoreTermVectors to be true.
	StoreTermVectorPayloads bool

	// OmitNorms determines whether field norms (boost factors) are omitted.
	// If true, the field will not have scoring based on field length.
	OmitNorms bool

	// IndexOptions controls what information is stored in the postings lists.
	// See index.IndexOptions for details.
	IndexOptions index.IndexOptions

	// DocValuesType determines the type of per-document values stored.
	// See index.DocValuesType for details.
	DocValuesType index.DocValuesType

	// DocValuesSkipIndex controls whether a skip index is built for the
	// associated numeric doc-values. See index.DocValuesSkipIndexType.
	// Added in Lucene 10.x.
	DocValuesSkipIndex index.DocValuesSkipIndexType

	// DimensionCount is the number of dimensions for point fields.
	// Only used for Point-based fields (IntPoint, LongPoint, etc.).
	DimensionCount int

	// IndexDimensionCount is the number of dimensions used for the BKD index.
	// Defaults to DimensionCount when only setDimensions(count, numBytes) is
	// used. See setDimensions(count, indexCount, numBytes).
	IndexDimensionCount int

	// DimensionNumBytes is the number of bytes per dimension for point fields.
	// Only used for Point-based fields.
	DimensionNumBytes int

	// VectorDimension is the number of dimensions for KNN vector fields.
	VectorDimension int

	// VectorEncoding controls how vector values are encoded.
	VectorEncoding index.VectorEncoding

	// VectorSimilarityFunction controls the similarity used by the KNN search.
	VectorSimilarityFunction index.VectorSimilarityFunction

	// attributes is the lazily-allocated string->string attribute map.
	attributes map[string]string

	// frozen tracks whether this FieldType has been frozen (made immutable).
	frozen bool
}

// NewFieldType creates a new FieldType with Gocene-compatible defaults.
//
// Defaults (back-compat; Tokenized defaults to false here, in contrast to
// Lucene's true — see file docstring):
//   - Tokenized=false
//   - IndexOptions=NONE
//   - DocValuesType=NONE
//   - DocValuesSkipIndex=NONE
//   - VectorEncoding=FLOAT32
//   - VectorSimilarityFunction=EUCLIDEAN
func NewFieldType() *FieldType {
	return &FieldType{
		IndexOptions:             index.IndexOptionsNone,
		DocValuesType:            index.DocValuesTypeNone,
		DocValuesSkipIndex:       index.DocValuesSkipIndexTypeNone,
		VectorEncoding:           index.VectorEncodingFloat32,
		VectorSimilarityFunction: index.VectorSimilarityFunctionEuclidean,
	}
}

// NewLuceneFieldType creates a new FieldType with Lucene-canonical defaults
// matching org.apache.lucene.document.FieldType (Tokenized=true).
//
// Use this when porting Lucene code where the caller relies on the JVM
// default for Tokenized.
func NewLuceneFieldType() *FieldType {
	ft := NewFieldType()
	ft.Tokenized = true
	return ft
}

// NewFieldTypeFrom creates a new FieldType copying every property from the
// provided reference (excluding the frozen flag — the returned FieldType is
// always mutable). Mirrors Lucene's `FieldType(IndexableFieldType ref)`.
func NewFieldTypeFrom(ref *FieldType) *FieldType {
	if ref == nil {
		return NewFieldType()
	}
	out := &FieldType{
		Indexed:                  ref.Indexed,
		Stored:                   ref.Stored,
		Tokenized:                ref.Tokenized,
		StoreTermVectors:         ref.StoreTermVectors,
		StoreTermVectorPositions: ref.StoreTermVectorPositions,
		StoreTermVectorOffsets:   ref.StoreTermVectorOffsets,
		StoreTermVectorPayloads:  ref.StoreTermVectorPayloads,
		OmitNorms:                ref.OmitNorms,
		IndexOptions:             ref.IndexOptions,
		DocValuesType:            ref.DocValuesType,
		DocValuesSkipIndex:       ref.DocValuesSkipIndex,
		DimensionCount:           ref.DimensionCount,
		IndexDimensionCount:      ref.IndexDimensionCount,
		DimensionNumBytes:        ref.DimensionNumBytes,
		VectorDimension:          ref.VectorDimension,
		VectorEncoding:           ref.VectorEncoding,
		VectorSimilarityFunction: ref.VectorSimilarityFunction,
	}
	if len(ref.attributes) > 0 {
		out.attributes = make(map[string]string, len(ref.attributes))
		for k, v := range ref.attributes {
			out.attributes[k] = v
		}
	}
	return out
}

// Freeze makes this FieldType immutable.
// After freezing, any attempt to modify the FieldType will panic.
func (ft *FieldType) Freeze() {
	ft.frozen = true
}

// IsFrozen returns true if this FieldType has been frozen.
func (ft *FieldType) IsFrozen() bool {
	return ft.frozen
}

// CheckIfFrozen panics if the FieldType is frozen. Mirrors Lucene's
// FieldType.checkIfFrozen(); kept exported for code that wants to assert
// mutability before performing a series of edits.
func (ft *FieldType) CheckIfFrozen() {
	ft.checkFrozen()
}

// checkFrozen panics if the FieldType is frozen.
func (ft *FieldType) checkFrozen() {
	if ft.frozen {
		panic("this FieldType is already frozen and cannot be changed")
	}
}

// SetIndexed sets whether the field is indexed.
// Callers that enable indexing must also call SetIndexOptions to select the
// desired posting detail level; Validate will reject an indexed field that
// has IndexOptions == None.  The auto-set that previously imposed
// DOCS_AND_FREQS_AND_POSITIONS was removed because it silently masked the
// invalid-configuration error that the test suite expects.
func (ft *FieldType) SetIndexed(indexed bool) *FieldType {
	ft.checkFrozen()
	ft.Indexed = indexed
	return ft
}

// SetStored sets whether the field value is stored.
func (ft *FieldType) SetStored(stored bool) *FieldType {
	ft.checkFrozen()
	ft.Stored = stored
	return ft
}

// SetTokenized sets whether the field value should be tokenized.
func (ft *FieldType) SetTokenized(tokenized bool) *FieldType {
	ft.checkFrozen()
	ft.Tokenized = tokenized
	return ft
}

// SetStoreTermVectors sets whether term vectors are stored.
func (ft *FieldType) SetStoreTermVectors(store bool) *FieldType {
	ft.checkFrozen()
	ft.StoreTermVectors = store
	return ft
}

// SetStoreTermVectorPositions sets whether positions are stored in term vectors.
func (ft *FieldType) SetStoreTermVectorPositions(store bool) *FieldType {
	ft.checkFrozen()
	ft.StoreTermVectorPositions = store
	return ft
}

// SetStoreTermVectorOffsets sets whether offsets are stored in term vectors.
func (ft *FieldType) SetStoreTermVectorOffsets(store bool) *FieldType {
	ft.checkFrozen()
	ft.StoreTermVectorOffsets = store
	return ft
}

// SetStoreTermVectorPayloads sets whether payloads are stored in term vectors.
func (ft *FieldType) SetStoreTermVectorPayloads(store bool) *FieldType {
	ft.checkFrozen()
	ft.StoreTermVectorPayloads = store
	return ft
}

// SetIndexOptions sets the indexing options.
func (ft *FieldType) SetIndexOptions(options index.IndexOptions) *FieldType {
	ft.checkFrozen()
	ft.IndexOptions = options
	return ft
}

// SetDocValuesType sets the doc values type.
func (ft *FieldType) SetDocValuesType(docValuesType index.DocValuesType) *FieldType {
	ft.checkFrozen()
	ft.DocValuesType = docValuesType
	return ft
}

// SetDocValuesSkipIndexType sets the doc-values skip-index type.
// Added in Lucene 10.x.
func (ft *FieldType) SetDocValuesSkipIndexType(skip index.DocValuesSkipIndexType) *FieldType {
	ft.checkFrozen()
	ft.DocValuesSkipIndex = skip
	return ft
}

// SetOmitNorms sets whether norms are omitted.
func (ft *FieldType) SetOmitNorms(omit bool) *FieldType {
	ft.checkFrozen()
	ft.OmitNorms = omit
	return ft
}

// SetDimensions configures point dimensions where the indexed dimension
// count equals the total dimension count. Mirrors Lucene's
// setDimensions(int, int).
func (ft *FieldType) SetDimensions(dimensionCount, dimensionNumBytes int) *FieldType {
	return ft.SetDimensionsIndexed(dimensionCount, dimensionCount, dimensionNumBytes)
}

// SetDimensionsIndexed configures point dimensions with an explicit
// indexedDimensionCount distinct from dimensionCount. Mirrors Lucene's
// setDimensions(int, int, int).
func (ft *FieldType) SetDimensionsIndexed(dimensionCount, indexDimensionCount, dimensionNumBytes int) *FieldType {
	ft.checkFrozen()
	if dimensionCount < 0 {
		panic(fmt.Sprintf("dimensionCount must be >= 0; got %d", dimensionCount))
	}
	if indexDimensionCount < 0 {
		panic(fmt.Sprintf("indexDimensionCount must be >= 0; got %d", indexDimensionCount))
	}
	if indexDimensionCount > dimensionCount {
		panic(fmt.Sprintf("indexDimensionCount (%d) must be <= dimensionCount (%d)", indexDimensionCount, dimensionCount))
	}
	if dimensionNumBytes < 0 {
		panic(fmt.Sprintf("dimensionNumBytes must be >= 0; got %d", dimensionNumBytes))
	}
	if dimensionCount == 0 {
		if indexDimensionCount != 0 {
			panic(fmt.Sprintf("when dimensionCount is 0, indexDimensionCount must be 0; got %d", indexDimensionCount))
		}
		if dimensionNumBytes != 0 {
			panic(fmt.Sprintf("when dimensionCount is 0, dimensionNumBytes must be 0; got %d", dimensionNumBytes))
		}
	} else if dimensionNumBytes == 0 {
		panic(fmt.Sprintf("when dimensionCount is > 0 (%d), dimensionNumBytes must be > 0", dimensionCount))
	}
	ft.DimensionCount = dimensionCount
	ft.IndexDimensionCount = indexDimensionCount
	ft.DimensionNumBytes = dimensionNumBytes
	return ft
}

// SetVectorAttributes configures KNN vector indexing parameters.
// Mirrors Lucene's setVectorAttributes(int, VectorEncoding, VectorSimilarityFunction).
func (ft *FieldType) SetVectorAttributes(numDimensions int, encoding index.VectorEncoding, similarity index.VectorSimilarityFunction) *FieldType {
	ft.checkFrozen()
	if numDimensions <= 0 {
		panic(fmt.Sprintf("vector numDimensions must be > 0; got %d", numDimensions))
	}
	ft.VectorDimension = numDimensions
	ft.VectorEncoding = encoding
	ft.VectorSimilarityFunction = similarity
	return ft
}

// PutAttribute associates value with key in the field-type attribute map and
// returns the previous value (or empty string when none).
// Mirrors Lucene's putAttribute(String, String).
func (ft *FieldType) PutAttribute(key, value string) string {
	ft.checkFrozen()
	if ft.attributes == nil {
		ft.attributes = make(map[string]string)
	}
	prev := ft.attributes[key]
	ft.attributes[key] = value
	return prev
}

// GetAttributes returns the underlying attribute map. May be nil if no
// attributes were ever set. Mirrors Lucene's getAttributes() which may
// return null.
func (ft *FieldType) GetAttributes() map[string]string {
	return ft.attributes
}

// Lucene-style getter methods. These mirror the bean-style getters on the
// Java FieldType (no "Get" prefix, just the property name).

// IsIndexed reports whether the field is indexed.
func (ft *FieldType) IsIndexed() bool { return ft.Indexed }

// IsStored reports whether the field value is stored.
func (ft *FieldType) IsStored() bool { return ft.Stored }

// IsTokenized reports whether the field value will be tokenized.
func (ft *FieldType) IsTokenized() bool { return ft.Tokenized }

// IsOmitNorms reports whether norms are omitted.
func (ft *FieldType) IsOmitNorms() bool { return ft.OmitNorms }

// GetStoreTermVectors reports whether term vectors are stored.
func (ft *FieldType) GetStoreTermVectors() bool { return ft.StoreTermVectors }

// GetStoreTermVectorPositions reports whether term-vector positions are stored.
func (ft *FieldType) GetStoreTermVectorPositions() bool { return ft.StoreTermVectorPositions }

// GetStoreTermVectorOffsets reports whether term-vector offsets are stored.
func (ft *FieldType) GetStoreTermVectorOffsets() bool { return ft.StoreTermVectorOffsets }

// GetStoreTermVectorPayloads reports whether term-vector payloads are stored.
func (ft *FieldType) GetStoreTermVectorPayloads() bool { return ft.StoreTermVectorPayloads }

// GetIndexOptions returns the indexing options.
func (ft *FieldType) GetIndexOptions() index.IndexOptions { return ft.IndexOptions }

// GetDocValuesType returns the doc-values type.
func (ft *FieldType) GetDocValuesType() index.DocValuesType { return ft.DocValuesType }

// DocValuesSkipIndexType returns the doc-values skip-index type.
func (ft *FieldType) DocValuesSkipIndexType() index.DocValuesSkipIndexType {
	return ft.DocValuesSkipIndex
}

// PointDimensionCount returns the total number of point dimensions.
func (ft *FieldType) PointDimensionCount() int { return ft.DimensionCount }

// PointIndexDimensionCount returns the number of point dimensions used for indexing (BKD).
func (ft *FieldType) PointIndexDimensionCount() int { return ft.IndexDimensionCount }

// PointNumBytes returns the per-dimension byte width for point fields.
func (ft *FieldType) PointNumBytes() int { return ft.DimensionNumBytes }

// GetVectorDimension returns the number of KNN vector dimensions.
func (ft *FieldType) GetVectorDimension() int { return ft.VectorDimension }

// GetVectorEncoding returns the KNN vector encoding.
func (ft *FieldType) GetVectorEncoding() index.VectorEncoding { return ft.VectorEncoding }

// GetVectorSimilarityFunction returns the KNN vector similarity function.
func (ft *FieldType) GetVectorSimilarityFunction() index.VectorSimilarityFunction {
	return ft.VectorSimilarityFunction
}

// Validate validates this FieldType configuration.
// Returns an error if the configuration is invalid.
func (ft *FieldType) Validate() error {
	// If term vectors positions/offsets/payloads are set, term vectors must be enabled
	if ft.StoreTermVectorPositions && !ft.StoreTermVectors {
		return &FieldTypeValidationError{msg: "cannot store term vector positions without storing term vectors"}
	}
	if ft.StoreTermVectorOffsets && !ft.StoreTermVectors {
		return &FieldTypeValidationError{msg: "cannot store term vector offsets without storing term vectors"}
	}
	if ft.StoreTermVectorPayloads && !ft.StoreTermVectors {
		return &FieldTypeValidationError{msg: "cannot store term vector payloads without storing term vectors"}
	}

	// If indexed via the inverted index (no point dimensions), IndexOptions
	// must be set. Point-only indexed fields (e.g. IntField/LongField) keep
	// IndexOptions=NONE — the index path is the BKD tree, not the postings.
	if ft.Indexed && ft.IndexOptions == index.IndexOptionsNone && ft.DimensionCount == 0 {
		return &FieldTypeValidationError{msg: "indexed field cannot have IndexOptionsNone"}
	}

	// If tokenized is true, indexed must also be true
	if ft.Tokenized && !ft.Indexed {
		// NOTE: Lucene does not enforce this — tokenized makes sense only
		// when IndexOptions != NONE, but the JVM check is on the latter.
		// Kept here for back-compat with already-shipped Gocene callers.
		return &FieldTypeValidationError{msg: "tokenized field must be indexed"}
	}

	return nil
}

// String returns a human-readable description of the FieldType, formatted
// closely to Lucene's FieldType.toString().
func (ft *FieldType) String() string {
	var b strings.Builder
	if ft.Stored {
		b.WriteString("stored")
	}
	if ft.IndexOptions != index.IndexOptionsNone {
		writeSep(&b)
		b.WriteString("indexed")
		writeSep(&b)
		b.WriteString("tokenized=")
		fmt.Fprintf(&b, "%t", ft.Tokenized)
		writeSep(&b)
		b.WriteString("omitNorms=")
		fmt.Fprintf(&b, "%t", ft.OmitNorms)
		writeSep(&b)
		b.WriteString("indexOptions=")
		b.WriteString(ft.IndexOptions.String())
		if ft.StoreTermVectors {
			writeSep(&b)
			b.WriteString("termVector")
			if ft.StoreTermVectorOffsets {
				writeSep(&b)
				b.WriteString("termVectorOffsets")
			}
			if ft.StoreTermVectorPositions {
				writeSep(&b)
				b.WriteString("termVectorPosition")
			}
			if ft.StoreTermVectorPayloads {
				writeSep(&b)
				b.WriteString("termVectorPayloads")
			}
		}
	}
	if ft.DimensionCount > 0 {
		writeSep(&b)
		fmt.Fprintf(&b, "pointDimensionCount=%d", ft.DimensionCount)
		writeSep(&b)
		fmt.Fprintf(&b, "pointIndexDimensionCount=%d", ft.IndexDimensionCount)
		writeSep(&b)
		fmt.Fprintf(&b, "pointNumBytes=%d", ft.DimensionNumBytes)
	}
	if ft.VectorDimension > 0 {
		writeSep(&b)
		fmt.Fprintf(&b, "vectorDimension=%d", ft.VectorDimension)
		writeSep(&b)
		fmt.Fprintf(&b, "vectorEncoding=%s", ft.VectorEncoding.String())
		writeSep(&b)
		fmt.Fprintf(&b, "vectorSimilarityFunction=%s", ft.VectorSimilarityFunction.String())
	}
	if ft.DocValuesType != index.DocValuesTypeNone {
		writeSep(&b)
		fmt.Fprintf(&b, "docValuesType=%s", ft.DocValuesType.String())
	}
	if ft.DocValuesSkipIndex != index.DocValuesSkipIndexTypeNone {
		writeSep(&b)
		fmt.Fprintf(&b, "docValuesSkipIndexType=%s", ft.DocValuesSkipIndex.String())
	}
	return b.String()
}

func writeSep(b *strings.Builder) {
	if b.Len() > 0 {
		b.WriteByte(',')
	}
}

// Equals reports whether two FieldType values represent the same
// configuration (frozen flag and attributes excluded).
func (ft *FieldType) Equals(other *FieldType) bool {
	if ft == other {
		return true
	}
	if ft == nil || other == nil {
		return false
	}
	return ft.Indexed == other.Indexed &&
		ft.Stored == other.Stored &&
		ft.Tokenized == other.Tokenized &&
		ft.StoreTermVectors == other.StoreTermVectors &&
		ft.StoreTermVectorPositions == other.StoreTermVectorPositions &&
		ft.StoreTermVectorOffsets == other.StoreTermVectorOffsets &&
		ft.StoreTermVectorPayloads == other.StoreTermVectorPayloads &&
		ft.OmitNorms == other.OmitNorms &&
		ft.IndexOptions == other.IndexOptions &&
		ft.DocValuesType == other.DocValuesType &&
		ft.DocValuesSkipIndex == other.DocValuesSkipIndex &&
		ft.DimensionCount == other.DimensionCount &&
		ft.IndexDimensionCount == other.IndexDimensionCount &&
		ft.DimensionNumBytes == other.DimensionNumBytes &&
		ft.VectorDimension == other.VectorDimension &&
		ft.VectorEncoding == other.VectorEncoding &&
		ft.VectorSimilarityFunction == other.VectorSimilarityFunction
}

// FieldTypeValidationError is returned when FieldType validation fails.
type FieldTypeValidationError struct {
	msg string
}

// Error returns the error message.
func (e *FieldTypeValidationError) Error() string {
	return "FieldType validation error: " + e.msg
}

// fieldTypeAsIndexInterface wraps *FieldType so that it satisfies
// index.FieldTypeInterface.  The wrapper bridges the naming mismatch between
// document.FieldType's Get-prefixed term-vector methods
// (GetStoreTermVectors/…) and the un-prefixed names required by
// index.FieldTypeInterface (StoreTermVectors/…).
type fieldTypeAsIndexInterface struct{ ft *FieldType }

func (w fieldTypeAsIndexInterface) IsIndexed() bool                       { return w.ft.Indexed }
func (w fieldTypeAsIndexInterface) IsStored() bool                        { return w.ft.Stored }
func (w fieldTypeAsIndexInterface) IsTokenized() bool                     { return w.ft.Tokenized }
func (w fieldTypeAsIndexInterface) GetIndexOptions() index.IndexOptions   { return w.ft.IndexOptions }
func (w fieldTypeAsIndexInterface) GetDocValuesType() index.DocValuesType { return w.ft.DocValuesType }
func (w fieldTypeAsIndexInterface) StoreTermVectors() bool                { return w.ft.StoreTermVectors }
func (w fieldTypeAsIndexInterface) StoreTermVectorPositions() bool {
	return w.ft.StoreTermVectorPositions
}
func (w fieldTypeAsIndexInterface) StoreTermVectorOffsets() bool { return w.ft.StoreTermVectorOffsets }

// VectorDimension exposes the KNN vector dimension so the index-side
// indexing chain can detect a vector field via its optional
// vectorFieldTypeProvider probe (it returns 0 for non-vector field types).
func (w fieldTypeAsIndexInterface) VectorDimension() int { return w.ft.VectorDimension }

// VectorEncoding exposes the KNN vector encoding (BYTE / FLOAT32) for the
// indexing chain's per-document value dispatch.
func (w fieldTypeAsIndexInterface) VectorEncoding() index.VectorEncoding {
	return w.ft.VectorEncoding
}

// VectorSimilarityFunction exposes the KNN similarity function recorded on
// the FieldInfo so the codec can score vector comparisons consistently.
func (w fieldTypeAsIndexInterface) VectorSimilarityFunction() index.VectorSimilarityFunction {
	return w.ft.VectorSimilarityFunction
}

// AsIndexFieldTypeInterface returns this FieldType wrapped as an
// index.FieldTypeInterface so that document.Field can satisfy
// index.IndexableField without renaming any struct fields.
func (ft *FieldType) AsIndexFieldTypeInterface() index.FieldTypeInterface {
	return fieldTypeAsIndexInterface{ft: ft}
}

// fieldAsIndexableField wraps *Field so that it satisfies index.IndexableField.
type fieldAsIndexableField struct{ f *Field }

func (w fieldAsIndexableField) Name() string              { return w.f.name }
func (w fieldAsIndexableField) StringValue() string       { return w.f.StringValue() }
func (w fieldAsIndexableField) BinaryValue() []byte       { return w.f.BinaryValue() }
func (w fieldAsIndexableField) NumericValue() interface{} { return w.f.NumericValue() }
func (w fieldAsIndexableField) FieldType() index.FieldTypeInterface {
	return w.f.ft.AsIndexFieldTypeInterface()
}

// AsIndexableField returns this Field wrapped as an index.IndexableField.
// This allows document.Field to participate in index.ProcessDocument without
// requiring a direct import of the document package from the index package.
func (f *Field) AsIndexableField() index.IndexableField {
	return fieldAsIndexableField{f: f}
}

// compile-time checks
var _ index.FieldTypeInterface = fieldTypeAsIndexInterface{}
var _ index.IndexableField = fieldAsIndexableField{}
