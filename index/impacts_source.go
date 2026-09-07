// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

// ImpactsSource produces Impacts and supports shallow-advance to allow callers
// to retrieve more precise impact information for upcoming docs. Mirrors
// org.apache.lucene.index.ImpactsSource from Apache Lucene 10.5.0.
type ImpactsSource interface {
	// AdvanceShallow shallow-advances to target. Cheaper than calling Advance
	// on the underlying iterator and lets subsequent GetImpacts calls ignore
	// doc IDs less than target.
	AdvanceShallow(target int) error

	// GetImpacts returns Impacts for upcoming doc IDs greater than or equal
	// to the maximum of the current docID and the last AdvanceShallow target.
	GetImpacts() (Impacts, error)
}
