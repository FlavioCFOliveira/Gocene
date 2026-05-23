// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package simpletext

import (
	"bytes"
	"fmt"
	"strconv"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	gfst "github.com/FlavioCFOliveira/Gocene/util/fst"
)

// ---------------------------------------------------------------------------
// SimpleTextFieldsWriter token constants (referenced by the reader).
// ---------------------------------------------------------------------------

var (
	stfwEnd         = []byte("END")
	stfwField       = []byte("field ")
	stfwTerm        = []byte("  term ")
	stfwDoc         = []byte("    doc ")
	stfwFreq        = []byte("      freq ")
	stfwPos         = []byte("      pos ")
	stfwStartOffset = []byte("      startOffset ")
	stfwEndOffset   = []byte("      endOffset ")
	stfwPayload     = []byte("        payload ")
	stfwSkipList    = []byte("    skipList ")
)

// postingsExtension is the file extension for the postings file.
const postingsExtension = "pst"

// emptyTermsEnum is a shared empty TermsEnum sentinel.
var emptyTermsEnum index.TermsEnum = &index.EmptyTermsEnum{}

// positionsFlagMask mirrors postingsFlagPositions from index package (package-
// private there); value matches Lucene's PostingsEnum.POSITIONS.
const positionsFlagMask = (1 << 3) | (1 << 4)

// ---------------------------------------------------------------------------
// SimpleTextFieldsReader
// ---------------------------------------------------------------------------

// SimpleTextFieldsReader reads postings from a plain-text segment file (.pst)
// written by SimpleTextFieldsWriter. It builds an in-memory FST per field to
// allow random term access.
//
// Port of org.apache.lucene.codecs.simpletext.SimpleTextFieldsReader
// (Lucene 10.4.0).
type SimpleTextFieldsReader struct {
	// fields maps field name → file pointer of the first line after "field N\n".
	fields map[string]int64

	// in is the master IndexInput for the postings file.
	in store.IndexInput

	// fieldInfos is the segment field metadata.
	fieldInfos *index.FieldInfos

	// maxDoc is the number of documents in the segment.
	maxDoc int

	// termsCache caches per-field SimpleTextTerms after first access.
	termsCache map[string]*simpleTextTerms
	termsMu    sync.Mutex
}

// NewSimpleTextFieldsReader opens the postings file and pre-scans field
// positions.
//
// Port of SimpleTextFieldsReader(SegmentReadState).
func NewSimpleTextFieldsReader(state *codecs.SegmentReadState) (*SimpleTextFieldsReader, error) {
	fileName := index.SegmentFileName(
		state.SegmentInfo.Name(),
		state.SegmentSuffix,
		postingsExtension,
	)
	in, err := state.Directory.OpenInput(fileName, store.IOContext{Context: store.ContextRead})
	if err != nil {
		return nil, fmt.Errorf("SimpleTextFieldsReader: open %s: %w", fileName, err)
	}

	r := &SimpleTextFieldsReader{
		in:         in,
		fieldInfos: state.FieldInfos,
		maxDoc:     state.SegmentInfo.DocCount(),
		termsCache: make(map[string]*simpleTextTerms),
	}

	var scanErr error
	r.fields, scanErr = r.readFields(in.Clone())
	if scanErr != nil {
		_ = in.Close()
		return nil, fmt.Errorf("SimpleTextFieldsReader: scan fields: %w", scanErr)
	}
	return r, nil
}

// readFields scans the postings file and builds field→FP map.
//
// Port of SimpleTextFieldsReader.readFields(IndexInput).
func (r *SimpleTextFieldsReader) readFields(raw store.IndexInput) (map[string]int64, error) {
	input := store.NewChecksumIndexInput(raw)
	scratch := util.NewBytesRefBuilder()
	fields := make(map[string]int64)

	for {
		if err := stReadLine(input, scratch); err != nil {
			return nil, err
		}
		line := scratch.Bytes()[:scratch.Length()]

		if bytes.Equal(line, stfwEnd) {
			if err := stCheckFooter(input); err != nil {
				return nil, err
			}
			return fields, nil
		}
		if bytes.HasPrefix(line, stfwField) {
			name := string(line[len(stfwField):])
			fields[name] = input.GetFilePointer()
		}
	}
}

