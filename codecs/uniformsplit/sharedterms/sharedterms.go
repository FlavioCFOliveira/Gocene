// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package sharedterms provides pluggable term index / block terms dictionary
// implementations.
//
// Extension of [github.com/FlavioCFOliveira/Gocene/codecs/uniformsplit] with
// the Shared Terms principle: terms are shared between all fields. It is
// particularly adapted to index a massive number of fields because all the
// terms are stored in a single FST dictionary.
//
//   - Designed to be extensible
//   - Highly reduced on-heap memory usage when dealing with a massive number
//     of fields.
//
// Mirrors org.apache.lucene.codecs.uniformsplit.sharedterms (package-info.java)
// from Apache Lucene 10.5.0.
package sharedterms
