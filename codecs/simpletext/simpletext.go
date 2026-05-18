// Package simpletext implements org.apache.lucene.codecs.simpletext: a
// debugging-friendly text-based codec where every per-segment file is a
// newline-delimited stream of key:value records.
package simpletext

// SimpleTextCodec is the codec wrapper that wires every SimpleText format
// together. Mirrors org.apache.lucene.codecs.simpletext.SimpleTextCodec.
type SimpleTextCodec struct{}

// NewSimpleTextCodec builds the codec.
func NewSimpleTextCodec() *SimpleTextCodec { return &SimpleTextCodec{} }

// Name returns "SimpleText" — the SPI-style codec identifier.
func (SimpleTextCodec) Name() string { return "SimpleText" }

// SimpleTextCompoundFormat groups files into a single "compound" file using
// a text manifest. Mirrors
// org.apache.lucene.codecs.simpletext.SimpleTextCompoundFormat.
type SimpleTextCompoundFormat struct{}

// NewSimpleTextCompoundFormat builds the format.
func NewSimpleTextCompoundFormat() *SimpleTextCompoundFormat {
	return &SimpleTextCompoundFormat{}
}

// SimpleTextFieldInfosFormat writes the per-field FieldInfos as a text file.
// Mirrors org.apache.lucene.codecs.simpletext.SimpleTextFieldInfosFormat.
type SimpleTextFieldInfosFormat struct{}

// NewSimpleTextFieldInfosFormat builds the format.
func NewSimpleTextFieldInfosFormat() *SimpleTextFieldInfosFormat {
	return &SimpleTextFieldInfosFormat{}
}

// SimpleTextKnnVectorsFormat is the KNN vectors variant.
type SimpleTextKnnVectorsFormat struct{}

// NewSimpleTextKnnVectorsFormat builds the format.
func NewSimpleTextKnnVectorsFormat() *SimpleTextKnnVectorsFormat {
	return &SimpleTextKnnVectorsFormat{}
}

// SimpleTextKnnVectorsReader reads SimpleText KNN vectors.
type SimpleTextKnnVectorsReader struct{}

// NewSimpleTextKnnVectorsReader builds the reader.
func NewSimpleTextKnnVectorsReader() *SimpleTextKnnVectorsReader {
	return &SimpleTextKnnVectorsReader{}
}

// SimpleTextKnnVectorsWriter writes SimpleText KNN vectors.
type SimpleTextKnnVectorsWriter struct{}

// NewSimpleTextKnnVectorsWriter builds the writer.
func NewSimpleTextKnnVectorsWriter() *SimpleTextKnnVectorsWriter {
	return &SimpleTextKnnVectorsWriter{}
}

// SimpleTextLiveDocsFormat writes per-segment live-doc bitmaps as text.
type SimpleTextLiveDocsFormat struct{}

// NewSimpleTextLiveDocsFormat builds the format.
func NewSimpleTextLiveDocsFormat() *SimpleTextLiveDocsFormat {
	return &SimpleTextLiveDocsFormat{}
}

// SimpleTextNormsFormat writes norms as text.
type SimpleTextNormsFormat struct{}

// NewSimpleTextNormsFormat builds the format.
func NewSimpleTextNormsFormat() *SimpleTextNormsFormat { return &SimpleTextNormsFormat{} }

// SimpleTextPointsFormat writes point values as text.
type SimpleTextPointsFormat struct{}

// NewSimpleTextPointsFormat builds the format.
func NewSimpleTextPointsFormat() *SimpleTextPointsFormat { return &SimpleTextPointsFormat{} }

// SimpleTextSegmentInfoFormat writes segment metadata as text.
type SimpleTextSegmentInfoFormat struct{}

// NewSimpleTextSegmentInfoFormat builds the format.
func NewSimpleTextSegmentInfoFormat() *SimpleTextSegmentInfoFormat {
	return &SimpleTextSegmentInfoFormat{}
}

// SimpleTextStoredFieldsFormat is the stored-fields text format.
type SimpleTextStoredFieldsFormat struct{}

// NewSimpleTextStoredFieldsFormat builds the format.
func NewSimpleTextStoredFieldsFormat() *SimpleTextStoredFieldsFormat {
	return &SimpleTextStoredFieldsFormat{}
}

// SimpleTextStoredFieldsReader reads stored fields from text.
type SimpleTextStoredFieldsReader struct{}

// NewSimpleTextStoredFieldsReader builds the reader.
func NewSimpleTextStoredFieldsReader() *SimpleTextStoredFieldsReader {
	return &SimpleTextStoredFieldsReader{}
}

// SimpleTextStoredFieldsWriter writes stored fields as text.
type SimpleTextStoredFieldsWriter struct{}

// NewSimpleTextStoredFieldsWriter builds the writer.
func NewSimpleTextStoredFieldsWriter() *SimpleTextStoredFieldsWriter {
	return &SimpleTextStoredFieldsWriter{}
}

// SimpleTextTermVectorsFormat is the term-vectors text format.
type SimpleTextTermVectorsFormat struct{}

// NewSimpleTextTermVectorsFormat builds the format.
func NewSimpleTextTermVectorsFormat() *SimpleTextTermVectorsFormat {
	return &SimpleTextTermVectorsFormat{}
}

// SimpleTextTermVectorsReader reads term vectors from text.
type SimpleTextTermVectorsReader struct{}

// NewSimpleTextTermVectorsReader builds the reader.
func NewSimpleTextTermVectorsReader() *SimpleTextTermVectorsReader {
	return &SimpleTextTermVectorsReader{}
}

// SimpleTextTermVectorsWriter writes term vectors as text.
type SimpleTextTermVectorsWriter struct{}

// NewSimpleTextTermVectorsWriter builds the writer.
func NewSimpleTextTermVectorsWriter() *SimpleTextTermVectorsWriter {
	return &SimpleTextTermVectorsWriter{}
}
