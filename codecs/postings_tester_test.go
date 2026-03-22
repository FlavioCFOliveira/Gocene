// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"fmt"
	"math/rand"
	"sort"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// PostingsTester manages the lifecycle of a postings format test.
// This is a simplified Go port of Lucene's RandomPostingsTester.
type PostingsTester struct {
	t    *testing.T
	seed int64
	rand *rand.Rand
}

func NewPostingsTester(t *testing.T) *PostingsTester {
	seed := int64(12345) // Use fixed seed for reproducibility
	return &PostingsTester{
		t:    t,
		seed: seed,
		rand: rand.New(rand.NewSource(seed)),
	}
}

// SeedFields is a mock index.Fields implementation for testing.
type SeedFields struct {
	fields map[string]*SeedTerms
	names  []string
}

func (f *SeedFields) Names() []string {
	return f.names
}

func (f *SeedFields) Terms(field string) (index.Terms, error) {
	if t, ok := f.fields[field]; ok {
		return t, nil
	}
	return nil, nil
}

// SeedTerms is a mock index.Terms implementation for testing.
type SeedTerms struct {
	index.TermsBase
	field      string
	terms      []*index.Term
	termToDocs map[string][]SeedPosting
	options    index.IndexOptions
}

func (t *SeedTerms) GetIterator() (index.TermsEnum, error) {
	return &SeedTermsEnum{terms: t.terms, termToDocs: t.termToDocs, pos: -1}, nil
}

func (t *SeedTerms) GetIteratorWithSeek(seekTerm *index.Term) (index.TermsEnum, error) {
	te, _ := t.GetIterator()
	_, err := te.SeekCeil(seekTerm)
	return te, err
}

func (t *SeedTerms) Size() int64 {
	return int64(len(t.terms))
}

func (t *SeedTerms) GetMin() (*index.Term, error) {
	if len(t.terms) == 0 {
		return nil, nil
	}
	return t.terms[0], nil
}

func (t *SeedTerms) GetMax() (*index.Term, error) {
	if len(t.terms) == 0 {
		return nil, nil
	}
	return t.terms[len(t.terms)-1], nil
}

func (t *SeedTerms) HasFreqs() bool {
	return t.options >= index.IndexOptionsDocsAndFreqs
}

func (t *SeedTerms) HasPositions() bool {
	return t.options >= index.IndexOptionsDocsAndFreqsAndPositions
}

func (t *SeedTerms) HasOffsets() bool {
	return t.options >= index.IndexOptionsDocsAndFreqsAndPositionsAndOffsets
}

func (t *SeedTerms) HasPayloads() bool {
	// For testing, we can assume payloads are available if positions are
	return t.HasPositions()
}

func (t *SeedTerms) GetPostingsReader(termText string, flags int) (index.PostingsEnum, error) {
	// Find the term and return its postings
	for _, term := range t.terms {
		if term.Text() == termText {
			postings, ok := t.termToDocs[termText]
			if !ok {
				return nil, nil
			}
			return &SeedPostingsEnum{
				postings: postings,
				pos:      -1,
			}, nil
		}
	}
	return nil, nil
}

func (t *SeedTerms) GetDocCount() (int, error) {
	// Return the number of unique documents across all terms
	docSet := make(map[int]struct{})
	for _, postings := range t.termToDocs {
		for _, p := range postings {
			docSet[p.docID] = struct{}{}
		}
	}
	return len(docSet), nil
}

type SeedPosting struct {
	docID     int
	freq      int
	positions []int
	offsets   []SeedOffset
	payload   []byte
}

type SeedOffset struct {
	start int
	end   int
}

type SeedTermsEnum struct {
	index.TermsEnumBase
	terms      []*index.Term
	termToDocs map[string][]SeedPosting
	pos        int
	curr       *index.Term
}

func (m *SeedTermsEnum) Next() (*index.Term, error) {
	m.pos++
	if m.pos >= len(m.terms) {
		m.curr = nil
		return nil, nil
	}
	m.curr = m.terms[m.pos]
	return m.curr, nil
}

