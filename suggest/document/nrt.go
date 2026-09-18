package document

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/store"
	util "github.com/FlavioCFOliveira/Gocene/util"
	utilfst "github.com/FlavioCFOliveira/Gocene/util/fst"
)

// NRTSuggester executes Top N search on a weighted FST. Mirrors
// org.apache.lucene.search.suggest.document.NRTSuggester.
type NRTSuggester struct {
	fst                       *utilfst.FST[*utilfst.Pair[int64, *util.BytesRef]]
	maxAnalyzedPathsPerOutput int
	payloadSep                int
}

// NewNRTSuggester builds an empty NRTSuggester.
func NewNRTSuggester() *NRTSuggester { return &NRTSuggester{} }

// seekingRandomAccessInput renders the anonymous RandomAccessInput that
// org.apache.lucene.store.IndexInput.randomAccessSlice(long, long) falls back
// to when the sliced IndexInput does not already implement RandomAccessInput
// (IndexInput.java, randomAccessSlice): every positioned read seeks the
// wrapped input and then performs the sequential read.
//
// Lucene wraps a slice, so its positions start at zero. Gocene's
// util/fst.OffHeapFSTStore keeps an explicit base offset instead of a virtual
// slice (see its type documentation), so this wrapper is handed the whole
// input and its positions stay absolute.
type seekingRandomAccessInput struct {
	in store.IndexInput
}

func (r *seekingRandomAccessInput) Length() int64 { return r.in.Length() }

func (r *seekingRandomAccessInput) ReadByteAt(pos int64) (byte, error) {
	if err := r.in.SetPosition(pos); err != nil {
		return 0, err
	}
	return r.in.ReadByte()
}

func (r *seekingRandomAccessInput) ReadShortAt(pos int64) (int16, error) {
	if err := r.in.SetPosition(pos); err != nil {
		return 0, err
	}
	return r.in.ReadShort()
}

func (r *seekingRandomAccessInput) ReadIntAt(pos int64) (int32, error) {
	if err := r.in.SetPosition(pos); err != nil {
		return 0, err
	}
	return r.in.ReadInt()
}

func (r *seekingRandomAccessInput) ReadLongAt(pos int64) (int64, error) {
	if err := r.in.SetPosition(pos); err != nil {
		return 0, err
	}
	return r.in.ReadLong()
}

var _ store.RandomAccessInput = (*seekingRandomAccessInput)(nil)

// randomAccessInputOf renders the IndexInput -> RandomAccessInput conversion
// performed by IndexInput.randomAccessSlice(long, long): an input that already
// supports random access is used as it is, anything else is wrapped in the
// seek-then-read default implementation. The clone keeps the positioned reads
// from disturbing the sequential file pointer of in, exactly as Lucene's slice
// does.
func randomAccessInputOf(in store.IndexInput) store.RandomAccessInput {
	if rai, ok := in.(store.RandomAccessInput); ok {
		return rai
	}
	return &seekingRandomAccessInput{in: in.Clone()}
}

// Load loads an NRTSuggester from an IndexInput.
// Mirrors NRTSuggester.load(IndexInput).
func Load(input store.IndexInput) (*NRTSuggester, error) {
	outputs := utilfst.NewPairOutputs[int64, *util.BytesRef](
		utilfst.PositiveIntOutputsSingleton(),
		utilfst.ByteSequenceOutputsSingleton(),
	)
	metadata, err := utilfst.ReadMetadata(input, outputs)
	if err != nil {
		return nil, fmt.Errorf("nrtsuggester: read metadata: %w", err)
	}
	fstStore, err := utilfst.NewOffHeapFSTStore(
		randomAccessInputOf(input), input.GetFilePointer(), metadata.NumBytes())
	if err != nil {
		return nil, fmt.Errorf("nrtsuggester: off-heap fst store: %w", err)
	}
	fst, err := utilfst.FromFSTReader(metadata, fstStore)
	if err != nil {
		return nil, fmt.Errorf("nrtsuggester: from fst reader: %w", err)
	}
	if err := input.SkipBytes(fstStore.Size()); err != nil {
		return nil, err
	}

	// read some meta info
	maxAnalyzedPathsPerOutput, err := input.ReadVInt()
	if err != nil {
		return nil, fmt.Errorf("nrtsuggester: read max analyzed paths: %w", err)
	}
	// Label used to denote the end of an input in the FST and the beginning of
	// dedup bytes.
	if _, err := input.ReadVInt(); err != nil {
		return nil, fmt.Errorf("nrtsuggester: read end byte: %w", err)
	}
	payloadSep, err := input.ReadVInt()
	if err != nil {
		return nil, fmt.Errorf("nrtsuggester: read payload sep: %w", err)
	}

	return &NRTSuggester{
		fst:                       fst,
		maxAnalyzedPathsPerOutput: int(maxAnalyzedPathsPerOutput),
		payloadSep:                int(payloadSep),
	}, nil
}

// SuggestIndexSearcher is the search-side facade exposed to callers. Mirrors
// org.apache.lucene.search.suggest.document.SuggestIndexSearcher.
type SuggestIndexSearcher struct {
	Suggester *NRTSuggester
}

// NewSuggestIndexSearcher wires the searcher to a suggester.
func NewSuggestIndexSearcher(s *NRTSuggester) *SuggestIndexSearcher {
	return &SuggestIndexSearcher{Suggester: s}
}
