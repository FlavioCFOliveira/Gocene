// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/index/TestPayloads.java
// (Apache Lucene 10.5.0).

package index_test

import (
	"fmt"
	"io"
	"math"
	"strings"
	"sync"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
	testanalysis "github.com/FlavioCFOliveira/Gocene/tests/analysis"
	"github.com/FlavioCFOliveira/Gocene/util"
)

const multiTermsGetTermPostingsEnumMissing = "org.apache.lucene.index.MultiTerms#getTermPostingsEnum(IndexReader, String, BytesRef) is not ported"

const fieldSetTokenStreamMissing = "org.apache.lucene.document.Field#setTokenStream(TokenStream) is not ported"

// Simple tests to test the Payload class
func TestPayloadsPayload(t *testing.T) {
	payload := util.NewBytesRef([]byte("This is a test!"))
	if payload.Length != len("This is a test!") {
		t.Fatalf("Wrong payload length.: %d", payload.Length)
	}

	clone := payload.Clone()
	if payload.Length != clone.Length {
		t.Fatalf("clone length: expected %d, got %d", payload.Length, clone.Length)
	}
	for i := 0; i < payload.Length; i++ {
		if payload.Bytes[i+payload.Offset] != clone.Bytes[i+clone.Offset] {
			t.Fatalf("byte %d differs", i)
		}
	}
}

func assertHasPayloads(t testing.TB, expected bool, fi *index.FieldInfos, field string) {
	t.Helper()
	info := fi.FieldInfo(field)
	if info == nil {
		t.Fatalf("fieldInfo(%q) is null", field)
	}
	if info.HasPayloads() != expected {
		if expected {
			t.Fatalf("Payload field bit should be set: %s", field)
		}
		t.Fatalf("Payload field bit should not be set: %s", field)
	}
}

// Tests whether the DocumentWriter and SegmentMerger correctly enable the
// payload bit in the FieldInfo
func TestPayloadsPayloadFieldBit(t *testing.T) {
	ram := newDirectory()
	analyzer := newPayloadAnalyzer()
	writer := mustNewIndexWriter(t, ram, newIndexWriterConfigWithAnalyzer(analyzer))
	d := document.NewDocument()
	// this field won't have any payloads
	d.Add(newTextField(t, "f1", "This field has no payloads", false))
	// this field will have payloads in all docs, however not for all term
	// positions, so this field is used to check if the DocumentWriter
	// correctly enables the payloads bit even if only some term positions
	// have payloads
	d.Add(newTextField(t, "f2", "This field has payloads in all docs", false))
	d.Add(newTextField(t, "f2", "This field has payloads in all docs NO PAYLOAD", false))
	// this field is used to verify if the SegmentMerger enables payloads for
	// a field if it has payloads enabled in only some documents
	d.Add(newTextField(t, "f3", "This field has payloads in some docs", false))
	// only add payload data for field f2
	analyzer.setPayloadData("f2", []byte("somedata"), 0, 1)
	mustAddDocument(t, writer, d)
	// flush
	mustClose(t, writer)

	dr := mustOpenDirectoryReader(t, ram)
	reader := getOnlyLeafReader(t, dr)
	fi := reader.GetFieldInfos()
	assertHasPayloads(t, false, fi, "f1")
	assertHasPayloads(t, true, fi, "f2")
	assertHasPayloads(t, false, fi, "f3")
	mustClose(t, dr)

	// now we add another document which has payloads for field f3 and verify
	// if the SegmentMerger enabled payloads for that field
	analyzer = newPayloadAnalyzer() // Clear payload state for each field
	conf := newIndexWriterConfigWithAnalyzer(analyzer)
	conf.SetOpenMode(index.Create)
	writer = mustNewIndexWriter(t, ram, conf)
	d = document.NewDocument()
	d.Add(newTextField(t, "f1", "This field has no payloads", false))
	d.Add(newTextField(t, "f2", "This field has payloads in all docs", false))
	d.Add(newTextField(t, "f2", "This field has payloads in all docs", false))
	d.Add(newTextField(t, "f3", "This field has payloads in some docs", false))
	// add payload data for field f2 and f3
	analyzer.setPayloadData("f2", []byte("somedata"), 0, 1)
	analyzer.setPayloadData("f3", []byte("somedata"), 0, 3)
	mustAddDocument(t, writer, d)

	// force merge
	if err := writer.ForceMerge(1); err != nil {
		t.Fatalf("forceMerge: %v", err)
	}
	// flush
	mustClose(t, writer)

	dr = mustOpenDirectoryReader(t, ram)
	reader = getOnlyLeafReader(t, dr)
	fi = reader.GetFieldInfos()
	assertHasPayloads(t, false, fi, "f1")
	assertHasPayloads(t, true, fi, "f2")
	assertHasPayloads(t, true, fi, "f3")
	mustClose(t, dr, ram)
}

