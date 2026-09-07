package search

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// CombinedFieldQuery treats multiple fields as a single stream and scores terms as if they had
// been indexed in a single field whose values would be the union of the values of the provided
// fields.
type CombinedFieldQuery struct {
	fieldAndWeights map[string]float32
	term            util.BytesRef
	fieldTerms      []index.Term
	ramBytesUsed    int64
}

type fieldAndWeight struct {
	field  string
	weight float32
}

// Builder is a builder for CombinedFieldQuery.
type Builder struct {
	fieldAndWeights map[string]float32
	term            util.BytesRef
}

func NewBuilder(term string) *Builder {
	return &Builder{
		fieldAndWeights: make(map[string]float32),
		term:            util.BytesRefFromString(term),
	}
}

func NewBuilderWithBytes(term util.BytesRef) *Builder {
	return &Builder{
		fieldAndWeights: make(map[string]float32),
		term:            term.DeepCopy(),
	}
}

func (b *Builder) AddField(field string) *Builder {
	return b.AddFieldWithWeight(field, 1.0)
}

func (b *Builder) AddFieldWithWeight(field string, weight float32) *Builder {
	if weight < 1.0 {
		panic("weight must be greater or equal to 1")
	}
	b.fieldAndWeights[field] = weight
	return b
}

func (b *Builder) Build() *CombinedFieldQuery {
	if len(b.fieldAndWeights) > index.GetMaxClauseCount() {
		panic("too many clauses")
	}

	// Sort fields to ensure deterministic order
	fields := make([]string, 0, len(b.fieldAndWeights))
	for f := range b.fieldAndWeights {
		fields = append(fields, f)
	}
	sort.Strings(fields)

	fieldTerms := make([]index.Term, 0, len(fields))
	for _, f := range fields {
		fieldTerms = append(fieldTerms, index.NewTerm(f, b.term))
	}

	return &CombinedFieldQuery{
		fieldAndWeights: b.fieldAndWeights,
		term:            b.term,
		fieldTerms:      fieldTerms,
		ramBytesUsed:    0, // simplified RAM usage
	}
}

func (q *CombinedFieldQuery) String() string {
	var sb strings.Builder
	sb.WriteString("CombinedFieldQuery((")

	fields := make([]string, 0, len(q.fieldAndWeights))
	for f := range q.fieldAndWeights {
		fields = append(fields, f)
	}
	sort.Strings(fields)

	for i, f := range fields {
		if i > 0 {
			sb.WriteString(" ")
		}
		sb.WriteString(f)
		w := q.fieldAndWeights[f]
		if w != 1.0 {
			sb.WriteString("^")
			sb.WriteString(fmt.Sprintf("%g", w))
		}
	}
	sb.WriteString(")(")
	sb.WriteString(q.term.String())
	sb.WriteString("))")
	return sb.String()
}

func (q *CombinedFieldQuery) Equals(other Query) bool {
	if q == other {
		return true
	}
	that, ok := other.(*CombinedFieldQuery)
	if !ok {
		return false
	}
	if len(q.fieldAndWeights) != len(that.fieldAndWeights) {
		return false
	}
	for f, w := range q.fieldAndWeights {
		if that.fieldAndWeights[f] != w {
			return false
		}
	}
	return q.term.Equals(that.term)
}

func (q *CombinedFieldQuery) HashCode() int {
	result := 31 // class hash simplified
	fields := make([]string, 0, len(q.fieldAndWeights))
	for f := range q.fieldAndWeights {
		fields = append(fields, f)
	}
	sort.Strings(fields)
	for _, f := range fields {
		result = 31*result + int(q.fieldAndWeights[f])
	}
	result = 31*result + q.term.HashCode()
	return result
}

func (q *CombinedFieldQuery) RamBytesUsed() int64 {
	return q.ramBytesUsed
}

func (q *CombinedFieldQuery) Rewrite(searcher *index.IndexSearcher) Query {
	if len(q.fieldAndWeights) == 0 {
		return NewBooleanQuery()
	}
	return q
}

