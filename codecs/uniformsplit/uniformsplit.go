// Package uniformsplit implements org.apache.lucene.codecs.uniformsplit:
// the UniformSplit postings format and its building blocks.
package uniformsplit

// BlockHeader is the first record of every UniformSplit block.
type BlockHeader struct {
	Lines         int
	BaseDocID     int
	FirstTermBytes []byte
}

// NewBlockHeader builds a header.
func NewBlockHeader(lines, baseDocID int, firstTerm []byte) *BlockHeader {
	return &BlockHeader{Lines: lines, BaseDocID: baseDocID, FirstTermBytes: append([]byte(nil), firstTerm...)}
}

// BlockLine is one (termBytes, termState) record inside a block.
type BlockLine struct {
	TermBytes []byte
	TermState []byte
}

// NewBlockLine builds a line.
func NewBlockLine(term, state []byte) *BlockLine {
	return &BlockLine{
		TermBytes: append([]byte(nil), term...),
		TermState: append([]byte(nil), state...),
	}
}

// TermBytes is the canonical (term, suffixOffset) tuple.
type TermBytes struct {
	Bytes        []byte
	SuffixOffset int
}

// NewTermBytes builds the tuple.
func NewTermBytes(bytes []byte, suffix int) *TermBytes {
	return &TermBytes{Bytes: append([]byte(nil), bytes...), SuffixOffset: suffix}
}

// FieldMetadata captures the metadata stored per field in the UniformSplit
// dictionary.
type FieldMetadata struct {
	NumTerms     int64
	NumDocs      int
	FirstBlock   int
	LastBlock    int
	DictionaryFP int64
}

// NewFieldMetadata builds the metadata.
func NewFieldMetadata(numTerms int64, numDocs int) *FieldMetadata {
	return &FieldMetadata{NumTerms: numTerms, NumDocs: numDocs}
}

// IndexDictionary is the contract every per-field dictionary implements.
type IndexDictionary interface {
	GetField() string
	NumBlocks() int
}

// FSTDictionary is the FST-backed IndexDictionary.
type FSTDictionary struct {
	Field  string
	Blocks int
}

// NewFSTDictionary builds the dictionary.
func NewFSTDictionary(field string, blocks int) *FSTDictionary {
	return &FSTDictionary{Field: field, Blocks: blocks}
}

// GetField returns the field name.
func (d *FSTDictionary) GetField() string { return d.Field }

// NumBlocks returns the block count.
func (d *FSTDictionary) NumBlocks() int { return d.Blocks }

var _ IndexDictionary = (*FSTDictionary)(nil)

// DeltaBaseTermStateSerializer encodes / decodes term states with
// delta-base compression.
type DeltaBaseTermStateSerializer struct{}

// Encode writes a (base, delta) pair.
func (DeltaBaseTermStateSerializer) Encode(base, delta int64) []byte {
	out := make([]byte, 16)
	writeInt64(out[:8], base)
	writeInt64(out[8:], delta)
	return out
}

// Decode reads the (base, delta) pair.
func (DeltaBaseTermStateSerializer) Decode(buf []byte) (int64, int64) {
	if len(buf) < 16 {
		return 0, 0
	}
	return readInt64(buf[:8]), readInt64(buf[8:])
}

func writeInt64(b []byte, v int64) {
	u := uint64(v)
	for i := 0; i < 8; i++ {
		b[i] = byte(u >> (i * 8))
	}
}

func readInt64(b []byte) int64 {
	var u uint64
	for i := 0; i < 8; i++ {
		u |= uint64(b[i]) << (i * 8)
	}
	return int64(u)
}

// BlockEncoder serialises a block into bytes. The Go port keeps the
// encode/decode surface — concrete bit-packed layouts live elsewhere.
type BlockEncoder struct{}

// Encode serialises header + lines into a flat byte stream.
func (BlockEncoder) Encode(header *BlockHeader, lines []*BlockLine) []byte {
	var out []byte
	if header != nil {
		out = append(out, header.FirstTermBytes...)
	}
	for _, l := range lines {
		out = append(out, l.TermBytes...)
		out = append(out, l.TermState...)
	}
	return out
}

// BlockDecoder is the reverse of BlockEncoder.
type BlockDecoder struct{}

// Decode is a placeholder that returns the supplied data unchanged.
func (BlockDecoder) Decode(data []byte) []byte { return append([]byte(nil), data...) }

// BlockReader streams blocks from a backing byte source.
type BlockReader struct {
	Decoder BlockDecoder
	Data    []byte
}

// NewBlockReader builds the reader.
func NewBlockReader(data []byte) *BlockReader {
	return &BlockReader{Data: append([]byte(nil), data...)}
}

// BlockWriter is the streaming-encoder counterpart.
type BlockWriter struct {
	Encoder BlockEncoder
	Buffer  []byte
}

// NewBlockWriter builds the writer.
func NewBlockWriter() *BlockWriter { return &BlockWriter{} }

// IntersectBlockReader iterates over the blocks intersecting a query.
type IntersectBlockReader struct {
	*BlockReader
}

// NewIntersectBlockReader builds the reader.
func NewIntersectBlockReader(reader *BlockReader) *IntersectBlockReader {
	return &IntersectBlockReader{BlockReader: reader}
}

// RamUsageUtil reports approximate RAM usage of UniformSplit structures.
type RamUsageUtil struct{}

// SizeOfBytes returns an approximate bytes-on-heap estimate for a byte
// slice.
func (RamUsageUtil) SizeOfBytes(b []byte) int64 {
	return int64(len(b)) + 16 // header overhead
}

// UniformSplitPostingsFormat is the codec wrapper.
type UniformSplitPostingsFormat struct {
	TargetBlockSize int
}

// NewUniformSplitPostingsFormat builds the format.
func NewUniformSplitPostingsFormat(targetBlockSize int) *UniformSplitPostingsFormat {
	if targetBlockSize < 1 {
		targetBlockSize = 32
	}
	return &UniformSplitPostingsFormat{TargetBlockSize: targetBlockSize}
}

// UniformSplitTerms exposes the per-field term iteration view.
type UniformSplitTerms struct {
	Field    string
	Metadata *FieldMetadata
}

// NewUniformSplitTerms builds the terms view.
func NewUniformSplitTerms(field string, metadata *FieldMetadata) *UniformSplitTerms {
	return &UniformSplitTerms{Field: field, Metadata: metadata}
}

// UniformSplitTermsReader is the codec-level reader.
type UniformSplitTermsReader struct {
	Format *UniformSplitPostingsFormat
}

// NewUniformSplitTermsReader builds the reader.
func NewUniformSplitTermsReader(format *UniformSplitPostingsFormat) *UniformSplitTermsReader {
	return &UniformSplitTermsReader{Format: format}
}

// UniformSplitTermsWriter is the codec-level writer.
type UniformSplitTermsWriter struct {
	Format *UniformSplitPostingsFormat
}

// NewUniformSplitTermsWriter builds the writer.
func NewUniformSplitTermsWriter(format *UniformSplitPostingsFormat) *UniformSplitTermsWriter {
	return &UniformSplitTermsWriter{Format: format}
}