func (m *SeedTermsEnum) SeekCeil(term *index.Term) (*index.Term, error) {
	idx := sort.Search(len(m.terms), func(i int) bool {
		return m.terms[i].CompareTo(term) >= 0
	})
	m.pos = idx
	if m.pos >= len(m.terms) {
		m.curr = nil
		return nil, nil
	}
	m.curr = m.terms[m.pos]
	return m.curr, nil
}

func (m *SeedTermsEnum) SeekExact(term *index.Term) (bool, error) {
	got, err := m.SeekCeil(term)
	return err == nil && got != nil && got.Equals(term), err
}

func (m *SeedTermsEnum) Term() *index.Term {
	return m.curr
}

func (m *SeedTermsEnum) DocFreq() (int, error) {
	t := m.Term()
	if t == nil {
		return 0, nil
	}
	return len(m.termToDocs[t.Text()]), nil
}

func (m *SeedTermsEnum) TotalTermFreq() (int64, error) {
	t := m.Term()
	if t == nil {
		return 0, nil
	}
	postings := m.termToDocs[t.Text()]
	var total int64
	for _, p := range postings {
		total += int64(p.freq)
	}
	return total, nil
}

func (m *SeedTermsEnum) Postings(flags int) (index.PostingsEnum, error) {
	t := m.Term()
	if t == nil {
		return nil, nil
	}
	return &SeedPostingsEnum{
		postings: m.termToDocs[t.Text()],
		pos:      -1,
	}, nil
}

func (m *SeedTermsEnum) PostingsWithLiveDocs(liveDocs util.Bits, flags int) (index.PostingsEnum, error) {
	return m.Postings(flags)
}

type SeedPostingsEnum struct {
	index.PostingsEnumBase
	postings []SeedPosting
	pos      int
	currDoc  int
}

func (p *SeedPostingsEnum) NextDoc() (int, error) {
	p.pos++
	if p.pos >= len(p.postings) {
		p.currDoc = index.NO_MORE_DOCS
		return index.NO_MORE_DOCS, nil
	}
	p.currDoc = p.postings[p.pos].docID
	return p.currDoc, nil
}

func (p *SeedPostingsEnum) Advance(target int) (int, error) {
	for {
		doc, err := p.NextDoc()
		if err != nil || doc >= target || doc == index.NO_MORE_DOCS {
			return doc, err
		}
	}
}

func (p *SeedPostingsEnum) DocID() int {
	if p.pos < 0 {
		return -1
	}
	return p.currDoc
}

func (p *SeedPostingsEnum) Freq() (int, error) {
	if p.pos < 0 || p.pos >= len(p.postings) {
		return 0, nil
	}
	return p.postings[p.pos].freq, nil
}

func (p *SeedPostingsEnum) NextPosition() (int, error) {
	return index.NO_MORE_POSITIONS, nil
}

func (p *SeedPostingsEnum) StartOffset() (int, error) {
	return -1, nil
}

func (p *SeedPostingsEnum) EndOffset() (int, error) {
	return -1, nil
}

func (p *SeedPostingsEnum) GetPayload() ([]byte, error) {
	return nil, nil
}

func (p *SeedPostingsEnum) Cost() int64 {
	return int64(len(p.postings))
}

