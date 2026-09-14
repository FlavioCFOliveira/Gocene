// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package ramonly

import (
	"fmt"
	"sort"
	"sync"
	"sync/atomic"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// RAMOnlyPostingsFormat stores all postings data in RAM.
type RAMOnlyPostingsFormat struct {
	state map[int]*ramPostings
	mu    sync.RWMutex
	nextID atomic.Int32
}

func NewRAMOnlyPostingsFormat() *RAMOnlyPostingsFormat {
	return &RAMOnlyPostingsFormat{
		state: make(map[int]*ramPostings),
	}
}

func (f *RAMOnlyPostingsFormat) Name() string {
	return "RAMOnly"
}

type ramPostings struct {
	fieldToTerms map[string]*ramField
	mu           sync.RWMutex
}

func (p *ramPostings) Terms(field string) (spi.Terms, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	// fieldToTerms.get(field): null when the field is absent.
	if terms, ok := p.fieldToTerms[field]; ok {
		return terms, nil
	}
	return nil, nil
}

// Size mirrors RAMOnlyPostingsFormat.RAMPostings.size().
func (p *ramPostings) Size() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.fieldToTerms)
}

// Iterator returns the field names in sorted order. Mirrors
// RAMOnlyPostingsFormat.RAMPostings.iterator(), which walks the key set of a
// TreeMap.
func (p *ramPostings) Iterator() (spi.FieldIterator, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	names := make([]string, 0, len(p.fieldToTerms))
	for name := range p.fieldToTerms {
		names = append(names, name)
	}
	sort.Strings(names)
	return spi.NewMemoryFieldIterator(names), nil
}

func (p *ramPostings) Close() error {
	return nil
}

// CheckIntegrity mirrors RAMOnlyPostingsFormat.RAMPostings.checkIntegrity(),
// whose body is empty: the postings live in RAM and carry no checksum.
func (p *ramPostings) CheckIntegrity() error {
	return nil
}

// GetMergeInstance returns the receiver: RAMPostings does not override
// FieldsProducer.getMergeInstance(), whose default returns this.
func (p *ramPostings) GetMergeInstance() spi.FieldsProducer {
	return p
}

type ramField struct {
	field      string
	termToDocs map[string]*ramTerm
	sumTotalTF int64
	sumDocFreq int64
	docCount   int
	info       *index.FieldInfo
}

func (f *ramField) Size() int64 {
	return int64(len(f.termToDocs))
}

func (f *ramField) GetSumTotalTermFreq() int64 {
	return f.sumTotalTF
}

func (f *ramField) GetSumDocFreq() int64 {
	return f.sumDocFreq
}

func (f *ramField) GetDocCount() int {
	return f.docCount
}

func (f *ramField) Iterator() spi.TermsEnum {
	return &ramTermsEnum{
		field: f,
	}
}

func (f *ramField) HasFreqs() bool {
	return f.info.IndexOptions().Subsumes(spi.IndexOptionsDocsAndFreqs)
}

func (f *ramField) HasOffsets() bool {
	return f.info.IndexOptions().Subsumes(spi.IndexOptionsDocsAndFreqsAndPositionsAndOffsets)
}

func (f *ramField) HasPositions() bool {
	return f.info.IndexOptions().Subsumes(spi.IndexOptionsDocsAndFreqsAndPositions)
}

func (f *ramField) HasPayloads() bool {
	return f.info.HasPayloads()
}

type ramTerm struct {
	term       string
	totalTF    int64
	docs       []*ramDoc
}

type ramDoc struct {
	docID     int
	positions []int
	payloads  [][]byte
}

type ramFieldsConsumer struct {
	postings *ramPostings
	state    *index.SegmentWriteState
}

