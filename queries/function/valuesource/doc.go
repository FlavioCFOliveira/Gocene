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
// Sprint 29 ships the constant + literal foundation
// (ConstNumberSource, ConstValueSource, DoubleConstValueSource,
// LiteralValueSource); the remaining 54 value sources (field sources,
// arithmetic functions, KNN vector sources, etc.) are tracked under
// Sprint 45 to keep Sprint 29 deliverables consistent with the
// pre-shipped store/index/search surface.
package valuesource