func (q *CombinedFieldQuery) Visit(visitor QueryVisitor) {
	var selectedTerms []index.Term
	for _, t := range q.fieldTerms {
		if visitor.AcceptField(t.Field()) {
			selectedTerms = append(selectedTerms, t)
		}
	}
	if len(selectedTerms) > 0 {
		v := visitor.GetSubVisitor(BooleanClauseOccurShould, q)
		v.ConsumeTerms(q, selectedTerms)
	}
}

func (q *CombinedFieldQuery) rewriteToBoolean() Query {
	bq := NewBooleanQueryBuilder()
	for _, t := range q.fieldTerms {
		bq.Add(NewTermQuery(t), BooleanClauseOccurShould)
	}
	return bq.Build()
}

func (q *CombinedFieldQuery) CreateWeight(searcher *index.IndexSearcher, scoreMode ScoreMode, boost float32) (Weight, error) {
	if err := q.validateConsistentNorms(searcher.GetIndexReader()); err != nil {
		return nil, err
	}
	if scoreMode.NeedsScores() {
		return NewCombinedFieldWeight(q, searcher, scoreMode, boost), nil
	}
	bq := q.rewriteToBoolean()
	rewritten, err := searcher.Rewrite(bq)
	if err != nil {
		return nil, err
	}
	return rewritten.CreateWeight(searcher, ScoreModeCompleteNoScores, boost)
}

func (q *CombinedFieldQuery) validateConsistentNorms(reader index.IndexReader) error {
	allFieldsHaveNorms := true
	noFieldsHaveNorms := true

	for _, context := range reader.Leaves() {
		fieldInfos := context.Reader().FieldInfos()
		for field := range q.fieldAndWeights {
			fieldInfo := fieldInfos.FieldInfo(field)
			if fieldInfo != nil {
				allFieldsHaveNorms = allFieldsHaveNorms && fieldInfo.HasNorms()
				noFieldsHaveNorms = noFieldsHaveNorms && fieldInfo.OmitsNorms()
			}
		}
	}

	if !allFieldsHaveNorms && !noFieldsHaveNorms {
		return fmt.Errorf("CombinedFieldQuery requires norms to be consistent across fields: some fields cannot have norms enabled, while others have norms disabled")
	}
	return nil
}

type combinedFieldWeight struct {
	query      *CombinedFieldQuery
	searcher   *index.IndexSearcher
	termStates []index.TermStates
	simWeight  SimilaritySimScorer
}

func NewCombinedFieldWeight(q *CombinedFieldQuery, searcher *index.IndexSearcher, scoreMode ScoreMode, boost float32) Weight {
	var docFreq int64
	var totalTermFreq float64
	termStates := make([]index.TermStates, len(q.fieldTerms))
	for i := 0; i < len(q.fieldTerms); i++ {
		ts := index.TermStatesBuild(searcher, q.fieldTerms[i], true)
		termStates[i] = ts
		if ts.DocFreq() > 0 {
			termStats := searcher.TermStatistics(q.fieldTerms[i], ts.DocFreq(), ts.TotalTermFreq())
			if termStats.DocFreq() > docFreq {
				docFreq = termStats.DocFreq()
			}
			weight := q.fieldAndWeights[q.fieldTerms[i].Field()]
			totalTermFreq += float64(weight) * float64(termStats.TotalTermFreq())
		}
	}

	var simWeight SimilaritySimScorer
	if docFreq > 0 {
		pseudoCollectionStats := mergeCollectionStatistics(q, searcher)
		pseudoTermStatistics := index.NewTermStatistics(util.BytesRefFromString("pseudo_term"), docFreq, int64(math.Max(1, totalTermFreq)))
		simWeight = searcher.GetSimilarity().Scorer(boost, pseudoCollectionStats, pseudoTermStatistics)
	}

	return &combinedFieldWeight{
		query:      q,
		searcher:   searcher,
		termStates: termStates,
		simWeight:  simWeight,
	}
}