// Tests if payloads are correctly stored and loaded.
func TestPayloadsPayloadsEncoding(t *testing.T) {
	dir := newDirectory()
	payloadsPerformTest(t, dir)
	mustClose(t, dir)
}

// payloadsPerformTest builds an index with payloads in the given Directory
// and performs different tests to verify the payload encoding
func payloadsPerformTest(t *testing.T, dir store.Directory) {
	t.Helper()
	analyzer := newPayloadAnalyzer()
	conf := newIndexWriterConfigWithAnalyzer(analyzer)
	conf.SetOpenMode(index.Create)
	conf.SetMergePolicy(newLogMergePolicy())
	writer := mustNewIndexWriter(t, dir, conf)

	// should be in sync with value in TermInfosWriter
	const skipInterval = 16

	const numTerms = 5
	const fieldName = "f1"

	numDocs := skipInterval + 1
	// create content for the test documents with just a few terms
	terms := payloadsGenerateTerms(fieldName, numTerms)
	var sb strings.Builder
	for _, term := range terms {
		sb.WriteString(term.Text())
		sb.WriteString(" ")
	}
	content := sb.String()

	payloadDataLength := numTerms*numDocs*2 + numTerms*numDocs*(numDocs-1)/2
	payloadData := payloadsGenerateRandomData(payloadDataLength)

	d := document.NewDocument()
	d.Add(newTextField(t, fieldName, content, false))
	// add the same document multiple times to have the same payload lengths
	// for all occurrences within two consecutive skip intervals
	offset := 0
	for i := 0; i < 2*numDocs; i++ {
		analyzer.setPayloadData(fieldName, payloadData, offset, 1)
		offset += numTerms
		mustAddDocument(t, writer, d)
	}

	// make sure we create more than one segment to test merging
	mustCommit(t, writer)

	// now we make sure to have different payload lengths next at the next
	// skip point
	for i := 0; i < numDocs; i++ {
		analyzer.setPayloadData(fieldName, payloadData, offset, i)
		offset += i * numTerms
		mustAddDocument(t, writer, d)
	}

	if err := writer.ForceMerge(1); err != nil {
		t.Fatalf("forceMerge: %v", err)
	}
	// flush
	mustClose(t, writer)

	// Verify the index: first we test if all payloads are stored correctly
	reader := mustOpenDirectoryReader(t, dir)
	defer mustClose(t, reader)
	t.Fatal(multiTermsGetTermPostingsEnumMissing)
}

// payloadsGenerateRandomDataInto renders the private
// generateRandomData(byte[]): this test needs the random data to be valid
// unicode.
func payloadsGenerateRandomDataInto(data []byte) {
	s := randomFixedByteLengthUnicodeString(len(data))
	b := []byte(s)
	if util.AssertsEnabled() && !(len(b) == len(data)) {
		panic(util.NewAssertionError(fmt.Sprintf("generated %d bytes, want %d", len(b), len(data))))
	}
	copy(data, b)
}

func payloadsGenerateRandomData(n int) []byte {
	data := make([]byte, n)
	payloadsGenerateRandomDataInto(data)
	return data
}

