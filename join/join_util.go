package join

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// JoinValueResolver extracts the join value of a single document from a
// searcher. It is the pluggable extraction point used by JoinUtil to bridge
// the algorithmic shape (run query, iterate matches, collect values) to the
// codec-level machinery that actually surfaces the bytes for a field on a
// given doc id.
//
// A resolver returns nil and a nil error when the document has no value for
// the requested field (the value is silently skipped). Returning an error
// aborts collection and bubbles up to the caller.
type JoinValueResolver interface {
	// ResolveJoinValue returns the bytes of the join value for the given doc
	// in the given field, or nil if the document has no value.
	ResolveJoinValue(searcher *search.IndexSearcher, doc int, field string) ([]byte, error)
}

// JoinUtil provides utility methods for join queries.
// This is the Go port of Lucene's org.apache.lucene.search.join.JoinUtil.
type JoinUtil struct {
	// resolver extracts the join value for a doc + field. When nil, JoinUtil
	// falls back to the stored-fields resolver returned by
	// StoredFieldsJoinValueResolver.
	resolver JoinValueResolver
}

// NewJoinUtil creates a new JoinUtil with the default stored-fields resolver.
func NewJoinUtil() *JoinUtil {
	return &JoinUtil{resolver: StoredFieldsJoinValueResolver{}}
}

// NewJoinUtilWithResolver creates a new JoinUtil that uses the provided
// resolver to extract join values from matching documents. Pass a custom
// resolver to plug in doc-values, payloads, or test fixtures.
func NewJoinUtilWithResolver(resolver JoinValueResolver) *JoinUtil {
	if resolver == nil {
		resolver = StoredFieldsJoinValueResolver{}
	}
	return &JoinUtil{resolver: resolver}
}

// StoredFieldsJoinValueResolver extracts join values from stored fields via
// IndexSearcher.Doc. It is the default resolver wired by NewJoinUtil.
type StoredFieldsJoinValueResolver struct{}

// ResolveJoinValue loads the document via the searcher and returns the bytes
// of the named field's stored value, or nil if the document does not contain
// the field.
func (StoredFieldsJoinValueResolver) ResolveJoinValue(searcher *search.IndexSearcher, doc int, field string) ([]byte, error) {
	if searcher == nil {
		return nil, nil
	}
	d, err := searcher.Doc(doc)
	if err != nil {
		return nil, err
	}
	if d == nil {
		return nil, nil
	}
	f := d.GetField(field)
	if f == nil {
		return nil, nil
	}
	// Prefer the binary representation; fall back to the string form.
	if br := f.BinaryValue(); br != nil {
		// Copy to detach from any reused buffer.
		out := make([]byte, len(br))
		copy(out, br)
		return out, nil
	}
	if sv := f.StringValue(); sv != "" {
		return []byte(sv), nil
	}
	return nil, nil
}

// CreateJoinQuery creates a query that matches parent documents
// based on child document criteria.
// Parameters:
//   - fromField: the field in the parent documents to match
//   - toField: the field in the child documents containing the join value
//   - fromQuery: the query to find matching child documents
//   - searcher: the index searcher
//   - scoreMode: how to combine scores from child documents
//
// Returns a query that matches parent documents.
func (ju *JoinUtil) CreateJoinQuery(fromField string, toField string, fromQuery search.Query, searcher *search.IndexSearcher, scoreMode ScoreMode) (search.Query, error) {
	// Collect the join values from matching child documents
	values, err := ju.CollectValues(fromQuery, toField, searcher)
	if err != nil {
		return nil, fmt.Errorf("failed to collect join values: %w", err)
	}

	if len(values) == 0 {
		// No matching children, return a query that matches nothing
		return &MatchNoDocsQuery{}, nil
	}

	// Create a query that matches parent documents with those values
	return NewTermsQuery(fromField, values), nil
}

// CollectValues collects the distinct values of the specified field from
// documents matching the given query.
//
// The query is executed via the supplied IndexSearcher with an unbounded
// TopDocs window (capped at the reader's maxDoc), and each matching
// document's field value is extracted via the configured JoinValueResolver.
// Duplicate values are filtered: each distinct byte sequence is returned
// once, preserving first-seen order so the result is deterministic for a
// given matching set.
func (ju *JoinUtil) CollectValues(query search.Query, field string, searcher *search.IndexSearcher) ([][]byte, error) {
	if query == nil || searcher == nil || field == "" {
		return nil, nil
	}
	resolver := ju.resolver
	if resolver == nil {
		resolver = StoredFieldsJoinValueResolver{}
	}

	reader := searcher.GetIndexReader()
	if reader == nil {
		return nil, nil
	}
	limit := reader.MaxDoc()
	if limit <= 0 {
		return nil, nil
	}

	topDocs, err := searcher.Search(query, limit)
	if err != nil {
		return nil, fmt.Errorf("join: search failed: %w", err)
	}

	seen := make(map[string]struct{}, len(topDocs.ScoreDocs))
	values := make([][]byte, 0, len(topDocs.ScoreDocs))
	for _, sd := range topDocs.ScoreDocs {
		v, err := resolver.ResolveJoinValue(searcher, sd.Doc, field)
		if err != nil {
			return nil, err
		}
		if v == nil {
			continue
		}
		key := string(v)
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		values = append(values, v)
	}
	return values, nil
}

