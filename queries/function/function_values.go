// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package function

import (
	"errors"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// ErrUnsupportedValue is returned by [FunctionValues] accessors whose typed
// variant has not been overridden by a concrete implementation. It mirrors
// Lucene's UnsupportedOperationException raised by the abstract base methods.
var ErrUnsupportedValue = errors.New("function: value accessor not supported by this FunctionValues")

// MutableValueFloat is the Go mirror of
// org.apache.lucene.util.mutable.MutableValueFloat. It holds a single float32
// payload plus the standard exists flag used by [ValueFiller].
//
// Gocene deviation: the full org.apache.lucene.util.mutable hierarchy is
// not ported in Sprint 29; this type covers the only consumer needed by
// the default [FunctionValues.GetValueFiller] implementation.
type MutableValueFloat struct {
	Value  float32
	Exists bool
}

// ValueFiller is the Lucene-faithful counterpart to
// FunctionValues.ValueFiller. Implementations expose a reusable
// [MutableValueFloat] (or other concrete mutable value) that callers
// re-read after every call to FillValue.
type ValueFiller interface {
	// GetValue returns the mutable value reused across calls.
	GetValue() *MutableValueFloat
	// FillValue updates the reusable value for the supplied docID.
	FillValue(doc int) error
}

// FunctionValues represents field values projected as different primitive
// types. Concrete implementations override the subset of typed accessors
// they natively support; the rest fall through to [ErrUnsupportedValue],
// matching the Java contract.
//
// FunctionValues is intentionally distinct from [ValueSource] because the
// per-segment evaluation object must not be referenced by the query (queries
// are used as cache keys and must stay lightweight + MT-safe).
type FunctionValues interface {
	// ByteVal returns the byte representation for doc.
	ByteVal(doc int) (int8, error)
	// ShortVal returns the int16 representation for doc.
	ShortVal(doc int) (int16, error)
	// FloatVal returns the float32 representation for doc.
	FloatVal(doc int) (float32, error)
	// IntVal returns the int32 representation for doc.
	IntVal(doc int) (int32, error)
	// LongVal returns the int64 representation for doc.
	LongVal(doc int) (int64, error)
	// DoubleVal returns the float64 representation for doc.
	DoubleVal(doc int) (float64, error)
	// StrVal returns the string representation for doc.
	StrVal(doc int) (string, error)
	// BoolVal returns the boolean representation for doc. The default
	// implementation derives the value from IntVal != 0.
	BoolVal(doc int) (bool, error)
	// FloatVectorVal returns the float vector representation for doc.
	FloatVectorVal(doc int) ([]float32, error)
	// ByteVectorVal returns the byte vector representation for doc.
	ByteVectorVal(doc int) ([]byte, error)
	// BytesVal copies the bytes representation of the value into target.
	// Returns true when the value exists.
	BytesVal(doc int, target *[]byte) (bool, error)
	// ObjectVal returns the native Go representation. Defaults to FloatVal.
	ObjectVal(doc int) (any, error)
	// Exists reports whether the document has a value.
	Exists(doc int) (bool, error)
	// OrdVal returns the sort ordinal of doc.
	OrdVal(doc int) (int, error)
	// NumOrd returns the count of distinct sort ordinals.
	NumOrd() (int, error)
	// Cost returns an estimate of the per-doc evaluation cost in
	// simple operations (additions, multiplications, comparisons).
	Cost() float32
	// ToString renders a human-readable representation of doc.
	ToString(doc int) (string, error)
	// GetValueFiller returns a reusable [ValueFiller] for doc-by-doc reads.
	GetValueFiller() ValueFiller
	// ByteValMulti fills vals with the doc's multi-byte payload.
	ByteValMulti(doc int, vals []byte) error
	// ShortValMulti fills vals with the doc's multi-short payload.
	ShortValMulti(doc int, vals []int16) error
	// FloatValMulti fills vals with the doc's multi-float payload.
	FloatValMulti(doc int, vals []float32) error
	// IntValMulti fills vals with the doc's multi-int payload.
	IntValMulti(doc int, vals []int32) error
	// LongValMulti fills vals with the doc's multi-long payload.
	LongValMulti(doc int, vals []int64) error
	// DoubleValMulti fills vals with the doc's multi-double payload.
	DoubleValMulti(doc int, vals []float64) error
	// StrValMulti fills vals with the doc's multi-string payload.
	StrValMulti(doc int, vals []string) error
	// Explain returns a string-form explanation for doc.
	Explain(doc int) (string, error)
	// GetScorer returns a [ValueSourceScorer] that matches every doc
	// in readerContext and yields FloatVal as the score.
	GetScorer(readerContext *index.LeafReaderContext) ValueSourceScorer
	// GetRangeScorer returns a [ValueSourceScorer] whose match predicate
	// restricts results to FloatVal in the requested range.
	GetRangeScorer(
		readerContext *index.LeafReaderContext,
		lowerVal, upperVal string,
		includeLower, includeUpper bool,
	) (ValueSourceScorer, error)
}

// BaseFunctionValues is an embeddable struct that supplies the default
// behaviour of [FunctionValues]. Concrete value families embed it and
// override the typed accessors they actually support.
//
// All un-overridden typed methods return [ErrUnsupportedValue] to match
// Java's UnsupportedOperationException, and the multi-valued helpers
// behave identically.
type BaseFunctionValues struct {
	// Self is the outer FunctionValues used by default-implemented helpers
	// (BoolVal, ObjectVal, BytesVal, GetScorer, GetRangeScorer, Explain,
	// GetValueFiller) so that they call the most-derived overrides.
	Self FunctionValues
}

// SetSelf binds the outermost implementation pointer. Concrete types should
// call SetSelf(self) immediately after construction so polymorphic default
// methods (e.g. [BaseFunctionValues.BoolVal]) dispatch to overrides.
func (b *BaseFunctionValues) SetSelf(self FunctionValues) {
	b.Self = self
}

// outer returns the externally visible FunctionValues (Self if bound, else b).
func (b *BaseFunctionValues) outer() FunctionValues {
	if b.Self != nil {
		return b.Self
	}
	// Should never happen for correctly initialised concrete impls.
	return nil
}

// ByteVal returns ErrUnsupportedValue by default.
func (b *BaseFunctionValues) ByteVal(_ int) (int8, error) { return 0, ErrUnsupportedValue }

// ShortVal returns ErrUnsupportedValue by default.
func (b *BaseFunctionValues) ShortVal(_ int) (int16, error) { return 0, ErrUnsupportedValue }

// FloatVal returns ErrUnsupportedValue by default.
func (b *BaseFunctionValues) FloatVal(_ int) (float32, error) { return 0, ErrUnsupportedValue }

// IntVal returns ErrUnsupportedValue by default.
func (b *BaseFunctionValues) IntVal(_ int) (int32, error) { return 0, ErrUnsupportedValue }

// LongVal returns ErrUnsupportedValue by default.
func (b *BaseFunctionValues) LongVal(_ int) (int64, error) { return 0, ErrUnsupportedValue }

// DoubleVal returns ErrUnsupportedValue by default.
func (b *BaseFunctionValues) DoubleVal(_ int) (float64, error) { return 0, ErrUnsupportedValue }

// StrVal returns ErrUnsupportedValue by default.
func (b *BaseFunctionValues) StrVal(_ int) (string, error) { return "", ErrUnsupportedValue }

// BoolVal derives the value from IntVal != 0, mirroring the Java default.
func (b *BaseFunctionValues) BoolVal(doc int) (bool, error) {
	v, err := b.outer().IntVal(doc)
	if err != nil {
		return false, err
	}
	return v != 0, nil
}

// FloatVectorVal returns ErrUnsupportedValue by default.
func (b *BaseFunctionValues) FloatVectorVal(_ int) ([]float32, error) {
	return nil, ErrUnsupportedValue
}

// ByteVectorVal returns ErrUnsupportedValue by default.
func (b *BaseFunctionValues) ByteVectorVal(_ int) ([]byte, error) {
	return nil, ErrUnsupportedValue
}

// BytesVal copies the StrVal output bytes into target.
func (b *BaseFunctionValues) BytesVal(doc int, target *[]byte) (bool, error) {
	s, err := b.outer().StrVal(doc)
	if err != nil {
		if target != nil {
			*target = (*target)[:0]
		}
		// StrVal not supported → no value, but propagate the error so callers
		// know the underlying state. Java returns false silently after copying;
		// Go callers can distinguish the two conditions through err.
		return false, err
	}
	if target == nil {
		return s != "", nil
	}
	*target = append((*target)[:0], s...)
	return true, nil
}

// ObjectVal defaults to FloatVal boxed as float32.
func (b *BaseFunctionValues) ObjectVal(doc int) (any, error) {
	return b.outer().FloatVal(doc)
}

// Exists reports presence for the given doc. The default returns true.
func (b *BaseFunctionValues) Exists(_ int) (bool, error) { return true, nil }

// OrdVal returns ErrUnsupportedValue by default.
func (b *BaseFunctionValues) OrdVal(_ int) (int, error) { return 0, ErrUnsupportedValue }

// NumOrd returns ErrUnsupportedValue by default.
func (b *BaseFunctionValues) NumOrd() (int, error) { return 0, ErrUnsupportedValue }

// Cost returns the default 100-unit estimate matching Java's default.
func (b *BaseFunctionValues) Cost() float32 { return 100 }

// ToString must be supplied by concrete implementations; the base returns
// a deterministic stub to keep zero-value safe.
func (b *BaseFunctionValues) ToString(doc int) (string, error) {
	return fmt.Sprintf("FunctionValues(doc=%d)", doc), nil
}

// GetValueFiller returns a reusable ValueFiller backed by FloatVal.
func (b *BaseFunctionValues) GetValueFiller() ValueFiller {
	return &floatValueFiller{vals: b.outer()}
}

// ByteValMulti is unsupported by default.
func (b *BaseFunctionValues) ByteValMulti(_ int, _ []byte) error { return ErrUnsupportedValue }

// ShortValMulti is unsupported by default.
func (b *BaseFunctionValues) ShortValMulti(_ int, _ []int16) error { return ErrUnsupportedValue }

// FloatValMulti is unsupported by default.
func (b *BaseFunctionValues) FloatValMulti(_ int, _ []float32) error { return ErrUnsupportedValue }

// IntValMulti is unsupported by default.
func (b *BaseFunctionValues) IntValMulti(_ int, _ []int32) error { return ErrUnsupportedValue }

// LongValMulti is unsupported by default.
func (b *BaseFunctionValues) LongValMulti(_ int, _ []int64) error { return ErrUnsupportedValue }

// DoubleValMulti is unsupported by default.
func (b *BaseFunctionValues) DoubleValMulti(_ int, _ []float64) error { return ErrUnsupportedValue }

// StrValMulti is unsupported by default.
func (b *BaseFunctionValues) StrValMulti(_ int, _ []string) error { return ErrUnsupportedValue }

// Explain renders a one-line representation of the doc and its float value,
// mirroring Lucene's default Explanation.match(floatVal, toString(doc)).
func (b *BaseFunctionValues) Explain(doc int) (string, error) {
	val, err := b.outer().FloatVal(doc)
	if err != nil {
		return "", err
	}
	desc, err := b.outer().ToString(doc)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%g (%s)", val, desc), nil
}

// GetScorer returns an always-matching scorer rooted at the outer FunctionValues.
func (b *BaseFunctionValues) GetScorer(readerContext *index.LeafReaderContext) ValueSourceScorer {
	return newAllValueSourceScorer(readerContext, b.outer())
}

// GetRangeScorer returns a scorer that filters docs by FloatVal range.
// Empty bounds map to ±Inf, identical to Java's Float.parseFloat handling.
func (b *BaseFunctionValues) GetRangeScorer(
	readerContext *index.LeafReaderContext,
	lowerVal, upperVal string,
	includeLower, includeUpper bool,
) (ValueSourceScorer, error) {
	lo, err := parseRangeBound(lowerVal, true)
	if err != nil {
		return nil, err
	}
	hi, err := parseRangeBound(upperVal, false)
	if err != nil {
		return nil, err
	}
	return newRangeValueSourceScorer(readerContext, b.outer(), lo, hi, includeLower, includeUpper), nil
}

// floatValueFiller is the default ValueFiller backed by FloatVal.
type floatValueFiller struct {
	vals FunctionValues
	mval MutableValueFloat
}

func (f *floatValueFiller) GetValue() *MutableValueFloat { return &f.mval }

func (f *floatValueFiller) FillValue(doc int) error {
	v, err := f.vals.FloatVal(doc)
	if err != nil {
		f.mval.Value = 0
		f.mval.Exists = false
		return err
	}
	f.mval.Value = v
	f.mval.Exists = true
	return nil
}
