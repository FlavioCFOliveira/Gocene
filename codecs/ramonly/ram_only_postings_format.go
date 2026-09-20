// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package ramonly is the Go port of
// org.apache.lucene.tests.codecs.ramonly (Apache Lucene 10.5.0,
// lucene/test-framework/src/java/org/apache/lucene/tests/codecs/ramonly).
package ramonly

import (
	"errors"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/automaton"
)

// Format constants, ported from RAMOnlyPostingsFormat.java:526-529.
const (
	// ramOnlyName is the codec name written into the ".id" token file.
	ramOnlyName = "RAMOnly"
	// versionStart is the first version of the ".id" token file.
	versionStart int32 = 0
	// versionLatest is the version this format writes.
	versionLatest = versionStart
	// idExtension is the extension of the one file this format writes.
	idExtension = "id"
)

// RAMOnlyPostingsFormat stores all postings data in RAM, but writes a small
// token (header + single int) to identify which "slot" the index is using in
// the RAM map.
//
// NOTE: this codec sorts terms by reverse-unicode-order!
//
// Port of org.apache.lucene.tests.codecs.ramonly.RAMOnlyPostingsFormat
// (RAMOnlyPostingsFormat.java:61).
type RAMOnlyPostingsFormat struct {
	// state holds all indexes created, keyed by the ID assigned in
	// FieldsConsumer (RAMOnlyPostingsFormat.java:521). Java guards the map with
	// `synchronized (state)`; the mutex renders that monitor.
	mu    sync.Mutex
	state map[int32]*ramPostings

	// nextID renders `private final AtomicInteger nextID`
	// (RAMOnlyPostingsFormat.java:523).
	nextID atomic.Int32
}

// NewRAMOnlyPostingsFormat constructs the format. Port of the sole constructor
// (RAMOnlyPostingsFormat.java:63), which is `super("RAMOnly")`.
func NewRAMOnlyPostingsFormat() *RAMOnlyPostingsFormat {
	return &RAMOnlyPostingsFormat{
		state: make(map[int32]*ramPostings),
	}
}

// Name returns the codec name this format registers under.
func (f *RAMOnlyPostingsFormat) Name() string {
	return ramOnlyName
}

// ─── RAMPostings ─────────────────────────────────────────────────────────────

// ramPostings is the FieldsProducer over the in-RAM postings of one segment.
//
// Port of the static nested class RAMOnlyPostingsFormat.RAMPostings
// (RAMOnlyPostingsFormat.java:68).
type ramPostings struct {
	// mu guards fieldToTerms: Java's RAMPostings is written by a single
	// RAMFieldsConsumer and read afterwards, but Gocene's FieldsConsumer.Write
	// is called once per field, so the map is published across those calls.
	mu sync.RWMutex

	// fieldToTerms renders `final Map<String, RAMField> fieldToTerms = new
	// TreeMap<>()` (RAMOnlyPostingsFormat.java:69). Go has no sorted map, so
	// the TreeMap's ordering is reproduced by sorting the key set wherever
	// Java's iteration order is observable.
	fieldToTerms map[string]*ramField
}

// Terms renders RAMPostings.terms(String) (RAMOnlyPostingsFormat.java:72):
// `return fieldToTerms.get(field)`, which is null for an absent field.
func (p *ramPostings) Terms(field string) (spi.Terms, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if terms, ok := p.fieldToTerms[field]; ok {
		return terms, nil
	}
	return nil, nil
}

// Size renders RAMPostings.size() (RAMOnlyPostingsFormat.java:77):
// `return fieldToTerms.size()`.
func (p *ramPostings) Size() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.fieldToTerms)
}

// Iterator renders RAMPostings.iterator() (RAMOnlyPostingsFormat.java:82),
// which walks the key set of the TreeMap and is therefore sorted.
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

// Close renders RAMPostings.close() (RAMOnlyPostingsFormat.java:87), whose
// body is empty.
func (p *ramPostings) Close() error { return nil }

// CheckIntegrity renders RAMPostings.checkIntegrity()
// (RAMOnlyPostingsFormat.java:90), whose body is empty: the postings live in
// RAM and carry no checksum.
func (p *ramPostings) CheckIntegrity() error { return nil }

// GetMergeInstance returns the receiver: RAMPostings does not override
// FieldsProducer.getMergeInstance(), whose default returns `this`.
func (p *ramPostings) GetMergeInstance() spi.FieldsProducer { return p }

// ─── RAMField ────────────────────────────────────────────────────────────────

