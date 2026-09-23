// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/index/TestCodecs.java
// (Apache Lucene 10.5.0).

package index_test

import (
	"bytes"
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"strconv"
	"sync"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/automaton"
)

var codecsFieldNames = []string{"one", "two", "three", "four"}

const (
	codecsNumTestThreads = 3
	codecsNumFields      = 4
	codecsNumTermsRand   = 50  // must be > 16 to test skipping
	codecsDocFreqRand    = 500 // must be > 16 to test skipping
	codecsTermDocFreq    = 20
	codecsSegment        = "0"
)

// codecsNumTestIter renders NUM_TEST_ITER, set by @BeforeClass to atLeast(20).
var codecsNumTestIter = atLeast(20)

// errUnsupportedOperation renders UnsupportedOperationException thrown by the
// data enums of this test.
var errUnsupportedOperation = errors.New("UnsupportedOperationException")

// termsEnumSeekExactOrdMissing names the TermsEnum member the Verify thread
// calls; the Java test tolerates an UnsupportedOperationException from it,
// but the Go TermsEnum does not declare it at all.
const termsEnumSeekExactOrdMissing = "org.apache.lucene.index.TermsEnum#seekExact(long) is not ported"

type codecsFieldData struct {
	fieldInfo     *index.FieldInfo
	terms         []*codecsTermData
	omitTF        bool
	storePayloads bool
}

// newCodecsFieldData renders the FieldData constructor, whose first statement
// reads FieldInfos.Builder#fieldInfo(String).
func newCodecsFieldData(t *testing.T, name string, fieldInfos *spi.FieldInfosBuilder, terms []*codecsTermData, omitTF, storePayloads bool) *codecsFieldData {
	t.Helper()
	t.Fatal("org.apache.lucene.index.FieldInfos.Builder#fieldInfo(String) is not ported")
	return nil
}

type codecsPositionData struct {
	pos     int
	payload []byte
}

type codecsTermData struct {
	text2     string
	text      *util.BytesRef
	docs      []int
	positions [][]codecsPositionData
	field     *codecsFieldData
}

func newCodecsTermData(text string, docs []int, positions [][]codecsPositionData) *codecsTermData {
	return &codecsTermData{text: util.NewBytesRef([]byte(text)), text2: text, docs: docs, positions: positions}
}

func codecsMakeRandomTerms(omitTF, storePayloads bool) []*codecsTermData {
	r := rand.New(rand.NewSource(rand.Int63()))
	numTerms := 1 + rand.Intn(codecsNumTermsRand)
	terms := make([]*codecsTermData, numTerms)

	termsSeen := map[string]bool{}

	for i := 0; i < numTerms; i++ {
		// Make term text
		var text2 string
		for {
			text2 = util.RandomUnicodeString(r, 20)
			if !termsSeen[text2] && !(len(text2) > 0 && text2[len(text2)-1] == '.') {
				termsSeen[text2] = true
				break
			}
		}

		docFreq := 1 + rand.Intn(codecsDocFreqRand)
		docs := make([]int, docFreq)
		var positions [][]codecsPositionData

		if !omitTF {
			positions = make([][]codecsPositionData, docFreq)
		}

		docID := 0
		for j := 0; j < docFreq; j++ {
			docID += nextInt(1, 10)
			docs[j] = docID

			if !omitTF {
				termFreq := 1 + rand.Intn(codecsTermDocFreq)
				positions[j] = make([]codecsPositionData, termFreq)
				position := 0
				for k := 0; k < termFreq; k++ {
					position += nextInt(1, 10)

					var payload []byte
					if storePayloads && rand.Intn(4) == 0 {
						payload = make([]byte, 1+rand.Intn(5))
						for l := range payload {
							payload[l] = byte(rand.Intn(255))
						}
					}

					positions[j][k] = codecsPositionData{pos: position, payload: payload}
				}
			}
		}

		terms[i] = newCodecsTermData(text2, docs, positions)
	}

	return terms
}