// Terms returns the Terms for the given field, or nil if the field is not
// present.
//
// Port of SimpleTextFieldsReader.terms(String).
func (r *SimpleTextFieldsReader) Terms(field string) (index.Terms, error) {
	r.termsMu.Lock()
	defer r.termsMu.Unlock()

	if t, ok := r.termsCache[field]; ok {
		return t, nil
	}
	fp, ok := r.fields[field]
	if !ok {
		return nil, nil
	}
	fi := r.fieldInfos.GetByName(field)
	t, err := newSimpleTextTerms(r, field, fi, fp, r.maxDoc)
	if err != nil {
		return nil, err
	}
	r.termsCache[field] = t
	return t, nil
}

// Close releases the postings file.
func (r *SimpleTextFieldsReader) Close() error {
	return r.in.Close()
}

// String implements fmt.Stringer.
func (r *SimpleTextFieldsReader) String() string {
	return fmt.Sprintf("SimpleTextFieldsReader(fields=%d)", len(r.fields))
}

// CheckIntegrity is a no-op for SimpleText.
func (r *SimpleTextFieldsReader) CheckIntegrity() error { return nil }

// compile-time assertion.
var _ codecs.FieldsProducer = (*SimpleTextFieldsReader)(nil)

// ---------------------------------------------------------------------------
// pairOutputType is the FST output type used by simpleTextTerms.
// Outer pair: (docsStart int64, skipPointer int64)
// Inner pair: (docFreq int64, totalTermFreq int64)
// Full type : *Pair[ *Pair[int64,int64], *Pair[int64,int64] ]
// ---------------------------------------------------------------------------

type (
	innerPair = gfst.Pair[int64, int64]
	outerPair = gfst.Pair[*innerPair, *innerPair]
)

// ---------------------------------------------------------------------------
// simpleTextTerms
// ---------------------------------------------------------------------------

// simpleTextTerms is the Terms implementation for one field in a SimpleText
// segment. It builds an FST from the term data and exposes TermsEnum.
//
// Port of SimpleTextFieldsReader.SimpleTextTerms.
type simpleTextTerms struct {
	index.TermsBase

	termsStart       int64
	fieldInfo        *index.FieldInfo
	maxDoc           int
	sumTotalTermFreq int64
	sumDocFreq       int64
	docCount         int
	termCount        int

	fst *gfst.FST[*outerPair]

	reader *SimpleTextFieldsReader
}

func newSimpleTextTerms(
	r *SimpleTextFieldsReader,
	_ string,
	fi *index.FieldInfo,
	termsStart int64,
	maxDoc int,
) (*simpleTextTerms, error) {
	t := &simpleTextTerms{
		termsStart: termsStart,
		fieldInfo:  fi,
		maxDoc:     maxDoc,
		reader:     r,
	}
	if err := t.loadTerms(); err != nil {
		return nil, err
	}
	return t, nil
}