// ramField is the Terms of one field.
//
// Port of the static nested class RAMOnlyPostingsFormat.RAMField
// (RAMOnlyPostingsFormat.java:93).
type ramField struct {
	// field renders `final String field` (RAMOnlyPostingsFormat.java:94).
	field string
	// termToDocs renders `final SortedMap<String, RAMTerm> termToDocs = new
	// TreeMap<>()` (RAMOnlyPostingsFormat.java:95); the sort order is applied
	// by sortedTerms wherever Java's TreeMap ordering is observable.
	termToDocs map[string]*ramTerm
	// sumTotalTermFreq renders `long sumTotalTermFreq`
	// (RAMOnlyPostingsFormat.java:96).
	sumTotalTermFreq int64
	// sumDocFreq renders `long sumDocFreq` (RAMOnlyPostingsFormat.java:97).
	sumDocFreq int64
	// docCount renders `int docCount` (RAMOnlyPostingsFormat.java:98).
	docCount int
	// info renders `final FieldInfo info` (RAMOnlyPostingsFormat.java:99).
	info *index.FieldInfo
}

// newRAMField renders RAMField(String, FieldInfo)
// (RAMOnlyPostingsFormat.java:101).
func newRAMField(field string, info *index.FieldInfo) *ramField {
	return &ramField{
		field:      field,
		termToDocs: make(map[string]*ramTerm),
		info:       info,
	}
}

// sortedTerms returns the term texts in the order RAMField's TreeMap yields
// them.
func (f *ramField) sortedTerms() []string {
	terms := make([]string, 0, len(f.termToDocs))
	for t := range f.termToDocs {
		terms = append(terms, t)
	}
	sort.Strings(terms)
	return terms
}

// Field returns the name of the field this Terms instance represents.
//
// org.apache.lucene.index.Terms declares no field() accessor, so Java reads
// RAMField.field directly (RAMOnlyPostingsFormat.java:94); Gocene's [spi.Terms]
// contract does declare one, and it is answered from the same value.
func (f *ramField) Field() string { return f.field }

// Size renders RAMField.size() (RAMOnlyPostingsFormat.java:107):
// `return termToDocs.size()`.
func (f *ramField) Size() int64 { return int64(len(f.termToDocs)) }

// GetSumTotalTermFreq renders RAMField.getSumTotalTermFreq()
// (RAMOnlyPostingsFormat.java:112). Java declares no checked exception on it;
// Gocene's [spi.Terms] carries an error return on every statistic.
func (f *ramField) GetSumTotalTermFreq() (int64, error) { return f.sumTotalTermFreq, nil }

// GetSumDocFreq renders RAMField.getSumDocFreq()
// (RAMOnlyPostingsFormat.java:117).
func (f *ramField) GetSumDocFreq() (int64, error) { return f.sumDocFreq, nil }

// GetDocCount renders RAMField.getDocCount() (RAMOnlyPostingsFormat.java:122).
func (f *ramField) GetDocCount() (int, error) { return f.docCount, nil }

// Iterator renders RAMField.iterator() (RAMOnlyPostingsFormat.java:127):
// `return new RAMTermsEnum(RAMField.this)`.
func (f *ramField) Iterator() (spi.TermsEnum, error) { return newRAMTermsEnum(f), nil }

// HasFreqs renders RAMField.hasFreqs() (RAMOnlyPostingsFormat.java:132).
func (f *ramField) HasFreqs() bool {
	return f.info.IndexOptions().Subsumes(spi.IndexOptionsDocsAndFreqs)
}

// HasOffsets renders RAMField.hasOffsets() (RAMOnlyPostingsFormat.java:137).
func (f *ramField) HasOffsets() bool {
	return f.info.IndexOptions().Subsumes(spi.IndexOptionsDocsAndFreqsAndPositionsAndOffsets)
}

// HasPositions renders RAMField.hasPositions() (RAMOnlyPostingsFormat.java:142).
func (f *ramField) HasPositions() bool {
	return f.info.IndexOptions().Subsumes(spi.IndexOptionsDocsAndFreqsAndPositions)
}

// HasPayloads renders RAMField.hasPayloads() (RAMOnlyPostingsFormat.java:147).
func (f *ramField) HasPayloads() bool { return f.info.HasPayloads() }

// GetIteratorWithSeek returns an iterator positioned at or after seekTerm.
//
// org.apache.lucene.index.Terms declares no such member, so RAMField overrides
// nothing here; the Gocene [spi.Terms] contract does declare it, and it is
// answered with the two Lucene operations a Java caller would spell out —
// Terms.iterator() followed by TermsEnum.seekCeil(BytesRef).
func (f *ramField) GetIteratorWithSeek(seekTerm *spi.Term) (spi.TermsEnum, error) {
	te, err := f.Iterator()
	if err != nil {
		return nil, err
	}
	if seekTerm != nil {
		if _, err := te.SeekCeil(seekTerm); err != nil {
			return nil, err
		}
	}
	return te, nil
}

// GetPostingsReader returns the postings of termText, or nil when the term is
// absent. Like GetIteratorWithSeek this is a Gocene-only [spi.Terms] member; it
// is answered with iterator() -> seekExact(BytesRef) -> postings(int).
func (f *ramField) GetPostingsReader(termText string, flags int) (spi.PostingsEnum, error) {
	te, err := f.Iterator()
	if err != nil {
		return nil, err
	}
	found, err := te.SeekExact(spi.NewTerm(f.field, termText))
	if err != nil || !found {
		return nil, err
	}
	return te.Postings(flags)
}