// payloadsGenerateTerms renders the private generateTerms(String, int).
// Math.log(0) is -Infinity in Java, so the int cast of the zero-index
// quotient saturates at Integer.MIN_VALUE; the int subtraction then wraps to a
// negative count and no zero is prepended. The arithmetic is int32 here to
// reproduce that wrap.
func payloadsGenerateTerms(fieldName string, n int) []*index.Term {
	maxDigits := int(math.Log(float64(n)) / math.Log(10))
	terms := make([]*index.Term, n)
	var sb strings.Builder
	for i := 0; i < n; i++ {
		sb.Reset()
		sb.WriteString("t")
		zeros := int(int32(maxDigits) - int32(javaDoubleToInt(math.Log(float64(i))/math.Log(10))))
		for j := 0; j < zeros; j++ {
			sb.WriteString("0")
		}
		fmt.Fprintf(&sb, "%d", i)
		terms[i] = index.NewTerm(fieldName, sb.String())
	}
	return terms
}

// javaDoubleToInt renders Java's (int) narrowing of a double: NaN becomes 0
// and out-of-range values saturate.
func javaDoubleToInt(v float64) int {
	switch {
	case math.IsNaN(v):
		return 0
	case v >= math.MaxInt32:
		return math.MaxInt32
	case v <= math.MinInt32:
		return math.MinInt32
	default:
		return int(v)
	}
}

// payloadData is the private PayloadData.
type payloadData struct {
	data   []byte
	offset int
	length int
}

// payloadAnalyzer is the private PayloadAnalyzer: an Analyzer using a
// MockTokenizer and a PayloadFilter, with PER_FIELD_REUSE_STRATEGY.
type payloadAnalyzer struct {
	analysis.Analyzer
	mu          sync.Mutex
	fieldToData map[string]*payloadData
}

func newPayloadAnalyzer() *payloadAnalyzer {
	pa := &payloadAnalyzer{fieldToData: map[string]*payloadData{}}
	a := analysis.NewAnalyzer(analysis.PerFieldReuseStrategy)
	a.CreateComponents = func(fieldName string) *analysis.TokenStreamComponents {
		payload := pa.get(fieldName)
		ts := testanalysis.NewMockTokenizer(testanalysis.WHITESPACE, false, testanalysis.DefaultMaxTokenLength)
		var tokenStream analysis.TokenStream = ts
		if payload != nil {
			tokenStream = newPayloadFilter(ts, fieldName, pa)
		}
		return &analysis.TokenStreamComponents{
			Source: func(r io.Reader) error {
				ts.SetReader(r)
				return nil
			},
			Sink: tokenStream,
		}
	}
	pa.Analyzer = a
	return pa
}

func (pa *payloadAnalyzer) setPayloadData(field string, data []byte, offset, length int) {
	pa.mu.Lock()
	defer pa.mu.Unlock()
	pa.fieldToData[field] = &payloadData{data: data, offset: offset, length: length}
}

func (pa *payloadAnalyzer) get(field string) *payloadData {
	pa.mu.Lock()
	defer pa.mu.Unlock()
	return pa.fieldToData[field]
}

// payloadFilter is the private PayloadFilter: it adds payloads to the tokens.
type payloadFilter struct {
	*analysis.BaseTokenFilter
	payloadAtt    analysis.PayloadAttribute
	termAttribute analysis.CharTermAttribute
	fieldToData   *payloadAnalyzer
	fieldName     string
	payloadData   *payloadData
	offset        int
}

func newPayloadFilter(in analysis.TokenStream, fieldName string, fieldToData *payloadAnalyzer) *payloadFilter {
	f := &payloadFilter{BaseTokenFilter: analysis.NewBaseTokenFilter(in), fieldToData: fieldToData, fieldName: fieldName}
	f.payloadAtt = f.AddAttribute(analysis.PayloadAttributeType).(analysis.PayloadAttribute)
	f.termAttribute = f.AddAttribute(analysis.CharTermAttributeType).(analysis.CharTermAttribute)
	return f
}

func (f *payloadFilter) IncrementToken() (bool, error) {
	hasNext, err := f.GetInput().IncrementToken()
	if err != nil || !hasNext {
		return false, err
	}

	// Some values of the same field are to have payloads and others not
	if f.offset+f.payloadData.length <= len(f.payloadData.data) && !strings.HasSuffix(f.termAttribute.String(), "NO PAYLOAD") {
		p := f.payloadData.data[f.offset : f.offset+f.payloadData.length]
		f.payloadAtt.SetPayload(p)
		f.offset += f.payloadData.length
	} else {
		f.payloadAtt.SetPayload(nil)
	}

	return true, nil
}

