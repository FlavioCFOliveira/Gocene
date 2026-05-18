// Package lucene94 hosts the Sprint 49 port for
// org.apache.lucene.codecs.lucene94.
package lucene94

// The Sprint 49 lucene94-codec port surfaces this type as a typed stub
// so dependent packages keep compiling; concrete behaviour ports (the
// .fnm field-info chunk format with codec header/footer) land in a
// follow-up deep-port sprint.

// Lucene94FieldInfosFormat mirrors
// org.apache.lucene.codecs.lucene94.Lucene94FieldInfosFormat.
type Lucene94FieldInfosFormat struct{}

// NewLucene94FieldInfosFormat builds a Lucene94FieldInfosFormat.
func NewLucene94FieldInfosFormat() *Lucene94FieldInfosFormat { return &Lucene94FieldInfosFormat{} }
