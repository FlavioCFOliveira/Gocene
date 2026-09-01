// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

// ImpactsSource is a source of Impacts.
// Mirrors org.apache.lucene.index.ImpactsSource from Apache Lucene 10.5.0.
type ImpactsSource interface {
	// AdvanceShallow is a shallow-advance to target. This is cheaper than calling
	// DocIdSetIterator.Advance(int).
	AdvanceShallow(target int) error

	// GetImpacts gets information about upcoming impacts for doc ids that are
	// greater than or equal to the maximum of DocIdSetIterator.DocID() and the
	// last target that was passed to AdvanceShallow(int).
	GetImpacts() (Impacts, error)
}
