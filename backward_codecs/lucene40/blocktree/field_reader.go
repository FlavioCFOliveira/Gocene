package blocktree

// FieldReader mirrors org.apache.lucene.backward_codecs.lucene40.blocktree.FieldReader.
type FieldReader struct{ Name, Version string }

// NewFieldReader builds a FieldReader with the supplied version.
func NewFieldReader(version string) *FieldReader {
	return &FieldReader{Name: "FieldReader", Version: version}
}