// codecsSegmentInfo renders new SegmentInfo(dir, Version.LATEST,
// Version.LATEST, SEGMENT, 10000, false, false, codec, emptyMap,
// StringHelper.randomId(), new HashMap<>(), null) through the setters.
func codecsSegmentInfo(t *testing.T, dir store.Directory, codec index.Codec) *index.SegmentInfo {
	t.Helper()
	si := index.NewSegmentInfo(codecsSegment, 10000, dir)
	si.SetVersion(util.Latest.String())
	si.SetMinVersion(util.Latest.String())
	si.SetUseCompoundFile(false)
	si.SetHasBlocks(false)
	si.SetCodec(codec)
	si.SetDiagnostics(map[string]string{})
	if err := si.SetID(util.RandomId()); err != nil {
		t.Fatalf("setID: %v", err)
	}
	si.SetAttributes(map[string]string{})
	return si
}

func TestCodecsFixedPostings(t *testing.T) {
	const numTerms = 100
	terms := make([]*codecsTermData, numTerms)
	for i := 0; i < numTerms; i++ {
		docs := []int{i}
		text := strconv.FormatInt(int64(i), 36)
		terms[i] = newCodecsTermData(text, docs, nil)
	}

	builder := spi.NewFieldInfosBuilder(spi.NewFieldNumbers("", ""))

	field := newCodecsFieldData(t, "field", builder, terms, true, false)
	fields := []*codecsFieldData{field}
	fieldInfos := builder.FieldInfos()
	dir := newDirectory()
	codec := index.GetDefaultCodec()
	si := codecsSegmentInfo(t, dir, codec)

	codecsWrite(t, si, fieldInfos, dir, fields)
	reader, err := codec.PostingsFormat().FieldsProducer(index.NewSegmentReadState(dir, si, fieldInfos, newIOContext()))
	if err != nil {
		t.Fatalf("fieldsProducer: %v", err)
	}

	fieldsEnum, err := reader.Iterator()
	if err != nil {
		t.Fatalf("iterator: %v", err)
	}
	fieldName, err := fieldsEnum.Next()
	if err != nil {
		t.Fatalf("fieldsEnum.next(): %v", err)
	}
	if fieldName == "" {
		t.Fatal("assertNotNull(fieldName)")
	}
	terms2, err := reader.Terms(fieldName)
	if err != nil {
		t.Fatalf("terms(%s): %v", fieldName, err)
	}
	if terms2 == nil {
		t.Fatal("assertNotNull(terms2)")
	}

	termsEnum, err := terms2.Iterator()
	if err != nil {
		t.Fatalf("terms.iterator(): %v", err)
	}

	for i := 0; i < numTerms; i++ {
		term, err := termsEnum.Next()
		if err != nil {
			t.Fatalf("termsEnum.next(): %v", err)
		}
		if term == nil {
			t.Fatalf("assertNotNull(term) at %d", i)
		}
		if got := term.Bytes.String(); got != terms[i].text2 {
			t.Fatalf("term %d: expected %q, got %q", i, terms[i].text2, got)
		}

		// do this twice to stress test the codec's reuse, ie,
		// make sure it properly fully resets (rewinds) its
		// internal state:
		for iter := 0; iter < 2; iter++ {
			postingsEnum, err := testUtilDocs(termsEnum, spi.PostingsFlagNone)
			if err != nil {
				t.Fatalf("docs: %v", err)
			}
			if doc, err := postingsEnum.NextDoc(); err != nil || doc != terms[i].docs[0] {
				t.Fatalf("nextDoc: expected %d, got %d (%v)", terms[i].docs[0], doc, err)
			}
			if doc, err := postingsEnum.NextDoc(); err != nil || doc != spi.NO_MORE_DOCS {
				t.Fatalf("nextDoc: expected NO_MORE_DOCS, got %d (%v)", doc, err)
			}
		}
	}
	if term, err := termsEnum.Next(); err != nil || term != nil {
		t.Fatalf("assertNull(termsEnum.next()): got %v (%v)", term, err)
	}

	for i := 0; i < numTerms; i++ {
		if status := codecsSeekCeil(t, termsEnum, terms[i].text2); status != spi.SeekStatusFound {
			t.Fatalf("seekCeil(%q): expected FOUND, got %v", terms[i].text2, status)
		}
	}

	if fieldsEnum.HasNext() {
		t.Fatal("assertFalse(fieldsEnum.hasNext())")
	}
	mustClose(t, reader, dir)
}

