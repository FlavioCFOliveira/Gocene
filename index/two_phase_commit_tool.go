// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import "fmt"

// TwoPhaseCommitToolExecute coordinates a fan-out PrepareCommit + Commit
// across several TwoPhaseCommit participants. Mirrors
// org.apache.lucene.index.TwoPhaseCommitTool.execute from Apache Lucene 10.4.0.
//
// Semantics:
//  1. PrepareCommit is invoked on every participant. The first error
//     short-circuits and triggers a rollback on every already-prepared
//     participant.
//  2. If every PrepareCommit succeeds, Commit is invoked on each. A Commit
//     failure on participant N triggers Rollback on every participant >= N
//     to preserve TPC semantics.
func TwoPhaseCommitToolExecute(objects ...TwoPhaseCommit) error {
	if len(objects) == 0 {
		return nil
	}
	// PHASE 1 — prepare
	for i, o := range objects {
		if _, err := o.PrepareCommit(); err != nil {
			// Rollback all participants prepared so far.
			for j := 0; j <= i; j++ {
				_ = objects[j].Rollback()
			}
			return fmt.Errorf("PrepareCommit failed on participant %d: %w", i, err)
		}
	}
	// PHASE 2 — commit
	for i, o := range objects {
		if _, err := o.Commit(); err != nil {
			// Roll back this and all subsequent participants.
			for j := i; j < len(objects); j++ {
				_ = objects[j].Rollback()
			}
			return fmt.Errorf("Commit failed on participant %d: %w", i, err)
		}
	}
	return nil
}