// loadTerms scans the field section of the postings file and builds the FST.
//
// Port of SimpleTextTerms.loadTerms().
func (t *simpleTextTerms) loadTerms() error {
	posIntOutputs := gfst.PositiveIntOutputs()
	outerOutputs := gfst.NewPairOutputs[int64, int64](posIntOutputs, posIntOutputs)
	innerOutputs := gfst.NewPairOutputs[int64, int64](posIntOutputs, posIntOutputs)
	pairOutputs := gfst.NewPairOutputs[*innerPair, *innerPair](outerOutputs, innerOutputs)

	compiler := gfst.NewFSTCompilerBuilder[*outerPair](gfst.InputTypeByte1, pairOutputs).Build()

	in := t.reader.in.Clone()
	if err := in.SetPosition(t.termsStart); err != nil {
		return fmt.Errorf("simpleTextTerms.loadTerms: seek: %w", err)
	}
	defer func() { _ = in.Close() }()

	scratch := util.NewBytesRefBuilder()
	lastTerm := util.NewBytesRefBuilder()
	scratchInts := util.NewIntsRefBuilder()
	visitedDocs, err := util.NewFixedBitSet(t.maxDoc)
	if err != nil {
		return err
	}

	var lastDocsStart int64 = -1
	var docFreq int
	var totalTermFreq int64
	var skipPointer int64

	for {
		if err := stReadLine(in, scratch); err != nil {
			return fmt.Errorf("simpleTextTerms.loadTerms: readLine: %w", err)
		}
		line := scratch.Bytes()[:scratch.Length()]

		if bytes.Equal(line, stfwEnd) || bytes.HasPrefix(line, stfwField) {
			// Flush last term.
			if lastDocsStart != -1 {
				termRef := lastTerm.Get()
				if err := compiler.Add(
					gfst.ToIntsRef(termRef, scratchInts),
					pairOutputs.NewPair(
						outerOutputs.NewPair(lastDocsStart, skipPointer),
						innerOutputs.NewPair(int64(docFreq), totalTermFreq),
					),
				); err != nil {
					return fmt.Errorf("simpleTextTerms.loadTerms: fst.Add: %w", err)
				}
				t.sumTotalTermFreq += totalTermFreq
			}
			break
		}

		if bytes.HasPrefix(line, stfwDoc) {
			docFreq++
			t.sumDocFreq++
			totalTermFreq++
			docID, err := strconv.Atoi(string(line[len(stfwDoc):]))
			if err != nil {
				return fmt.Errorf("simpleTextTerms.loadTerms: parse docID: %w", err)
			}
			visitedDocs.Set(docID)
		} else if bytes.HasPrefix(line, stfwFreq) {
			f, err := strconv.Atoi(string(line[len(stfwFreq):]))
			if err != nil {
				return fmt.Errorf("simpleTextTerms.loadTerms: parse freq: %w", err)
			}
			totalTermFreq += int64(f) - 1
		} else if bytes.HasPrefix(line, stfwSkipList) {
			skipPointer = in.GetFilePointer()
		} else if bytes.HasPrefix(line, stfwTerm) {
			// Flush previous term.
			if lastDocsStart != -1 {
				termRef := lastTerm.Get()
				if err := compiler.Add(
					gfst.ToIntsRef(termRef, scratchInts),
					pairOutputs.NewPair(
						outerOutputs.NewPair(lastDocsStart, skipPointer),
						innerOutputs.NewPair(int64(docFreq), totalTermFreq),
					),
				); err != nil {
					return fmt.Errorf("simpleTextTerms.loadTerms: fst.Add (term): %w", err)
				}
			}
			// Next term starts right after this "  term " line.
			lastDocsStart = in.GetFilePointer()
			termBytes := line[len(stfwTerm):]
			lastTerm.CopyBytes(termBytes, 0, len(termBytes))
			docFreq = 0
			t.sumTotalTermFreq += totalTermFreq
			totalTermFreq = 0
			t.termCount++
			skipPointer = 0
		}
	}

	t.docCount = visitedDocs.Cardinality()

	meta, err := compiler.Compile()
	if err != nil {
		return fmt.Errorf("simpleTextTerms.loadTerms: fst.Compile: %w", err)
	}
	if meta != nil {
		t.fst, err = gfst.FromFSTReader[*outerPair](meta, compiler.GetFSTReader())
		if err != nil {
			return fmt.Errorf("simpleTextTerms.loadTerms: FST.fromFSTReader: %w", err)
		}
	}
	return nil
}

// GetIterator returns a TermsEnum for this field.
func (t *simpleTextTerms) GetIterator() (index.TermsEnum, error) {
	if t.fst == nil {
		return emptyTermsEnum, nil
	}
	enum, err := gfst.NewBytesRefFSTEnum[*outerPair](t.fst)
	if err != nil {
		return nil, err
	}
	return newSimpleTextTermsEnum(enum, t.fieldInfo.IndexOptions(), t.reader), nil
}

// GetIteratorWithSeek positions the returned TermsEnum at or after seekTerm.
func (t *simpleTextTerms) GetIteratorWithSeek(seekTerm *index.Term) (index.TermsEnum, error) {
	te, err := t.GetIterator()
	if err != nil {
		return nil, err
	}
	if te == emptyTermsEnum {
		return te, nil
	}
	status, err := te.SeekCeil(seekTerm)
	if err != nil {
		return nil, err
	}
	if status == nil {
		return emptyTermsEnum, nil
	}
	return te, nil
}