func (f *payloadFilter) Reset() error {
	if err := f.BaseTokenFilter.Reset(); err != nil {
		return err
	}
	f.payloadData = f.fieldToData.get(f.fieldName)
	f.offset = f.payloadData.offset
	return nil
}

func TestPayloadsThreadSafety(t *testing.T) {
	const numThreads = 5
	numDocs := atLeast(50)
	pool := newByteArrayPool(numThreads, 5)

	dir := newDirectory()
	writer := mustNewIndexWriter(t, dir, newIndexWriterConfigWithAnalyzer(newMockAnalyzer()))
	const field = "test"

	var wg sync.WaitGroup
	for i := 0; i < numThreads; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < numDocs; j++ {
				d := document.NewDocument()
				f, err := document.NewField(field, newPoolingPayloadTokenStream(pool), document.TextFieldTypeNotStored)
				if err != nil {
					t.Errorf("TextField: %v", err)
					return
				}
				d.Add(f)
				if _, err := writer.AddDocument(d); err != nil {
					t.Errorf("addDocument: %v", err)
					return
				}
			}
		}()
	}

	wg.Wait()
	if t.Failed() {
		t.FailNow()
	}
	mustClose(t, writer)
	reader := mustOpenDirectoryReader(t, dir)
	multiTerms, err := index.MultiTermsGetTerms(reader, field)
	if err != nil {
		t.Fatalf("MultiTerms.getTerms: %v", err)
	}
	terms, err := multiTerms.Iterator()
	if err != nil {
		t.Fatalf("iterator: %v", err)
	}
	for {
		term, err := terms.Next()
		if err != nil {
			t.Fatalf("next: %v", err)
		}
		if term == nil {
			break
		}
		termText := term.Bytes.String()
		tp, err := terms.Postings(spi.PostingsFlagPayloads)
		if err != nil {
			t.Fatalf("postings: %v", err)
		}
		for {
			doc, err := tp.NextDoc()
			if err != nil {
				t.Fatalf("nextDoc: %v", err)
			}
			if doc == spi.NO_MORE_DOCS {
				break
			}
			freq, err := tp.Freq()
			if err != nil {
				t.Fatalf("freq: %v", err)
			}
			for i := 0; i < freq; i++ {
				if _, err := tp.NextPosition(); err != nil {
					t.Fatalf("nextPosition: %v", err)
				}
				payload, err := tp.GetPayload()
				if err != nil {
					t.Fatalf("getPayload: %v", err)
				}
				if termText != string(payload) {
					t.Fatalf("payload: expected %q, got %q", termText, payload)
				}
			}
		}
	}
	mustClose(t, reader, dir)
	if pool.size() != numThreads {
		t.Fatalf("pool.size(): expected %d, got %d", numThreads, pool.size())
	}
}

// poolingPayloadTokenStream is the private PoolingPayloadTokenStream.
type poolingPayloadTokenStream struct {
	*analysis.BaseTokenStream
	payload    []byte
	first      bool
	pool       *byteArrayPool
	term       string
	termAtt    analysis.CharTermAttribute
	payloadAtt analysis.PayloadAttribute
}

func newPoolingPayloadTokenStream(pool *byteArrayPool) *poolingPayloadTokenStream {
	s := &poolingPayloadTokenStream{BaseTokenStream: analysis.NewBaseTokenStream(), pool: pool}
	s.payload = pool.get()
	payloadsGenerateRandomDataInto(s.payload)
	s.term = string(s.payload)
	s.first = true
	s.payloadAtt = s.AddAttribute(analysis.PayloadAttributeType).(analysis.PayloadAttribute)
	s.termAtt = s.AddAttribute(analysis.CharTermAttributeType).(analysis.CharTermAttribute)
	return s
}

func (s *poolingPayloadTokenStream) IncrementToken() (bool, error) {
	if !s.first {
		return false, nil
	}
	s.first = false
	s.ClearAttributes()
	s.termAtt.AppendString(s.term)
	s.payloadAtt.SetPayload(append([]byte(nil), s.payload...))
	return true, nil
}

func (s *poolingPayloadTokenStream) Close() error {
	s.pool.release(s.payload)
	return nil
}

