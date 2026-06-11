// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package valuesource

// BoolFunction is an abstract parent for ValueSource implementations
// that apply boolean logic to their values.
//
// Go port of org.apache.lucene.queries.function.valuesource.BoolFunction.
type BoolFunction struct{}

// NewBoolFunction creates a BoolFunction.
func NewBoolFunction() *BoolFunction { return &BoolFunction{} }
