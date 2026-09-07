package document

// NRTSuggester executes Top N search on a weighted FST. Mirrors
// org.apache.lucene.search.suggest.document.NRTSuggester.
type NRTSuggester struct {
	fst                      *utilfst.FST[*utilfst.Pair[int64, *util.BytesRef]]
	maxAnalyzedPathsPerOutput int
	payloadSep               int
}

// NewNRTSuggester builds an empty NRTSuggester.
func NewNRTSuggester() *NRTSuggester { return &NRTSuggester{} }

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
	store := utilfst.NewOffHeapFSTStore(input, input.GetFilePointer(), metadata)
	fst, err := utilfst.FromFSTReader(metadata, store)
	if err != nil {
		return nil, fmt.Errorf("nrtsuggester: from fst reader: %w", err)
	}
	if err := input.SkipBytes(store.Size()); err != nil {
		return nil, err
	}

	maxAnalyzedPathsPerOutput, err := store.ReadVInt(input)
	if err != nil {
		return nil, fmt.Errorf("nrtsuggester: read max analyzed paths: %w", err)
	}
	if _, err := store.ReadVInt(input); err != nil {
		return nil, fmt.Errorf("nrtsuggester: read end byte: %w", err)
	}
	payloadSep, err := store.ReadVInt(input)
	if err != nil {
		return nil, fmt.Errorf("nrtsuggester: read payload sep: %w", err)
	}

	return &NRTSuggester{
		fst:                      fst,
		maxAnalyzedPathsPerOutput: int(maxAnalyzedPathsPerOutput),
		payloadSep:               int(payloadSep),
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
