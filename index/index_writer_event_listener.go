// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

// IndexWriterEventListener observes high-level IndexWriter lifecycle events.
// Mirrors org.apache.lucene.index.IndexWriterEventListener from Apache
// Lucene 10.4.0.
//
// Lucene defines four hooks: BeginMergeOnFullFlush, EndMergeOnFullFlush,
// BeginMergeOnCommit, EndMergeOnCommit. Gocene exposes them as methods on
// the interface; the no-op default is achieved by embedding the
// IndexWriterEventListenerNoop value.
type IndexWriterEventListener interface {
	// BeginMergeOnFullFlush is invoked at the start of a full-flush merge.
	BeginMergeOnFullFlush(spec *MergeSpecification)

	// EndMergeOnFullFlush is invoked when a full-flush merge finishes.
	EndMergeOnFullFlush(spec *MergeSpecification)

	// BeginMergeOnCommit is invoked at the start of a commit-driven merge.
	BeginMergeOnCommit(spec *MergeSpecification)

	// EndMergeOnCommit is invoked when a commit-driven merge finishes.
	EndMergeOnCommit(spec *MergeSpecification)
}

// IndexWriterEventListenerNoop is a zero-value listener that implements every
// method as a no-op. Embed by value to obtain default behaviour.
type IndexWriterEventListenerNoop struct{}

// BeginMergeOnFullFlush is a no-op.
func (IndexWriterEventListenerNoop) BeginMergeOnFullFlush(_ *MergeSpecification) {}

// EndMergeOnFullFlush is a no-op.
func (IndexWriterEventListenerNoop) EndMergeOnFullFlush(_ *MergeSpecification) {}

// BeginMergeOnCommit is a no-op.
func (IndexWriterEventListenerNoop) BeginMergeOnCommit(_ *MergeSpecification) {}

// EndMergeOnCommit is a no-op.
func (IndexWriterEventListenerNoop) EndMergeOnCommit(_ *MergeSpecification) {}

// IndexWriterEventListenerNoopInstance is the canonical zero-value listener.
var IndexWriterEventListenerNoopInstance IndexWriterEventListener = IndexWriterEventListenerNoop{}