func mergeCollectionStatistics(q *CombinedFieldQuery, searcher *index.IndexSearcher) index.CollectionStatistics {
	var maxDoc int64
	var docCount int64
	var sumTotalTermFreq float64
	var sumDocFreq int64
	for field, weight := range q.fieldAndWeights {
		collectionStats := searcher.CollectionStatistics(field)
		if collectionStats != nil {
			if collectionStats.MaxDoc() > maxDoc {
				maxDoc = collectionStats.MaxDoc()
			}
			if collectionStats.DocCount() > docCount {
				docCount = collectionStats.DocCount()
			}
			if collectionStats.SumDocFreq() > sumDocFreq {
				sumDocFreq = collectionStats.SumDocFreq()
			}
			sumTotalTermFreq += float64(weight) * float64(collectionStats.SumTotalTermFreq())
		}
	}

	return index.NewCollectionStatistics("pseudo_field", maxDoc, docCount, int64(sumTotalTermFreq), sumDocFreq)
}

func (w *combinedFieldWeight) Matches(context index.LeafReaderContext, doc int) (Matches, error) {
	weight, err := w.searcher.Rewrite(w.query.rewriteToBoolean()).CreateWeight(w.searcher, ScoreModeComplete, 1.0)
	if err != nil {
		return nil, err
	}
	return weight.Matches(context, doc)
}

func (w *combinedFieldWeight) Explain(context index.LeafReaderContext, doc int) (Explanation, error) {
	scorer, err := w.ScorerSupplier(context).Get(0)
	if err != nil {
		return nil, err
	}
	if scorer != nil {
		newDoc := scorer.Iterator().Advance(doc)
		if newDoc == doc {
			cfScorer, ok := scorer.(*combinedFieldScorer)
			if !ok {
				return nil, fmt.Errorf("unexpected scorer type")
			}
			freq := cfScorer.freq()
			docScorer := newMultiNormsLeafSimScorer(w.simWeight, context.Reader(), w.query.fieldAndWeights, true)
			freqExplanation := ExplanationMatch(freq, fmt.Sprintf("termFreq=%g", freq))
			scoreExplanation, err := docScorer.Explain(doc, freqExplanation)
			if err != nil {
				return nil, err
			}
			return ExplanationMatch(
				scoreExplanation.Value(),
				fmt.Sprintf("weight(%s in %d), result of:", w.query.String(), doc),
				scoreExplanation,
			), nil
		}
	}
	return ExplanationNoMatch("no matching term"), nil
}

func (w *combinedFieldWeight) ScorerSupplier(context index.LeafReaderContext) (ScorerSupplier, error) {
	var iterators []index.PostingsEnum
	var fields []fieldAndWeight
	var cost int64
	for i := 0; i < len(w.query.fieldTerms); i++ {
		state := w.termStates[i].Get(context)
		if state != nil {
			termsEnum := context.Reader().Terms(w.query.fieldTerms[i].Field()).Iterator()
			termsEnum.SeekExact(w.query.fieldTerms[i].Bytes(), state)
			postingsEnum := termsEnum.Postings(nil, index.PostingsEnumFreqs)
			iterators = append(iterators, postingsEnum)

			// Find the weight for this field
			weight := w.query.fieldAndWeights[w.query.fieldTerms[i].Field()]
			fields = append(fields, fieldAndWeight{w.query.fieldTerms[i].Field(), weight})
			cost += postingsEnum.Cost()
		}
	}

	if len(iterators) == 0 {
		return nil, nil
	}

	scoringSimScorer := newMultiNormsLeafSimScorer(w.simWeight, context.Reader(), w.query.fieldAndWeights, true)

	return &combinedFieldWeightScorerSupplier{
		iterators: iterators,
		fields:    fields,
		cost:      cost,
		simScorer: scoringSimScorer,
	}, nil
}

func (w *combinedFieldWeight) IsCacheable(ctx index.LeafReaderContext) bool {
	return false
}

type combinedFieldWeightScorerSupplier struct {
	iterators []index.PostingsEnum
	fields    []fieldAndWeight
	cost      int64
	simScorer *multiNormsLeafSimScorer
}

