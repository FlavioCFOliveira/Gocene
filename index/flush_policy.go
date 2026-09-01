// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"github.com/FlavioCFOliveira/Gocene/util"
)

// FlushPolicy controls when segments are flushed from a RAM resident internal
// data-structure to the IndexWriter's Directory.
// Mirrors org.apache.lucene.index.FlushPolicy from Apache Lucene 10.5.0.
type FlushPolicy interface {
	// OnChange is called for each delete, insert or update.
	// For pure deletes, the given DocumentsWriterPerThread may be nil.
	OnChange(control *DocumentsWriterFlushControl, perThread *DocumentsWriterPerThread)

	// Init initializes the FlushPolicy with the writer config.
	Init(indexWriterConfig *LiveIndexWriterConfig)
}

// baseFlushPolicy provides common functionality for FlushPolicy implementations.
type baseFlushPolicy struct {
	indexWriterConfig *LiveIndexWriterConfig
	infoStream        *util.InfoStream
}

func (b *baseFlushPolicy) init(indexWriterConfig *LiveIndexWriterConfig) {
	b.indexWriterConfig = indexWriterConfig
	b.infoStream = indexWriterConfig.GetInfoStream()
}

func (b *baseFlushPolicy) findLargestNonPendingWriter(control *DocumentsWriterFlushControl, perThread *DocumentsWriterPerThread) *DocumentsWriterPerThread {
	// The dwpt which needs to be flushed eventually.
	return control.FindLargestNonPendingWriter()
}

func (b *baseFlushPolicy) assertMessage(s string) bool {
	if b.infoStream != nil && b.infoStream.IsEnabled("FP") {
		b.infoStream.Message("FP", s)
	}
	return true
}
