// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import "github.com/FlavioCFOliveira/Gocene/util"

// DocValuesIterator is the Go port of Lucene's package-private
// org.apache.lucene.index.DocValuesIterator abstract class, which extends
// DocIdSetIterator. Lucene models it as an abstract class with a single
// extra abstract method; Gocene models it as an interface that embeds the
// DocIdSetIterator contract (util.DocIdSetIterator, the import-cycle-free
// copy used throughout the index package) and adds AdvanceExact.
type DocValuesIterator interface {
	util.DocIdSetIterator

	// AdvanceExact advances the iterator to exactly target and reports
	// whether target has a value. target must be greater than or equal to
	// the current DocID and must be a valid doc ID, i.e. >= 0 and < maxDoc.
	// After this method returns, DocID returns target.
	//
	// Note: it is illegal to call IntoBitSet or DocIDRunEnd when this
	// method returns false.
	AdvanceExact(target int) (bool, error)
}