// GetPostingsReader returns a PostingsEnum for the named term.
func (t *simpleTextTerms) GetPostingsReader(termText string, flags int) (index.PostingsEnum, error) {
	te, err := t.GetIterator()
	if err != nil {
		return nil, err
	}
	term := index.NewTerm(t.fieldInfo.Name(), termText)
	found, err := te.SeekExact(term)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, nil
	}
	return te.Postings(flags)
}

// Size returns the number of unique terms.
func (t *simpleTextTerms) Size() int64 { return int64(t.termCount) }

// GetDocCount returns the number of documents with at least one term.
func (t *simpleTextTerms) GetDocCount() (int, error) { return t.docCount, nil }

// GetSumDocFreq returns the sum of docFreq across all terms.
func (t *simpleTextTerms) GetSumDocFreq() (int64, error) { return t.sumDocFreq, nil }

// GetSumTotalTermFreq returns the sum of totalTermFreq across all terms.
func (t *simpleTextTerms) GetSumTotalTermFreq() (int64, error) { return t.sumTotalTermFreq, nil }

// HasFreqs reports whether term frequencies are indexed.
func (t *simpleTextTerms) HasFreqs() bool {
	return t.fieldInfo.IndexOptions() >= index.IndexOptionsDocsAndFreqs
}

// HasOffsets reports whether term offsets are indexed.
func (t *simpleTextTerms) HasOffsets() bool {
	return t.fieldInfo.IndexOptions() >= index.IndexOptionsDocsAndFreqsAndPositionsAndOffsets
}

// HasPositions reports whether term positions are indexed.
func (t *simpleTextTerms) HasPositions() bool {
	return t.fieldInfo.IndexOptions() >= index.IndexOptionsDocsAndFreqsAndPositions
}

// HasPayloads reports whether payloads are stored.
func (t *simpleTextTerms) HasPayloads() bool { return t.fieldInfo.HasPayloads() }

// GetMin/GetMax are not efficiently available without a full scan.
func (t *simpleTextTerms) GetMin() (*index.Term, error) { return nil, nil }
func (t *simpleTextTerms) GetMax() (*index.Term, error) { return nil, nil }

// String implements fmt.Stringer.
func (t *simpleTextTerms) String() string {
	return fmt.Sprintf("SimpleTextTerms(terms=%d,postings=%d,positions=%d,docs=%d)",
		t.termCount, t.sumDocFreq, t.sumTotalTermFreq, t.docCount)
}

// ---------------------------------------------------------------------------
// simpleTextTermsEnum
// ---------------------------------------------------------------------------

// simpleTextTermsEnum walks the FST index for one field and provides postings
// access.
//
// Port of SimpleTextFieldsReader.SimpleTextTermsEnum.
type simpleTextTermsEnum struct {
	index.BaseTermsEnum

	indexOptions  index.IndexOptions
	docFreq       int
	totalTermFreq int64
	docsStart     int64
	skipPointer   int64
	ended         bool

	fstEnum *gfst.BytesRefFSTEnum[*outerPair]
	reader  *SimpleTextFieldsReader
}

func newSimpleTextTermsEnum(
	e *gfst.BytesRefFSTEnum[*outerPair],
	opts index.IndexOptions,
	r *SimpleTextFieldsReader,
) *simpleTextTermsEnum {
	return &simpleTextTermsEnum{
		fstEnum:      e,
		indexOptions: opts,
		reader:       r,
	}
}

// unpackOutput extracts docsStart, skipPointer, docFreq, totalTermFreq from a
// pair output.
func (te *simpleTextTermsEnum) unpackOutput(out *outerPair) {
	te.docsStart = out.Output1.Output1
	te.skipPointer = out.Output1.Output2
	te.docFreq = int(out.Output2.Output1)
	te.totalTermFreq = out.Output2.Output2
}

// SeekExact seeks to exactly term.
func (te *simpleTextTermsEnum) SeekExact(term *index.Term) (bool, error) {
	if term == nil || term.Bytes == nil {
		return false, nil
	}
	result, err := te.fstEnum.SeekExact(term.Bytes)
	if err != nil {
		return false, err
	}
	if result == nil {
		return false, nil
	}
	te.unpackOutput(result.Output)
	return true, nil
}