// BuildBitSet builds a bit set of matching documents for the given query.
//
// It executes the query against the supplied reader through an IndexSearcher
// and sets one bit per matching document. The bit set is sized to the
// reader's MaxDoc so callers can safely intersect it with other producers
// of the same reader.
func (ju *JoinUtil) BuildBitSet(query search.Query, reader *index.IndexReader) (*DocIdBitSet, error) {
	if reader == nil {
		return NewDocIdBitSet(0), nil
	}
	bitSet := NewDocIdBitSet(reader.MaxDoc())
	if query == nil || reader.MaxDoc() == 0 {
		return bitSet, nil
	}

	searcher := search.NewIndexSearcher(reader)
	collector := &bitSetDocCollector{bitSet: bitSet}
	if err := searcher.SearchWithCollector(query, collector); err != nil {
		return nil, fmt.Errorf("join: BuildBitSet search failed: %w", err)
	}
	return bitSet, nil
}

// bitSetDocCollector is an internal Collector/LeafCollector that sets the
// bit corresponding to every matching doc id (translated to the top-level
// id space via the leaf's docBase).
type bitSetDocCollector struct {
	bitSet  *DocIdBitSet
	docBase int
}

// GetLeafCollector returns a leaf collector bound to the leaf's docBase so doc
// ids are translated into the composite reader id space.
func (c *bitSetDocCollector) GetLeafCollector(context *index.LeafReaderContext) (search.LeafCollector, error) {
	if context != nil {
		c.docBase = context.DocBase()
	}
	return c, nil
}

// leafReaderFromContext extracts the concrete *index.LeafReader from a leaf
// context, transparently unwrapping a *index.SegmentReader (which embeds a
// *LeafReader). It returns nil when the context or its reader is nil, so
// callers can degrade gracefully. This is shared by the join collectors that
// resolve doc-values from the segment.
func leafReaderFromContext(context *index.LeafReaderContext) *index.LeafReader {
	if context == nil {
		return nil
	}
	switch v := context.Reader().(type) {
	case *index.LeafReader:
		return v
	case *index.SegmentReader:
		return v.LeafReader
	default:
		return nil
	}
}

// ScoreMode returns COMPLETE_NO_SCORES: the bit set only needs the doc ids.
func (c *bitSetDocCollector) ScoreMode() search.ScoreMode {
	return search.COMPLETE_NO_SCORES
}

// SetScorer is a no-op: scores are not required to populate a bit set.
func (c *bitSetDocCollector) SetScorer(scorer search.Scorer) error {
	return nil
}

// Collect sets the bit for the given (leaf-local) doc id.
func (c *bitSetDocCollector) Collect(doc int) error {
	c.bitSet.Set(doc + c.docBase)
	return nil
}

// ScoreMode determines how scores from multiple documents are combined.
type ScoreMode int

const (
	// None - no scores are needed
	None ScoreMode = iota
	// Avg - average of child scores
	Avg
	// Max - maximum of child scores
	Max
	// Total - sum of child scores
	Total
	// Min - minimum of child scores
	Min
)

// String returns the string representation of this ScoreMode.
func (sm ScoreMode) String() string {
	switch sm {
	case None:
		return "None"
	case Avg:
		return "Avg"
	case Max:
		return "Max"
	case Total:
		return "Total"
	case Min:
		return "Min"
	default:
		return "Unknown"
	}
}

// DocIdBitSet is a bit set for document IDs.
type DocIdBitSet struct {
	bits   []uint64
	length int
}

// NewDocIdBitSet creates a new DocIdBitSet for the given number of documents.
func NewDocIdBitSet(length int) *DocIdBitSet {
	numWords := (length + 63) / 64
	return &DocIdBitSet{
		bits:   make([]uint64, numWords),
		length: length,
	}
}

// Set sets the bit at the given document ID.
func (bs *DocIdBitSet) Set(doc int) {
	if doc < 0 || doc >= bs.length {
		return
	}
	word := doc / 64
	bit := uint(doc % 64)
	bs.bits[word] |= 1 << bit
}

