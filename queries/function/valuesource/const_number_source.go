// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package valuesource

// ConstNumberSource is the abstract contract shared by all constant
// numeric value sources. Concrete implementations expose typed accessors
// so callers don't need to construct FunctionValues for what is
// effectively a compile-time constant.
//
// Go port of org.apache.lucene.queries.function.valuesource.ConstNumberSource.
type ConstNumberSource interface {
	GetInt() int32
	GetLong() int64
	GetFloat() float32
	GetDouble() float64
	GetNumber() any
	GetBool() bool
}
