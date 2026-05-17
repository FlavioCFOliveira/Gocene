// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package function is the Go port of org.apache.lucene.queries.function
// (Apache Lucene 10.4.0).
//
// It exposes the abstract [ValueSource] type along with [FunctionValues]
// and the [FunctionQuery] / [FunctionRangeQuery] / [FunctionMatchQuery] /
// [FunctionScoreQuery] families. Concrete value sources live in the
// `function/valuesource` and `function/docvalues` sub-packages.
//
// Gocene deviations from the Java reference are documented next to each
// type that intentionally differs (notably around the IdentityHashMap
// context, exception model, and the Wrapped{Long,Double}ValuesSource
// adapters which delegate to the trimmed-down search package contracts).
package function