// codecsSeekCeil renders TermsEnum.seekCeil(BytesRef) returning SeekStatus:
// Gocene's SeekCeil returns the term landed on (nil at END).
func codecsSeekCeil(t testing.TB, termsEnum index.TermsEnum, text string) spi.SeekStatus {
	t.Helper()
	target := spi.NewTerm("", text)
	got, err := termsEnum.SeekCeil(target)
	if err != nil {
		t.Fatalf("seekCeil(%q): %v", text, err)
	}
	if got == nil {
		return spi.SeekStatusEnd
	}
	if bytes.Equal(got.Bytes.ValidBytes(), target.Bytes.ValidBytes()) {
		return spi.SeekStatusFound
	}
	return spi.SeekStatusNotFound
}

func TestCodecsRandomPostings(t *testing.T) {
	builder := spi.NewFieldInfosBuilder(spi.NewFieldNumbers("", ""))

	fields := make([]*codecsFieldData, codecsNumFields)
	for i := 0; i < codecsNumFields; i++ {
		omitTF := 0 == i%3
		storePayloads := 1 == i%3
		fields[i] = newCodecsFieldData(t, codecsFieldNames[i], builder, codecsMakeRandomTerms(omitTF, storePayloads), omitTF, storePayloads)
	}

	dir := newDirectory()
	fieldInfos := builder.FieldInfos()

	codec := index.GetDefaultCodec()
	si := codecsSegmentInfo(t, dir, codec)
	codecsWrite(t, si, fieldInfos, dir, fields)

	terms, err := codec.PostingsFormat().FieldsProducer(index.NewSegmentReadState(dir, si, fieldInfos, newIOContext()))
	if err != nil {
		t.Fatalf("fieldsProducer: %v", err)
	}

	var wg sync.WaitGroup
	threads := make([]*codecsVerify, codecsNumTestThreads-1)
	for i := range threads {
		threads[i] = &codecsVerify{t: t, fields: fields, termsDict: terms}
		wg.Add(1)
		go func(v *codecsVerify) {
			defer wg.Done()
			v.run()
		}(threads[i])
	}

	(&codecsVerify{t: t, fields: fields, termsDict: terms}).run()

	wg.Wait()
	for _, v := range threads {
		if util.AssertsEnabled() && !(!v.failed) {
			panic(util.NewAssertionError("verify thread failed"))
		}
	}

	mustClose(t, terms, dir)
}

// codecsVerify is the private Verify thread.
type codecsVerify struct {
	t         *testing.T
	termsDict index.Fields
	fields    []*codecsFieldData
	failed    bool
}

func (v *codecsVerify) run() {
	if err := v.runInternal(); err != nil {
		v.failed = true
		v.t.Errorf("Verify: %v", err)
	}
}

