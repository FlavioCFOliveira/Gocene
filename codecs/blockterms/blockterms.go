// Package blockterms implements org.apache.lucene.codecs.blockterms.
package blockterms

// TermsIndexReaderBase is the abstract base for every block-terms index
// reader. Mirrors
// org.apache.lucene.codecs.blockterms.TermsIndexReaderBase.
type TermsIndexReaderBase struct {
	Path string
}

// NewTermsIndexReaderBase builds the base.
func NewTermsIndexReaderBase(path string) *TermsIndexReaderBase {
	return &TermsIndexReaderBase{Path: path}
}

// TermsIndexWriterBase is the abstract base for every block-terms index
// writer. Mirrors
// org.apache.lucene.codecs.blockterms.TermsIndexWriterBase.
type TermsIndexWriterBase struct {
	Path string
}

// NewTermsIndexWriterBase builds the base.
func NewTermsIndexWriterBase(path string) *TermsIndexWriterBase {
	return &TermsIndexWriterBase{Path: path}
}

// BlockTermsReader reads block-terms data. Mirrors
// org.apache.lucene.codecs.blockterms.BlockTermsReader.
type BlockTermsReader struct {
	Base *TermsIndexReaderBase
}

// NewBlockTermsReader builds the reader.
func NewBlockTermsReader(base *TermsIndexReaderBase) *BlockTermsReader {
	return &BlockTermsReader{Base: base}
}

// BlockTermsWriter writes block-terms data. Mirrors
// org.apache.lucene.codecs.blockterms.BlockTermsWriter.
type BlockTermsWriter struct {
	Base *TermsIndexWriterBase
}

// NewBlockTermsWriter builds the writer.
func NewBlockTermsWriter(base *TermsIndexWriterBase) *BlockTermsWriter {
	return &BlockTermsWriter{Base: base}
}

// FixedGapTermsIndexReader is the fixed-gap variant of the terms index.
// Mirrors org.apache.lucene.codecs.blockterms.FixedGapTermsIndexReader.
type FixedGapTermsIndexReader struct {
	Base *TermsIndexReaderBase
	Gap  int
}

// NewFixedGapTermsIndexReader builds the reader.
func NewFixedGapTermsIndexReader(base *TermsIndexReaderBase, gap int) *FixedGapTermsIndexReader {
	if gap < 1 {
		gap = 32
	}
	return &FixedGapTermsIndexReader{Base: base, Gap: gap}
}

// FixedGapTermsIndexWriter is the fixed-gap variant of the terms index writer.
// Mirrors org.apache.lucene.codecs.blockterms.FixedGapTermsIndexWriter.
type FixedGapTermsIndexWriter struct {
	Base *TermsIndexWriterBase
	Gap  int
}

// NewFixedGapTermsIndexWriter builds the writer.
func NewFixedGapTermsIndexWriter(base *TermsIndexWriterBase, gap int) *FixedGapTermsIndexWriter {
	if gap < 1 {
		gap = 32
	}
	return &FixedGapTermsIndexWriter{Base: base, Gap: gap}
}

// VariableGapTermsIndexReader is the variable-gap variant. Mirrors
// org.apache.lucene.codecs.blockterms.VariableGapTermsIndexReader.
type VariableGapTermsIndexReader struct {
	Base *TermsIndexReaderBase
}

// NewVariableGapTermsIndexReader builds the reader.
func NewVariableGapTermsIndexReader(base *TermsIndexReaderBase) *VariableGapTermsIndexReader {
	return &VariableGapTermsIndexReader{Base: base}
}

// VariableGapTermsIndexWriter is the variable-gap variant writer. Mirrors
// org.apache.lucene.codecs.blockterms.VariableGapTermsIndexWriter.
type VariableGapTermsIndexWriter struct {
	Base *TermsIndexWriterBase
}

// NewVariableGapTermsIndexWriter builds the writer.
func NewVariableGapTermsIndexWriter(base *TermsIndexWriterBase) *VariableGapTermsIndexWriter {
	return &VariableGapTermsIndexWriter{Base: base}
}