// byteArrayPool is the private ByteArrayPool.
type byteArrayPool struct {
	mu   sync.Mutex
	pool [][]byte
}

func newByteArrayPool(capacity, size int) *byteArrayPool {
	p := &byteArrayPool{}
	for i := 0; i < capacity; i++ {
		p.pool = append(p.pool, make([]byte, size))
	}
	return p
}

func (p *byteArrayPool) get() []byte {
	p.mu.Lock()
	defer p.mu.Unlock()
	b := p.pool[0]
	p.pool = p.pool[1:]
	return b
}

func (p *byteArrayPool) release(b []byte) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.pool = append(p.pool, b)
}

func (p *byteArrayPool) size() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.pool)
}

func newWhitespaceLowerCaseMockAnalyzer() analysis.Analyzer {
	return testanalysis.NewMockAnalyzer(testanalysis.WHITESPACE, true, testanalysis.DefaultMaxTokenLength, nil, true)
}

func TestPayloadsAcrossFields(t *testing.T) {
	dir := newDirectory()
	writer := newRandomIndexWriterWithAnalyzer(t, dir, newWhitespaceLowerCaseMockAnalyzer())
	doc := document.NewDocument()
	doc.Add(newTextField(t, "hasMaybepayload", "here we go", true))
	if _, err := writer.AddDocument(doc); err != nil {
		t.Fatalf("addDocument: %v", err)
	}
	mustClose(t, writer)

	writer = newRandomIndexWriterWithAnalyzer(t, dir, newWhitespaceLowerCaseMockAnalyzer())
	doc = document.NewDocument()
	doc.Add(newTextField(t, "hasMaybepayload2", "here we go", true))
	for i := 0; i < 2; i++ {
		if _, err := writer.AddDocument(doc); err != nil {
			t.Fatalf("addDocument: %v", err)
		}
	}
	if err := writer.ForceMerge(1); err != nil {
		t.Fatalf("forceMerge: %v", err)
	}
	mustClose(t, writer, dir)
}

func newWhitespaceMockTokenizer(t testing.TB, text string) *testanalysis.MockTokenizer {
	t.Helper()
	ts := testanalysis.NewMockTokenizer(testanalysis.WHITESPACE, true, testanalysis.DefaultMaxTokenLength)
	ts.SetReader(strings.NewReader(text))
	return ts
}

// some docs have payload att, some not
func TestPayloadsMixupDocs(t *testing.T) {
	dir := newDirectory()
	iwc := index.NewIndexWriterConfigWithAnalyzer(nil)
	iwc.SetMergePolicy(newLogMergePolicy())
	writer := newRandomIndexWriterWithConfig(t, dir, iwc)
	defer mustClose(t, writer, dir)
	doc := document.NewDocument()
	ts := newWhitespaceMockTokenizer(t, "here we go")
	field, err := document.NewField("field", ts, document.TextFieldTypeNotStored)
	if err != nil {
		t.Fatalf("Field: %v", err)
	}
	doc.Add(field)
	if _, err := writer.AddDocument(doc); err != nil {
		t.Fatalf("addDocument: %v", err)
	}
	withPayload := testanalysis.NewToken("withPayload", 0, 11).WithPayload([]byte("test"))
	canned := testanalysis.NewCannedTokenStream(withPayload)
	if !canned.HasAttribute(analysis.PayloadAttributeType) {
		t.Fatal("assertTrue(ts.hasAttribute(PayloadAttribute.class))")
	}
	t.Fatal(fieldSetTokenStreamMissing)
}

// some field instances have payload att, some not
func TestPayloadsMixupMultiValued(t *testing.T) {
	dir := newDirectory()
	writer := newRandomIndexWriter(t, dir)
	defer mustClose(t, writer, dir)
	ts := testanalysis.NewMockTokenizer(testanalysis.WHITESPACE, true, testanalysis.DefaultMaxTokenLength)
	if _, err := document.NewField("field", ts, document.TextFieldTypeNotStored); err != nil {
		t.Fatalf("Field: %v", err)
	}
	ts.SetReader(strings.NewReader("here we go"))
	t.Fatal(fieldSetTokenStreamMissing)
}
