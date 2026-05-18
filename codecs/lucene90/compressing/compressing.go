// Package compressing hosts the Sprint 48 ports for
// org.apache.lucene.codecs.lucene90.compressing.
package compressing

// The Sprint 48 lucene90-compressing port surfaces these types as typed
// stubs so dependent packages keep compiling; concrete behaviour ports
// (chunk-based LZ4/Deflate compression layout with chunk-index slicing)
// land progressively in follow-up deep-port sprints.

// FieldsIndexWriter mirrors
// org.apache.lucene.codecs.lucene90.compressing.FieldsIndexWriter.
type FieldsIndexWriter struct{}

// NewFieldsIndexWriter builds a FieldsIndexWriter.
func NewFieldsIndexWriter() *FieldsIndexWriter { return &FieldsIndexWriter{} }

// Lucene90CompressingStoredFieldsFormat mirrors
// org.apache.lucene.codecs.lucene90.compressing.Lucene90CompressingStoredFieldsFormat.
type Lucene90CompressingStoredFieldsFormat struct{}

// NewLucene90CompressingStoredFieldsFormat builds a
// Lucene90CompressingStoredFieldsFormat.
func NewLucene90CompressingStoredFieldsFormat() *Lucene90CompressingStoredFieldsFormat {
	return &Lucene90CompressingStoredFieldsFormat{}
}

// Lucene90CompressingStoredFieldsReader mirrors
// org.apache.lucene.codecs.lucene90.compressing.Lucene90CompressingStoredFieldsReader.
type Lucene90CompressingStoredFieldsReader struct{}

// NewLucene90CompressingStoredFieldsReader builds a
// Lucene90CompressingStoredFieldsReader.
func NewLucene90CompressingStoredFieldsReader() *Lucene90CompressingStoredFieldsReader {
	return &Lucene90CompressingStoredFieldsReader{}
}

// Lucene90CompressingStoredFieldsWriter mirrors
// org.apache.lucene.codecs.lucene90.compressing.Lucene90CompressingStoredFieldsWriter.
type Lucene90CompressingStoredFieldsWriter struct{}

// NewLucene90CompressingStoredFieldsWriter builds a
// Lucene90CompressingStoredFieldsWriter.
func NewLucene90CompressingStoredFieldsWriter() *Lucene90CompressingStoredFieldsWriter {
	return &Lucene90CompressingStoredFieldsWriter{}
}

// Lucene90CompressingTermVectorsFormat mirrors
// org.apache.lucene.codecs.lucene90.compressing.Lucene90CompressingTermVectorsFormat.
type Lucene90CompressingTermVectorsFormat struct{}

// NewLucene90CompressingTermVectorsFormat builds a
// Lucene90CompressingTermVectorsFormat.
func NewLucene90CompressingTermVectorsFormat() *Lucene90CompressingTermVectorsFormat {
	return &Lucene90CompressingTermVectorsFormat{}
}

// Lucene90CompressingTermVectorsReader mirrors
// org.apache.lucene.codecs.lucene90.compressing.Lucene90CompressingTermVectorsReader.
type Lucene90CompressingTermVectorsReader struct{}

// NewLucene90CompressingTermVectorsReader builds a
// Lucene90CompressingTermVectorsReader.
func NewLucene90CompressingTermVectorsReader() *Lucene90CompressingTermVectorsReader {
	return &Lucene90CompressingTermVectorsReader{}
}

// Lucene90CompressingTermVectorsWriter mirrors
// org.apache.lucene.codecs.lucene90.compressing.Lucene90CompressingTermVectorsWriter.
type Lucene90CompressingTermVectorsWriter struct{}

// NewLucene90CompressingTermVectorsWriter builds a
// Lucene90CompressingTermVectorsWriter.
func NewLucene90CompressingTermVectorsWriter() *Lucene90CompressingTermVectorsWriter {
	return &Lucene90CompressingTermVectorsWriter{}
}
