// Package blocktreeords implements org.apache.lucene.codecs.blocktreeords:
// the BlockTree postings format with extra ordinal lookup support.
package blocktreeords

// BlockTreeOrdsPostingsFormat is the postings format that records term
// ordinals alongside the standard BlockTree data. Mirrors
// org.apache.lucene.codecs.blocktreeords.BlockTreeOrdsPostingsFormat.
type BlockTreeOrdsPostingsFormat struct {
	MinTermsInBlock int
	MaxTermsInBlock int
}

// NewBlockTreeOrdsPostingsFormat builds the format with Lucene defaults
// (min=25, max=48).
func NewBlockTreeOrdsPostingsFormat(min, max int) *BlockTreeOrdsPostingsFormat {
	if min < 2 {
		min = 25
	}
	if max < min {
		max = min * 2
	}
	return &BlockTreeOrdsPostingsFormat{MinTermsInBlock: min, MaxTermsInBlock: max}
}

// OrdsBlockTreeTermsReader is the reader. Mirrors
// org.apache.lucene.codecs.blocktreeords.OrdsBlockTreeTermsReader.
type OrdsBlockTreeTermsReader struct {
	Format *BlockTreeOrdsPostingsFormat
}

// NewOrdsBlockTreeTermsReader builds the reader.
func NewOrdsBlockTreeTermsReader(format *BlockTreeOrdsPostingsFormat) *OrdsBlockTreeTermsReader {
	return &OrdsBlockTreeTermsReader{Format: format}
}

// OrdsBlockTreeTermsWriter is the writer. Mirrors
// org.apache.lucene.codecs.blocktreeords.OrdsBlockTreeTermsWriter.
type OrdsBlockTreeTermsWriter struct {
	Format *BlockTreeOrdsPostingsFormat
}

// NewOrdsBlockTreeTermsWriter builds the writer.
func NewOrdsBlockTreeTermsWriter(format *BlockTreeOrdsPostingsFormat) *OrdsBlockTreeTermsWriter {
	return &OrdsBlockTreeTermsWriter{Format: format}
}

// OrdsSegmentTermsEnum is the segment-local terms enumerator. Mirrors
// org.apache.lucene.codecs.blocktreeords.OrdsSegmentTermsEnum.
type OrdsSegmentTermsEnum struct {
	Field string
}

// NewOrdsSegmentTermsEnum builds the enumerator.
func NewOrdsSegmentTermsEnum(field string) *OrdsSegmentTermsEnum {
	return &OrdsSegmentTermsEnum{Field: field}
}