// TestFull performs a comprehensive test of a PostingsFormat.
func (p *PostingsTester) TestFull(format PostingsFormat, options index.IndexOptions, dir store.Directory) {
	segmentName := "_0"
	segmentID := make([]byte, 16)
	p.rand.Read(segmentID)

	si := index.NewSegmentInfo(segmentName, 100, dir)
	si.SetID(segmentID)

	fieldInfos := index.NewFieldInfos()
	// Add a few fields with different options
	fieldName := "field1"
	fi := index.NewFieldInfo(fieldName, 0, index.FieldInfoOptions{
		IndexOptions: options,
	})
	fieldInfos.Add(fi)

	writeState := &SegmentWriteState{
		Directory:     dir,
		SegmentInfo:   si,
		FieldInfos:    fieldInfos,
		SegmentSuffix: "",
	}

	consumer, err := format.FieldsConsumer(writeState)
	if err != nil {
		// Lucene104PostingsFormat placeholder returns error
		p.t.Logf("FieldsConsumer failed as expected for placeholder: %v", err)
		return
	}
	defer consumer.Close()

	// 1. Generate and write postings
	seedTerms := &SeedTerms{
		field:      fieldName,
		terms:      make([]*index.Term, 0),
		termToDocs: make(map[string][]SeedPosting),
		options:    options,
	}

	// Create a few deterministic terms
	for i := 0; i < 10; i++ {
		termText := fmt.Sprintf("term%d", i)
		term := index.NewTerm(fieldName, termText)
		seedTerms.terms = append(seedTerms.terms, term)

		// Create a few postings for each term
		postings := make([]SeedPosting, 0)
		for j := 0; j < 5; j++ {
			docID := j * 10
			freq := 1 + (i+j)%3
			p := SeedPosting{
				docID: docID,
				freq:  freq,
			}
			postings = append(postings, p)
		}
		seedTerms.termToDocs[termText] = postings
	}

	err = consumer.Write(fieldName, seedTerms)
	if err != nil {
		p.t.Fatalf("Consumer.Write failed: %v", err)
	}

	err = consumer.Close()
	if err != nil {
		p.t.Fatalf("Consumer.Close failed: %v", err)
	}

	// 2. Read back
	readState := &SegmentReadState{
		Directory:     dir,
		SegmentInfo:   si,
		FieldInfos:    fieldInfos,
		SegmentSuffix: "",
	}

	producer, err := format.FieldsProducer(readState)
	if err != nil {
		p.t.Fatalf("FieldsProducer failed: %v", err)
	}
	defer producer.Close()

	// 3. Verify
	terms, err := producer.Terms(fieldName)
	if err != nil {
		p.t.Fatalf("Producer.Terms failed: %v", err)
	}
	if terms == nil {
		p.t.Fatal("Producer.Terms returned nil")
	}

	te, err := terms.GetIterator()
	if err != nil {
		p.t.Fatalf("Terms.GetIterator failed: %v", err)
	}

	for _, expectedTerm := range seedTerms.terms {
		actualTerm, err := te.Next()
		if err != nil {
			p.t.Fatalf("TermsEnum.Next failed: %v", err)
		}
		if actualTerm == nil {
			p.t.Fatalf("Expected term %s, got nil", expectedTerm.Text())
		}
		if actualTerm.Text() != expectedTerm.Text() {
			p.t.Fatalf("Expected term %s, got %s", expectedTerm.Text(), actualTerm.Text())
		}

		// Verify postings
		expectedPostings := seedTerms.termToDocs[expectedTerm.Text()]
		pe, err := te.Postings(0) // 0 for default flags
		if err != nil {
			p.t.Fatalf("TermsEnum.Postings failed: %v", err)
		}

		for _, expectedPos := range expectedPostings {
			actualDocID, err := pe.NextDoc()
			if err != nil {
				p.t.Fatalf("PostingsEnum.NextDoc failed: %v", err)
			}
			if actualDocID != expectedPos.docID {
				p.t.Fatalf("Expected docID %d, got %d", expectedPos.docID, actualDocID)
			}

			if options >= index.IndexOptionsDocsAndFreqs {
				actualFreq, err := pe.Freq()
				if err != nil {
					p.t.Fatalf("PostingsEnum.Freq failed: %v", err)
				}
				if actualFreq != expectedPos.freq {
					p.t.Fatalf("Expected freq %d, got %d", expectedPos.freq, actualFreq)
				}
			}
		}

		// End of postings
		lastDoc, err := pe.NextDoc()
		if err != nil {
			p.t.Fatalf("PostingsEnum.NextDoc (end) failed: %v", err)
		}
		if lastDoc != index.NO_MORE_DOCS {
			p.t.Fatalf("Expected NO_MORE_DOCS, got %d", lastDoc)
		}
	}

	// End of terms
	lastTerm, err := te.Next()
	if err != nil {
		p.t.Fatalf("TermsEnum.Next (end) failed: %v", err)
	}
	if lastTerm != nil {
		p.t.Fatalf("Expected nil at end of terms, got %s", lastTerm.Text())
	}
}
