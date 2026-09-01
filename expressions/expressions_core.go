// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package expressions

import (
	"github.com/FlavioCFOliveira/Gocene/index"
)

// Expression is a function that can be evaluated against a document.
//
// This is the Go port of Lucene's org.apache.lucene.expressions.Expression.
type Expression interface {
	// Evaluate evaluates the expression for the given document.
	Evaluate(bindings Bindings, doc int) (float64, error)
}

// Bindings provides a way to look up values for an expression.
//
// This is the Go port of Lucene's org.apache.lucene.expressions.Bindings.
type Bindings interface {
	// GetValue returns the value for the given key.
	GetValue(key string) (float64, bool)
}

// SimpleBindings is a simple implementation of Bindings.
//
// This is the Go port of Lucene's org.apache.lucene.expressions.SimpleBindings.
type SimpleBindings struct {
	values map[string]float64
}

func NewSimpleBindings() *SimpleBindings {
	return &SimpleBindings{
		values: make(map[string]float64),
	}
}

func (s *SimpleBindings) Put(key string, value float64) {
	s.values[key] = value
}

func (s *SimpleBindings) GetValue(key string) (float64, bool) {
	val, ok := s.values[key]
	return val, ok
}