func (v *codecsVerify) verifyDocs(docs []int, positions [][]codecsPositionData, postingsEnum index.PostingsEnum, doPos bool) error {
	for i := range docs {
		doc, err := postingsEnum.NextDoc()
		if err != nil {
			return err
		}
		if doc == spi.NO_MORE_DOCS {
			return fmt.Errorf("assertTrue(doc != NO_MORE_DOCS) at %d", i)
		}
		if docs[i] != doc {
			return fmt.Errorf("doc %d: expected %d, got %d", i, docs[i], doc)
		}
		if doPos {
			if err := v.verifyPositions(positions[i], postingsEnum); err != nil {
				return err
			}
		}
	}
	if doc, err := postingsEnum.NextDoc(); err != nil || doc != spi.NO_MORE_DOCS {
		return fmt.Errorf("expected NO_MORE_DOCS, got %d (%v)", doc, err)
	}
	return nil
}

func (v *codecsVerify) verifyPositions(positions []codecsPositionData, posEnum index.PostingsEnum) error {
	for i := range positions {
		pos, err := posEnum.NextPosition()
		if err != nil {
			return err
		}
		if positions[i].pos != pos {
			return fmt.Errorf("position %d: expected %d, got %d", i, positions[i].pos, pos)
		}
		payload, err := posEnum.GetPayload()
		if err != nil {
			return err
		}
		if positions[i].payload != nil {
			if payload == nil {
				return errors.New("assertNotNull(posEnum.getPayload())")
			}
			if rand.Intn(3) < 2 {
				// Verify the payload bytes
				if !bytes.Equal(positions[i].payload, payload) {
					return fmt.Errorf("expected=%v got=%v", positions[i].payload, payload)
				}
			}
		} else if payload != nil {
			return fmt.Errorf("assertNull(posEnum.getPayload()): got %v", payload)
		}
	}
	return nil
}

func (v *codecsVerify) postingsFor(field *codecsFieldData, termsEnum index.TermsEnum) (index.PostingsEnum, error) {
	if field.omitTF {
		return testUtilDocs(termsEnum, spi.PostingsFlagNone)
	}
	return termsEnum.Postings(spi.PostingsFlagAll)
}

func (v *codecsVerify) runInternal() error {
	for iter := 0; iter < codecsNumTestIter; iter++ {
		field := v.fields[rand.Intn(len(v.fields))]
		terms, err := v.termsDict.Terms(field.fieldInfo.Name())
		if err != nil {
			return err
		}
		termsEnum, err := terms.Iterator()
		if err != nil {
			return err
		}

		upto := 0
		// Test straight enum of the terms:
		for {
			term, err := termsEnum.Next()
			if err != nil {
				return err
			}
			if term == nil {
				break
			}
			expected := []byte(field.terms[upto].text2)
			upto++
			if !bytes.Equal(expected, term.Bytes.ValidBytes()) {
				return fmt.Errorf("expected=%q vs actual %q", expected, term.Bytes.ValidBytes())
			}
		}
		if upto != len(field.terms) {
			return fmt.Errorf("term count: expected %d, got %d", len(field.terms), upto)
		}

		// Test random seek:
		term := field.terms[rand.Intn(len(field.terms))]
		status := codecsSeekCeil(v.t, termsEnum, term.text2)
		if status != spi.SeekStatusFound {
			return fmt.Errorf("seekCeil(%q): expected FOUND, got %v", term.text2, status)
		}
		docFreq, err := termsEnum.DocFreq()
		if err != nil {
			return err
		}
		if len(term.docs) != docFreq {
			return fmt.Errorf("docFreq: expected %d, got %d", len(term.docs), docFreq)
		}
		postings, err := v.postingsFor(field, termsEnum)
		if err != nil {
			return err
		}
		if err := v.verifyDocs(term.docs, term.positions, postings, !field.omitTF); err != nil {
			return err
		}

		// Test random seek by ord:
		return errors.New(termsEnumSeekExactOrdMissing)
	}
	return nil
}

// codecsDataFields is the private DataFields.
type codecsDataFields struct {
	fields []*codecsFieldData
}

type codecsFieldIterator struct {
	fields []*codecsFieldData
	upto   int
}

func (it *codecsFieldIterator) HasNext() bool { return it.upto+1 < len(it.fields) }

