package blocktree

// Lucene40BlockTreeTermsReader mirrors org.apache.lucene.backward_codecs.lucene40.blocktree.Lucene40BlockTreeTermsReader.
type Lucene40BlockTreeTermsReader struct{ Name, Version string }

// NewLucene40BlockTreeTermsReader builds a Lucene40BlockTreeTermsReader with the supplied version.
func NewLucene40BlockTreeTermsReader(version string) *Lucene40BlockTreeTermsReader {
	return &Lucene40BlockTreeTermsReader{Name: "Lucene40BlockTreeTermsReader", Version: version}
}