// Intersect is the default org.apache.lucene.index.Terms#intersect(
// CompiledAutomaton, BytesRef) (Terms.java:64) that RAMField inherits:
// iterator() wrapped in an AutomatonTermsEnum, rejecting any CompiledAutomaton
// that is not AUTOMATON_TYPE.NORMAL.
func (f *ramField) Intersect(compiled *automaton.CompiledAutomaton, startTerm *spi.Term) (spi.TermsEnum, error) {
	termsEnum, err := f.Iterator()
	if err != nil {
		return nil, err
	}
	if compiled.Type != automaton.AutomatonTypeNormal {
		return nil, errors.New("please use CompiledAutomaton.getTermsEnum instead")
	}
	automatonTermsEnum := index.NewAutomatonTermsEnum(termsEnum, compiled)
	if startTerm != nil {
		automatonTermsEnum.SetInitialSeekTerm(startTerm)
	}
	return automatonTermsEnum, nil
}

// GetMin is the default org.apache.lucene.index.Terms#getMin()
// (Terms.java:143) that RAMField inherits: `return iterator().next()`.
func (f *ramField) GetMin() (*spi.Term, error) {
	te, err := f.Iterator()
	if err != nil {
		return nil, err
	}
	return te.Next()
}

// GetMax is the default org.apache.lucene.index.Terms#getMax()
// (Terms.java:153) that RAMField inherits. Java first tries a seek-by-ord and
// falls back to a digit-by-digit binary search when the enumerator throws
// UnsupportedOperationException; RAMTermsEnum.ord()
// (RAMOnlyPostingsFormat.java:421) always does, so the ord attempt is spelled
// out as the branch it always takes: straight to the binary search.
func (f *ramField) GetMax() (*spi.Term, error) {
	if f.Size() == 0 {
		// empty: only possible from a FilteredTermsEnum...
		return nil, nil
	}

	// otherwise: binary search
	iterator, err := f.Iterator()
	if err != nil {
		return nil, err
	}
	v, err := iterator.Next()
	if err != nil {
		return nil, err
	}
	if v == nil {
		// empty: only possible from a FilteredTermsEnum...
		return nil, nil
	}

	scratch := []byte{0}

	// Iterates over digits:
	for {
		low := 0
		high := 256

		// Binary search current digit to find the highest
		// digit before END:
		for low != high {
			mid := int(uint(low+high) >> 1)
			scratch[len(scratch)-1] = byte(mid)
			term, err := iterator.SeekCeil(spi.NewTermFromBytes(f.field, scratch))
			if err != nil {
				return nil, err
			}
			if term == nil {
				// Scratch was too high
				if mid == 0 {
					scratch = scratch[:len(scratch)-1]
					return spi.NewTermFromBytes(f.field, scratch), nil
				}
				high = mid
			} else {
				// Scratch was too low; there is at least one term
				// still after it:
				if low == mid {
					break
				}
				low = mid
			}
		}

		// Recurse to next digit:
		scratch = append(scratch, 0)
	}
}

// ─── RAMTerm / RAMDoc ────────────────────────────────────────────────────────

// ramTerm is the postings list of one term.
//
// Port of the static nested class RAMOnlyPostingsFormat.RAMTerm
// (RAMOnlyPostingsFormat.java:152).
type ramTerm struct {
	// term renders `final String term` (RAMOnlyPostingsFormat.java:153).
	term string
	// totalTermFreq renders `long totalTermFreq`
	// (RAMOnlyPostingsFormat.java:154).
	totalTermFreq int64
	// docs renders `final List<RAMDoc> docs = new ArrayList<>()`
	// (RAMOnlyPostingsFormat.java:157).
	docs []*ramDoc
}

// newRAMTerm renders RAMTerm(String) (RAMOnlyPostingsFormat.java:157).
func newRAMTerm(term string) *ramTerm {
	return &ramTerm{term: term}
}

// ramDoc is one document posting of one term.
//
// Port of the static nested class RAMOnlyPostingsFormat.RAMDoc
// (RAMOnlyPostingsFormat.java:162), which implements Accountable.
type ramDoc struct {
	// docID renders `final int docID` (RAMOnlyPostingsFormat.java:163).
	docID int
	// positions renders `final int[] positions`
	// (RAMOnlyPostingsFormat.java:164).
	positions []int
	// payloads renders `byte[][] payloads` (RAMOnlyPostingsFormat.java:167).
	payloads [][]byte
}

// newRAMDoc renders RAMDoc(int, int) (RAMOnlyPostingsFormat.java:167), whose
// body allocates `positions = new int[freq]`.
func newRAMDoc(docID, freq int) *ramDoc {
	return &ramDoc{docID: docID, positions: make([]int, freq)}
}

