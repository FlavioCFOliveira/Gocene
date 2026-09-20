package index

import (
	"github.com/FlavioCFOliveira/Gocene/util"
)

// FlushPolicy controls when segments are flushed from a RAM resident internal
// data-structure to the IndexWriter's Directory.
//
// This is the Go port of Lucene's org.apache.lucene.index.FlushPolicy.
type FlushPolicy interface {
	// OnChange is called for each delete, insert or update. For pure deletes, the given
	// DocumentsWriterPerThread may be nil.
	//
	// Note: This method is called synchronized on the given DocumentsWriterFlushControl
	// and it is guaranteed that the calling thread holds the lock on the given
	// DocumentsWriterPerThread.
	OnChange(control *DocumentsWriterFlushControl, perThread *DocumentsWriterPerThread)
}

// BaseFlushPolicy provides the common functionality for flush policies.
type BaseFlushPolicy struct {
	indexWriterConfig *LiveIndexWriterConfig
	infoStream        util.InfoStream
}

// Init initializes the FlushPolicy with the given index writer configuration.
func (p *BaseFlushPolicy) Init(indexWriterConfig *LiveIndexWriterConfig) {
	p.indexWriterConfig = indexWriterConfig
	p.infoStream = indexWriterConfig.GetInfoStream()
}

// FindLargestNonPendingWriter returns the current most RAM consuming non-pending
// DocumentsWriterPerThread with at least one indexed document.
func (p *BaseFlushPolicy) FindLargestNonPendingWriter(
	control *DocumentsWriterFlushControl, perThread *DocumentsWriterPerThread) *DocumentsWriterPerThread {
	// the dwpt which needs to be flushed eventually
	return control.FindLargestNonPendingWriter()
}

func (p *BaseFlushPolicy) assertMessage(s string) bool {
	if p.infoStream != nil && p.infoStream.IsEnabled("FP") {
		p.infoStream.Message("FP", s)
	}
	return true
}
