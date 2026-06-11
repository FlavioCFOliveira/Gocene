// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package valuesource

import (
	"github.com/FlavioCFOliveira/Gocene/queries/function"
)

// SingleFunction is a ValueSource with a single argument. Concrete
// implementations override Name() and provide their own GetValues.
//
// Go port of org.apache.lucene.queries.function.valuesource.SingleFunction.
type SingleFunction struct {
	function.BaseValueSource
	Source function.ValueSource
	name   string
}

// NewSingleFunction returns a SingleFunction wrapping source.
func NewSingleFunction(source function.ValueSource, name string) *SingleFunction {
	return &SingleFunction{Source: source, name: name}
}

// Name returns the function name.
func (s *SingleFunction) Name() string { return s.name }

// Description returns "name(source)".
func (s *SingleFunction) Description() string { return s.name + "(" + s.Source.Description() + ")" }

// CreateWeight delegates to the wrapped source.
func (s *SingleFunction) CreateWeight(ctx function.Context, searcher any) error {
	return s.Source.CreateWeight(ctx, searcher)
}
