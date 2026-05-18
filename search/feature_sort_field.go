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

// ErrFeatureSortFieldMissingValueUnsupported reports the error raised when a
// caller tries to set a missing value on a FeatureSortField. Mirrors the
// IllegalArgumentException thrown by Lucene's FeatureSortField.setMissingValue.
var ErrFeatureSortFieldMissingValueUnsupported = errors.New(
	"missing value not supported for FeatureSortField",
)

// FeatureSortField sorts hits by the value of a particular feature name
// stored inside a FeatureField. It mirrors org.apache.lucene.document
// .FeatureSortField; the Go port lives in the search package to avoid the
// document -> search dependency cycle that the Java original sidesteps via
// package-private visibility.
//
// The sort is always reversed (descending) because higher feature values are
// considered better, exactly as in Lucene.
type FeatureSortField struct {
	*SortField

	featureName string
}

// NewFeatureSortField creates a FeatureSortField that sorts hits by the value
// of featureName inside the named FeatureField. Both field and featureName
// must be non-empty; otherwise an error is returned. Mirrors the
// requireNonNull checks on the Java constructor.
func NewFeatureSortField(field, featureName string) (*FeatureSortField, error) {
	if field == "" {
		return nil, errors.New("field must not be empty")
	}
	if featureName == "" {
		return nil, errors.New("featureName must not be empty")
	}
	sf := NewSortField(field, SortFieldTypeCustom)
	sf.Reverse = true
	return &FeatureSortField{
		SortField:   sf,
		featureName: featureName,
	}, nil
}

// FeatureName returns the feature name this sort field is targeting.
func (f *FeatureSortField) FeatureName() string {
	return f.featureName
}

// GetComparator returns a FeatureComparator sized for numHits queue slots.
// The pruning parameter mirrors Lucene's signature; the comparator does not
// currently exploit pruning hints, matching the reference implementation.
func (f *FeatureSortField) GetComparator(numHits int, pruning Pruning) *FeatureComparator {
	return NewFeatureComparator(numHits, f.SortField.Field, f.featureName)
}

// SetMissingValue rejects the call with
// ErrFeatureSortFieldMissingValueUnsupported, mirroring Lucene's
// IllegalArgumentException. The base SortField.MissingValue field remains
// untouched.
func (f *FeatureSortField) SetMissingValue(value interface{}) error {
	_ = value
	return ErrFeatureSortFieldMissingValueUnsupported
}

// Equals reports whether other is a FeatureSortField targeting the same field,
// type, reverse flag, and feature name. Mirrors the Java equals contract.
func (f *FeatureSortField) Equals(other *FeatureSortField) bool {
	if f == other {
		return true
	}
	if f == nil || other == nil {
		return false
	}
	if f.SortField == nil || other.SortField == nil {
		return false
	}
	if f.SortField.Field != other.SortField.Field ||
		f.SortField.Type != other.SortField.Type ||
		f.SortField.Reverse != other.SortField.Reverse {
		return false
	}
	return f.featureName == other.featureName
}

// HashCode returns a hash compatible with Equals. The combination mirrors the
// Java implementation: prime-31 chaining over the parent fields plus the
// feature name's string hash.
func (f *FeatureSortField) HashCode() int {
	const prime = 31
	h := 0
	if f.SortField != nil {
		for i := 0; i < len(f.SortField.Field); i++ {
			h = prime*h + int(f.SortField.Field[i])
		}
		h = prime*h + int(f.SortField.Type)
		if f.SortField.Reverse {
			h = prime*h + 1
		} else {
			h = prime * h
		}
	}
	for i := 0; i < len(f.featureName); i++ {
		h = prime*h + int(f.featureName[i])
	}
	return h
}

// String returns the Lucene-equivalent toString representation.
func (f *FeatureSortField) String() string {
	field := ""
	if f.SortField != nil {
		field = f.SortField.Field
	}
	return fmt.Sprintf(`<feature:"%s" featureName=%s>`, field, f.featureName)
}

// FeatureComparator parses feature-field term frequencies as float32 values
// and sorts by descending value. It mirrors the nested
// FeatureSortField.FeatureComparator class in Lucene. Concurrency: instances
// are not safe for concurrent use; each TopFieldCollector slot owns one.
type FeatureComparator struct {
	field       string
	featureTerm *index.Term

	values   []float32
	bottom   float32
	topValue float32

	currentReaderPostingsValues index.PostingsEnum
}

// NewFeatureComparator builds a FeatureComparator backed by numHits slots,
// reading from the postings of featureName inside field.
func NewFeatureComparator(numHits int, field, featureName string) *FeatureComparator {
	return &FeatureComparator{
		field:       field,
		featureTerm: index.NewTerm(field, featureName),
		values:      make([]float32, numHits),
	}
}