func (s *combinedFieldWeightScorerSupplier) Get(leadCost int64) (Scorer, error) {
	wrappers := make([]*disiWrapper, 0, len(s.iterators))
	for i := 0; i < len(s.iterators); i++ {
		weight := s.fields[i].weight
		scorer := NewTermScorer(s.iterators[i], s.simWeight, nil)
		w := newDisiWrapper(scorer, false, weight)
		wrappers = append(wrappers, w)
	}
	iterator := newDisjunctionDISIApproximation(wrappers, leadCost)
	return &combinedFieldScorer{
		iterator:  iterator,
		simScorer: s.simScorer,
	}, nil
}

func (s *combinedFieldWeightScorerSupplier) Cost() int64 {
	return s.cost
}

func (s *combinedFieldWeightScorerSupplier) BulkScorer() (BulkScorer, error) {
	scorer, err := s.Get(math.MaxInt64)
	if err != nil {
		return nil, err
	}
	return NewBatchScoreBulkScorer(scorer), nil
}

type combinedFieldScorer struct {
	iterator  *disjunctionDISIApproximation
	simScorer *multiNormsLeafSimScorer
	maxScore  float32
}

func newCombinedFieldScorer(iterator *disjunctionDISIApproximation, simScorer *multiNormsLeafSimScorer) *combinedFieldScorer {
	return &combinedFieldScorer{
		iterator:  iterator,
		simScorer: simScorer,
		maxScore:  simScorer.simScorer.Score(float32(math.Inf(1)), 1),
	}
}

func (s *combinedFieldScorer) DocID() int {
	return s.iterator.DocID()
}

func (s *combinedFieldScorer) freq() float32 {
	w := s.iterator.TopList()
	freq := float32(w.postingsEnum.Freq()) * w.weight
	for w = w.next; w != nil; w = w.next {
		freq += float32(w.postingsEnum.Freq()) * w.weight
		if freq < 0 {
			return float32(math.MaxInt32)
		}
	}
	return freq
}

func (s *combinedFieldScorer) Score() (float32, error) {
	return s.simScorer.Score(s.iterator.DocID(), s.freq()), nil
}

func (s *combinedFieldScorer) Iterator() DocIdSetIterator {
	return s.iterator
}

func (s *combinedFieldScorer) GetMaxScore(upTo int) (float32, error) {
	return s.maxScore, nil
}

func (s *combinedFieldScorer) NextDocsAndScores(upTo int, liveDocs *util.Bits, buffer *index.DocAndFloatFeatureBuffer) error {
	batchSize := 64
	buffer.GrowNoCopy(batchSize)
	size := 0
	iterator := s.iterator
	for doc := s.DocID(); doc < upTo && size < batchSize; doc = iterator.NextDoc() {
		if liveDocs == nil || liveDocs.Get(doc) {
			buffer.Docs[size] = doc
			buffer.Features[size] = s.freq()
			size++
		}
	}
	buffer.Size = size
	s.simScorer.ScoreRange(buffer)
	return nil
}

// --- Internal Helpers ---

type multiNormsLeafSimScorer struct {
	simScorer  SimilaritySimScorer
	bulkScorer SimilarityBulkSimScorer
	norms      index.NumericDocValues
	normValues []int64
}

var lengthTable = func() []float32 {
	t := make([]float32, 256)
	for i := 0; i < 256; i++ {
		t[i] = float32(util.SmallFloatByte4ToInt(byte(i)))
	}
	return t
}()

func newMultiNormsLeafSimScorer(scorer SimilaritySimScorer, reader index.LeafReader, normFields map[string]float32, needsScores bool) *multiNormsLeafSimScorer {
	var norms index.NumericDocValues
	if needsScores {
		var normsList []index.NumericDocValues
		var weightList []float32
		for field, weight := range normFields {
			nv := reader.GetNormValues(field)
			if nv != nil {
				normsList = append(normsList, nv)
				weightList = append(weightList, weight)
			}
		}

		if len(normsList) > 0 {
			norms = &multiFieldNormValues{
				normsArr:  normsList,
				weightArr: weightList,
			}
		}
	}

	return &multiNormsLeafSimScorer{
		simScorer:  scorer,
		bulkScorer: scorer.AsBulkSimScorer(),
		norms:      norms,
	}
}

