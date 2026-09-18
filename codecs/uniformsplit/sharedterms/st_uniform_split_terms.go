package sharedterms

import (
	"io"

	"github.com/FlavioCFOliveira/Gocene/codecs/uniformsplit"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// STUniformSplitTerms is the shared-terms implementation of UniformSplitTerms.
type STUniformSplitTerms struct {
	*uniformsplit.UniformSplitTerms
	unionFieldMetadata *uniformsplit.FieldMetadata
	fieldInfos         *index.FieldInfos
}

// NewSTUniformSplitTerms builds an STUniformSplitTerms.
func NewSTUniformSplitTerms(
	blockInput store.DataInput,
	fieldMetadata *uniformsplit.FieldMetadata,
	unionFieldMetadata *uniformsplit.FieldMetadata,
	postingsReader any,
	blockDecoder uniformsplit.BlockDecoder,
	fieldInfos *index.FieldInfos,
	dictionaryBrowser any) *STUniformSplitTerms {

	return &STUniformSplitTerms{
		UniformSplitTerms:  uniformsplit.NewUniformSplitTerms(blockInput, fieldMetadata, postingsReader, blockDecoder),
		unionFieldMetadata: unionFieldMetadata,
		fieldInfos:         fieldInfos,
	}
}

// Iterator returns a TermsEnum for the field.
func (s *STUniformSplitTerms) Iterator() (*STBlockReader, error) {
	return NewSTBlockReader(
		s.DictionaryBrowser,
		s.Input,
		s.PostingsReader,
		s.FieldMetadata,
		s.Decoder,
		s.fieldInfos), nil
}

// Intersect returns a TermsEnum that intersects with an automaton.
func (s *STUniformSplitTerms) Intersect(compiled *util.CompiledAutomaton, startTerm *util.BytesRef) (*STIntersectBlockReader, error) {
	return NewSTIntersectBlockReader(
		compiled,
		startTerm,
		s.DictionaryBrowser,
		s.Input,
		s.PostingsReader,
		s.FieldMetadata,
		s.Decoder,
		s.fieldInfos), nil
}

// CreateMergingBlockReader creates a reader used for merging.
func (s *STUniformSplitTerms) CreateMergingBlockReader() (*STMergingBlockReader, error) {
	return NewSTMergingBlockReader(
		s.DictionaryBrowser,
		s.Input,
		s.PostingsReader,
		s.unionFieldMetadata,
		s.Decoder,
		s.fieldInfos), nil
}
