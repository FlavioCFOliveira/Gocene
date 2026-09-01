// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package function

// ConstNumberSource is the base interface for all constant numbers.
// Mirrors org.apache.lucene.queries.function.valuesource.ConstNumberSource from Apache Lucene 10.5.0.
type ConstNumberSource interface {
	ValueSource
	GetInt() int32
	GetLong() int64
	GetFloat() float32
	GetDouble() float64
	GetNumber() any
	GetBool() bool
}
