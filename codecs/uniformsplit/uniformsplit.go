// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package uniformsplit provides pluggable term index / block terms dictionary
// implementations.
//
// Structure similar to
// org.apache.lucene.codecs.blockterms.VariableGapTermsIndexWriter with
// additional optimizations.
//
//   - Designed to be extensible
//   - Reduced on-heap memory usage.
//   - Efficient to seek terms (TermQuery, PhraseQuery)
//   - Quite efficient for PrefixQuery
//   - Not efficient for spell-check and FuzzyQuery, in this case prefer
//     Lucene104PostingsFormat
//
// Mirrors org.apache.lucene.codecs.uniformsplit (package-info.java) from Apache
// Lucene 10.5.0.
package uniformsplit
