// Package backward_codecs implements org.apache.lucene.backward_codecs: the
// reader-only port of every previous Lucene codec version. The bundled types
// are intentionally minimal — each one is a typed placeholder that records
// the codec name and version so the codec registry can resolve old segments
// for read-only access.
//
// Importing this package as a side effect (import _ "...") triggers the
// init() functions in every sub-package that register their backward-
// compatibility formats into the global codecs.PostingsFormatByName,
// codecs.DocValuesFormatByName, and codecs.KnnVectorsFormatByName
// registries. This mirrors Lucene's META-INF/services ServiceLoader
// registration mechanism.
package backward_codecs

import (
	// Blank imports trigger the init() registration functions in each
	// backward-compatibility sub-package.
	_ "github.com/FlavioCFOliveira/Gocene/backward_codecs/lucene101"
	_ "github.com/FlavioCFOliveira/Gocene/backward_codecs/lucene102"
	_ "github.com/FlavioCFOliveira/Gocene/backward_codecs/lucene50"
	_ "github.com/FlavioCFOliveira/Gocene/backward_codecs/lucene80"
	_ "github.com/FlavioCFOliveira/Gocene/backward_codecs/lucene84"
	_ "github.com/FlavioCFOliveira/Gocene/backward_codecs/lucene90"
	_ "github.com/FlavioCFOliveira/Gocene/backward_codecs/lucene91"
	_ "github.com/FlavioCFOliveira/Gocene/backward_codecs/lucene912"
	_ "github.com/FlavioCFOliveira/Gocene/backward_codecs/lucene92"
	_ "github.com/FlavioCFOliveira/Gocene/backward_codecs/lucene94"
	_ "github.com/FlavioCFOliveira/Gocene/backward_codecs/lucene95"
	_ "github.com/FlavioCFOliveira/Gocene/backward_codecs/lucene99"
)

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
