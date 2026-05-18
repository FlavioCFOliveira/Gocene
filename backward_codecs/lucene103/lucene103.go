// Package lucene103 implements org.apache.lucene.backward_codecs.lucene103.
package lucene103

// Codec types in this package are read-only stubs that record the format
// metadata so the codec registry can resolve segments written by the
// matching Lucene version.

// ForUtil mirrors org.apache.lucene.backward_codecs.lucene103.ForUtil.
type ForUtil struct { Name, Version string }

// NewForUtil builds a ForUtil with the supplied version.
func NewForUtil(version string) *ForUtil { return &ForUtil{Name: "ForUtil", Version: version} }

// Lucene103Codec mirrors org.apache.lucene.backward_codecs.lucene103.Lucene103Codec.
type Lucene103Codec struct { Name, Version string }

// NewLucene103Codec builds a Lucene103Codec with the supplied version.
func NewLucene103Codec(version string) *Lucene103Codec { return &Lucene103Codec{Name: "Lucene103Codec", Version: version} }

// Lucene103PostingsFormat mirrors org.apache.lucene.backward_codecs.lucene103.Lucene103PostingsFormat.
type Lucene103PostingsFormat struct { Name, Version string }

// NewLucene103PostingsFormat builds a Lucene103PostingsFormat with the supplied version.
func NewLucene103PostingsFormat(version string) *Lucene103PostingsFormat { return &Lucene103PostingsFormat{Name: "Lucene103PostingsFormat", Version: version} }

// ForDeltaUtil mirrors org.apache.lucene.backward_codecs.lucene103.ForDeltaUtil.
type ForDeltaUtil struct { Name, Version string }

// NewForDeltaUtil builds a ForDeltaUtil with the supplied version.
func NewForDeltaUtil(version string) *ForDeltaUtil { return &ForDeltaUtil{Name: "ForDeltaUtil", Version: version} }

// Lucene103PostingsReader mirrors org.apache.lucene.backward_codecs.lucene103.Lucene103PostingsReader.
type Lucene103PostingsReader struct { Name, Version string }

// NewLucene103PostingsReader builds a Lucene103PostingsReader with the supplied version.
func NewLucene103PostingsReader(version string) *Lucene103PostingsReader { return &Lucene103PostingsReader{Name: "Lucene103PostingsReader", Version: version} }

