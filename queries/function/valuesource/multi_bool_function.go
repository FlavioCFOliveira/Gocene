// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package valuesource

import (
	"fmt"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/queries/function"
	"github.com/FlavioCFOliveira/Gocene/queries/function/docvalues"
)

// MultiBoolFunction is a BoolFunction that wraps multiple ValueSources and applies
// an extendible boolean function to their values.
//
// Go port of org.apache.lucene.queries.function.valuesource.MultiBoolFunction.
type MultiBoolFunction struct {
	*BoolFunction
	Sources []function.ValueSource
	name    string
	Func    func(doc int, vals []function.FunctionValues) (bool, error)
}

// NewMultiBoolFunction creates a MultiBoolFunction.
func NewMultiBoolFunction(sources []function.ValueSource, name string, fn func(doc int, vals []function.FunctionValues) (bool, error)) *MultiBoolFunction {
	return &MultiBoolFunction{
		BoolFunction: &BoolFunction{},
		Sources:      sources,
		name:         name,
		Func:         fn,
	}
}

// Name returns the function name.
func (s *MultiBoolFunction) Name() string { return s.name }

// Description returns "name(source1,source2,...)".
func (s *MultiBoolFunction) Description() string {
	var sb strings.Builder
	sb.WriteString(s.name)
	sb.WriteByte('(')
	for i, source := range s.Sources {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(source.Description())
	}
	sb.WriteByte(')')
	return sb.String()
}

// CreateWeight delegates to all sources.
func (s *MultiBoolFunction) CreateWeight(ctx function.Context, searcher any) error {
	for _, source := range s.Sources {
		if err := source.CreateWeight(ctx, searcher); err != nil {
			return err
		}
	}
	return nil
}

// GetValues returns BoolDocValues applying Func.
func (s *MultiBoolFunction) GetValues(ctx function.Context, readerContext *index.LeafReaderContext) (function.FunctionValues, error) {
	vals := make([]function.FunctionValues, len(s.Sources))
	for i, source := range s.Sources {
		v, err := source.GetValues(ctx, readerContext)
		if err != nil {
			return nil, err
		}
		vals[i] = v
	}

	v := &multiBoolFunctionValues{
		BoolDocValues: *docvalues.NewBoolDocValues(s, func(doc int) (bool, error) {
			return s.Func(doc, vals)
		}),
		vals: vals,
		name: s.name,
	}
	v.SetSelf(v)
	return v, nil
}

// Equals reports value equality.
func (s *MultiBoolFunction) Equals(other function.ValueSource) bool {
	o, ok := other.(*MultiBoolFunction)
	if !ok || o == nil {
		return false
	}
	if s.name != o.name || len(s.Sources) != len(o.Sources) {
		return false
	}
	for i := range s.Sources {
		if !s.Sources[i].Equals(o.Sources[i]) {
			return false
		}
	}
	return true
}

// HashCode returns a stable hash.
func (s *MultiBoolFunction) HashCode() int32 {
	h := hashString(s.name)
	for _, source := range s.Sources {
		h = h*31 + source.HashCode()
	}
	return h
}

type multiBoolFunctionValues struct {
	docvalues.BoolDocValues
	vals []function.FunctionValues
	name string
}

func (v *multiBoolFunctionValues) ToString(doc int) (string, error) {
	var sb strings.Builder
	sb.WriteString(v.name)
	sb.WriteByte('(')
	for i, val := range v.vals {
		if i > 0 {
			sb.WriteByte(',')
		}
		s, err := val.ToString(doc)
		if err != nil {
			return "", err
		}
		sb.WriteString(s)
	}
	sb.WriteByte(')')
	return sb.String(), nil
}

var _ function.ValueSource = (*MultiBoolFunction)(nil)