func (m *multiNormsLeafSimScorer) getNormValue(doc int) (int64, error) {
	if m.norms != nil {
		found, err := m.norms.AdvanceExact(doc)
		if err != nil {
			return 0, err
		}
		if !found {
			return 1, nil // default
		}
		return m.norms.LongValue(), nil
	}
	return 1, nil
}

func (m *multiNormsLeafSimScorer) Score(doc int, freq float32) (float32, error) {
	norm, err := m.getNormValue(doc)
	if err != nil {
		return 0, err
	}
	return m.simScorer.Score(freq, norm), nil
}

func (m *multiNormsLeafSimScorer) ScoreRange(buffer *index.DocAndFloatFeatureBuffer) error {
	m.normValues = util.GrowNoCopy(m.normValues, buffer.Size)
	if m.norms != nil {
		m.norms.LongValues(buffer.Size, buffer.Docs, m.normValues, 1)
	} else {
		for i := 0; i < buffer.Size; i++ {
			m.normValues[i] = 1
		}
	}
	m.bulkScorer.Score(buffer.Size, buffer.Features, m.normValues, buffer.Features)
	return nil
}

func (m *multiNormsLeafSimScorer) Explain(doc int, freqExpl Explanation) (Explanation, error) {
	norm, err := m.getNormValue(doc)
	if err != nil {
		return nil, err
	}
	return m.simScorer.Explain(freqExpl, norm), nil
}

type multiFieldNormValues struct {
	normsArr  []index.NumericDocValues
	weightArr []float32
	accBuf    []float32
	current   int64
}

func (m *multiFieldNormValues) LongValue() int64 {
	return m.current
}

func (m *multiFieldNormValues) AdvanceExact(target int) (bool, error) {
	var normValue float32
	found := false
	for i := 0; i < len(m.normsArr); i++ {
		f, err := m.normsArr[i].AdvanceExact(target)
		if err != nil {
			return false, err
		}
		if f {
			normValue += m.weightArr[i] * lengthTable[byte(m.normsArr[i].LongValue())]
			found = true
		}
	}
	m.current = util.SmallFloatIntToByte4(int(math.Round(float64(normValue))))
	return found, nil
}

func (m *multiFieldNormValues) LongValues(size int, docs []int, values []int64, defaultValue int64) error {
	if len(m.accBuf) < size {
		m.accBuf = make([]float32, size)
	} else {
		for i := 0; i < size; i++ {
			m.accBuf[i] = 0
		}
	}

	for i := 0; i < len(m.normsArr); i++ {
		v := make([]int64, size)
		m.normsArr[i].LongValues(size, docs, v, 0)
		weight := m.weightArr[i]
		for j := 0; j < size; j++ {
			m.accBuf[j] += weight * lengthTable[byte(v[j])]
		}
	}

	for i := 0; i < size; i++ {
		if m.accBuf[i] == 0 {
			values[i] = defaultValue
		} else {
			values[i] = util.SmallFloatIntToByte4(int(math.Round(float64(m.accBuf[i]))))
		}
	}
	return nil
}

type disiWrapper struct {
	iterator     DocIdSetIterator
	postingsEnum index.PostingsEnum
	scorer       Scorer
	scorable     Scorable
	cost         int64
	matchCost    float32
	doc          int
	next         *disiWrapper
	approximation DocIdSetIterator
	twoPhaseView TwoPhaseIterator
	weight       float32
}