// RamBytesUsed renders RAMDoc.ramBytesUsed() (RAMOnlyPostingsFormat.java:173):
// the size of the positions array plus the size of every non-null payload.
func (d *ramDoc) RamBytesUsed() int64 {
	var sizeInBytes int64
	if d.positions != nil {
		sizeInBytes += util.SizeOfIntSlice(d.positions)
	}
	if d.payloads != nil {
		for _, payload := range d.payloads {
			if payload != nil {
				sizeInBytes += util.SizeOfByteSlice(payload)
			}
		}
	}
	return sizeInBytes
}

// ─── RAMFieldsConsumer ───────────────────────────────────────────────────────

// ramFieldsConsumer is the write side of the format.
//
// Port of the private static nested class
// RAMOnlyPostingsFormat.RAMFieldsConsumer (RAMOnlyPostingsFormat.java:187).
type ramFieldsConsumer struct {
	postings      *ramPostings
	termsConsumer *ramTermsConsumer
	state         *index.SegmentWriteState
}

// newRAMFieldsConsumer renders RAMFieldsConsumer(SegmentWriteState, RAMPostings)
// (RAMOnlyPostingsFormat.java:193).
func newRAMFieldsConsumer(writeState *index.SegmentWriteState, postings *ramPostings) *ramFieldsConsumer {
	return &ramFieldsConsumer{
		postings:      postings,
		termsConsumer: &ramTermsConsumer{postingsWriter: &ramPostingsWriterImpl{}},
		state:         writeState,
	}
}

// ErrCannotIndexOffsets is the sentinel behind the UnsupportedOperationException
// RAMFieldsConsumer.write throws for a field that indexes offsets
// (RAMOnlyPostingsFormat.java:213).
var ErrCannotIndexOffsets = errors.New("ramonly: this codec cannot index offsets")

// Write renders RAMFieldsConsumer.write(Fields, NormsProducer)
// (RAMOnlyPostingsFormat.java:199-301): it iterates every field the Fields
// exposes and drives writeField for each non-nil Terms.
func (c *ramFieldsConsumer) Write(fields spi.Fields, norms spi.NormsProducer) error {
	if fields == nil {
		return nil
	}
	it, err := fields.Iterator()
	if err != nil {
		return err
	}
	for {
		field, err := it.Next()
		if err != nil {
			return err
		}
		if field == "" {
			return nil
		}
		terms, err := fields.Terms(field)
		if err != nil {
			return err
		}
		if terms == nil {
			continue
		}
		if err := c.writeField(field, terms); err != nil {
			return err
		}
	}
}

// writeField carries the per-field body of the Java write loop.
func (c *ramFieldsConsumer) writeField(field string, terms spi.Terms) error {
	termsEnum, err := terms.Iterator()
	if err != nil {
		return err
	}

	fieldInfo := c.state.FieldInfos.FieldInfo(field)
	if fieldInfo.IndexOptions().Subsumes(spi.IndexOptionsDocsAndFreqsAndPositionsAndOffsets) {
		return ErrCannotIndexOffsets
	}

	ramField := newRAMField(field, fieldInfo)
	c.postings.mu.Lock()
	c.postings.fieldToTerms[field] = ramField
	c.postings.mu.Unlock()
	c.termsConsumer.reset(ramField)

	docsSeen, err := util.NewFixedBitSet(c.state.SegmentInfo.MaxDoc())
	if err != nil {
		return err
	}
	var sumTotalTermFreq int64
	var sumDocFreq int64
	var enumFlags int

	indexOptions := fieldInfo.IndexOptions()
	writeFreqs := indexOptions.Subsumes(spi.IndexOptionsDocsAndFreqs)
	writePositions := indexOptions.Subsumes(spi.IndexOptionsDocsAndFreqsAndPositions)
	writeOffsets := indexOptions.Subsumes(spi.IndexOptionsDocsAndFreqsAndPositionsAndOffsets)
	writePayloads := fieldInfo.HasPayloads()

	if !writeFreqs {
		enumFlags = 0
	} else if !writePositions {
		enumFlags = spi.PostingsFlagFreqs
	} else if !writeOffsets {
		if writePayloads {
			enumFlags = spi.PostingsFlagPayloads
		} else {
			enumFlags = 0
		}
	} else {
		if writePayloads {
			enumFlags = spi.PostingsFlagPayloads | spi.PostingsFlagOffsets
		} else {
			enumFlags = spi.PostingsFlagOffsets
		}
	}

	for {
		term, err := termsEnum.Next()
		if err != nil {
			return err
		}
		if term == nil {
			break
		}
		postingsWriter := c.termsConsumer.startTerm(term.BytesValue())
		postingsEnum, err := termsEnum.Postings(enumFlags)
		if err != nil {
			return err
		}

		docFreq := 0
		var totalTermFreq int64
		for {
			docID, err := postingsEnum.NextDoc()
			if err != nil {
				return err
			}
			if docID == spi.NO_MORE_DOCS {
				break
			}
			docsSeen.Set(docID)
			docFreq++

			var freq int
			if writeFreqs {
				freq, err = postingsEnum.Freq()
				if err != nil {
					return err
				}
				totalTermFreq += int64(freq)
			} else {
				freq = -1
			}

			postingsWriter.startDoc(docID, freq)
			if writePositions {
				for i := 0; i < freq; i++ {
					pos, err := postingsEnum.NextPosition()
					if err != nil {
						return err
					}
					var payload []byte
					if writePayloads {
						payload, err = postingsEnum.GetPayload()
						if err != nil {
							return err
						}
					}
					startOffset := -1
					endOffset := -1
					if writeOffsets {
						startOffset, err = postingsEnum.StartOffset()
						if err != nil {
							return err
						}
						endOffset, err = postingsEnum.EndOffset()
						if err != nil {
							return err
						}
					}
					postingsWriter.addPosition(pos, payload, startOffset, endOffset)
				}
			}

			postingsWriter.finishDoc()
		}
		c.termsConsumer.finishTerm(term.BytesValue(), codecs.NewTermStats(docFreq, totalTermFreq))
		sumDocFreq += int64(docFreq)
		sumTotalTermFreq += totalTermFreq
	}

	c.termsConsumer.finish(sumTotalTermFreq, sumDocFreq, docsSeen.Cardinality())
	return nil
}

