// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// Ported from Apache Lucene 10.5.0:
//
//	lucene/core/src/java/org/apache/lucene/search/CombinedFieldQuery.java
//
// The FieldAndWeight record nested in the Java class is declared in
// multi_norms_leaf_sim_scorer.go, alongside the MultiNormsLeafSimScorer that
// consumes it. DisiWrapper, DisjunctionDISIApproximation and DisiPriorityQueue
// are their own Lucene classes and live in their own files; this file declares
// none of them.

import (
	"fmt"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"math"
	"sort"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// CombinedFieldQuery treats multiple fields as a single stream and scores terms
// as if they had been indexed in a single field whose values would be the union
// of the values of the provided fields.
type CombinedFieldQuery struct {
	*BaseQuery

	// fieldAndWeights and fields together render Java's
	// TreeMap<String, FieldAndWeight>: the mapping itself plus the ascending
	// key order a TreeMap iterates in. Every traversal below walks fields so
	// that the iteration order — and therefore the order of the float
	// accumulations that depend on it — matches Java exactly.
	fieldAndWeights map[string]FieldAndWeight
	fields          []string

	// term holds the term bytes.
	term *util.BytesRef

	// fieldTerms is the array of terms per field, sorted by field.
	fieldTerms []*index.Term

	ramBytesUsed int64
}

// Builder is a builder for CombinedFieldQuery.
//
// Mirrors the static nested class CombinedFieldQuery.Builder.
type Builder struct {
	fieldAndWeights map[string]FieldAndWeight
	term            *util.BytesRef
}

// NewBuilder creates a builder for the given term string.
//
// Mirrors Builder(String term).
func NewBuilder(term string) *Builder {
	return &Builder{
		fieldAndWeights: make(map[string]FieldAndWeight),
		term:            util.BytesRefFromString(term),
	}
}

// NewBuilderWithBytes creates a builder for the given term bytes.
//
// Mirrors Builder(BytesRef term).
func NewBuilderWithBytes(term *util.BytesRef) *Builder {
	return &Builder{
		fieldAndWeights: make(map[string]FieldAndWeight),
		term:            util.DeepCopyOfBytesRef(term),
	}
}

// AddField adds a field to this builder.
//
// Mirrors Builder.addField(String).
func (b *Builder) AddField(field string) *Builder {
	return b.AddFieldWithWeight(field, 1.0)
}

// AddFieldWithWeight adds a field with the given weight to this builder.
//
// Mirrors Builder.addField(String, float).
func (b *Builder) AddFieldWithWeight(field string, weight float32) *Builder {
	if weight < 1.0 {
		panic("weight must be greater or equal to 1")
	}
	b.fieldAndWeights[field] = FieldAndWeight{Field: field, Weight: weight}
	return b
}

// Build builds the CombinedFieldQuery.
//
// Mirrors Builder.build().
func (b *Builder) Build() *CombinedFieldQuery {
	if len(b.fieldAndWeights) > GetMaxClauseCount() {
		panic(NewTooManyClauses())
	}
	return newCombinedFieldQuery(b.fieldAndWeights, b.term)
}

// newCombinedFieldQuery mirrors the private constructor
// CombinedFieldQuery(TreeMap<String, FieldAndWeight>, BytesRef).
func newCombinedFieldQuery(fieldAndWeights map[string]FieldAndWeight, term *util.BytesRef) *CombinedFieldQuery {
	if term == nil {
		panic("term must not be nil")
	}
	if len(fieldAndWeights) > GetMaxClauseCount() {
		panic(NewTooManyClauses())
	}

	// The TreeMap copy: the keys in ascending order.
	fields := make([]string, 0, len(fieldAndWeights))
	for f := range fieldAndWeights {
		fields = append(fields, f)
	}
	sort.Strings(fields)

	owned := make(map[string]FieldAndWeight, len(fieldAndWeights))
	for f, fw := range fieldAndWeights {
		owned[f] = fw
	}

	fieldTerms := make([]*index.Term, len(fields))
	pos := 0
	for _, field := range fields {
		fieldTerms[pos] = index.NewTermFromBytesRef(field, term)
		pos++
	}

	return &CombinedFieldQuery{
		BaseQuery:       &BaseQuery{},
		fieldAndWeights: owned,
		fields:          fields,
		term:            term,
		fieldTerms:      fieldTerms,
		// RamUsageEstimator has no Gocene counterpart yet; the accounted size
		// is therefore reported as 0 rather than guessed at.
		ramBytesUsed: 0,
	}
}

// ToString mirrors CombinedFieldQuery.toString(String).
func (q *CombinedFieldQuery) ToString(field string) string {
	var builder strings.Builder
	builder.WriteString("CombinedFieldQuery((")
	pos := 0
	for _, f := range q.fields {
		fieldWeight := q.fieldAndWeights[f]
		if pos != 0 {
			builder.WriteString(" ")
		}
		pos++
		builder.WriteString(fieldWeight.Field)
		if fieldWeight.Weight != 1.0 {
			builder.WriteString("^")
			builder.WriteString(fmt.Sprintf("%v", fieldWeight.Weight))
		}
	}
	builder.WriteString(")(")
	builder.WriteString(q.term.String())
	builder.WriteString("))")
	return builder.String()
}

// String renders Query.toString(), the no-argument form that delegates to
// toString(String) with the empty default field.
func (q *CombinedFieldQuery) String() string {
	return q.ToString("")
}

// Equals mirrors CombinedFieldQuery.equals(Object).
func (q *CombinedFieldQuery) Equals(other spi.Query) bool {
	if Query(q) == other {
		return true
	}
	that, ok := other.(*CombinedFieldQuery)
	if !ok {
		return false
	}
	if len(q.fieldAndWeights) != len(that.fieldAndWeights) {
		return false
	}
	for f, fw := range q.fieldAndWeights {
		otherFw, ok := that.fieldAndWeights[f]
		if !ok || otherFw != fw {
			return false
		}
	}
	return util.BytesRefEquals(q.term, that.term)
}

// HashCode mirrors CombinedFieldQuery.hashCode(). classHash() is rendered as 0,
// the value Gocene's Query ports use for Java's per-class hash seed.
func (q *CombinedFieldQuery) HashCode() int {
	result := 0
	result = 31*result + combinedFieldWeightsHash(q.fieldAndWeights, q.fields)
	result = 31*result + q.term.HashCode()
	return result
}

// combinedFieldWeightsHash renders Objects.hash(fieldAndWeights), the hash of a
// Map, which is the sum of its entry hashes and therefore order-independent.
func combinedFieldWeightsHash(fieldAndWeights map[string]FieldAndWeight, fields []string) int {
	sum := 0
	for _, f := range fields {
		fw := fieldAndWeights[f]
		keyHash := 0
		for i := 0; i < len(f); i++ {
			keyHash = 31*keyHash + int(f[i])
		}
		valueHash := keyHash*31 + int(math.Float32bits(fw.Weight))
		sum += keyHash ^ valueHash
	}
	// Objects.hash(Object...) is Arrays.hashCode(new Object[]{map}).
	return 31*1 + sum
}

// RamBytesUsed mirrors CombinedFieldQuery.ramBytesUsed().
func (q *CombinedFieldQuery) RamBytesUsed() int64 {
	return q.ramBytesUsed
}

// Rewrite mirrors CombinedFieldQuery.rewrite(IndexSearcher).
func (q *CombinedFieldQuery) Rewrite(indexSearcher *IndexSearcher) (Query, error) {
	if len(q.fieldAndWeights) == 0 {
		return NewBooleanQueryBuilder().Build(), nil
	}
	return q, nil
}

// Visit mirrors CombinedFieldQuery.visit(QueryVisitor).
func (q *CombinedFieldQuery) Visit(visitor QueryVisitor) {
	var selectedTerms []*index.Term
	for _, t := range q.fieldTerms {
		if visitor.AcceptField(t.Field) {
			selectedTerms = append(selectedTerms, t)
		}
	}
	if len(selectedTerms) > 0 {
		v := visitor.GetSubVisitor(SHOULD, q)
		v.ConsumeTerms(q, selectedTerms...)
	}
}

// rewriteToBoolean mirrors the private CombinedFieldQuery.rewriteToBoolean().
func (q *CombinedFieldQuery) rewriteToBoolean() *BooleanQuery {
	// rewrite to a simple disjunction if the score is not needed.
	bq := NewBooleanQueryBuilder()
	for _, term := range q.fieldTerms {
		bq.Add(NewTermQuery(term), SHOULD)
	}
	return bq.Build()
}

// CreateWeight mirrors CombinedFieldQuery.createWeight(IndexSearcher, ScoreMode, float).
func (q *CombinedFieldQuery) CreateWeight(searcher *IndexSearcher, scoreMode ScoreMode, boost float32) (Weight, error) {
	if err := q.validateConsistentNorms(searcher.GetIndexReader()); err != nil {
		return nil, err
	}
	if scoreMode.NeedsScores() {
		return newCombinedFieldWeight(q, searcher, scoreMode, boost)
	}
	// rewrite to a simple disjunction if the score is not needed.
	bq := q.rewriteToBoolean()
	rewritten, err := searcher.Rewrite(bq)
	if err != nil {
		return nil, err
	}
	return rewritten.CreateWeight(searcher, ScoreModeCompleteNoScores, boost)
}

// validateConsistentNorms mirrors the private
// CombinedFieldQuery.validateConsistentNorms(IndexReader).
func (q *CombinedFieldQuery) validateConsistentNorms(reader index.IndexReaderInterface) error {
	allFieldsHaveNorms := true
	noFieldsHaveNorms := true

	leaves, err := reader.Leaves()
	if err != nil {
		return err
	}
	for _, context := range leaves {
		fieldInfos := context.LeafReader().GetFieldInfos()
		for _, field := range q.fields {
			fieldInfo := fieldInfos.FieldInfo(field)
			if fieldInfo != nil {
				allFieldsHaveNorms = allFieldsHaveNorms && fieldInfo.HasNorms()
				noFieldsHaveNorms = noFieldsHaveNorms && fieldInfo.OmitNorms()
			}
		}
	}

	if !allFieldsHaveNorms && !noFieldsHaveNorms {
		return fmt.Errorf("CombinedFieldQuery requires norms to be consistent across fields: some fields cannot  have norms enabled, while others have norms disabled")
	}
	return nil
}

// combinedFieldWeight mirrors the inner class CombinedFieldQuery.CombinedFieldWeight.
type combinedFieldWeight struct {
	BaseWeight
	query      *CombinedFieldQuery
	searcher   *IndexSearcher
	termStates []*index.TermStates
	simWeight  SimScorer
}

// newCombinedFieldWeight mirrors
// CombinedFieldWeight(Query, IndexSearcher, ScoreMode, float).
func newCombinedFieldWeight(q *CombinedFieldQuery, searcher *IndexSearcher, scoreMode ScoreMode, boost float32) (Weight, error) {
	w := &combinedFieldWeight{
		BaseWeight: BaseWeight{query: q},
		query:      q,
		searcher:   searcher,
	}

	var docFreq int64
	var totalTermFreq int64
	w.termStates = make([]*index.TermStates, len(q.fieldTerms))
	for i := 0; i < len(w.termStates); i++ {
		field := q.fieldAndWeights[q.fieldTerms[i].Field]
		ts, err := index.BuildTermStates(searcher, q.fieldTerms[i], true)
		if err != nil {
			return nil, err
		}
		w.termStates[i] = ts
		if ts.DocFreq() > 0 {
			termStats := searcher.TermStatistics(q.fieldTerms[i], ts.DocFreq(), ts.TotalTermFreq())
			if int64(termStats.DocFreq()) > docFreq {
				docFreq = int64(termStats.DocFreq())
			}
			totalTermFreq += int64(float64(field.Weight) * float64(termStats.TotalTermFreq()))
		}
	}

	if docFreq > 0 {
		pseudoCollectionStats, err := w.mergeCollectionStatistics(searcher)
		if err != nil {
			return nil, err
		}
		if totalTermFreq < 1 {
			totalTermFreq = 1
		}
		pseudoTermStatistics := NewTermStatistics(
			index.NewTermFromBytesRef("pseudo_field", util.BytesRefFromString("pseudo_term")),
			int(docFreq),
			totalTermFreq,
		)
		w.simWeight = searcher.GetSimilarity().Scorer104(boost, pseudoCollectionStats, pseudoTermStatistics)
	} else {
		w.simWeight = nil
	}

	return w, nil
}

// mergeCollectionStatistics mirrors the private
// CombinedFieldWeight.mergeCollectionStatistics(IndexSearcher).
func (w *combinedFieldWeight) mergeCollectionStatistics(searcher *IndexSearcher) (*CollectionStatistics, error) {
	var maxDoc int64
	var docCount int64
	var sumTotalTermFreq int64
	var sumDocFreq int64
	for _, f := range w.query.fields {
		fieldWeight := w.query.fieldAndWeights[f]
		collectionStats, err := searcher.CollectionStatistics(fieldWeight.Field)
		if err != nil {
			return nil, err
		}
		if collectionStats != nil {
			if int64(collectionStats.MaxDoc()) > maxDoc {
				maxDoc = int64(collectionStats.MaxDoc())
			}
			if int64(collectionStats.DocCount()) > docCount {
				docCount = int64(collectionStats.DocCount())
			}
			if collectionStats.SumDocFreq() > sumDocFreq {
				sumDocFreq = collectionStats.SumDocFreq()
			}
			sumTotalTermFreq += int64(float64(fieldWeight.Weight) * float64(collectionStats.SumTotalTermFreq()))
		}
	}

	return NewCollectionStatistics("pseudo_field", int(maxDoc), int(docCount), sumTotalTermFreq, sumDocFreq), nil
}

// Matches mirrors CombinedFieldWeight.matches(LeafReaderContext, int).
func (w *combinedFieldWeight) Matches(context *index.LeafReaderContext, doc int) (Matches, error) {
	rewritten, err := w.searcher.Rewrite(w.query.rewriteToBoolean())
	if err != nil {
		return nil, err
	}
	weight, err := rewritten.CreateWeight(w.searcher, ScoreModeComplete, 1.0)
	if err != nil {
		return nil, err
	}
	return weight.Matches(context, doc)
}

// Scorer mirrors `public final Scorer scorer(LeafReaderContext)` of Weight.java.
//
// It is restated here rather than inherited from BaseWeight because Go resolves
// the embedded BaseWeight.Scorer call to BaseWeight.ScorerSupplier, not to the
// override below; Java's virtual dispatch reaches the override.
func (w *combinedFieldWeight) Scorer(context *index.LeafReaderContext) (Scorer, error) {
	supplier, err := w.ScorerSupplier(context)
	if err != nil {
		return nil, err
	}
	if supplier == nil {
		return nil, nil
	}
	return supplier.Get(math.MaxInt64)
}

// Explain mirrors CombinedFieldWeight.explain(LeafReaderContext, int).
func (w *combinedFieldWeight) Explain(context *index.LeafReaderContext, doc int) (Explanation, error) {
	scorer, err := w.Scorer(context)
	if err != nil {
		return nil, err
	}
	if scorer != nil {
		newDoc, err := scorer.Iterator().Advance(doc)
		if err != nil {
			return nil, err
		}
		if newDoc == doc {
			cfScorer, ok := scorer.(*combinedFieldScorer)
			if !ok {
				return nil, fmt.Errorf("CombinedFieldWeight: expected a combinedFieldScorer, got %T", scorer)
			}
			freq, err := cfScorer.freq()
			if err != nil {
				return nil, err
			}
			docScorer, err := newMultiNormsLeafSimScorer(w.simWeight, context.LeafReader(), w.query.orderedFieldAndWeights(), true)
			if err != nil {
				return nil, err
			}
			freqExplanation := MatchExplanation(freq, fmt.Sprintf("termFreq=%v", freq))
			scoreExplanation, err := docScorer.explain(doc, freqExplanation)
			if err != nil {
				return nil, err
			}
			return MatchExplanationWithDetails(
				scoreExplanation.GetValue(),
				fmt.Sprintf("weight(%s in %d), result of:", queryToString(w.GetQuery(), ""), doc),
				scoreExplanation,
			), nil
		}
	}
	return NoMatchExplanation("no matching term"), nil
}

// orderedFieldAndWeights renders fieldAndWeights.values() on Java's TreeMap:
// the FieldAndWeight values in ascending field order.
func (q *CombinedFieldQuery) orderedFieldAndWeights() []FieldAndWeight {
	values := make([]FieldAndWeight, 0, len(q.fields))
	for _, f := range q.fields {
		values = append(values, q.fieldAndWeights[f])
	}
	return values
}

// ScorerSupplier mirrors CombinedFieldWeight.scorerSupplier(LeafReaderContext).
func (w *combinedFieldWeight) ScorerSupplier(context *index.LeafReaderContext) (ScorerSupplier, error) {
	var iterators []index.PostingsEnum
	var fields []FieldAndWeight
	var cost int64
	for i := 0; i < len(w.query.fieldTerms); i++ {
		supplier, err := w.termStates[i].Get(context)
		if err != nil {
			return nil, err
		}
		var state index.TermState
		if supplier != nil {
			state, err = supplier()
			if err != nil {
				return nil, err
			}
		}
		if state != nil {
			terms, err := context.LeafReader().Terms(w.query.fieldTerms[i].Field)
			if err != nil {
				return nil, err
			}
			termsEnum, err := terms.Iterator()
			if err != nil {
				return nil, err
			}
			if err := index.SeekExactWithState(termsEnum, w.query.fieldTerms[i], state); err != nil {
				return nil, err
			}
			postingsEnum, err := termsEnum.Postings(index.PostingsFlagFreqs)
			if err != nil {
				return nil, err
			}
			iterators = append(iterators, postingsEnum)
			fields = append(fields, w.query.fieldAndWeights[w.query.fieldTerms[i].Field])
			cost += postingsEnum.Cost()
		}
	}

	if len(iterators) == 0 {
		return nil, nil
	}

	scoringSimScorer, err := newMultiNormsLeafSimScorer(w.simWeight, context.LeafReader(), w.query.orderedFieldAndWeights(), true)
	if err != nil {
		return nil, err
	}

	return &combinedFieldScorerSupplier{
		iterators: iterators,
		fields:    fields,
		simWeight: w.simWeight,
		simScorer: scoringSimScorer,
		cost:      cost,
	}, nil
}

// IsCacheable mirrors CombinedFieldWeight.isCacheable(LeafReaderContext).
func (w *combinedFieldWeight) IsCacheable(ctx *index.LeafReaderContext) bool {
	return false
}

// combinedFieldScorerSupplier is the anonymous ScorerSupplier returned by
// CombinedFieldWeight.scorerSupplier.
type combinedFieldScorerSupplier struct {
	BaseScorerSupplier
	iterators []index.PostingsEnum
	fields    []FieldAndWeight
	simWeight SimScorer
	simScorer *multiNormsLeafSimScorer
	cost      int64
}

// Get mirrors the anonymous ScorerSupplier.get(long).
func (s *combinedFieldScorerSupplier) Get(leadCost int64) (Scorer, error) {
	// we use termscorers + disjunction as an impl detail
	wrappers := make([]*DisiWrapper, 0, len(s.iterators))
	for i := 0; i < len(s.iterators); i++ {
		weight := s.fields[i].Weight
		scorer, err := NewTermScorer(s.iterators[i], s.simWeight, nil)
		if err != nil {
			return nil, err
		}
		w := newDisiWrapper(scorer, false, weight)
		if w.postingsEnum == nil { // needed to access term frequencies
			return nil, fmt.Errorf("CombinedFieldQuery: TermScorer iterator is not a PostingsEnum")
		}
		wrappers = append(wrappers, w)
	}
	// Even though it is called approximation, it is accurate since none of
	// the sub iterators are two-phase iterators.
	iterator := NewDisjunctionDISIApproximation(wrappers, leadCost)
	return newCombinedFieldScorer(iterator, s.simScorer), nil
}

// Cost mirrors the anonymous ScorerSupplier.cost().
func (s *combinedFieldScorerSupplier) Cost() int64 {
	return s.cost
}

// BulkScorer mirrors the anonymous ScorerSupplier.bulkScorer().
func (s *combinedFieldScorerSupplier) BulkScorer() (BulkScorer, error) {
	scorer, err := s.Get(math.MaxInt64)
	if err != nil {
		return nil, err
	}
	return NewBatchScoreBulkScorer(scorer), nil
}

// combinedFieldScorer mirrors the private static nested class
// CombinedFieldQuery.CombinedFieldScorer.
type combinedFieldScorer struct {
	BaseScorer
	iterator  *DisjunctionDISIApproximation
	simScorer *multiNormsLeafSimScorer
	maxScore  float32
}

// newCombinedFieldScorer mirrors
// CombinedFieldScorer(DisjunctionDISIApproximation, MultiNormsLeafSimScorer).
func newCombinedFieldScorer(iterator *DisjunctionDISIApproximation, simScorer *multiNormsLeafSimScorer) *combinedFieldScorer {
	return &combinedFieldScorer{
		iterator:  iterator,
		simScorer: simScorer,
		maxScore:  simScorer.getSimScorer().Score104(float32(math.Inf(1)), 1),
	}
}

// DocID mirrors CombinedFieldScorer.docID().
func (s *combinedFieldScorer) DocID() int {
	return s.iterator.DocID()
}

// freq mirrors the package-private CombinedFieldScorer.freq().
func (s *combinedFieldScorer) freq() (float32, error) {
	w := s.iterator.TopList()
	f, err := w.postingsEnum.Freq()
	if err != nil {
		return 0, err
	}
	freq := float32(f) * w.weight
	for w = w.next; w != nil; w = w.next {
		f, err := w.postingsEnum.Freq()
		if err != nil {
			return 0, err
		}
		freq += float32(f) * w.weight
		if freq < 0 { // overflow
			return float32(math.MaxInt32), nil
		}
	}
	return freq, nil
}

// Score mirrors CombinedFieldScorer.score().
func (s *combinedFieldScorer) Score() (float32, error) {
	freq, err := s.freq()
	if err != nil {
		return 0, err
	}
	return s.simScorer.score(s.iterator.DocID(), freq)
}

// Iterator mirrors CombinedFieldScorer.iterator().
func (s *combinedFieldScorer) Iterator() DocIdSetIterator {
	return s.iterator
}

// GetMaxScore mirrors CombinedFieldScorer.getMaxScore(int).
func (s *combinedFieldScorer) GetMaxScore(upTo int) (float32, error) {
	return s.maxScore, nil
}

// NextDocsAndScores mirrors
// CombinedFieldScorer.nextDocsAndScores(int, Bits, DocAndFloatFeatureBuffer).
func (s *combinedFieldScorer) NextDocsAndScores(upTo int, liveDocs util.Bits, buffer *DocAndFloatFeatureBuffer) error {
	batchSize := 64 // arbitrary
	buffer.GrowNoCopy(batchSize)
	size := 0
	iterator := s.Iterator()
	for doc := s.DocID(); doc < upTo && size < batchSize; {
		if liveDocs == nil || liveDocs.Get(doc) {
			freq, err := s.freq()
			if err != nil {
				return err
			}
			buffer.Docs[size] = doc
			buffer.Features[size] = freq
			size++
		}
		next, err := iterator.NextDoc()
		if err != nil {
			return err
		}
		doc = next
	}
	buffer.Size = size
	return s.simScorer.scoreRange(buffer)
}

// BulkScorer renders the final Weight.bulkScorer(LeafReaderContext):
// scorerSupplier(context), marked as the top-level scoring clause, supplies
// the bulk scorer; nil when no document matches. It is restated because the
// embedded BaseWeight.BulkScorer would call BaseWeight.ScorerSupplier, not
// this type's override.
func (w *combinedFieldWeight) BulkScorer(context *index.LeafReaderContext) (BulkScorer, error) {
	scorerSupplier, err := w.ScorerSupplier(context)
	if err != nil || scorerSupplier == nil {
		// No docs match
		return nil, err
	}
	if err := scorerSupplier.SetTopLevelScoringClause(); err != nil {
		return nil, err
	}
	return scorerSupplier.BulkScorer()
}
