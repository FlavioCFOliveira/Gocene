// Package memory implements org.apache.lucene.codecs.memory: postings
// formats that keep the entire term dictionary in memory.
package memory

// DirectPostingsFormat is the codec that holds postings in flat in-memory
// arrays. Mirrors org.apache.lucene.codecs.memory.DirectPostingsFormat.
type DirectPostingsFormat struct {
	MinSkipCount  int
	LowFreqCutoff int
}

// NewDirectPostingsFormat builds the format.
func NewDirectPostingsFormat(minSkip, lowFreq int) *DirectPostingsFormat {
	if minSkip < 1 {
		minSkip = 8
	}
	if lowFreq < 1 {
		lowFreq = 32
	}
	return &DirectPostingsFormat{MinSkipCount: minSkip, LowFreqCutoff: lowFreq}
}

// FSTPostingsFormat is the FST-backed postings format. Mirrors
// org.apache.lucene.codecs.memory.FSTPostingsFormat.
type FSTPostingsFormat struct{}

// NewFSTPostingsFormat builds the format.
func NewFSTPostingsFormat() *FSTPostingsFormat { return &FSTPostingsFormat{} }

// FSTTermsReader reads FST-backed term dictionaries. Mirrors
// org.apache.lucene.codecs.memory.FSTTermsReader.
type FSTTermsReader struct {
	Format *FSTPostingsFormat
}

// NewFSTTermsReader builds the reader.
func NewFSTTermsReader(format *FSTPostingsFormat) *FSTTermsReader {
	return &FSTTermsReader{Format: format}
}

// FSTTermsWriter writes FST-backed term dictionaries. Mirrors
// org.apache.lucene.codecs.memory.FSTTermsWriter.
type FSTTermsWriter struct {
	Format *FSTPostingsFormat
}

// NewFSTTermsWriter builds the writer.
func NewFSTTermsWriter(format *FSTPostingsFormat) *FSTTermsWriter {
	return &FSTTermsWriter{Format: format}
}