// Close renders RAMFieldsConsumer.close() (RAMOnlyPostingsFormat.java:307),
// whose body is empty.
func (c *ramFieldsConsumer) Close() error { return nil }

// ─── RAMTermsConsumer ────────────────────────────────────────────────────────

// ramTermsConsumer accumulates one field's terms.
//
// Port of the private static nested class
// RAMOnlyPostingsFormat.RAMTermsConsumer (RAMOnlyPostingsFormat.java:310).
type ramTermsConsumer struct {
	field          *ramField
	postingsWriter *ramPostingsWriterImpl
	current        *ramTerm
}

// reset renders RAMTermsConsumer.reset(RAMField)
// (RAMOnlyPostingsFormat.java:315).
func (c *ramTermsConsumer) reset(field *ramField) { c.field = field }

// startTerm renders RAMTermsConsumer.startTerm(BytesRef)
// (RAMOnlyPostingsFormat.java:319).
func (c *ramTermsConsumer) startTerm(text *util.BytesRef) *ramPostingsWriterImpl {
	term := text.String()
	c.current = newRAMTerm(term)
	c.postingsWriter.reset(c.current)
	return c.postingsWriter
}

// finishTerm renders RAMTermsConsumer.finishTerm(BytesRef, TermStats)
// (RAMOnlyPostingsFormat.java:326). The two `assert` statements Java opens with
// are disabled at runtime unless the JVM is started with -ea, so only the last
// two statements are behaviour.
func (c *ramTermsConsumer) finishTerm(_ *util.BytesRef, stats codecs.TermStats) {
	c.current.totalTermFreq = stats.TotalTermFreq
	c.field.termToDocs[c.current.term] = c.current
}

// finish renders RAMTermsConsumer.finish(long, long, int)
// (RAMOnlyPostingsFormat.java:333).
func (c *ramTermsConsumer) finish(sumTotalTermFreq, sumDocFreq int64, docCount int) {
	c.field.sumTotalTermFreq = sumTotalTermFreq
	c.field.sumDocFreq = sumDocFreq
	c.field.docCount = docCount
}

// ─── RAMPostingsWriterImpl ───────────────────────────────────────────────────

// ramPostingsWriterImpl accumulates one term's documents and positions.
//
// Port of the static nested class RAMOnlyPostingsFormat.RAMPostingsWriterImpl
// (RAMOnlyPostingsFormat.java:340).
type ramPostingsWriterImpl struct {
	term    *ramTerm
	current *ramDoc
	posUpto int
}

// reset renders RAMPostingsWriterImpl.reset(RAMTerm)
// (RAMOnlyPostingsFormat.java:345).
func (w *ramPostingsWriterImpl) reset(term *ramTerm) { w.term = term }

// startDoc renders RAMPostingsWriterImpl.startDoc(int, int)
// (RAMOnlyPostingsFormat.java:349).
func (w *ramPostingsWriterImpl) startDoc(docID, freq int) {
	w.current = newRAMDoc(docID, freq)
	w.term.docs = append(w.term.docs, w.current)
	w.posUpto = 0
}

// addPosition renders RAMPostingsWriterImpl.addPosition(int, BytesRef, int, int)
// (RAMOnlyPostingsFormat.java:355). The two leading `assert` statements are
// disabled at runtime unless the JVM is started with -ea.
func (w *ramPostingsWriterImpl) addPosition(position int, payload []byte, _, _ int) {
	w.current.positions[w.posUpto] = position
	if len(payload) > 0 {
		if w.current.payloads == nil {
			w.current.payloads = make([][]byte, len(w.current.positions))
		}
		bytes := make([]byte, len(payload))
		copy(bytes, payload)
		w.current.payloads[w.posUpto] = bytes
	}
	w.posUpto++
}

// finishDoc renders RAMPostingsWriterImpl.finishDoc()
// (RAMOnlyPostingsFormat.java:369), whose only statement is an `assert`, and is
// therefore a no-op unless the JVM is started with -ea.
func (w *ramPostingsWriterImpl) finishDoc() {}

