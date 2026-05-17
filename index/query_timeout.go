// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

// QueryTimeout abstracts a query timeout policy that controls whether a query
// should continue or be stopped. Implementations may be set on the searcher
// (so bulk scoring becomes time-bound) or combined with an ExitableDirectoryReader.
//
// Mirrors org.apache.lucene.index.QueryTimeout from Apache Lucene 10.4.0.
type QueryTimeout interface {
	// ShouldExit reports whether processing should stop.
	// Returns true to terminate the query, false to continue.
	ShouldExit() bool
}