// SeekCeil seeks to the smallest term >= target. Returns the found term or nil.
func (te *simpleTextTermsEnum) SeekCeil(term *index.Term) (*index.Term, error) {
	if term == nil || term.Bytes == nil {
		return nil, nil
	}
	result, err := te.fstEnum.SeekCeil(term.Bytes)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, nil
	}
	te.unpackOutput(result.Output)
	return index.NewTermFromBytesRef(term.Field, result.Input), nil
}

// Next advances to the next term.
func (te *simpleTextTermsEnum) Next() (*index.Term, error) {
	if te.ended {
		return nil, nil
	}
	result, err := te.fstEnum.Next()
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, nil
	}
	te.unpackOutput(result.Output)
	return index.NewTermFromBytesRef("", result.Input), nil
}

// Term returns the current term (no field name — caller must know field).
func (te *simpleTextTermsEnum) Term() *index.Term {
	cur := te.fstEnum.Current()
	if cur == nil {
		return nil
	}
	return index.NewTermFromBytesRef("", cur.Input)
}

// DocFreq returns the number of documents containing the current term.
func (te *simpleTextTermsEnum) DocFreq() (int, error) { return te.docFreq, nil }

// TotalTermFreq returns the total term frequency for the current term.
func (te *simpleTextTermsEnum) TotalTermFreq() (int64, error) {
	if te.indexOptions == index.IndexOptionsDocs {
		return int64(te.docFreq), nil
	}
	return te.totalTermFreq, nil
}

// Postings returns a PostingsEnum for the current term.
func (te *simpleTextTermsEnum) Postings(flags int) (index.PostingsEnum, error) {
	hasPositions := te.indexOptions >= index.IndexOptionsDocsAndFreqsAndPositions
	if hasPositions && (flags&positionsFlagMask) != 0 {
		e := newSimpleTextPostingsEnum(te.reader)
		return e.reset(te.docsStart, te.indexOptions, te.docFreq, te.skipPointer)
	}
	e := newSimpleTextDocsEnum(te.reader)
	omitTF := te.indexOptions == index.IndexOptionsDocs
	return e.reset(te.docsStart, omitTF, te.docFreq, te.skipPointer)
}

// PostingsWithLiveDocs ignores live docs (SimpleText has no deletions).
func (te *simpleTextTermsEnum) PostingsWithLiveDocs(_ util.Bits, flags int) (index.PostingsEnum, error) {
	return te.Postings(flags)
}

// ---------------------------------------------------------------------------
// simpleTextDocsEnum — docs/freqs only (no positions).
// ---------------------------------------------------------------------------

// simpleTextDocsEnum reads doc IDs (and optionally term frequencies) from the
// postings stream, advancing through the text format line-by-line.
//
// Port of SimpleTextFieldsReader.SimpleTextDocsEnum.
type simpleTextDocsEnum struct {
	inStart     store.IndexInput
	in          store.IndexInput
	omitTF      bool
	docID       int
	tf          int
	scratch     *util.BytesRefBuilder
	cost_       int
	skipReader  *SimpleTextSkipReader
	nextSkipDoc int
	seekTo      int64
}

func newSimpleTextDocsEnum(r *SimpleTextFieldsReader) *simpleTextDocsEnum {
	inClone := r.in.Clone()
	return &simpleTextDocsEnum{
		inStart:    r.in,
		in:         inClone,
		scratch:    util.NewBytesRefBuilder(),
		skipReader: NewSimpleTextSkipReader(r.in.Clone()),
	}
}

func (e *simpleTextDocsEnum) canReuse(in store.IndexInput) bool { return in == e.inStart }

func (e *simpleTextDocsEnum) reset(fp int64, omitTF bool, docFreq int, skipPointer int64) (*simpleTextDocsEnum, error) {
	if err := e.in.SetPosition(fp); err != nil {
		return nil, err
	}
	e.omitTF = omitTF
	e.docID = -1
	e.tf = 1
	e.cost_ = docFreq
	if err := e.skipReader.Reset(skipPointer, docFreq); err != nil {
		return nil, err
	}
	e.nextSkipDoc = 0
	e.seekTo = -1
	return e, nil
}

