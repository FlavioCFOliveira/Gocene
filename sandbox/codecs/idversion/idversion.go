// Package idversion implements
// org.apache.lucene.sandbox.codecs.idversion: a postings format optimised for
// ID-with-version updates.
package idversion

// IDVersionPostingsFormat is the codec wrapper.
type IDVersionPostingsFormat struct{}

// NewIDVersionPostingsFormat builds the format.
func NewIDVersionPostingsFormat() *IDVersionPostingsFormat { return &IDVersionPostingsFormat{} }

// IDVersionSegmentTermsEnum is the per-segment terms enumerator.
type IDVersionSegmentTermsEnum struct {
	Field string
}

// NewIDVersionSegmentTermsEnum builds the enumerator.
func NewIDVersionSegmentTermsEnum(field string) *IDVersionSegmentTermsEnum {
	return &IDVersionSegmentTermsEnum{Field: field}
}

// VersionBlockTreeTermsReader is the BlockTree variant that records a
// per-term version.
type VersionBlockTreeTermsReader struct{}

// NewVersionBlockTreeTermsReader builds the reader.
func NewVersionBlockTreeTermsReader() *VersionBlockTreeTermsReader {
	return &VersionBlockTreeTermsReader{}
}

// VersionBlockTreeTermsWriter is the writer counterpart.
type VersionBlockTreeTermsWriter struct{}

// NewVersionBlockTreeTermsWriter builds the writer.
func NewVersionBlockTreeTermsWriter() *VersionBlockTreeTermsWriter {
	return &VersionBlockTreeTermsWriter{}
}