func (it *codecsFieldIterator) Next() (string, error) {
	it.upto++
	return it.fields[it.upto].fieldInfo.Name(), nil
}

func (f *codecsDataFields) Iterator() (index.FieldIterator, error) {
	return &codecsFieldIterator{fields: f.fields, upto: -1}, nil
}

func (f *codecsDataFields) Terms(field string) (index.Terms, error) {
	// Slow linear search:
	for _, fieldData := range f.fields {
		if fieldData.fieldInfo.Name() == field {
			return &codecsDataTerms{fieldData: fieldData}, nil
		}
	}
	return nil, nil
}

func (f *codecsDataFields) Size() int { return len(f.fields) }

// codecsDataTerms is the private DataTerms. The members Terms declares
// concretely in Java (intersect, getMin, getMax) and the Go-only members of
// the Terms interface are not reached by FieldsConsumer.write; they answer
// with UnsupportedOperationException.
type codecsDataTerms struct {
	fieldData *codecsFieldData
}

func (d *codecsDataTerms) Field() string { return d.fieldData.fieldInfo.Name() }

func (d *codecsDataTerms) Iterator() (index.TermsEnum, error) {
	return &codecsDataTermsEnum{fieldData: d.fieldData, upto: -1}, nil
}

func (d *codecsDataTerms) GetIteratorWithSeek(*index.Term) (index.TermsEnum, error) {
	return nil, errUnsupportedOperation
}

func (d *codecsDataTerms) Intersect(*automaton.CompiledAutomaton, *index.Term) (index.TermsEnum, error) {
	return nil, errUnsupportedOperation
}

func (d *codecsDataTerms) GetPostingsReader(string, int) (index.PostingsEnum, error) {
	return nil, errUnsupportedOperation
}

func (d *codecsDataTerms) Size() int64 { panic(errUnsupportedOperation) }

func (d *codecsDataTerms) GetSumTotalTermFreq() (int64, error) { return 0, errUnsupportedOperation }

func (d *codecsDataTerms) GetSumDocFreq() (int64, error) { return 0, errUnsupportedOperation }

func (d *codecsDataTerms) GetDocCount() (int, error) { return 0, errUnsupportedOperation }

func (d *codecsDataTerms) HasFreqs() bool {
	return d.fieldData.fieldInfo.GetIndexOptions() >= index.IndexOptionsDocsAndFreqs
}

func (d *codecsDataTerms) HasOffsets() bool {
	return d.fieldData.fieldInfo.GetIndexOptions() >= index.IndexOptionsDocsAndFreqsAndPositionsAndOffsets
}

func (d *codecsDataTerms) HasPositions() bool {
	return d.fieldData.fieldInfo.GetIndexOptions() >= index.IndexOptionsDocsAndFreqsAndPositions
}

func (d *codecsDataTerms) HasPayloads() bool { return d.fieldData.fieldInfo.HasPayloads() }

func (d *codecsDataTerms) GetMin() (*index.Term, error) { return nil, errUnsupportedOperation }

func (d *codecsDataTerms) GetMax() (*index.Term, error) { return nil, errUnsupportedOperation }

// codecsDataTermsEnum is the private DataTermsEnum (extends BaseTermsEnum).
type codecsDataTermsEnum struct {
	index.BaseTermsEnum
	fieldData *codecsFieldData
	upto      int
}

func (e *codecsDataTermsEnum) Next() (*index.Term, error) {
	e.upto++
	if e.upto == len(e.fieldData.terms) {
		return nil, nil
	}
	return e.Term(), nil
}

func (e *codecsDataTermsEnum) Term() *index.Term {
	return spi.NewTermFromBytesRef(e.fieldData.fieldInfo.Name(), e.fieldData.terms[e.upto].text)
}

