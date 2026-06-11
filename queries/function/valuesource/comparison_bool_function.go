// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package valuesource

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/queries/function"
	"github.com/FlavioCFOliveira/Gocene/queries/function/docvalues"
)

// ComparisonBoolFunction is a base class for comparison operators useful
// within an "if"/conditional.
//
// Go port of org.apache.lucene.queries.function.valuesource.ComparisonBoolFunction.
type ComparisonBoolFunction struct {
	*BoolFunction
	LHS      function.ValueSource
	RHS      function.ValueSource
	compName string
}

// NewComparisonBoolFunction creates a ComparisonBoolFunction.
func NewComparisonBoolFunction(lhs, rhs function.ValueSource, compName string) *ComparisonBoolFunction {
	return &ComparisonBoolFunction{
		BoolFunction: &BoolFunction{},
		LHS:          lhs,
		RHS:          rhs,
		compName:     compName,
	}
}

// Name returns the comparison name.
func (s *ComparisonBoolFunction) Name() string { return s.compName }

// Description returns "name(lhs,rhs)".
func (s *ComparisonBoolFunction) Description() string {
	return s.compName + "(" + s.LHS.Description() + "," + s.RHS.Description() + ")"
}

// CreateWeight delegates to both sources.
func (s *ComparisonBoolFunction) CreateWeight(ctx function.Context, searcher any) error {
	if err := s.LHS.CreateWeight(ctx, searcher); err != nil {
		return err
	}
	return s.RHS.CreateWeight(ctx, searcher)
}

// GetValues returns BoolDocValues using the Compare function.
func (s *ComparisonBoolFunction) GetValues(ctx function.Context, readerContext *index.LeafReaderContext) (function.FunctionValues, error) {
	lhsVals, err := s.LHS.GetValues(ctx, readerContext)
	if err != nil {
		return nil, err
	}
	rhsVals, err := s.RHS.GetValues(ctx, readerContext)
	if err != nil {
		return nil, err
	}
	v := &comparisonBoolFunctionValues{
		BoolDocValues: *docvalues.NewBoolDocValues(s, func(doc int) (bool, error) {
			return s.Compare(doc, lhsVals, rhsVals)
		}),
		lhsVals: lhsVals,
		rhsVals: rhsVals,
		name:    s.compName,
	}
	v.SetSelf(v)
	return v, nil
}

// Compare performs the comparison. Concrete types implement this.
func (s *ComparisonBoolFunction) Compare(doc int, lhs, rhs function.FunctionValues) (bool, error) {
	return false, nil
}

// Equals reports value equality.
func (s *ComparisonBoolFunction) Equals(other function.ValueSource) bool {
	o, ok := other.(*ComparisonBoolFunction)
	if !ok || o == nil {
		return false
	}
	return s.compName == o.compName && s.LHS.Equals(o.LHS) && s.RHS.Equals(o.RHS)
}

// HashCode returns a stable hash.
func (s *ComparisonBoolFunction) HashCode() int32 {
	h := hashString(s.compName)
	h = h*31 + s.LHS.HashCode()
	h = h*31 + s.RHS.HashCode()
	return h
}

type comparisonBoolFunctionValues struct {
	docvalues.BoolDocValues
	lhsVals, rhsVals function.FunctionValues
	name             string
}

func (v *comparisonBoolFunctionValues) Exists(doc int) (bool, error) {
	lok, err := v.lhsVals.Exists(doc)
	if err != nil || !lok {
		return false, err
	}
	return v.rhsVals.Exists(doc)
}

func (v *comparisonBoolFunctionValues) ToString(doc int) (string, error) {
	ls, err := v.lhsVals.ToString(doc)
	if err != nil {
		return "", err
	}
	rs, err := v.rhsVals.ToString(doc)
	if err != nil {
		return "", err
	}
	return v.name + "(" + ls + "," + rs + ")", nil
}

var _ function.ValueSource = (*ComparisonBoolFunction)(nil)