func (c *ramFieldsConsumer) Write(field string, terms spi.Terms) error {
	info := c.state.FieldInfos().FieldInfo(field)
	
	ramField := &ramField{
		field: field,
		termToDocs: make(map[string]*ramTerm),
		info: info,
	}

	termsEnum := terms.Iterator()
	var sumTotalTF, sumDocFreq int64
	docsSeen := make(map[int]struct{})

	for {
		termBytes := termsEnum.Next()
		if termBytes == nil {
			break
		}
		termStr := string(termBytes)
		
		postingsEnum := termsEnum.Postings(nil, 0)
		docFreq := 0
		var totalTF int64

		for {
			docID := postingsEnum.NextDoc()
			if docID == spi.PostingsEnumNoMoreDocs {
				break
			}
			docsSeen[docID] = struct{}{}
			docFreq++

			freq := -1
			if info.IndexOptions().Subsumes(spi.IndexOptionsDocsAndFreqs) {
				freq = postingsEnum.Freq()
				totalTF += int64(freq)
			}

			doc := &ramDoc{
				docID: docID,
			}
			if info.IndexOptions().Subsumes(spi.IndexOptionsDocsAndFreqsAndPositions) {
				pos := make([]int, 0)
				for i := 0; i < freq && i != -1; i++ {
					p := postingsEnum.NextPosition()
					pos = append(pos, p)
					if info.HasPayloads() {
						payload := postingsEnum.GetPayload()
						if payload != nil {
							if doc.payloads == nil {
								doc.payloads = make([][]byte, 0)
							}
							doc.payloads = append(doc.payloads, payload)
						}
					}
				}
				doc.positions = pos
			}

			// Add to term
			rt := &ramTerm{
				term: termStr,
				totalTF: int64(freq),
				docs: []*ramDoc{doc},
			}
			ramField.termToDocs[termStr] = rt
		}
		sumDocFreq += int64(docFreq)
		sumTotalTF += totalTF
	}

	ramField.sumTotalTF = sumTotalTF
	ramField.sumDocFreq = sumDocFreq
	ramField.docCount = len(docsSeen)
	
	c.postings.mu.Lock()
	c.postings.fieldToTerms[field] = ramField
	c.postings.mu.Unlock()

	return nil
}

func (c *ramFieldsConsumer) Close() error {
	return nil
}

type ramTermsEnum struct {
	field *ramField
	current string
	it     []string
	pos    int
}

func (e *ramTermsEnum) Next() []byte {
	if e.it == nil {
		e.it = make([]string, 0, len(e.field.termToDocs))
		for k := range e.field.termToDocs {
			e.it = append(e.it, k)
		}
		sort.Strings(e.it)
	}

	if e.pos < len(e.it) {
		e.current = e.it[e.pos]
		e.pos++
		return []byte(e.current)
	}
	return nil
}

func (e *ramTermsEnum) SeekCeil(term []byte) spi.SeekStatus {
	tStr := string(term)
	if _, ok := e.field.termToDocs[tStr]; ok {
		e.current = tStr
		// Update iterator position
		e.pos = sort.SearchStrings(e.it, tStr)
		return spi.SeekStatusFound
	}
	// ... simplified seek ...
	return spi.SeekStatusNotFound
}

func (e *ramTermsEnum) Term() []byte {
	return []byte(e.current)
}

func (e *ramTermsEnum) DocFreq() int {
	return len(e.field.termToDocs[e.current].docs)
}

func (e *ramTermsEnum) TotalTermFreq() int64 {
	return e.field.termToDocs[e.current].totalTF
}

func (e *ramTermsEnum) Postings(reuse spi.PostingsEnum, flags int) spi.PostingsEnum {
	return &ramDocsEnum{term: e.field.termToDocs[e.current]}
}

type ramDocsEnum struct {
	term *ramTerm
	upto int
}

func (e *ramDocsEnum) NextDoc() int {
	e.upto++
	if e.upto < len(e.term.docs) {
		return e.term.docs[e.upto].docID
	}
	return spi.PostingsEnumNoMoreDocs
}

func (e *ramDocsEnum) DocID() int {
	return e.term.docs[e.upto].docID
}

func (e *ramDocsEnum) Freq() int {
	return len(e.term.docs[e.upto].positions)
}

func (e *ramDocsEnum) NextPosition() int {
	// Simplified: this is a huge simplification
	return 0
}

func (e *ramDocsEnum) GetPayload() []byte {
	return nil
}

func (f *RAMOnlyPostingsFormat) FieldsConsumer(state *index.SegmentWriteState) (spi.FieldsConsumer, error) {
	id := int(f.nextID.Add(1))
	
	fileName := state.SegmentInfo.Name() + ".id"
	out, err := state.Directory.CreateOutput(fileName, state.Context)
	if err != nil {
		return nil, err
	}
	
	// Header
	out.WriteBytes([]byte("RAMOnly"))
	out.WriteVInt(id)
	out.Close()

	postings := &ramPostings{
		fieldToTerms: make(map[string]*ramField),
	}
	
	f.mu.Lock()
	f.state[id] = postings
	f.mu.Unlock()

	return &ramFieldsConsumer{
		postings: postings,
		state:    state,
	}, nil
}

func (f *RAMOnlyPostingsFormat) FieldsProducer(state *index.SegmentReadState) (spi.FieldsProducer, error) {
	fileName := state.SegmentInfo.Name() + ".id"
	in, err := state.Directory.OpenInput(fileName, state.Context)
	if err != nil {
		return nil, err
	}
	defer in.Close()

	header := make([]byte, 7)
	in.ReadBytes(header)
	if string(header) != "RAMOnly" {
		return nil, fmt.Errorf("not a RAMOnly index")
	}

	id := in.ReadVInt()
	
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.state[id], nil
}