func (e *simpleTextDocsEnum) DocID() int { return e.docID }

func (e *simpleTextDocsEnum) Freq() (int, error) { return e.tf, nil }

func (e *simpleTextDocsEnum) NextPosition() (int, error) { return -1, nil }

func (e *simpleTextDocsEnum) StartOffset() (int, error) { return -1, nil }

func (e *simpleTextDocsEnum) EndOffset() (int, error) { return -1, nil }

func (e *simpleTextDocsEnum) GetPayload() ([]byte, error) { return nil, nil }

func (e *simpleTextDocsEnum) NextDoc() (int, error) {
	return e.Advance(e.docID + 1)
}

func (e *simpleTextDocsEnum) readDoc() (int, error) {
	if e.docID == index.NO_MORE_DOCS {
		return e.docID, nil
	}
	first := true
	termFreq := 0
	for {
		lineStart := e.in.GetFilePointer()
		if err := stReadLine(e.in, e.scratch); err != nil {
			return 0, err
		}
		line := e.scratch.Bytes()[:e.scratch.Length()]

		if bytes.HasPrefix(line, stfwDoc) {
			if !first {
				if err := e.in.SetPosition(lineStart); err != nil {
					return 0, err
				}
				if !e.omitTF {
					e.tf = termFreq
				}
				return e.docID, nil
			}
			docID, err := strconv.Atoi(string(line[len(stfwDoc):]))
			if err != nil {
				return 0, fmt.Errorf("simpleTextDocsEnum: parse docID: %w", err)
			}
			e.docID = docID
			termFreq = 0
			first = false
		} else if bytes.HasPrefix(line, stfwFreq) {
			f, err := strconv.Atoi(string(line[len(stfwFreq):]))
			if err != nil {
				return 0, fmt.Errorf("simpleTextDocsEnum: parse freq: %w", err)
			}
			termFreq = f
		} else if bytes.HasPrefix(line, stfwPos) ||
			bytes.HasPrefix(line, stfwStartOffset) ||
			bytes.HasPrefix(line, stfwEndOffset) ||
			bytes.HasPrefix(line, stfwPayload) {
			// skip
		} else {
			// SKIP_LIST, TERM, FIELD, or END
			if !first {
				if err := e.in.SetPosition(lineStart); err != nil {
					return 0, err
				}
				if !e.omitTF {
					e.tf = termFreq
				}
				return e.docID, nil
			}
			e.docID = index.NO_MORE_DOCS
			return e.docID, nil
		}
	}
}

func (e *simpleTextDocsEnum) advanceTarget(target int) (int, error) {
	if e.seekTo > 0 {
		if err := e.in.SetPosition(e.seekTo); err != nil {
			return 0, err
		}
		e.seekTo = -1
	}
	for {
		doc, err := e.readDoc()
		if err != nil {
			return 0, err
		}
		if doc >= target {
			return doc, nil
		}
	}
}

func (e *simpleTextDocsEnum) AdvanceShallow(target int) error {
	if target > e.nextSkipDoc {
		_, err := e.skipReader.SkipTo(target)
		if err != nil {
			return err
		}
		if e.skipReader.GetNextSkipDoc() != index.NO_MORE_DOCS {
			e.seekTo = e.skipReader.GetNextSkipDocFP()
		}
		e.nextSkipDoc = e.skipReader.GetNextSkipDoc()
	}
	return nil
}

func (e *simpleTextDocsEnum) Advance(target int) (int, error) {
	if err := e.AdvanceShallow(target); err != nil {
		return 0, err
	}
	return e.advanceTarget(target)
}

func (e *simpleTextDocsEnum) Cost() int64 { return int64(e.cost_) }

func (e *simpleTextDocsEnum) GetImpacts() (index.Impacts, error) {
	if err := e.AdvanceShallow(e.docID); err != nil {
		return nil, err
	}
	return e.skipReader.GetImpacts(), nil
}

// compile-time assertion: simpleTextDocsEnum implements index.ImpactsEnum.
var _ index.ImpactsEnum = (*simpleTextDocsEnum)(nil)

