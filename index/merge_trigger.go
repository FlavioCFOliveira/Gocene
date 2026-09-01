// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

// MergeTrigger indicates the event that triggered the merge.
// Mirrors org.apache.lucene.index.MergeTrigger from Apache Lucene 10.5.0.
type MergeTrigger int

const (
	// MergeTriggerSegmentFlush: Merge was triggered by a segment flush.
	MergeTriggerSegmentFlush MergeTrigger = iota
	// MergeTriggerFullFlush: Merge was triggered by a full flush (commit, NRT reopen, or close).
	MergeTriggerFullFlush
	// MergeTriggerExplicit: Merge has been triggered explicitly by the user.
	MergeTriggerExplicit
	// MergeTriggerMergeFinished: Merge was triggered by a successfully finished merge.
	MergeTriggerMergeFinished
	// MergeTriggerClosing: Merge was triggered by a closing IndexWriter.
	MergeTriggerClosing
	// MergeTriggerCommit: Merge was triggered on commit.
	MergeTriggerCommit
	// MergeTriggerGetReader: Merge was triggered on opening NRT readers.
	MergeTriggerGetReader
	// MergeTriggerAddIndexes: Merge was triggered by an IndexWriter.AddIndexes operation.
	MergeTriggerAddIndexes
)
