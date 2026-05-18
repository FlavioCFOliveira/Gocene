// Package backward_codecs implements org.apache.lucene.backward_codecs: the
// reader-only port of every previous Lucene codec version. The bundled types
// are intentionally minimal — each one is a typed placeholder that records
// the codec name and version so the codec registry can resolve old segments
// for read-only access.
package backward_codecs

// Placeholder is the root tag exposed by every backward-compat codec. It
// records the format name + version string the segment-info on disk carries.
type Placeholder struct {
	Name    string
	Version string
}

// NewPlaceholder builds a Placeholder.
func NewPlaceholder(name, version string) *Placeholder {
	return &Placeholder{Name: name, Version: version}
}