// ---------------------------------------------------------------------------
// simpleTextPostingsEnum — docs/freqs/positions/offsets/payloads.
// ---------------------------------------------------------------------------

// simpleTextPostingsEnum reads full posting information including positions,
// offsets, and payloads from the postings stream.
//
// Port of SimpleTextFieldsReader.SimpleTextPostingsEnum.
type simpleTextPostingsEnum struct {
	inStart       store.IndexInput
	in            store.IndexInput
	docID         int
	tf            int
	scratch       *util.BytesRefBuilder
	pos           int
	payload       []byte
	nextDocStart  int64
	readOffsets   bool
	readPositions bool
	startOffset   int
	endOffset     int
	cost_         int
	skipReader    *SimpleTextSkipReader
	nextSkipDoc   int
	seekTo        int64
}

func newSimpleTextPostingsEnum(r *SimpleTextFieldsReader) *simpleTextPostingsEnum {
	inClone := r.in.Clone()
	return &simpleTextPostingsEnum{
		inStart:    r.in,
		in:         inClone,
		scratch:    util.NewBytesRefBuilder(),
		skipReader: NewSimpleTextSkipReader(r.in.Clone()),
	}
}

func (e *simpleTextPostingsEnum) canReuse(in store.IndexInput) bool { return in == e.inStart }

func (e *simpleTextPostingsEnum) reset(
	fp int64,
	indexOptions index.IndexOptions,
	docFreq int,
	skipPointer int64,
) (*simpleTextPostingsEnum, error) {
	e.nextDocStart = fp
	e.docID = -1
	e.readPositions = indexOptions >= index.IndexOptionsDocsAndFreqsAndPositions
	e.readOffsets = indexOptions >= index.IndexOptionsDocsAndFreqsAndPositionsAndOffsets
	if !e.readOffsets {
		e.startOffset = -1
		e.endOffset = -1
	}
	e.cost_ = docFreq
	if err := e.skipReader.Reset(skipPointer, docFreq); err != nil {
		return nil, err
	}
	e.nextSkipDoc = 0
	e.seekTo = -1
	return e, nil
}

func (e *simpleTextPostingsEnum) DocID() int { return e.docID }

func (e *simpleTextPostingsEnum) Freq() (int, error) { return e.tf, nil }

func (e *simpleTextPostingsEnum) NextDoc() (int, error) {
	return e.Advance(e.docID + 1)
}

func (e *simpleTextPostingsEnum) readDoc() (int, error) {
	first := true
	if err := e.in.SetPosition(e.nextDocStart); err != nil {
		return 0, err
	}
	var posStart int64
	for {
		lineStart := e.in.GetFilePointer()
		if err := stReadLine(e.in, e.scratch); err != nil {
			return 0, err
		}
		line := e.scratch.Bytes()[:e.scratch.Length()]

		if bytes.HasPrefix(line, stfwDoc) {
			if !first {
				e.nextDocStart = lineStart
				if err := e.in.SetPosition(posStart); err != nil {
					return 0, err
				}
				return e.docID, nil
			}
			docID, err := strconv.Atoi(string(line[len(stfwDoc):]))
			if err != nil {
				return 0, fmt.Errorf("simpleTextPostingsEnum: parse docID: %w", err)
			}
			e.docID = docID
			e.tf = 0
			first = false
		} else if bytes.HasPrefix(line, stfwFreq) {
			f, err := strconv.Atoi(string(line[len(stfwFreq):]))
			if err != nil {
				return 0, fmt.Errorf("simpleTextPostingsEnum: parse freq: %w", err)
			}
			e.tf = f
			posStart = e.in.GetFilePointer()
		} else if bytes.HasPrefix(line, stfwPos) ||
			bytes.HasPrefix(line, stfwStartOffset) ||
			bytes.HasPrefix(line, stfwEndOffset) ||
			bytes.HasPrefix(line, stfwPayload) {
			// skip
		} else {
			// SKIP_LIST, TERM, FIELD, or END
			if !first {
				e.nextDocStart = lineStart
				if err := e.in.SetPosition(posStart); err != nil {
					return 0, err
				}
				return e.docID, nil
			}
			e.docID = index.NO_MORE_DOCS
			return e.docID, nil
		}
	}
}

