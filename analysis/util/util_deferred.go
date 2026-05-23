// Package util hosts the deferred Sprint 28 ports for
// org.apache.lucene.analysis.util.
package util

// The Sprint 28 analysis-common deferral surfaces these types as typed
// stubs so dependent packages keep compiling; concrete behaviour ports
// land progressively.

// CharArrayIterator mirrors org.apache.lucene.analysis.util.CharArrayIterator.
type CharArrayIterator struct{}

// NewCharArrayIterator builds a CharArrayIterator.
func NewCharArrayIterator() *CharArrayIterator { return &CharArrayIterator{} }

// FilesystemResourceLoader mirrors org.apache.lucene.analysis.util.FilesystemResourceLoader.
type FilesystemResourceLoader struct{}

// NewFilesystemResourceLoader builds a FilesystemResourceLoader.
func NewFilesystemResourceLoader() *FilesystemResourceLoader { return &FilesystemResourceLoader{} }





