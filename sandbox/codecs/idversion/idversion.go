// Package idversion implements
// org.apache.lucene.sandbox.codecs.idversion: a postings format optimised for
// ID-with-version updates.
package idversion

// IDVersionPostingsFormat is the codec wrapper.
type IDVersionPostingsFormat struct{}

// NewIDVersionPostingsFormat builds the format.
func NewIDVersionPostingsFormat() *IDVersionPostingsFormat { return &IDVersionPostingsFormat{} }

// VersionBlockTreeTermsReader is the BlockTree variant that records a
// per-term version.
//
// This is a minimal stub; the full port is deferred to a follow-up sprint.
type VersionBlockTreeTermsReader struct {
	// PostingsReader is the per-term postings reader.
	PostingsReader *IDVersionPostingsReader

	// In is the terms file IndexInput.
	// In store.IndexInput — kept as interface{} to avoid import cycle until
	// the full port lands.
	In interface{}

	// Fields maps field name → VersionFieldReader.
	Fields map[string]*VersionFieldReader
}

// VersionBlockTreeTermsWriter is the writer counterpart.
//
// This is a minimal stub; the full port is deferred to a follow-up sprint.
type VersionBlockTreeTermsWriter struct{}

// NewVersionBlockTreeTermsWriter builds the writer.
func NewVersionBlockTreeTermsWriter() *VersionBlockTreeTermsWriter {
	return &VersionBlockTreeTermsWriter{}
}
