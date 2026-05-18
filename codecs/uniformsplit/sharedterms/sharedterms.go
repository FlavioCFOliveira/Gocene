// Package sharedterms implements
// org.apache.lucene.codecs.uniformsplit.sharedterms: the variant that shares
// terms across multiple fields.
package sharedterms

import "github.com/FlavioCFOliveira/Gocene/codecs/uniformsplit"

// FieldMetadataTermState is the per-(field, term) state record for the
// shared-terms variant. Mirrors
// org.apache.lucene.codecs.uniformsplit.sharedterms.FieldMetadataTermState.
type FieldMetadataTermState struct {
	Field     string
	TermState []byte
}

// NewFieldMetadataTermState builds the record.
func NewFieldMetadataTermState(field string, state []byte) *FieldMetadataTermState {
	return &FieldMetadataTermState{Field: field, TermState: append([]byte(nil), state...)}
}

// STBlockLine is the shared-terms equivalent of BlockLine: a single term
// shared across one or more fields together with their term states.
type STBlockLine struct {
	TermBytes []byte
	PerField  []*FieldMetadataTermState
}

// NewSTBlockLine builds the line.
func NewSTBlockLine(term []byte, perField []*FieldMetadataTermState) *STBlockLine {
	return &STBlockLine{TermBytes: append([]byte(nil), term...), PerField: append([]*FieldMetadataTermState(nil), perField...)}
}

// STBlockReader reads shared-terms blocks.
type STBlockReader struct {
	Inner *uniformsplit.BlockReader
}

// NewSTBlockReader builds the reader.
func NewSTBlockReader(inner *uniformsplit.BlockReader) *STBlockReader {
	return &STBlockReader{Inner: inner}
}

// STBlockWriter is the writer counterpart.
type STBlockWriter struct {
	Inner *uniformsplit.BlockWriter
}

// NewSTBlockWriter builds the writer.
func NewSTBlockWriter(inner *uniformsplit.BlockWriter) *STBlockWriter {
	return &STBlockWriter{Inner: inner}
}

// STIntersectBlockReader is the intersect variant.
type STIntersectBlockReader struct {
	*STBlockReader
}

// NewSTIntersectBlockReader builds the reader.
func NewSTIntersectBlockReader(reader *STBlockReader) *STIntersectBlockReader {
	return &STIntersectBlockReader{STBlockReader: reader}
}

// STMergingBlockReader merges multiple shared-terms readers into one.
type STMergingBlockReader struct {
	Readers []*STBlockReader
}

// NewSTMergingBlockReader builds the merging reader.
func NewSTMergingBlockReader(readers []*STBlockReader) *STMergingBlockReader {
	return &STMergingBlockReader{Readers: append([]*STBlockReader(nil), readers...)}
}

// STUniformSplitPostingsFormat is the shared-terms codec wrapper.
type STUniformSplitPostingsFormat struct {
	*uniformsplit.UniformSplitPostingsFormat
}

// NewSTUniformSplitPostingsFormat builds the format.
func NewSTUniformSplitPostingsFormat(targetBlockSize int) *STUniformSplitPostingsFormat {
	return &STUniformSplitPostingsFormat{UniformSplitPostingsFormat: uniformsplit.NewUniformSplitPostingsFormat(targetBlockSize)}
}

// STUniformSplitTerms exposes the per-field term view for the shared variant.
type STUniformSplitTerms struct {
	*uniformsplit.UniformSplitTerms
}

// NewSTUniformSplitTerms builds the wrapper.
func NewSTUniformSplitTerms(field string, metadata *uniformsplit.FieldMetadata) *STUniformSplitTerms {
	return &STUniformSplitTerms{UniformSplitTerms: uniformsplit.NewUniformSplitTerms(field, metadata)}
}

// STUniformSplitTermsReader is the reader.
type STUniformSplitTermsReader struct {
	Format *STUniformSplitPostingsFormat
}

// NewSTUniformSplitTermsReader builds the reader.
func NewSTUniformSplitTermsReader(format *STUniformSplitPostingsFormat) *STUniformSplitTermsReader {
	return &STUniformSplitTermsReader{Format: format}
}

// STUniformSplitTermsWriter is the writer.
type STUniformSplitTermsWriter struct {
	Format *STUniformSplitPostingsFormat
}

// NewSTUniformSplitTermsWriter builds the writer.
func NewSTUniformSplitTermsWriter(format *STUniformSplitPostingsFormat) *STUniformSplitTermsWriter {
	return &STUniformSplitTermsWriter{Format: format}
}

// UnionFieldMetadataBuilder merges multiple FieldMetadata snapshots into a
// union that the shared-terms codec uses to enumerate the global field set.
type UnionFieldMetadataBuilder struct {
	fields map[string]*uniformsplit.FieldMetadata
}

// NewUnionFieldMetadataBuilder builds the helper.
func NewUnionFieldMetadataBuilder() *UnionFieldMetadataBuilder {
	return &UnionFieldMetadataBuilder{fields: make(map[string]*uniformsplit.FieldMetadata)}
}

// Add merges a snapshot for field.
func (b *UnionFieldMetadataBuilder) Add(field string, metadata *uniformsplit.FieldMetadata) {
	if existing, ok := b.fields[field]; ok {
		existing.NumTerms += metadata.NumTerms
		existing.NumDocs += metadata.NumDocs
		return
	}
	clone := *metadata
	b.fields[field] = &clone
}

// Build returns the merged map keyed by field name.
func (b *UnionFieldMetadataBuilder) Build() map[string]*uniformsplit.FieldMetadata {
	out := make(map[string]*uniformsplit.FieldMetadata, len(b.fields))
	for k, v := range b.fields {
		clone := *v
		out[k] = &clone
	}
	return out
}
