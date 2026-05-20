// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0

package search

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/java/org/apache/lucene/search/DocIdStream.java

// IntConsumer is a callback that receives an int and may return an error.
// Mirrors CheckedIntConsumer<IOException> in Java.
type IntConsumer func(docID int) error

// DocIdStream is a one-shot stream of document IDs in ascending order.
// Doc IDs may be consumed at most once.
//
// Mirrors org.apache.lucene.search.DocIdStream (Lucene 10.4.0).
//
// In Java DocIdStream is an abstract class with default non-abstract
// implementations for forEach() and count() / intoArray() that delegate
// to their upTo variants. Go uses an interface with a BaseDocIdStream
// helper struct that provides those defaults.
type DocIdStream interface {
	// ForEachUpTo iterates over doc IDs below upTo (exclusive), calling
	// consumer for each. This is a consuming (terminal) operation for
	// the range [current, upTo).
	ForEachUpTo(upTo int, consumer IntConsumer) error

	// CountUpTo counts doc IDs below upTo (exclusive) and advances the
	// stream past that range. Terminal for [current, upTo).
	CountUpTo(upTo int) (int, error)

	// IntoArrayUpTo copies doc IDs below upTo (exclusive) into array,
	// returning the number of elements written. Returns 0 when no IDs
	// remain below upTo.
	IntoArrayUpTo(upTo int, array []int) int

	// MayHaveRemaining reports whether any doc IDs remain unconsumed.
	// Must eventually return false when the stream is exhausted.
	MayHaveRemaining() bool
}

// DocIdStreamBase provides default full-range implementations of the
// ForEach / Count / IntoArray convenience wrappers by delegating to
// the upTo variants. Embed in concrete implementations.
type DocIdStreamBase struct{}

// ForEach iterates over all remaining doc IDs, calling consumer for each.
// Mirrors DocIdStream.forEach(CheckedIntConsumer).
func ForEachAll(s DocIdStream, consumer IntConsumer) error {
	return s.ForEachUpTo(NO_MORE_DOCS, consumer)
}

// Count counts all remaining doc IDs. Mirrors DocIdStream.count().
func CountAll(s DocIdStream) (int, error) {
	return s.CountUpTo(NO_MORE_DOCS)
}

// IntoArray copies available doc IDs into array.
// Mirrors DocIdStream.intoArray(int[]).
func IntoArray(s DocIdStream, array []int) int {
	return s.IntoArrayUpTo(NO_MORE_DOCS, array)
}
