// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package valuesource is the Go port of
// org.apache.lucene.queries.function.valuesource (Apache Lucene 10.4.0).
//
// It contains the concrete ValueSource implementations that build on the
// abstract base [function.ValueSource] and the typed DocValues helpers
// in [docvalues].
//
// Implemented classes (Tiers 1-4):
//
// Tier 1 - Simple field sources:
//   - FieldCacheSource (abstract base)
//   - FloatFieldSource
//   - DoubleFieldSource
//   - IntFieldSource
//   - LongFieldSource
//   - BytesRefFieldSource
//   - EnumFieldSource
//
// Tier 2 - Multi-valued field sources:
//   - MultiValuedFloatFieldSource
//   - MultiValuedIntFieldSource
//   - MultiValuedLongFieldSource
//   - MultiValuedDoubleFieldSource
//   - SortedSetFieldSource
//
// Tier 3 - Function wrappers:
//   - SingleFunction, SimpleFloatFunction, DualFloatFunction
//   - MultiFunction, MultiFloatFunction
//   - DivFloatFunction, ProductFloatFunction, SumFloatFunction
//   - MaxFloatFunction, MinFloatFunction, PowFloatFunction
//   - ReciprocalFloatFunction, ScaleFloatFunction, LinearFloatFunction
//
// Tier 4 - Complex functions:
//   - BoolFunction, SimpleBoolFunction, ComparisonBoolFunction
//   - IfFunction, RangeMapFloatFunction
//
// Constant + literal foundation:
//   - ConstNumberSource, ConstValueSource, DoubleConstValueSource
//   - LiteralValueSource
//
// Tier 5 (KNN vector classes) are not yet ported; they require the
// KNN vector index infrastructure.
package valuesource
