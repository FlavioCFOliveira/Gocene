// Package lucene50 implements org.apache.lucene.backward_codecs.lucene50.
package lucene50

// Codec types in this package are read-only stubs that record the format
// metadata so the codec registry can resolve segments written by the
// matching Lucene version.

// Lucene50CompoundFormat mirrors org.apache.lucene.backward_codecs.lucene50.Lucene50CompoundFormat.
type Lucene50CompoundFormat struct { Name, Version string }

// NewLucene50CompoundFormat builds a Lucene50CompoundFormat with the supplied version.
func NewLucene50CompoundFormat(version string) *Lucene50CompoundFormat { return &Lucene50CompoundFormat{Name: "Lucene50CompoundFormat", Version: version} }

// Lucene50LiveDocsFormat mirrors org.apache.lucene.backward_codecs.lucene50.Lucene50LiveDocsFormat.
type Lucene50LiveDocsFormat struct { Name, Version string }

// NewLucene50LiveDocsFormat builds a Lucene50LiveDocsFormat with the supplied version.
func NewLucene50LiveDocsFormat(version string) *Lucene50LiveDocsFormat { return &Lucene50LiveDocsFormat{Name: "Lucene50LiveDocsFormat", Version: version} }

// Lucene50StoredFieldsFormat mirrors org.apache.lucene.backward_codecs.lucene50.Lucene50StoredFieldsFormat.
type Lucene50StoredFieldsFormat struct { Name, Version string }

// NewLucene50StoredFieldsFormat builds a Lucene50StoredFieldsFormat with the supplied version.
func NewLucene50StoredFieldsFormat(version string) *Lucene50StoredFieldsFormat { return &Lucene50StoredFieldsFormat{Name: "Lucene50StoredFieldsFormat", Version: version} }

// Lucene50PostingsFormat mirrors org.apache.lucene.backward_codecs.lucene50.Lucene50PostingsFormat.
type Lucene50PostingsFormat struct { Name, Version string }

// NewLucene50PostingsFormat builds a Lucene50PostingsFormat with the supplied version.
func NewLucene50PostingsFormat(version string) *Lucene50PostingsFormat { return &Lucene50PostingsFormat{Name: "Lucene50PostingsFormat", Version: version} }

// Lucene50TermVectorsFormat mirrors org.apache.lucene.backward_codecs.lucene50.Lucene50TermVectorsFormat.
type Lucene50TermVectorsFormat struct { Name, Version string }

// NewLucene50TermVectorsFormat builds a Lucene50TermVectorsFormat with the supplied version.
func NewLucene50TermVectorsFormat(version string) *Lucene50TermVectorsFormat { return &Lucene50TermVectorsFormat{Name: "Lucene50TermVectorsFormat", Version: version} }

// Lucene50PostingsReader mirrors org.apache.lucene.backward_codecs.lucene50.Lucene50PostingsReader.
type Lucene50PostingsReader struct { Name, Version string }

// NewLucene50PostingsReader builds a Lucene50PostingsReader with the supplied version.
func NewLucene50PostingsReader(version string) *Lucene50PostingsReader { return &Lucene50PostingsReader{Name: "Lucene50PostingsReader", Version: version} }

