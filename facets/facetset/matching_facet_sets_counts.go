package facetset

// MatchingFacetSetsCounts aggregates per-matcher hit counts across documents
// whose FacetSets satisfy the configured matchers. Mirrors
// org.apache.lucene.facet.facetset.MatchingFacetSetsCounts.
type MatchingFacetSetsCounts struct {
	dim       string
	matchers  []FacetSetMatcher
	decoder   FacetSetDecoder
	counts    []int
	totalHits int
}

// NewMatchingFacetSetsCounts builds an aggregator on the supplied dimension
// label, matcher list, and FacetSet decoder.
func NewMatchingFacetSetsCounts(dim string, matchers []FacetSetMatcher, decoder FacetSetDecoder) *MatchingFacetSetsCounts {
	return &MatchingFacetSetsCounts{
		dim:      dim,
		matchers: matchers,
		decoder:  decoder,
		counts:   make([]int, len(matchers)),
	}
}

// Accumulate decodes the binary payload, walks every FacetSet, and increments
// the count of any matcher whose Matches returns true.
func (m *MatchingFacetSetsCounts) Accumulate(binaryValue []byte) error {
	if len(binaryValue) == 0 || len(m.matchers) == 0 {
		return nil
	}
	numSets, off, err := readVInt(binaryValue, 0)
	if err != nil {
		return err
	}
	dims, off, err := readVInt(binaryValue, off)
	if err != nil {
		return err
	}
	tmp := make([]int64, dims)
	for i := uint32(0); i < numSets; i++ {
		consumed := m.decoder(binaryValue, off, int(dims), tmp)
		off += consumed
		for j, mat := range m.matchers {
			if mat.Dims() == int(dims) && mat.Matches(tmp) {
				m.counts[j]++
			}
		}
	}
	m.totalHits++
	return nil
}

// GetTotalHits returns the number of documents accumulated.
func (m *MatchingFacetSetsCounts) GetTotalHits() int { return m.totalHits }

// CountForMatcher returns the count recorded for the matcher at index i.
func (m *MatchingFacetSetsCounts) CountForMatcher(i int) int {
	if i < 0 || i >= len(m.counts) {
		return 0
	}
	return m.counts[i]
}

// GetCounts returns a copy of the per-matcher counts.
func (m *MatchingFacetSetsCounts) GetCounts() []int {
	out := make([]int, len(m.counts))
	copy(out, m.counts)
	return out
}

// GetDim returns the dimension label.
func (m *MatchingFacetSetsCounts) GetDim() string { return m.dim }

func readVInt(buf []byte, off int) (uint32, int, error) {
	if off >= len(buf) {
		return 0, off, errShortBuffer
	}
	var shift uint
	var result uint32
	for {
		b := buf[off]
		off++
		result |= uint32(b&0x7f) << shift
		if b&0x80 == 0 {
			return result, off, nil
		}
		shift += 7
		if shift > 35 || off >= len(buf) {
			return 0, off, errVarintOverflow
		}
	}
}

type shortBufferError struct{}

func (shortBufferError) Error() string { return "facetset: short buffer" }

type varintOverflowError struct{}

func (varintOverflowError) Error() string { return "facetset: varint overflow" }

var (
	errShortBuffer    = shortBufferError{}
	errVarintOverflow = varintOverflowError{}
)