func (e *codecsDataTermsEnum) SeekCeil(text *index.Term) (*index.Term, error) {
	// Stupid linear impl:
	for i, term := range e.fieldData.terms {
		cmp := term.text.BytesRefCompareTo(text.Bytes)
		if cmp >= 0 {
			e.upto = i
			return e.Term(), nil
		}
	}
	return nil, nil
}

func (e *codecsDataTermsEnum) SeekExact(text *index.Term) (bool, error) {
	return index.SeekExactDelegated(e, text)
}

func (e *codecsDataTermsEnum) Ord() int64 { panic(errUnsupportedOperation) }

func (e *codecsDataTermsEnum) DocFreq() (int, error) { return 0, errUnsupportedOperation }

func (e *codecsDataTermsEnum) TotalTermFreq() (int64, error) { return 0, errUnsupportedOperation }

func (e *codecsDataTermsEnum) Postings(int) (index.PostingsEnum, error) {
	return &codecsDataPostingsEnum{termData: e.fieldData.terms[e.upto], docUpto: -1}, nil
}

func (e *codecsDataTermsEnum) PostingsWithLiveDocs(util.Bits, int) (index.PostingsEnum, error) {
	return nil, errUnsupportedOperation
}

func (e *codecsDataTermsEnum) Impacts(int) (index.ImpactsEnum, error) {
	return nil, errUnsupportedOperation
}

// codecsDataPostingsEnum is the private DataPostingsEnum.
type codecsDataPostingsEnum struct {
	termData *codecsTermData
	docUpto  int
	posUpto  int
}

func (p *codecsDataPostingsEnum) Cost() int64 { panic(errUnsupportedOperation) }

func (p *codecsDataPostingsEnum) NextDoc() (int, error) {
	p.docUpto++
	if p.docUpto == len(p.termData.docs) {
		return spi.NO_MORE_DOCS, nil
	}
	p.posUpto = -1
	return p.DocID(), nil
}

func (p *codecsDataPostingsEnum) DocID() int {
	if p.docUpto >= len(p.termData.docs) {
		// Java indexes past the end here and throws; NO_MORE_DOCS is what the
		// callers observe from nextDoc() before reaching it.
		return spi.NO_MORE_DOCS
	}
	return p.termData.docs[p.docUpto]
}

func (p *codecsDataPostingsEnum) Advance(target int) (int, error) {
	// Slow linear impl:
	if _, err := p.NextDoc(); err != nil {
		return 0, err
	}
	for p.DocID() < target {
		if _, err := p.NextDoc(); err != nil {
			return 0, err
		}
	}
	return p.DocID(), nil
}

func (p *codecsDataPostingsEnum) Freq() (int, error) {
	return len(p.termData.positions[p.docUpto]), nil
}

func (p *codecsDataPostingsEnum) NextPosition() (int, error) {
	p.posUpto++
	return p.termData.positions[p.docUpto][p.posUpto].pos, nil
}

func (p *codecsDataPostingsEnum) GetPayload() ([]byte, error) {
	return p.termData.positions[p.docUpto][p.posUpto].payload, nil
}

func (p *codecsDataPostingsEnum) StartOffset() (int, error) { return 0, errUnsupportedOperation }

func (p *codecsDataPostingsEnum) EndOffset() (int, error) { return 0, errUnsupportedOperation }

func (p *codecsDataPostingsEnum) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return util.DefaultIntoBitSet(p, upTo, bitSet, offset)
}

func (p *codecsDataPostingsEnum) DocIDRunEnd() (int, error) { return util.DefaultDocIDRunEnd(p) }

// codecsFakeNorms is the anonymous NormsProducer of write(...): every
// document has norm 1.
type codecsFakeNorms struct {
	si *index.SegmentInfo
}

func (n *codecsFakeNorms) Close() error { return nil }

// GetMergeInstance carries NormsProducer#getMergeInstance(): return this.
func (n *codecsFakeNorms) GetMergeInstance() index.NormsProducer { return n }
func (n *codecsFakeNorms) CheckIntegrity() error                 { return nil }

