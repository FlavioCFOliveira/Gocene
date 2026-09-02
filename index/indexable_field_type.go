// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

// IndexableFieldType describes how a field should be indexed, stored,
// vectorized and analyzed. Mirrors org.apache.lucene.index.IndexableFieldType
// from Apache Lucene 10.4.0.
//
// Implementations are typically immutable value types attached to each
// IndexableField. Codec-facing read paths in Gocene depend on this contract
// being identical to Lucene's, since the answers fed into IndexableField
// indirectly drive FieldInfo construction.
type IndexableFieldType interface {
	// Stored reports whether the field's value should be stored verbatim in
	// the stored-fields stream so it can be returned with each document.
	Stored() bool

	// Tokenized reports whether the field's value should be tokenized.
	Tokenized() bool

	// StoreTermVectors reports whether term vectors should be stored for this
	// field. Implies index options at least DOCS.
	StoreTermVectors() bool

	// StoreTermVectorOffsets reports whether offsets should be recorded in
	// the term vectors. Requires StoreTermVectors().
	StoreTermVectorOffsets() bool

	// StoreTermVectorPositions reports whether positions should be recorded
	// in the term vectors. Requires StoreTermVectors().
	StoreTermVectorPositions() bool

	// StoreTermVectorPayloads reports whether payloads should be recorded in
	// the term vectors. Requires StoreTermVectorPositions().
	StoreTermVectorPayloads() bool

	// OmitNorms reports whether norms should be omitted for this field
	// (saves space at the cost of length-based scoring).
	OmitNorms() bool

	// IndexOptions returns the index options for the postings list.
	IndexOptions() IndexOptions

	// DocValuesType returns the type of per-document values stored.
	DocValuesType() DocValuesType

	// DocValuesSkipIndexType returns the type of the doc-values skip index.
	DocValuesSkipIndexType() DocValuesSkipIndexType

	// PointDimensionCount returns the number of dimensions configured for
	// this field's BKD point index, or 0 if points are not indexed.
	PointDimensionCount() int

	// PointIndexDimensionCount returns the number of dimensions that are
	// actually used for the index (may be less than PointDimensionCount).
	PointIndexDimensionCount() int

	// PointNumBytes returns the number of bytes per dimension.
	PointNumBytes() int

	// VectorDimension returns the dimension of the vector field, or 0 if
	// vectors are not stored for this field.
	VectorDimension() int

	// VectorEncoding returns the vector encoding (BYTE or FLOAT32).
	VectorEncoding() VectorEncoding

	// VectorSimilarityFunction returns the similarity function used to
	// score vector comparisons (e.g. COSINE, DOT_PRODUCT).
	VectorSimilarityFunction() VectorSimilarityFunction

	// GetAttributes returns a map of arbitrary attributes attached to this
	// field type. May return nil if no attributes are set.
	GetAttributes() map[string]string
}