// Get returns true if the bit at the given document ID is set.
func (bs *DocIdBitSet) Get(doc int) bool {
	if doc < 0 || doc >= bs.length {
		return false
	}
	word := doc / 64
	bit := uint(doc % 64)
	return (bs.bits[word] & (1 << bit)) != 0
}

// Clear clears the bit at the given document ID.
func (bs *DocIdBitSet) Clear(doc int) {
	if doc < 0 || doc >= bs.length {
		return
	}
	word := doc / 64
	bit := uint(doc % 64)
	bs.bits[word] &= ^(1 << bit)
}

// Length returns the length of this bit set.
func (bs *DocIdBitSet) Length() int {
	return bs.length
}

// Cardinality returns the number of set bits.
func (bs *DocIdBitSet) Cardinality() int {
	count := 0
	for _, word := range bs.bits {
		count += popCount(word)
	}
	return count
}

// popCount returns the number of set bits in a uint64.
func popCount(x uint64) int {
	// Brian Kernighan's algorithm
	count := 0
	for x != 0 {
		x &= x - 1
		count++
	}
	return count
}

// And performs a bitwise AND with another bit set.
func (bs *DocIdBitSet) And(other *DocIdBitSet) {
	minLen := len(bs.bits)
	if len(other.bits) < minLen {
		minLen = len(other.bits)
	}
	for i := 0; i < minLen; i++ {
		bs.bits[i] &= other.bits[i]
	}
	for i := minLen; i < len(bs.bits); i++ {
		bs.bits[i] = 0
	}
}

// Or performs a bitwise OR with another bit set.
func (bs *DocIdBitSet) Or(other *DocIdBitSet) {
	minLen := len(bs.bits)
	if len(other.bits) < minLen {
		minLen = len(other.bits)
	}
	for i := 0; i < minLen; i++ {
		bs.bits[i] |= other.bits[i]
	}
}

// MatchNoDocsQuery is a query that matches no documents.
type MatchNoDocsQuery struct{}

// Rewrite rewrites this query.
func (q *MatchNoDocsQuery) Rewrite(reader search.IndexReader) (search.Query, error) {
	return q, nil
}

// Clone creates a copy of this query.
func (q *MatchNoDocsQuery) Clone() search.Query {
	return &MatchNoDocsQuery{}
}

// Equals checks if this query equals another.
func (q *MatchNoDocsQuery) Equals(other search.Query) bool {
	_, ok := other.(*MatchNoDocsQuery)
	return ok
}

// HashCode returns a hash code for this query.
func (q *MatchNoDocsQuery) HashCode() int {
	return 0
}

// CreateWeight creates a Weight for this query.
func (q *MatchNoDocsQuery) CreateWeight(searcher *search.IndexSearcher, needsScores bool, boost float32) (search.Weight, error) {
	return nil, nil
}

// TermsQuery is a query that matches documents with specific terms.
type TermsQuery struct {
	field string
	terms [][]byte
}

// NewTermsQuery creates a new TermsQuery.
func NewTermsQuery(field string, terms [][]byte) *TermsQuery {
	return &TermsQuery{
		field: field,
		terms: terms,
	}
}

// Rewrite rewrites this query.
func (q *TermsQuery) Rewrite(reader search.IndexReader) (search.Query, error) {
	return q, nil
}

// Clone creates a copy of this query.
func (q *TermsQuery) Clone() search.Query {
	terms := make([][]byte, len(q.terms))
	for i, term := range q.terms {
		terms[i] = make([]byte, len(term))
		copy(terms[i], term)
	}
	return &TermsQuery{
		field: q.field,
		terms: terms,
	}
}

// Equals checks if this query equals another.
func (q *TermsQuery) Equals(other search.Query) bool {
	if o, ok := other.(*TermsQuery); ok {
		if q.field != o.field || len(q.terms) != len(o.terms) {
			return false
		}
		for i, term := range q.terms {
			if string(term) != string(o.terms[i]) {
				return false
			}
		}
		return true
	}
	return false
}

// HashCode returns a hash code for this query.
func (q *TermsQuery) HashCode() int {
	hash := 0
	for _, c := range q.field {
		hash = 31*hash + int(c)
	}
	for _, term := range q.terms {
		for _, b := range term {
			hash = 31*hash + int(b)
		}
	}
	return hash
}

// CreateWeight creates a Weight for this query.
func (q *TermsQuery) CreateWeight(searcher *search.IndexSearcher, needsScores bool, boost float32) (search.Weight, error) {
	return nil, nil
}
