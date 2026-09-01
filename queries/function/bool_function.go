// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package function

// BoolFunction is the base for ValueSource implementations that apply boolean logic.
//
// Mirrors org.apache.lucene.queries.function.valuesource.BoolFunction.
type BoolFunction struct {
	BaseValueSource
}