// DoSetNextReader prepares per-leaf state by seeking the postings list for the
// configured feature term inside the leaf's terms dictionary. If the leaf has
// no postings for the feature term (or the field itself is absent), the
// comparator falls back to zero values, mirroring Lucene's behaviour.
func (c *FeatureComparator) DoSetNextReader(ctx *index.LeafReaderContext) error {
	if ctx == nil {
		return errors.New("leaf reader context must not be nil")
	}
	leaf := ctx.LeafReader()
	if leaf == nil {
		c.currentReaderPostingsValues = nil
		return nil
	}
	terms, err := leaf.Terms(c.field)
	if err != nil {
		return fmt.Errorf("feature sort: read terms for %q: %w", c.field, err)
	}
	if terms == nil {
		c.currentReaderPostingsValues = nil
		return nil
	}
	iterator, err := terms.GetIterator()
	if err != nil {
		return fmt.Errorf("feature sort: iterator for %q: %w", c.field, err)
	}
	if iterator == nil {
		c.currentReaderPostingsValues = nil
		return nil
	}
	found, err := iterator.SeekExact(c.featureTerm)
	if err != nil {
		return fmt.Errorf("feature sort: seek feature %q: %w", c.featureTerm.Text(), err)
	}
	if !found {
		c.currentReaderPostingsValues = nil
		return nil
	}
	postings, err := iterator.Postings(postingsFlagFreqs)
	if err != nil {
		return fmt.Errorf("feature sort: postings for %q: %w", c.featureTerm.Text(), err)
	}
	c.currentReaderPostingsValues = postings
	return nil
}

// postingsFlagFreqs mirrors PostingsEnum.FREQS from Lucene (0x8). The Gocene
// PostingsEnum interface does not expose named flag constants, so we keep a
// package-private mirror to make the intent explicit at call sites.
const postingsFlagFreqs = 0x8

// Compare returns Float.compare(values[slot1], values[slot2]).
func (c *FeatureComparator) Compare(slot1, slot2 int) int {
	return compareFloat32(c.values[slot1], c.values[slot2])
}

// CompareBottom returns Float.compare(bottom, getValueForDoc(doc)).
func (c *FeatureComparator) CompareBottom(doc int) (int, error) {
	v, err := c.getValueForDoc(doc)
	if err != nil {
		return 0, err
	}
	return compareFloat32(c.bottom, v), nil
}

// Copy stores the decoded value for doc into values[slot].
func (c *FeatureComparator) Copy(slot, doc int) error {
	v, err := c.getValueForDoc(doc)
	if err != nil {
		return err
	}
	c.values[slot] = v
	return nil
}

// SetBottom records the bottom slot's value as the threshold for subsequent
// CompareBottom calls.
func (c *FeatureComparator) SetBottom(slot int) {
	c.bottom = c.values[slot]
}

// SetTopValue stores the value used as the top reference for CompareTop. Used
// for deep pagination.
func (c *FeatureComparator) SetTopValue(value float32) {
	c.topValue = value
}

// Value returns the decoded float value stored in the given slot.
func (c *FeatureComparator) Value(slot int) float32 {
	return c.values[slot]
}

// CompareTop returns Float.compare(topValue, getValueForDoc(doc)).
func (c *FeatureComparator) CompareTop(doc int) (int, error) {
	v, err := c.getValueForDoc(doc)
	if err != nil {
		return 0, err
	}
	return compareFloat32(c.topValue, v), nil
}

// getValueForDoc returns the decoded feature value for doc, or 0.0 if the
// current leaf has no postings for the feature term or the document is past
// the postings cursor. Mirrors Lucene's guarded advance dance.
func (c *FeatureComparator) getValueForDoc(doc int) (float32, error) {
	if c.currentReaderPostingsValues == nil {
		return 0, nil
	}
	postings := c.currentReaderPostingsValues
	currentDoc := postings.DocID()
	if doc < currentDoc {
		return 0, nil
	}
	if currentDoc != doc {
		next, err := postings.Advance(doc)
		if err != nil {
			return 0, fmt.Errorf("feature sort: advance to %d: %w", doc, err)
		}
		if next != doc {
			return 0, nil
		}
	}
	freq, err := postings.Freq()
	if err != nil {
		return 0, fmt.Errorf("feature sort: read freq at doc %d: %w", doc, err)
	}
	return document.DecodeFeatureValueFromTermFreq(int32(freq)), nil
}

// compareFloat32 mirrors Java's Float.compare semantics: it returns -1, 0, or
// 1 and treats NaN as larger than any non-NaN value. The Gocene FeatureField
// pipeline rejects NaN on the write side, but the comparator stays defensive.
func compareFloat32(a, b float32) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	case a == b:
		return 0
	}
	// At least one operand is NaN. Match Float.compare's convention.
	aNaN := a != a
	bNaN := b != b
	switch {
	case aNaN && bNaN:
		return 0
	case aNaN:
		return 1
	default:
		return -1
	}
}
