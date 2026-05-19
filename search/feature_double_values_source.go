// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"errors"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
)

// FeatureDoubleValuesSource reads the value of a single feature stored inside
// a FeatureField and exposes it as a DoubleValuesSource-compatible producer.
// Mirrors org.apache.lucene.document.FeatureDoubleValuesSource; the Go port
// lives in the search package to avoid the document -> search dependency
// cycle that the Java original sidesteps via package-private visibility.
//
// Concurrency: instances are immutable after construction and safe for
// concurrent use; the per-leaf FeatureDoubleValues returned by GetValues is
// not safe for concurrent use and is owned by the calling collector slot.
type FeatureDoubleValuesSource struct {
	field       string
	featureName string
	featureTerm *index.Term
}

// NewFeatureDoubleValuesSource builds a FeatureDoubleValuesSource for the
// given field and featureName. Both arguments must be non-empty; otherwise
// an error is returned, mirroring Lucene's NullPointerException contract on
// requireNonNull.
func NewFeatureDoubleValuesSource(field, featureName string) (*FeatureDoubleValuesSource, error) {
	if field == "" {
		return nil, errors.New("field must not be empty")
	}
	if featureName == "" {
		return nil, errors.New("featureName must not be empty")
	}
	return &FeatureDoubleValuesSource{
		field:       field,
		featureName: featureName,
		featureTerm: index.NewTerm(field, featureName),
	}, nil
}

// Field returns the indexed field name backing this source.
func (s *FeatureDoubleValuesSource) Field() string { return s.field }

// FeatureName returns the feature name targeted by this source.
func (s *FeatureDoubleValuesSource) FeatureName() string { return s.featureName }

// IsCacheable returns true; the source has no per-leaf mutable state, so
// per-leaf caching is always safe. Matches the Java override.
func (s *FeatureDoubleValuesSource) IsCacheable(ctx *index.LeafReaderContext) bool {
	_ = ctx
	return true
}

// NeedsScores reports whether the source consumes the underlying query's
// scores. Feature values are independent of scoring, so this always returns
// false. Matches the Java override.
func (s *FeatureDoubleValuesSource) NeedsScores() bool { return false }

// Rewrite returns the source unchanged because feature lookups do not depend
// on the IndexSearcher's state. Matches the Java override.
func (s *FeatureDoubleValuesSource) Rewrite(searcher *IndexSearcher) *FeatureDoubleValuesSource {
	_ = searcher
	return s
}

// GetValues returns a per-leaf FeatureDoubleValues that decodes the feature
// term's postings frequency into the matching float64 score. If the leaf has
// no postings for the feature term (or the field itself is absent), an empty
// reader is returned which always reports false from AdvanceExact, mirroring
// the Java DoubleValues.EMPTY sentinel.
//
// The scores argument is accepted for API parity with Lucene's signature but
// is unused: feature values do not consume the underlying query score.
func (s *FeatureDoubleValuesSource) GetValues(ctx *index.LeafReaderContext, scores *FeatureDoubleValues) (*FeatureDoubleValues, error) {
	_ = scores
	if ctx == nil {
		return nil, errors.New("leaf reader context must not be nil")
	}
	leaf := ctx.LeafReader()
	if leaf == nil {
		return newEmptyFeatureDoubleValues(), nil
	}
	terms, err := leaf.Terms(s.field)
	if err != nil {
		return nil, fmt.Errorf("feature double values: read terms for %q: %w", s.field, err)
	}
	if terms == nil {
		return newEmptyFeatureDoubleValues(), nil
	}
	iterator, err := terms.GetIterator()
	if err != nil {
		return nil, fmt.Errorf("feature double values: iterator for %q: %w", s.field, err)
	}
	if iterator == nil {
		return newEmptyFeatureDoubleValues(), nil
	}
	found, err := iterator.SeekExact(s.featureTerm)
	if err != nil {
		return nil, fmt.Errorf("feature double values: seek feature %q: %w", s.featureName, err)
	}
	if !found {
		return newEmptyFeatureDoubleValues(), nil
	}
	postings, err := iterator.Postings(postingsFlagFreqs)
	if err != nil {
		return nil, fmt.Errorf("feature double values: postings for %q: %w", s.featureName, err)
	}
	return newFeatureDoubleValues(postings), nil
}

