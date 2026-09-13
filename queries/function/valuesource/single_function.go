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

// HashCode mirrors hashCode(): source.hashCode() + name().hashCode().
func (s *SingleFunction) HashCode() int32 {
	return s.Source.HashCode() + hashString(s.name)
}

// GetSource returns the wrapped source. It exposes the protected field
// `source` so that Equals and HashCode can read it through an embedding
// concrete type.
func (s *SingleFunction) GetSource() function.ValueSource { return s.Source }

// singleFunctionOperands is the accessor pair SingleFunction.equals reads: Java
// compares `this.name().equals(other.name()) && this.source.equals(other.source)`.
// A Go type that embeds SingleFunction is never assertable to the embedded
// struct, so both are read through the promoted accessors.
type singleFunctionOperands interface {
	Name() string
	GetSource() function.ValueSource
}

// Equals mirrors equals(Object): same class, equal name and equal source.
func (s *SingleFunction) Equals(other function.ValueSource) bool {
	o, ok := other.(singleFunctionOperands)
	if !ok {
		return false
	}
	return s.name == o.Name() && s.Source.Equals(o.GetSource())
}