func newDisiWrapper(scorer Scorer, impacts bool, weight float32) *disiWrapper {
	var iter DocIdSetIterator
	if impacts {
		iter = ScorerUtilLikelyImpactsEnum(scorer.Iterator())
	} else {
		iter = scorer.Iterator()
	}

	var postingsEnum index.PostingsEnum
	if pe, ok := iter.(index.PostingsEnum); ok {
		postingsEnum = pe
	}

	cost := iter.Cost()
	var approximation DocIdSetIterator
	var twoPhaseView TwoPhaseIterator
	var matchCost float32

	if tpv := scorer.TwoPhaseIterator(); tpv != nil {
		twoPhaseView = tpv
		approximation = tpv.Approximation()
		matchCost = tpv.MatchCost()
	} else {
		approximation = iter
		matchCost = 0
	}

	return &disiWrapper{
		iterator:     iter,
		postingsEnum: postingsEnum,
		scorer:       scorer,
		scorable:     ScorerUtilLikelyTermScorer(scorer),
		cost:         cost,
		doc:          -1,
		twoPhaseView: twoPhaseView,
		approximation: approximation,
		matchCost:    matchCost,
		weight:       weight,
	}
}

type disjunctionDISIApproximation struct {
	leadIterators  disiPriorityQueue
	otherIterators []*disiWrapper
	cost           int64
	leadTop        *disiWrapper
	minOtherDoc    int
	doc            int
}

func newDisjunctionDISIApproximation(subIterators []*disiWrapper, leadCost int64) *disjunctionDISIApproximation {
	wrappers := make([]*disiWrapper, len(subIterators))
	copy(wrappers, subIterators)
	sort.Slice(wrappers, func(i, j int) bool {
		return wrappers[i].cost > wrappers[j].cost
	})

	reorderThreshold := leadCost + (leadCost >> 1)
	if reorderThreshold < 0 {
		reorderThreshold = math.MaxInt64
	}

	var totalCost int64
	var reorderCost int64
	lastIdx := len(wrappers) - 1
	for ; lastIdx >= 0; lastIdx-- {
		lastCost := wrappers[lastIdx].cost
		inc := lastCost
		if inc > leadCost {
			inc = leadCost
		}
		if reorderCost+inc < 0 || reorderCost+inc > reorderThreshold {
			break
		}
		reorderCost += inc
		totalCost += lastCost
	}

	if lastIdx == len(wrappers)-1 {
		totalCost += wrappers[lastIdx].cost
		lastIdx--
	}

	pqLen := len(wrappers) - lastIdx - 1
	pq := newDisiPriorityQueue(pqLen)
	for i := lastIdx + 1; i < len(wrappers); i++ {
		pq.Add(wrappers[i])
	}

	others := make([]*disiWrapper, 0, lastIdx+1)
	minOtherDoc := math.MaxInt32
	for i := 0; i <= lastIdx; i++ {
		others = append(others, wrappers[i])
		if wrappers[i].doc < minOtherDoc {
			minOtherDoc = wrappers[i].doc
		}
	}

	return &disjunctionDISIApproximation{
		leadIterators:  pq,
		otherIterators: others,
		cost:           totalCost,
		leadTop:        pq.Top(),
		minOtherDoc:    minOtherDoc,
		doc:            -1,
	}
}

func (d *disjunctionDISIApproximation) DocID() int {
	return d.doc
}

func (d *disjunctionDISIApproximation) NextDoc() int {
	if d.leadTop.doc < d.minOtherDoc {
		curDoc := d.leadTop.doc
		for {
			d.leadTop.doc = d.leadTop.approximation.NextDoc()
			d.leadTop = d.leadIterators.UpdateTop()
			if d.leadTop.doc != curDoc {
				break
			}
		}
		d.doc = d.leadTop.doc
		if d.minOtherDoc < d.doc {
			d.doc = d.minOtherDoc
		}
		return d.doc
	}
	return d.Advance(d.minOtherDoc + 1)
}

func (d *disjunctionDISIApproximation) Advance(target int) int {
	for d.leadTop.doc < target {
		d.leadTop.doc = d.leadTop.approximation.Advance(target)
		d.leadTop = d.leadIterators.UpdateTop()
	}

	d.minOtherDoc = math.MaxInt32
	for _, w := range d.otherIterators {
		if w.doc < target {
			w.doc = w.approximation.Advance(target)
		}
		if w.doc < d.minOtherDoc {
			d.minOtherDoc = w.doc
		}
	}

	d.doc = d.leadTop.doc
	if d.minOtherDoc < d.doc {
		d.doc = d.minOtherDoc
	}
	return d.doc
}