// ─── RAMTermsEnum ────────────────────────────────────────────────────────────

// errRAMTermsEnumUnsupported is the sentinel behind the
// UnsupportedOperationException that RAMTermsEnum.seekExact(long) and
// RAMTermsEnum.ord() throw (RAMOnlyPostingsFormat.java:416, :419).
var errRAMTermsEnumUnsupported = errors.New("ramonly: RAMTermsEnum: operation is not supported")

// ErrRAMTermsEnumUnsupported is the exported sentinel for errors.Is.
var ErrRAMTermsEnumUnsupported = errRAMTermsEnumUnsupported

// ramTermsEnum iterates one field's terms.
//
// Port of the static nested class RAMOnlyPostingsFormat.RAMTermsEnum
// (RAMOnlyPostingsFormat.java:374), which extends BaseTermsEnum.
type ramTermsEnum struct {
	// TermsEnumBase renders `extends BaseTermsEnum`: it carries the lazily
	// created AttributeSource behind BaseTermsEnum.attributes().
	spi.TermsEnumBase

	// it renders `Iterator<String> it` (RAMOnlyPostingsFormat.java:375): nil
	// until next() materialises it, and reset to nil by seekCeil. Java's
	// iterator is a cursor over a key set; here it is the sorted key slice plus
	// the position within it.
	it    []string
	itPos int
	// current renders `String current` (RAMOnlyPostingsFormat.java:376).
	current string
	// hasCurrent distinguishes Java's `current == null` from the empty term,
	// which Go's zero string cannot.
	hasCurrent bool
	// ramField renders `private final RAMField ramField`
	// (RAMOnlyPostingsFormat.java:377).
	ramField *ramField
}

// newRAMTermsEnum renders RAMTermsEnum(RAMField)
// (RAMOnlyPostingsFormat.java:379).
func newRAMTermsEnum(field *ramField) *ramTermsEnum {
	return &ramTermsEnum{ramField: field}
}

// Next renders RAMTermsEnum.next() (RAMOnlyPostingsFormat.java:384): the
// iterator is materialised on first use over the whole key set, or over the
// tail map starting at the current term when seekCeil has moved the cursor.
func (e *ramTermsEnum) Next() (*spi.Term, error) {
	if e.it == nil {
		sorted := e.ramField.sortedTerms()
		if !e.hasCurrent {
			e.it = sorted
			e.itPos = 0
		} else {
			// tailMap(current): the keys greater than or equal to current.
			e.it = sorted
			e.itPos = sort.SearchStrings(sorted, e.current)
		}
	}
	if e.itPos < len(e.it) {
		e.current = e.it[e.itPos]
		e.hasCurrent = true
		e.itPos++
		return spi.NewTermFromBytes(e.ramField.field, []byte(e.current)), nil
	}
	return nil, nil
}

// seekCeilStatus renders RAMTermsEnum.seekCeil(BytesRef)
// (RAMOnlyPostingsFormat.java:401) with Java's SeekStatus return value, which
// Gocene's [spi.TermsEnum] does not carry; SeekCeil and SeekExact below are the
// two contract members that consume it.
func (e *ramTermsEnum) seekCeilStatus(term *spi.Term) spi.SeekStatus {
	e.current = term.BytesValue().String()
	e.hasCurrent = true
	e.it = nil
	if _, ok := e.ramField.termToDocs[e.current]; ok {
		return spi.SeekStatusFound
	}
	sorted := e.ramField.sortedTerms()
	if len(sorted) == 0 || e.current > sorted[len(sorted)-1] {
		return spi.SeekStatusEnd
	}
	return spi.SeekStatusNotFound
}

// SeekCeil seeks to term or, when it is absent, leaves the cursor on it so that
// Next() resumes from the tail map — exactly what RAMTermsEnum.seekCeil does
// (RAMOnlyPostingsFormat.java:401). Gocene's contract reports SeekStatus.END as
// a nil term.
func (e *ramTermsEnum) SeekCeil(term *spi.Term) (*spi.Term, error) {
	if e.seekCeilStatus(term) == spi.SeekStatusEnd {
		return nil, nil
	}
	return e.Term(), nil
}

// SeekExact renders the default BaseTermsEnum.seekExact(BytesRef)
// (BaseTermsEnum.java:57): `return seekCeil(text) == SeekStatus.FOUND`.
func (e *ramTermsEnum) SeekExact(term *spi.Term) (bool, error) {
	return e.seekCeilStatus(term) == spi.SeekStatusFound, nil
}

// SeekExactOrd renders RAMTermsEnum.seekExact(long)
// (RAMOnlyPostingsFormat.java:416), whose body is
// `throw new UnsupportedOperationException()`.
func (e *ramTermsEnum) SeekExactOrd(ord int64) error { return errRAMTermsEnumUnsupported }