// Equals reports whether other targets the same field and feature name.
// Mirrors the Java equals contract; nil-safe in both directions.
func (s *FeatureDoubleValuesSource) Equals(other *FeatureDoubleValuesSource) bool {
	if s == other {
		return true
	}
	if s == nil || other == nil {
		return false
	}
	return s.field == other.field && s.featureName == other.featureName
}

// HashCode returns a hash compatible with Equals. Mirrors the Java
// Objects.hash(field, featureName) combination using the prime-31 chaining
// convention the rest of the package follows for string hashing.
func (s *FeatureDoubleValuesSource) HashCode() int {
	const prime = 31
	h := 0
	for i := 0; i < len(s.field); i++ {
		h = prime*h + int(s.field[i])
	}
	for i := 0; i < len(s.featureName); i++ {
		h = prime*h + int(s.featureName[i])
	}
	return h
}

// String returns the Lucene-equivalent toString representation, including the
// utf8 form of the feature name.
func (s *FeatureDoubleValuesSource) String() string {
	return fmt.Sprintf("FeatureDoubleValuesSource(%s, %s)", s.field, s.featureName)
}

// FeatureDoubleValues decodes a feature field's term-frequency postings into
// double-precision scores, advancing through postings on demand. It mirrors
// the nested FeatureDoubleValuesSource.FeatureDoubleValues class in Lucene.
//
// A nil postings field marks the empty reader (Lucene's DoubleValues.EMPTY
// equivalent); AdvanceExact then always returns false and DoubleValue
// returns 0 without consulting any backing storage.
//
// Concurrency: instances are not safe for concurrent use. Each call to
// FeatureDoubleValuesSource.GetValues yields a fresh reader; collectors
// must not share instances across goroutines.
type FeatureDoubleValues struct {
	postings index.PostingsEnum
}

// newFeatureDoubleValues wraps postings into a reader that decodes feature
// values on demand. postings must not be nil.
func newFeatureDoubleValues(postings index.PostingsEnum) *FeatureDoubleValues {
	return &FeatureDoubleValues{postings: postings}
}

// newEmptyFeatureDoubleValues returns the empty reader sentinel — equivalent
// to Lucene's DoubleValues.EMPTY for this source.
func newEmptyFeatureDoubleValues() *FeatureDoubleValues {
	return &FeatureDoubleValues{postings: nil}
}

// DoubleValue returns the decoded feature value for the currently positioned
// document. The float32 produced by DecodeFeatureValueFromTermFreq is widened
// to float64 to match the Java return type without losing the underlying
// representation. Returns 0 on the empty reader.
func (v *FeatureDoubleValues) DoubleValue() (float64, error) {
	if v.postings == nil {
		return 0, nil
	}
	freq, err := v.postings.Freq()
	if err != nil {
		return 0, fmt.Errorf("feature double values: read freq: %w", err)
	}
	return float64(document.DecodeFeatureValueFromTermFreq(int32(freq))), nil
}

// AdvanceExact positions the reader on doc and reports whether the document
// is present in the feature term's postings list. Mirrors Lucene's guarded
// advance dance: if the current cursor is already past doc, the call is a
// no-op and returns false; otherwise it advances to or past doc and reports
// equality.
func (v *FeatureDoubleValues) AdvanceExact(doc int) (bool, error) {
	if v.postings == nil {
		return false, nil
	}
	currentDoc := v.postings.DocID()
	if doc < currentDoc {
		return false, nil
	}
	if currentDoc == doc {
		return true, nil
	}
	next, err := v.postings.Advance(doc)
	if err != nil {
		return false, fmt.Errorf("feature double values: advance to %d: %w", doc, err)
	}
	return next == doc, nil
}
