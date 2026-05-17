// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package docvalues is the Go port of
// org.apache.lucene.queries.function.docvalues (Apache Lucene 10.4.0).
//
// It provides typed [function.FunctionValues] base implementations that
// concrete value sources embed: BoolDocValues, ByteDocValues (n/a in
// Lucene 10), DoubleDocValues, FloatDocValues, IntDocValues,
// LongDocValues, StrDocValues, and DocTermsIndexDocValues.
//
// Each *DocValues type funnels every typed accessor through a single
// abstract method (boolVal/doubleVal/floatVal/intVal/longVal/strVal) so
// implementers only override one method and inherit consistent cross-
// type coercion semantics.
package docvalues