// Ord renders RAMTermsEnum.ord() (RAMOnlyPostingsFormat.java:421), whose body
// is `throw new UnsupportedOperationException()`. Ord carries no error in the
// TermsEnum contract (Java's ord() declares no checked exception), so the
// unsupported call panics, mirroring the unchecked Java exception.
func (e *ramTermsEnum) Ord() int64 { panic(errRAMTermsEnumUnsupported) }

// Term renders RAMTermsEnum.term() (RAMOnlyPostingsFormat.java:426):
// `return new BytesRef(current)`. Gocene's [spi.Term] carries the field name
// too, which Java reads off the enclosing RAMField.
func (e *ramTermsEnum) Term() *spi.Term {
	return spi.NewTermFromBytes(e.ramField.field, []byte(e.current))
}

// DocFreq renders RAMTermsEnum.docFreq() (RAMOnlyPostingsFormat.java:432):
// `return ramField.termToDocs.get(current).docs.size()`.
func (e *ramTermsEnum) DocFreq() (int, error) {
	return len(e.ramField.termToDocs[e.current].docs), nil
}

// TotalTermFreq renders RAMTermsEnum.totalTermFreq()
// (RAMOnlyPostingsFormat.java:437):
// `return ramField.termToDocs.get(current).totalTermFreq`.
func (e *ramTermsEnum) TotalTermFreq() (int64, error) {
	return e.ramField.termToDocs[e.current].totalTermFreq, nil
}

// Postings renders RAMTermsEnum.postings(PostingsEnum, int)
// (RAMOnlyPostingsFormat.java:442): `return new RAMDocsEnum(
// ramField.termToDocs.get(current))`. Gocene's [spi.TermsEnum] carries no reuse
// parameter, and Java ignores the one it is given.
func (e *ramTermsEnum) Postings(flags int) (spi.PostingsEnum, error) {
	return newRAMDocsEnum(e.ramField.termToDocs[e.current]), nil
}

// PostingsWithLiveDocs returns the same postings as Postings: RAMTermsEnum has
// no live-docs-aware overload, and org.apache.lucene.index.TermsEnum has no
// such member either — the live docs are applied by the caller.
func (e *ramTermsEnum) PostingsWithLiveDocs(liveDocs util.Bits, flags int) (spi.PostingsEnum, error) {
	return e.Postings(flags)
}

// Impacts renders RAMTermsEnum.impacts(int)
// (RAMOnlyPostingsFormat.java:447): `return new SlowImpactsEnum(postings(null,
// PostingsEnum.FREQS))`.
func (e *ramTermsEnum) Impacts(flags int) (spi.ImpactsEnum, error) {
	postings, err := e.Postings(spi.PostingsFlagFreqs)
	if err != nil {
		return nil, err
	}
	return index.NewSlowImpactsEnum(postings), nil
}

// ─── RAMDocsEnum ─────────────────────────────────────────────────────────────

// ramDocsEnum iterates the documents of one term.
//
// Port of the private static nested class RAMOnlyPostingsFormat.RAMDocsEnum
// (RAMOnlyPostingsFormat.java:452), which extends PostingsEnum.
type ramDocsEnum struct {
	// ramTerm renders `private final RAMTerm ramTerm`
	// (RAMOnlyPostingsFormat.java:453).
	ramTerm *ramTerm
	// current renders `private RAMDoc current`
	// (RAMOnlyPostingsFormat.java:454).
	current *ramDoc
	// upto renders `int upto = -1` (RAMOnlyPostingsFormat.java:455).
	upto int
	// posUpto renders `int posUpto = 0` (RAMOnlyPostingsFormat.java:456).
	posUpto int
}

// newRAMDocsEnum renders RAMDocsEnum(RAMTerm)
// (RAMOnlyPostingsFormat.java:458).
func newRAMDocsEnum(term *ramTerm) *ramDocsEnum {
	return &ramDocsEnum{ramTerm: term, upto: -1}
}

// Advance renders RAMDocsEnum.advance(int)
// (RAMOnlyPostingsFormat.java:463): `return slowAdvance(targetDocID)`.
func (e *ramDocsEnum) Advance(targetDocID int) (int, error) {
	return util.SlowAdvance(e, targetDocID)
}

// NextDoc renders RAMDocsEnum.nextDoc() (RAMOnlyPostingsFormat.java:469).
func (e *ramDocsEnum) NextDoc() (int, error) {
	e.upto++
	if e.upto < len(e.ramTerm.docs) {
		e.current = e.ramTerm.docs[e.upto]
		e.posUpto = 0
		return e.current.docID, nil
	}
	return spi.NO_MORE_DOCS, nil
}

// Freq renders RAMDocsEnum.freq() (RAMOnlyPostingsFormat.java:481):
// `return current.positions.length`.
func (e *ramDocsEnum) Freq() (int, error) { return len(e.current.positions), nil }

// DocID renders RAMDocsEnum.docID() (RAMOnlyPostingsFormat.java:486):
// `return current.docID`.
func (e *ramDocsEnum) DocID() int {
	if e.current == nil {
		return -1
	}
	return e.current.docID
}