func (e *simpleTextPostingsEnum) advanceTarget(target int) (int, error) {
	if e.seekTo > 0 {
		e.nextDocStart = e.seekTo
		e.seekTo = -1
	}
	for {
		doc, err := e.readDoc()
		if err != nil {
			return 0, err
		}
		if doc >= target {
			return doc, nil
		}
	}
}

func (e *simpleTextPostingsEnum) AdvanceShallow(target int) error {
	if target > e.nextSkipDoc {
		_, err := e.skipReader.SkipTo(target)
		if err != nil {
			return err
		}
		if e.skipReader.GetNextSkipDoc() != index.NO_MORE_DOCS {
			e.seekTo = e.skipReader.GetNextSkipDocFP()
		}
		e.nextSkipDoc = e.skipReader.GetNextSkipDoc()
	}
	return nil
}

func (e *simpleTextPostingsEnum) Advance(target int) (int, error) {
	if err := e.AdvanceShallow(target); err != nil {
		return 0, err
	}
	return e.advanceTarget(target)
}

func (e *simpleTextPostingsEnum) NextPosition() (int, error) {
	if e.readPositions {
		if err := stReadLine(e.in, e.scratch); err != nil {
			return 0, err
		}
		line := e.scratch.Bytes()[:e.scratch.Length()]
		if !bytes.HasPrefix(line, stfwPos) {
			return 0, fmt.Errorf("simpleTextPostingsEnum.NextPosition: expected pos line, got: %s", line)
		}
		p, err := strconv.Atoi(string(line[len(stfwPos):]))
		if err != nil {
			return 0, err
		}
		e.pos = p
	} else {
		e.pos = -1
	}

	if e.readOffsets {
		if err := stReadLine(e.in, e.scratch); err != nil {
			return 0, err
		}
		soLine := e.scratch.Bytes()[:e.scratch.Length()]
		if !bytes.HasPrefix(soLine, stfwStartOffset) {
			return 0, fmt.Errorf("simpleTextPostingsEnum.NextPosition: expected startOffset, got: %s", soLine)
		}
		so, err := strconv.Atoi(string(soLine[len(stfwStartOffset):]))
		if err != nil {
			return 0, err
		}
		e.startOffset = so

		if err := stReadLine(e.in, e.scratch); err != nil {
			return 0, err
		}
		eoLine := e.scratch.Bytes()[:e.scratch.Length()]
		if !bytes.HasPrefix(eoLine, stfwEndOffset) {
			return 0, fmt.Errorf("simpleTextPostingsEnum.NextPosition: expected endOffset, got: %s", eoLine)
		}
		eo, err := strconv.Atoi(string(eoLine[len(stfwEndOffset):]))
		if err != nil {
			return 0, err
		}
		e.endOffset = eo
	}

	fp := e.in.GetFilePointer()
	if err := stReadLine(e.in, e.scratch); err != nil {
		return 0, err
	}
	line := e.scratch.Bytes()[:e.scratch.Length()]
	if bytes.HasPrefix(line, stfwPayload) {
		payloadBytes := line[len(stfwPayload):]
		buf := make([]byte, len(payloadBytes))
		copy(buf, payloadBytes)
		e.payload = buf
	} else {
		e.payload = nil
		if err := e.in.SetPosition(fp); err != nil {
			return 0, err
		}
	}
	return e.pos, nil
}

func (e *simpleTextPostingsEnum) StartOffset() (int, error) { return e.startOffset, nil }

func (e *simpleTextPostingsEnum) EndOffset() (int, error) { return e.endOffset, nil }

func (e *simpleTextPostingsEnum) GetPayload() ([]byte, error) { return e.payload, nil }

func (e *simpleTextPostingsEnum) Cost() int64 { return int64(e.cost_) }

func (e *simpleTextPostingsEnum) GetImpacts() (index.Impacts, error) {
	if err := e.AdvanceShallow(e.docID); err != nil {
		return nil, err
	}
	return e.skipReader.GetImpacts(), nil
}

// compile-time assertion: simpleTextPostingsEnum implements index.ImpactsEnum.
var _ index.ImpactsEnum = (*simpleTextPostingsEnum)(nil)