func (d *disjunctionDISIApproximation) IntoBitSet(upTo int, bitSet *util.Bits, offset int) {
	for d.leadTop.doc < upTo {
		d.leadTop.approximation.IntoBitSet(upTo, bitSet, offset)
		d.leadTop.doc = d.leadTop.approximation.DocID()
		d.leadTop = d.leadIterators.UpdateTop()
	}

	d.minOtherDoc = math.MaxInt32
	for _, w := range d.otherIterators {
		w.approximation.IntoBitSet(upTo, bitSet, offset)
		w.doc = w.approximation.DocID()
		if w.doc < d.minOtherDoc {
			d.minOtherDoc = w.doc
		}
	}

	d.doc = d.leadTop.doc
	if d.minOtherDoc < d.doc {
		d.doc = d.minOtherDoc
	}
}

func (d *disjunctionDISIApproximation) TopList() *disiWrapper {
	if d.leadTop.doc < d.minOtherDoc {
		return d.leadIterators.TopList()
	}

	var topList *disiWrapper
	if d.leadTop.doc == d.minOtherDoc {
		topList = d.leadIterators.TopList()
	}
	for _, w := range d.otherIterators {
		if w.doc == d.minOtherDoc {
			w.next = topList
			topList = w
		}
	}
	return topList
}

func (d *disjunctionDISIApproximation) DocIDRunEnd() int {
	maxDocIDRunEnd := d.doc // simplified base case
	if d.leadTop.doc == d.doc {
		for w := d.leadIterators.TopList(); w != nil; w = w.next {
			runEnd := w.approximation.DocIDRunEnd()
			if runEnd > maxDocIDRunEnd {
				maxDocIDRunEnd = runEnd
			}
		}
	}
	return maxDocIDRunEnd
}

func (d *disjunctionDISIApproximation) Cost() int64 {
	return d.cost
}

type disiPriorityQueue interface {
	Size() int
	Top() *disiWrapper
	Top2() *disiWrapper
	TopList() *disiWrapper
	Add(*disiWrapper) *disiWrapper
	Pop() *disiWrapper
	UpdateTop() *disiWrapper
	UpdateTopWith(*disiWrapper) *disiWrapper
	Clear()
}

func newDisiPriorityQueue(maxSize int) disiPriorityQueue {
	if maxSize <= 2 {
		return &disiPriorityQueue2{}
	}
	return &disiPriorityQueueN{heap: make([]*disiWrapper, maxSize)}
}

type disiPriorityQueue2 struct {
	top  *disiWrapper
	top2 *disiWrapper
}

func (q *disiPriorityQueue2) Size() int {
	if q.top2 != nil {
		return 2
	}
	if q.top != nil {
		return 1
	}
	return 0
}

func (q *disiPriorityQueue2) Top() *disiWrapper {
	return q.top
}

func (q *disiPriorityQueue2) Top2() *disiWrapper {
	return q.top2
}

func (q *disiPriorityQueue2) TopList() *disiWrapper {
	var topList *disiWrapper
	if q.top != nil {
		q.top.next = nil
		topList = q.top
		if q.top2 != nil && q.top.doc == q.top2.doc {
			q.top2.next = topList
			topList = q.top2
		}
	}
	return topList
}

func (q *disiPriorityQueue2) Add(entry *disiWrapper) *disiWrapper {
	if q.top == nil {
		q.top = entry
		return q.top
	} else if q.top2 == nil {
		q.top2 = entry
		return q.UpdateTop()
	}
	panic("trying to add a 3rd element to a DisiPriorityQueue configured with a max size of 2")
}

func (q *disiPriorityQueue2) Pop() *disiWrapper {
	ret := q.top
	q.top = q.top2
	q.top2 = nil
	return ret
}

