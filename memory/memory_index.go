// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package memory

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// MemoryIndex is an implementation of Index that stores the index in memory.
//
// This is the Go port of Lucene's org.apache.lucene.index.memory.MemoryIndex.
type MemoryIndex struct {
	writer *index.IndexWriter
	reader *index.IndexReader
}

func NewMemoryIndex() *MemoryIndex {
	// In a real implementation, this would set up an in-memory directory.
	return &MemoryIndex{}
}

func (mi *MemoryIndex) GetWriter() *index.IndexWriter {
	return mi.writer
}

func (mi *MemoryIndex) GetReader() *index.IndexReader {
	return mi.reader
}

func (mi *MemoryIndex) Close() error {
	if mi.writer != nil {
		mi.writer.Close()
	}
	if mi.reader != nil {
		mi.reader.Close()
	}
	return nil
}
