// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "github.com/FlavioCFOliveira/Gocene/index"

// ReferenceManager is org.apache.lucene.search.ReferenceManager (Apache Lucene
// 10.5.0): a utility class to safely share instances of a certain type across
// multiple threads, while periodically refreshing them.
//
// Lucene declares the class in this package, but
// org.apache.lucene.index.ReaderManager extends it, and Go forbids the
// resulting index<->search import cycle. The class is therefore declared in
// the index package ([index.ReferenceManager]) and aliased here, so both
// Lucene spellings name one type with one member set.
type ReferenceManager[G comparable] = index.ReferenceManager[G]

// ReferenceManagerOverrides is the Go rendering of the abstract and protected
// overridable members of ReferenceManager; see [index.ReferenceManagerOverrides].
type ReferenceManagerOverrides[G comparable] = index.ReferenceManagerOverrides[G]

// RefreshListener is org.apache.lucene.search.ReferenceManager.RefreshListener;
// see [index.RefreshListener].
type RefreshListener = index.RefreshListener
