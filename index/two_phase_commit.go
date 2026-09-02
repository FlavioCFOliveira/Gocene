// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

// TwoPhaseCommit is the contract for resources that participate in a
// two-phase commit (TPC) protocol. Mirrors
// org.apache.lucene.index.TwoPhaseCommit from Apache Lucene 10.4.0.
//
// The protocol is:
//   - PrepareCommit  — durably stage the changes, but don't make them visible.
//   - Commit         — make the prepared changes visible.
//   - Rollback       — discard the prepared changes.
type TwoPhaseCommit interface {
	// PrepareCommit stages the pending changes. Returns the long sequence
	// number Lucene attaches to the prepared commit.
	PrepareCommit() (int64, error)

	// Commit makes the prepared changes visible. Returns the sequence number.
	Commit() (int64, error)

	// Rollback discards the prepared changes.
	Rollback() error
}
