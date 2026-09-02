// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import "fmt"

// Port of org.apache.lucene.index.FlushByRamOrCountsPolicy from Apache
// Lucene 10.4.0 (commit 9983b7c).
//
// Default FlushPolicy implementation that flushes new segments based on
// RAM used and document count depending on the IndexWriter's configuration.
// It also applies pending deletes based on the number of buffered delete
// terms.
//
// All configured settings are used to mark DocumentsWriterPerThread as
// flush-pending during indexing with respect to their live updates. If the
// configured RAMBufferSizeMB is enabled, the largest RAM-consuming
// DocumentsWriterPerThread will be marked as pending iff the global active
// RAM consumption is >= the configured max RAM buffer.
//
// Gocene port notes
// -----------------
// Java's FlushPolicy is an abstract class carrying two protected fields
// (indexWriterConfig, infoStream) that the package-private init() populates.
// Go has no abstract classes; the Gocene equivalent threads the same
// information through the package-internal flushControlConfig surface,
// which DocumentsWriterFlushControl already consumes. Construction takes
// the same flushControlConfig so the policy and the control read from a
// single source of truth.
//
// The exported index.FlushPolicy interface (in documents_writer.go) is a
// separate, simpler ShouldFlush-shaped contract used by the legacy
// DocumentsWriter scaffolding. This policy implements the OnChange-shaped
// flushControlPolicy contract that DocumentsWriterFlushControl was
// designed against, matching the upstream FlushPolicy.onChange method.

// FlushByRamOrCountsPolicy is the default flushControlPolicy implementation.
// It marks a DWPT as flush-pending when either the buffered-doc count or
// the active RAM consumption crosses the configured threshold, and forces
// pending deletes to be applied when the buffered-delete RAM exceeds the
// same threshold.
//
// Instances are safe for concurrent use: OnChange relies on the caller
// (DocumentsWriterFlushControl) holding the relevant monitor, exactly
// like the Java original.
type FlushByRamOrCountsPolicy struct {
	cfg flushControlConfig
}

// NewFlushByRamOrCountsPolicy constructs a FlushByRamOrCountsPolicy bound
// to the given live config. The config is read on every OnChange call so
// runtime mutations of MaxBufferedDocs and RAMBufferSizeMB take effect
// immediately, mirroring Lucene's LiveIndexWriterConfig semantics.
func NewFlushByRamOrCountsPolicy(cfg flushControlConfig) *FlushByRamOrCountsPolicy {
	return &FlushByRamOrCountsPolicy{cfg: cfg}
}

// Compile-time interface conformance check.
var _ flushControlPolicy = (*FlushByRamOrCountsPolicy)(nil)

// OnChange is called for each delete, insert or update. For pure delete
// events, perThread may be nil.
//
// The caller (DocumentsWriterFlushControl) is responsible for holding its
// own monitor (c.mu) and the perThread lock, matching the Java contract
// where FlushPolicy.onChange runs synchronized on the control. This
// method therefore uses the control's *Locked accessors and must never
// re-acquire c.mu directly.
func (p *FlushByRamOrCountsPolicy) OnChange(
	control *DocumentsWriterFlushControl,
	perThread flushControlDWPT,
) {
	if perThread != nil && p.FlushOnDocCount() &&
		perThread.GetNumDocsInRAMLocked() >= p.cfg.GetMaxBufferedDocs() {
		// Flush this state by num docs.
		control.setFlushPendingLocked(perThread)
		return
	}
	if !p.FlushOnRAM() {
		return
	}
	// Flush by RAM. Pre-compute the byte limit once; the multiplication is
	// trivial but the integer cast is what the Java original does too.
	limit := int64(p.cfg.GetRAMBufferSizeMB() * 1024.0 * 1024.0)
	activeRAM := control.activeBytesLocked()
	deletesRAM := control.GetDeleteBytesUsed()
	switch {
	case deletesRAM >= limit && activeRAM >= limit && perThread != nil:
		p.flushDeletes(control)
		p.flushActiveBytes(control, perThread)
	case deletesRAM >= limit:
		p.flushDeletes(control)
	case activeRAM+deletesRAM >= limit && perThread != nil:
		p.flushActiveBytes(control, perThread)
	}
}

// flushDeletes raises the apply-all-deletes flag and emits an FP trace.
func (p *FlushByRamOrCountsPolicy) flushDeletes(control *DocumentsWriterFlushControl) {
	control.SetApplyAllDeletes()
	if stream := p.cfg.GetInfoStream(); stream != nil && stream.IsEnabled("FP") {
		// Build the message lazily — IsEnabled gates the formatting cost.
		stream.Message("FP", fpDeletesMessage(control.GetDeleteBytesUsed(), p.cfg.GetRAMBufferSizeMB()))
	}
}

// flushActiveBytes traces and delegates to markLargestWriterPending. The
// caller (OnChange) must already hold control.mu — we read activeBytes
// via the unlocked sibling to avoid re-entering the monitor.
func (p *FlushByRamOrCountsPolicy) flushActiveBytes(
	control *DocumentsWriterFlushControl,
	perThread flushControlDWPT,
) {
	if stream := p.cfg.GetInfoStream(); stream != nil && stream.IsEnabled("FP") {
		stream.Message("FP", fpActiveBytesMessage(
			control.activeBytesLocked(), control.GetDeleteBytesUsed(), p.cfg.GetRAMBufferSizeMB(),
		))
	}
	p.markLargestWriterPending(control, perThread)
}

// markLargestWriterPending marks the most RAM-consuming active DWPT
// flush-pending. Exposed as a method so subclasses (or test doubles) can
// override the selection — Java's FlushPolicy uses protected here. The
// caller must already hold control.mu.
func (p *FlushByRamOrCountsPolicy) markLargestWriterPending(
	control *DocumentsWriterFlushControl,
	_ flushControlDWPT,
) {
	if largest := control.findLargestNonPendingWriterLocked(); largest != nil {
		control.setFlushPendingLocked(largest)
	}
}

// FlushOnDocCount reports whether this policy will flush when the buffered
// doc count crosses MaxBufferedDocs.
func (p *FlushByRamOrCountsPolicy) FlushOnDocCount() bool {
	return p.cfg.GetMaxBufferedDocs() != DISABLE_AUTO_FLUSH
}

// FlushOnRAM reports whether this policy will flush when the active RAM
// consumption crosses RAMBufferSizeMB.
func (p *FlushByRamOrCountsPolicy) FlushOnRAM() bool {
	return p.cfg.GetRAMBufferSizeMB() != float64(DISABLE_AUTO_FLUSH)
}

// fpDeletesMessage formats the FP trace emitted by flushDeletes. Lifted
// to a free function so the caller can keep the hot path branch-free.
func fpDeletesMessage(deleteBytesUsed int64, ramBufferMB float64) string {
	return fmt.Sprintf("force apply deletes bytesUsed=%d vs ramBufferMB=%g",
		deleteBytesUsed, ramBufferMB)
}

// fpActiveBytesMessage formats the FP trace emitted by flushActiveBytes.
func fpActiveBytesMessage(activeBytes, deleteBytes int64, ramBufferMB float64) string {
	return fmt.Sprintf("trigger flush: activeBytes=%d deleteBytes=%d vs ramBufferMB=%g",
		activeBytes, deleteBytes, ramBufferMB)
}