func (n *codecsFakeNorms) GetNorms(*index.FieldInfo) (index.NumericDocValues, error) {
	return &codecsFakeNormValues{si: n.si, doc: -1}, nil
}

type codecsFakeNormValues struct {
	si  *index.SegmentInfo
	doc int
}

func (v *codecsFakeNormValues) NextDoc() (int, error) { return v.Advance(v.doc + 1) }
func (v *codecsFakeNormValues) DocID() int            { return v.doc }
func (v *codecsFakeNormValues) Cost() int64           { return int64(v.si.MaxDoc()) }

func (v *codecsFakeNormValues) Advance(target int) (int, error) {
	if target >= v.si.MaxDoc() {
		v.doc = spi.NO_MORE_DOCS
	} else {
		v.doc = target
	}
	return v.doc, nil
}

func (v *codecsFakeNormValues) AdvanceExact(target int) (bool, error) {
	v.doc = target
	return true, nil
}

func (v *codecsFakeNormValues) LongValue() (int64, error) { return 1, nil }

func (v *codecsFakeNormValues) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return util.DefaultIntoBitSet(v, upTo, bitSet, offset)
}

func (v *codecsFakeNormValues) DocIDRunEnd() (int, error) { return util.DefaultDocIDRunEnd(v) }

// codecsWrite renders the private write(SegmentInfo, FieldInfos, Directory,
// FieldData[]).
func codecsWrite(t *testing.T, si *index.SegmentInfo, fieldInfos *index.FieldInfos, dir store.Directory, fields []*codecsFieldData) {
	t.Helper()
	codec := si.Codec()
	state := index.NewSegmentWriteState(dir, si, fieldInfos, nil, newIOContext())

	sort.Slice(fields, func(i, j int) bool { return fields[i].fieldInfo.Name() < fields[j].fieldInfo.Name() })
	consumer, err := codec.PostingsFormat().FieldsConsumer(state)
	if err != nil {
		t.Fatalf("fieldsConsumer: %v", err)
	}
	fakeNorms := &codecsFakeNorms{si: si}
	if err := consumer.Write(&codecsDataFields{fields: fields}, fakeNorms); err != nil {
		closeErr := consumer.Close()
		t.Fatalf("write: %v (close: %v)", err, closeErr)
	}
	if err := consumer.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
}

func TestCodecsDocsOnlyFreq(t *testing.T) {
	// tests that when fields are indexed with DOCS_ONLY, the Codec
	// returns 1 in docsEnum.freq()
	dir := newDirectory()
	writer := mustNewIndexWriter(t, dir, newIndexWriterConfigWithAnalyzer(newMockAnalyzer()))
	// we don't need many documents to assert this, but don't use one document either
	numDocs := atLeast(50)
	for i := 0; i < numDocs; i++ {
		doc := document.NewDocument()
		doc.Add(newStringField(t, "f", "doc", false))
		mustAddDocument(t, writer, doc)
	}
	mustClose(t, writer)

	term := spi.NewTermFromBytes("f", []byte("doc"))
	reader := mustOpenDirectoryReader(t, dir)
	leaves, err := reader.Leaves()
	if err != nil {
		t.Fatalf("leaves: %v", err)
	}
	for _, ctx := range leaves {
		de, err := ctx.LeafReader().Postings(*term, spi.PostingsFlagFreqs)
		if err != nil {
			t.Fatalf("postings: %v", err)
		}
		for {
			doc, err := de.NextDoc()
			if err != nil {
				t.Fatalf("nextDoc: %v", err)
			}
			if doc == spi.NO_MORE_DOCS {
				break
			}
			if freq, err := de.Freq(); err != nil || freq != 1 {
				t.Fatalf("wrong freq for doc %d: %d (%v)", de.DocID(), freq, err)
			}
		}
	}
	mustClose(t, reader, dir)
}