func (q *disiPriorityQueue2) UpdateTop() *disiWrapper {
	if q.top2 != nil && q.top2.doc < q.top.doc {
		q.top, q.top2 = q.top2, q.top
	}
	return q.top
}

func (q *disiPriorityQueue2) UpdateTopWith(topReplacement *disiWrapper) *disiWrapper {
	q.top = topReplacement
	return q.UpdateTop()
}

func (q *disiPriorityQueue2) Clear() {
	q.top = nil
	q.top2 = nil
}

type disiPriorityQueueN struct {
	heap []*disiWrapper
	size int
}

func (q *disiPriorityQueueN) Size() int {
	return q.size
}

func (q *disiPriorityQueueN) Top() *disiWrapper {
	return q.heap[0]
}

func (q *disiPriorityQueueN) Top2() *disiWrapper {
	switch q.size {
	case 0, 1:
		return nil
	case 2:
		return q.heap[1]
	default:
		if q.heap[1].doc <= q.heap[2].doc {
			return q.heap[1]
		}
		return q.heap[2]
	}
}

func (q *disiPriorityQueueN) TopList() *disiWrapper {
	if q.size == 0 {
		return nil
	}
	list := q.heap[0]
	list.next = nil
	if q.size >= 3 {
		list = q.topListRec(list, 1)
		list = q.topListRec(list, 2)
	} else if q.size == 2 && q.heap[1].doc == list.doc {
		q.heap[1].next = list
		list = q.heap[1]
	}
	return list
}

func (q *disiPriorityQueueN) topListRec(list *disiWrapper, i int) *disiWrapper {
	w := q.heap[i]
	if w.doc == list.doc {
		w.next = list
		list = w
		left := ((i + 1) << 1) - 1
		right := left + 1
		if right < q.size {
			list = q.topListRec(list, left)
			list = q.topListRec(list, right)
		} else if left < q.size && q.heap[left].doc == list.doc {
			q.heap[left].next = list
			list = q.heap[left]
		}
	}
	return list
}

func (q *disiPriorityQueueN) Add(entry *disiWrapper) *disiWrapper {
	q.heap[q.size] = entry
	q.upHeap(q.size)
	q.size++
	return q.heap[0]
}

func (q *disiPriorityQueueN) Pop() *disiWrapper {
	result := q.heap[0]
	q.size--
	q.heap[0] = q.heap[q.size]
	q.heap[q.size] = nil
	q.downHeap(q.size)
	return result
}

func (q *disiPriorityQueueN) UpdateTop() *disiWrapper {
	q.downHeap(q.size)
	return q.heap[0]
}

func (q *disiPriorityQueueN) UpdateTopWith(topReplacement *disiWrapper) *disiWrapper {
	q.heap[0] = topReplacement
	return q.UpdateTop()
}

func (q *disiPriorityQueueN) Clear() {
	for i := range q.heap {
		q.heap[i] = nil
	}
	q.size = 0
}

func (q *disiPriorityQueueN) upHeap(i int) {
	node := q.heap[i]
	nodeDoc := node.doc
	j := ((i + 1) >>> 1) - 1
	for j >= 0 && nodeDoc < q.heap[j].doc {
		q.heap[i] = q.heap[j]
		i = j
		j = ((j + 1) >>> 1) - 1
	}
	q.heap[i] = node
}

func (q *disiPriorityQueueN) downHeap(size int) {
	i := 0
	node := q.heap[0]
	j := ((i + 1) << 1) - 1
	if j < size {
		k := j + 1
		if k < size && q.heap[k].doc < q.heap[j].doc {
			j = k
		}
		if q.heap[j].doc < node.doc {
			for {
				q.heap[i] = q.heap[j]
				i = j
				j = ((i + 1) << 1) - 1
				k = j + 1
				if j >= size {
					break
				}
				if k < size && q.heap[k].doc < q.heap[j].doc {
					j = k
				}
				if q.heap[j].doc >= node.doc {
					break
				}
			}
			q.heap[i] = node
		}
	}
}
