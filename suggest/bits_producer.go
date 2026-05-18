// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package suggest

// BitsProducer is the contract used by document-based suggesters to obtain
// the set of live documents for a leaf. Mirrors
// org.apache.lucene.search.suggest.BitsProducer.
type BitsProducer interface {
	// GetBits returns the live-docs bitset for the supplied leaf identifier.
	GetBits(leafID int) ([]uint64, error)
}
