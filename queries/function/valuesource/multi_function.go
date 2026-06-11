// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package valuesource

import (
	"bytes"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/queries/function"
)

// MultiFunction is the abstract parent for ValueSource implementations
// that wrap multiple ValueSources and apply their own logic.
//
// It also provides static helper utilities (allExists, anyExists,
// description, toString).
//
// Go port of org.apache.lucene.queries.function.valuesource.MultiFunction.
type MultiFunction struct {
	function.BaseValueSource
	Sources []function.ValueSource
	name    string
}

// NewMultiFunction returns a MultiFunction wrapping the given sources.
func NewMultiFunction(sources []function.ValueSource, name string) *MultiFunction {
	return &MultiFunction{Sources: sources, name: name}
}

// Name returns the function name.
func (m *MultiFunction) Name() string { return m.name }

// Description returns "name(src1,src2,...)".
func (m *MultiFunction) Description() string {
	return MultiFunctionDescription(m.name, m.Sources)
}

// CreateWeight delegates to all sources.
func (m *MultiFunction) CreateWeight(ctx function.Context, searcher any) error {
	for _, src := range m.Sources {
		if err := src.CreateWeight(ctx, searcher); err != nil {
			return err
		}
	}
	return nil
}

// AllExists reports whether all values exist for doc.
func AllExists(doc int, values []function.FunctionValues) (bool, error) {
	for _, v := range values {
		ok, err := v.Exists(doc)
		if err != nil {
			return false, err
		}
		if !ok {
			return false, nil
		}
	}
	return true, nil
}

// AnyExists reports whether any value exists for doc.
func AnyExists(doc int, values []function.FunctionValues) (bool, error) {
	for _, v := range values {
		ok, err := v.Exists(doc)
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
	}
	return false, nil
}

// MultiFunctionDescription renders the standard description for a named
// function with the given sources.
func MultiFunctionDescription(name string, sources []function.ValueSource) string {
	buf := bytes.NewBufferString(name)
	buf.WriteByte('(')
	for i, src := range sources {
		if i > 0 {
			buf.WriteByte(',')
		}
		buf.WriteString(src.Description())
	}
	buf.WriteByte(')')
	return buf.String()
}

// MultiFunctionToString renders the standard toString for a named function
// with the given per-doc values.
func MultiFunctionToString(name string, values []function.FunctionValues, doc int) (string, error) {
	buf := bytes.NewBufferString(name)
	buf.WriteByte('(')
	for i, v := range values {
		if i > 0 {
			buf.WriteByte(',')
		}
		vs, err := v.ToString(doc)
		if err != nil {
			return "", fmt.Errorf("multi function tostring: %w", err)
		}
		buf.WriteString(vs)
	}
	buf.WriteByte(')')
	return buf.String(), nil
}

// valsArrFromSources resolves all sources to FunctionValues.
func valsArrFromSources(sources []function.ValueSource, ctx function.Context, readerContext *index.LeafReaderContext) ([]function.FunctionValues, error) {
	vals := make([]function.FunctionValues, len(sources))
	for i, src := range sources {
		v, err := src.GetValues(ctx, readerContext)
		if err != nil {
			return nil, err
		}
		vals[i] = v
	}
	return vals, nil
}