// NextPosition renders RAMDocsEnum.nextPosition()
// (RAMOnlyPostingsFormat.java:491). The leading `assert` is disabled at runtime
// unless the JVM is started with -ea.
func (e *ramDocsEnum) NextPosition() (int, error) {
	pos := e.current.positions[e.posUpto]
	e.posUpto++
	return pos, nil
}

// StartOffset renders RAMDocsEnum.startOffset()
// (RAMOnlyPostingsFormat.java:497): `return -1`.
func (e *ramDocsEnum) StartOffset() (int, error) { return -1, nil }

// EndOffset renders RAMDocsEnum.endOffset()
// (RAMOnlyPostingsFormat.java:502): `return -1`.
func (e *ramDocsEnum) EndOffset() (int, error) { return -1, nil }

// GetPayload renders RAMDocsEnum.getPayload()
// (RAMOnlyPostingsFormat.java:507).
func (e *ramDocsEnum) GetPayload() ([]byte, error) {
	if e.current.payloads != nil && e.current.payloads[e.posUpto-1] != nil {
		return e.current.payloads[e.posUpto-1], nil
	}
	return nil, nil
}

// Cost renders RAMDocsEnum.cost() (RAMOnlyPostingsFormat.java:516):
// `return ramTerm.docs.size()`.
func (e *ramDocsEnum) Cost() int64 { return int64(len(e.ramTerm.docs)) }

// IntoBitSet carries the default body of
// DocIdSetIterator.intoBitSet(int, FixedBitSet, int), which RAMDocsEnum does
// not override.
func (e *ramDocsEnum) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return util.DefaultIntoBitSet(e, upTo, bitSet, offset)
}

// DocIDRunEnd carries the default body of DocIdSetIterator.docIDRunEnd(), which
// RAMDocsEnum does not override: one past the current doc.
func (e *ramDocsEnum) DocIDRunEnd() (int, error) { return e.DocID() + 1, nil }

// ─── PostingsFormat entry points ─────────────────────────────────────────────

// FieldsConsumer renders RAMOnlyPostingsFormat.fieldsConsumer(SegmentWriteState)
// (RAMOnlyPostingsFormat.java:533): allocate the next id, write the header and
// that id to the ".id" file, and register a fresh RAMPostings under it.
func (f *RAMOnlyPostingsFormat) FieldsConsumer(writeState *index.SegmentWriteState) (spi.FieldsConsumer, error) {
	// getAndIncrement: Java hands out the pre-increment value.
	id := f.nextID.Add(1) - 1

	// TODO -- ok to do this up front instead of
	// on close....?  should be ok?
	// Write our ID:
	idFileName := index.SegmentFileName(
		writeState.SegmentInfo.Name(), writeState.SegmentSuffix, idExtension)
	out, err := writeState.Directory.CreateOutput(idFileName, writeState.Context)
	if err != nil {
		return nil, err
	}
	success := false
	defer func() {
		if !success {
			_ = out.Close()
		}
	}()
	if err := codecs.WriteHeader(out, ramOnlyName, versionLatest); err != nil {
		return nil, err
	}
	if err := out.WriteVInt(id); err != nil {
		return nil, err
	}
	success = true
	if err := out.Close(); err != nil {
		return nil, err
	}

	postings := &ramPostings{fieldToTerms: make(map[string]*ramField)}
	consumer := newRAMFieldsConsumer(writeState, postings)

	f.mu.Lock()
	f.state[id] = postings
	f.mu.Unlock()
	return consumer, nil
}

// FieldsProducer renders RAMOnlyPostingsFormat.fieldsProducer(SegmentReadState)
// (RAMOnlyPostingsFormat.java:566): read the id back out of the ".id" file and
// return the RAMPostings registered under it.
func (f *RAMOnlyPostingsFormat) FieldsProducer(readState *index.SegmentReadState) (spi.FieldsProducer, error) {
	// Load our ID:
	idFileName := index.SegmentFileName(
		readState.SegmentInfo.Name(), readState.SegmentSuffix, idExtension)
	in, err := readState.Directory.OpenInput(idFileName, readState.Context)
	if err != nil {
		return nil, err
	}
	success := false
	defer func() {
		if !success {
			_ = in.Close()
		}
	}()
	if _, err := codecs.CheckHeader(in, ramOnlyName, versionStart, versionLatest); err != nil {
		return nil, err
	}
	id, err := in.ReadVInt()
	if err != nil {
		return nil, err
	}
	success = true
	if err := in.Close(); err != nil {
		return nil, err
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	postings, ok := f.state[id]
	if !ok {
		return nil, fmt.Errorf("ramonly: no RAM postings registered for id %d", id)
	}
	return postings, nil
}

// interface compliance
var (
	_ spi.FieldsProducer = (*ramPostings)(nil)
	_ spi.Terms          = (*ramField)(nil)
	_ spi.TermsEnum      = (*ramTermsEnum)(nil)
	_ spi.PostingsEnum   = (*ramDocsEnum)(nil)
	_ spi.FieldsConsumer = (*ramFieldsConsumer)(nil)
)
