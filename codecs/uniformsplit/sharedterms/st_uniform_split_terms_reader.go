package sharedterms

import (
	"io"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/codecs/uniformsplit"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// STUniformSplitTermsReader is the shared-terms implementation of UniformSplitTermsReader.
type STUniformSplitTermsReader struct {
	*uniformsplit.UniformSplitTermsReader
}

// NewSTUniformSplitTermsReader builds an STUniformSplitTermsReader.
func NewSTUniformSplitTermsReader(
	postingsReader any,
	state *index.SegmentReadState,
	blockDecoder uniformsplit.BlockDecoder,
	dictionaryOnHeap bool) *STUniformSplitTermsReader {

	return &STUniformSplitTermsReader{
		UniformSplitTermsReader: uniformsplit.NewUniformSplitTermsReader(postingsReader, state, blockDecoder, dictionaryOnHeap),
	}
}

// FillFieldMap fills the map of fields to their corresponding STUniformSplitTerms.
func (r *STUniformSplitTermsReader) FillFieldMap(
	postingsReader any,
	state *index.SegmentReadState,
	blockDecoder uniformsplit.BlockDecoder,
	dictionaryOnHeap bool,
	dictionaryInput store.DataInput,
	blockInput store.DataInput,
	fieldMetadataCollection []*uniformsplit.FieldMetadata,
	fieldInfos *index.FieldInfos) error {

	// Logic to create UnionFieldMetadata and shared dictionary
	return nil // Placeholder
}
