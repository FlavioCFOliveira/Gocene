// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"errors"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// FreqProxFields is the Go port of org.apache.lucene.index.FreqProxFields
// from Apache Lucene 10.4.0. It exposes a limited (iterators only, no stats)
// Fields view over a slice of FreqProxTermsWriterPerField instances so the
// active PostingsFormat can flush the in-RAM buffered fields/terms/postings
// for the current segment.
//
// The Lucene type is package-private and consumed exclusively by
// FreqProxTermsWriter.flush. Gocene exports it because the codec layer lives
// in a sibling package; the contract is identical to Lucene: per-term
// statistics (docFreq, totalTermFreq, sum*Freq, docCount) are not buffered
// in RAM and surface as errors, mirroring Java's UnsupportedOperationException.
//
// Divergences from Lucene 10.4.0:
//
//   - Iterator/Terms surface follows Gocene's Fields/Terms/TermsEnum
//     interfaces (Term-based seeks) instead of Lucene's BytesRef-based ones.
//     The term scratch is exposed through a fresh *Term per Next/SeekCeil call
//     so the caller can hold on to results across the iterator's lifetime.
//
//   - Stats methods (Size, GetDocCount, GetSumDocFreq, GetSumTotalTermFreq,
//     DocFreq, TotalTermFreq) return ErrFreqProxFieldsUnsupported instead of
//     panicking; this lets callers test for the condition without recovering.
//
//   - Term bytes are read back through BytesRefHash.Get rather than
//     constructing a BytesRefBlockPool over TermsHashPerField.bytePool.
//     Lucene's BytesRefBlockPool wraps the stream pool, but BytesRefHash
//     stores the term bytes in termBytePool, and the standard TermsHash
//     happens to make the two pools identical. Gocene's FreqProxTermsHash
//     keeps them as distinct construction arguments (see freshFreqProxPools
//     in tests), so we defensively read through the same pool the hash
//     interned into. The result is byte-for-byte identical in the standard
//     pipeline and correct for non-standard pool wirings.
//
//   - SeekStatus is the local index package SeekStatus (Found/NotFound/End).
//     Gocene's TermsEnum.SeekCeil returns (*Term, error) rather than a status
//     enum; the seek outcome is exposed through SeekStatus() on the
//     FreqProxTermsEnum so callers performing flush-time iteration can mirror
//     Lucene's three-way branching exactly.
//
//   - ImpactsEnum is not yet ported in Gocene, so Impacts returns
//     ErrFreqProxFieldsUnsupported.
//
//   - TermState is a placeholder OrdTermState (Gocene's package default)
//     whose CopyFrom rejects any copy, mirroring Lucene's anonymous TermState
//     subclass that throws on copyFrom.
type FreqProxFields struct {
	FieldsBase

	// fields is keyed by field name and preserves the insertion order
	// supplied to NewFreqProxFields (which is the sorted-by-name order
	// produced by FreqProxTermsWriter, identical to Lucene's contract).
	fields     map[string]*FreqProxTermsWriterPerField
	fieldOrder []string
}

// ErrFreqProxFieldsUnsupported is returned by stats accessors that Lucene's
// FreqProxFields throws UnsupportedOperationException for. The in-RAM buffer
// does not carry per-term statistics, and recomputing them at flush time
// would require an extra pass over the postings.
var ErrFreqProxFieldsUnsupported = errors.New("FreqProxFields: operation not supported on in-RAM postings")

// NewFreqProxFields wraps a slice of per-field writers in a Fields view.
// The caller must supply the writers in field-name order, mirroring the
// invariant FreqProxTermsWriter establishes before invoking the constructor
// in Lucene ("NOTE: fields are already sorted by field name").
//
// Passing a nil or empty slice yields a Fields view with no fields; calling
// Terms on any name returns (nil, nil), matching Lucene's null-on-miss.
func NewFreqProxFields(fieldList []*FreqProxTermsWriterPerField) *FreqProxFields {
	f := &FreqProxFields{
		fields:     make(map[string]*FreqProxTermsWriterPerField, len(fieldList)),
		fieldOrder: make([]string, 0, len(fieldList)),
	}
	for _, field := range fieldList {
		if field == nil {
			continue
		}
		name := field.GetFieldName()
		if _, dup := f.fields[name]; dup {
			// Lucene's LinkedHashMap silently overwrites on duplicates and
			// keeps the original insertion position. Mirror that to keep the
			// iteration order stable, but record the new writer.
			f.fields[name] = field
			continue
		}
		f.fields[name] = field
		f.fieldOrder = append(f.fieldOrder, name)
	}
	return f
}

// Iterator returns a FieldIterator over the field names in the order they
// were supplied to NewFreqProxFields.
func (f *FreqProxFields) Iterator() (FieldIterator, error) {
	// Copy the slice so the iterator is decoupled from later mutations and
	// from subsequent Iterator calls (each iterator owns its cursor).
	names := make([]string, len(f.fieldOrder))
	copy(names, f.fieldOrder)
	return NewMemoryFieldIterator(names), nil
}

// Size always returns -1: Lucene throws UnsupportedOperationException here,
// and Gocene's FieldsBase contract uses -1 to signal "unknown".
func (f *FreqProxFields) Size() int {
	return -1
}

// Terms returns a Terms view over the requested field, or (nil, nil) when
// the field was not buffered for this segment.
func (f *FreqProxFields) Terms(field string) (Terms, error) {
	perField, ok := f.fields[field]
	if !ok {
		return nil, nil
	}
	return newFreqProxTerms(perField), nil
}

// FreqProxTerms is the Terms view over a single per-field writer. It is
// returned by FreqProxFields.Terms and is the Go port of the nested
// FreqProxFields.FreqProxTerms class from Lucene 10.4.0.
type FreqProxTerms struct {
	TermsBase

	terms *FreqProxTermsWriterPerField
}

func newFreqProxTerms(terms *FreqProxTermsWriterPerField) *FreqProxTerms {
	return &FreqProxTerms{terms: terms}
}

// GetIterator returns a FreqProxTermsEnum pre-positioned before the first
// term. The returned enumerator is independent of any prior iterator over
// the same FreqProxTerms.
func (t *FreqProxTerms) GetIterator() (TermsEnum, error) {
	return newFreqProxTermsEnum(t.terms), nil
}

// GetIteratorWithSeek returns a FreqProxTermsEnum positioned at the seek
// term (or after it). If seekTerm is nil, the enumerator is positioned
// before the first term, mirroring GetIterator.
func (t *FreqProxTerms) GetIteratorWithSeek(seekTerm *Term) (TermsEnum, error) {
	enum := newFreqProxTermsEnum(t.terms)
	if seekTerm == nil {
		return enum, nil
	}
	if _, err := enum.SeekCeil(seekTerm); err != nil {
		return nil, err
	}
	if enum.SeekStatus() == SeekStatusEnd {
		// Lucene's contract returns the enumerator regardless of status.
		// Gocene's Terms.GetIteratorWithSeek contract requires returning nil
		// when there are no more terms at or after the seek key.
		return nil, nil
	}
	return enum, nil
}

// GetPostingsReader looks up termText and, on a match, returns a postings
// enumerator with the requested flags. Returns (nil, nil) when the term is
// not present.
func (t *FreqProxTerms) GetPostingsReader(termText string, flags int) (PostingsEnum, error) {
	enum := newFreqProxTermsEnum(t.terms)
	found, err := enum.SeekExact(NewTerm(t.terms.GetFieldName(), termText))
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, nil
	}
	return enum.Postings(flags)
}

// Size is not buffered in RAM; callers must derive it elsewhere.
func (t *FreqProxTerms) Size() int64 { return -1 }

// GetDocCount, GetSumDocFreq, GetSumTotalTermFreq mirror Lucene's
// UnsupportedOperationException by returning ErrFreqProxFieldsUnsupported.
func (t *FreqProxTerms) GetDocCount() (int, error) {
	return 0, ErrFreqProxFieldsUnsupported
}

func (t *FreqProxTerms) GetSumDocFreq() (int64, error) {
	return 0, ErrFreqProxFieldsUnsupported
}

func (t *FreqProxTerms) GetSumTotalTermFreq() (int64, error) {
	return 0, ErrFreqProxFieldsUnsupported
}

// HasFreqs reports whether term frequencies were buffered for this field.
func (t *FreqProxTerms) HasFreqs() bool {
	return t.terms.IndexOptions >= IndexOptionsDocsAndFreqs
}

// HasOffsets reports whether offsets were buffered for this field. As in
// Lucene, the in-memory buffer may have been downgraded relative to what
// FieldInfo originally requested; this getter reflects the live state.
func (t *FreqProxTerms) HasOffsets() bool {
	return t.terms.IndexOptions >= IndexOptionsDocsAndFreqsAndPositionsAndOffsets
}

// HasPositions reports whether positions were buffered for this field. See
// HasOffsets for the downgrade caveat.
func (t *FreqProxTerms) HasPositions() bool {
	return t.terms.IndexOptions >= IndexOptionsDocsAndFreqsAndPositions
}

// HasPayloads reports whether any non-empty payload was observed during
// indexing of this segment, mirroring Lucene's sawPayloads flag.
func (t *FreqProxTerms) HasPayloads() bool {
	return t.terms.sawPayloads
}

// GetMin and GetMax are not buffered in RAM; callers must enumerate.
func (t *FreqProxTerms) GetMin() (*Term, error) {
	return nil, ErrFreqProxFieldsUnsupported
}

func (t *FreqProxTerms) GetMax() (*Term, error) {
	return nil, ErrFreqProxFieldsUnsupported
}

// FreqProxTermsEnum is the TermsEnum over the sorted-by-bytes per-field
// term list. It is the Go port of FreqProxFields.FreqProxTermsEnum.
type FreqProxTermsEnum struct {
	BaseTermsEnum

	terms         *FreqProxTermsWriterPerField
	sortedTermIDs []int
	postingsArray *FreqProxPostingsArray
	numTerms      int

	// scratchRef holds the BytesRef view of the current term, refilled in
	// place by readTermAt to mirror Lucene's BytesRef scratch reuse.
	scratchRef util.BytesRef

	// scratchTerm wraps scratchRef in the public *Term view returned by
	// Next / SeekCeil. The slice underneath scratchRef is a live window into
	// the term pool; callers that need to retain the term across Next calls
	// must Clone the result.
	scratchTerm *Term

	ord        int
	seekStatus SeekStatus
}

func newFreqProxTermsEnum(terms *FreqProxTermsWriterPerField) *FreqProxTermsEnum {
	// SortTerms collapses the BytesRefHash and caches the sorted IDs. Lucene
	// performs the sort inside FreqProxTermsWriter before constructing the
	// Fields view; Gocene's FreqProxTermsWriter is not ported yet so we
	// trigger the sort lazily here, mirroring the contract on first iterator
	// access. Repeated TermsEnum allocations on the same Terms instance reuse
	// the cached sortedTermIDs because TermsHashPerField only resorts after
	// Reset/ReinitHash. We read sortedTermIDs directly to avoid the panic in
	// GetSortedTermIDs when SortTerms has not run yet.
	if terms.TermsHashPerField.sortedTermIDs == nil {
		terms.SortTerms()
	}
	e := &FreqProxTermsEnum{
		terms:         terms,
		sortedTermIDs: terms.GetSortedTermIDs(),
		postingsArray: terms.postingsArray,
		numTerms:      terms.GetNumTerms(),
		ord:           -1,
		seekStatus:    SeekStatusNotFound,
	}
	return e
}

// SeekStatus reports the outcome of the most recent SeekCeil call. The value
// is SeekStatusNotFound after construction and after a successful Next.
func (e *FreqProxTermsEnum) SeekStatus() SeekStatus {
	return e.seekStatus
}

// readTermAt fills scratchRef and scratchTerm with the term at sortedTermIDs[ord].
func (e *FreqProxTermsEnum) readTermAt(ord int) {
	termID := e.sortedTermIDs[ord]
	// BytesRefHash.Get fills scratchRef in place against the same pool
	// where the term was originally interned.
	e.terms.bytesHash.Get(termID, &e.scratchRef)
	// Build a *Term wrapping the live window; CompareTo / Equals / Text on
	// the returned Term consume the scratchRef directly without copying.
	// Callers must Clone when they need to hold the term across Next calls.
	e.scratchTerm = NewTermFromBytesRef(e.terms.GetFieldName(), &e.scratchRef)
	e.SetCurrentTerm(e.scratchTerm)
}

// Next advances the enumerator one term. Returns nil when exhausted.
func (e *FreqProxTermsEnum) Next() (*Term, error) {
	e.ord++
	if e.ord >= e.numTerms {
		e.SetCurrentTerm(nil)
		return nil, nil
	}
	e.readTermAt(e.ord)
	e.seekStatus = SeekStatusNotFound
	return e.scratchTerm, nil
}

// SeekCeil moves the enumerator to seekTerm (or the next term after it).
// Returns the current term, or nil if the seek key is greater than all
// buffered terms. Mirrors Lucene's binary search.
func (e *FreqProxTermsEnum) SeekCeil(seekTerm *Term) (*Term, error) {
	if seekTerm == nil {
		// Lucene rejects null; Gocene's TermsEnum.SeekCeil contract treats
		// nil as "position before the first term", which is what Next on a
		// fresh enumerator already does.
		e.ord = -1
		e.SetCurrentTerm(nil)
		e.seekStatus = SeekStatusNotFound
		return e.Next()
	}

	target := seekTerm.GetBytesRef()
	lo := 0
	hi := e.numTerms - 1
	for hi >= lo {
		mid := int(uint(lo+hi) >> 1)
		e.readTermAt(mid)
		cmp := util.BytesRefCompare(&e.scratchRef, target)
		switch {
		case cmp < 0:
			lo = mid + 1
		case cmp > 0:
			hi = mid - 1
		default:
			// found
			e.ord = mid
			e.seekStatus = SeekStatusFound
			return e.scratchTerm, nil
		}
	}

	// not found
	e.ord = lo
	if e.ord >= e.numTerms {
		e.SetCurrentTerm(nil)
		e.seekStatus = SeekStatusEnd
		return nil, nil
	}
	e.readTermAt(e.ord)
	e.seekStatus = SeekStatusNotFound
	return e.scratchTerm, nil
}

// SeekExact reports whether the term was found. Mirrors Lucene's
// BaseTermsEnum default implementation.
func (e *FreqProxTermsEnum) SeekExact(term *Term) (bool, error) {
	got, err := e.SeekCeil(term)
	if err != nil {
		return false, err
	}
	if got == nil {
		return false, nil
	}
	return e.seekStatus == SeekStatusFound, nil
}

// SeekExactOrd positions the enumerator at the given ordinal, mirroring
// Lucene's seekExact(long). The ordinal is the sorted position, not the
// raw term ID. Out-of-range values panic, matching Java's
// ArrayIndexOutOfBoundsException.
func (e *FreqProxTermsEnum) SeekExactOrd(ord int64) {
	if ord < 0 || ord >= int64(e.numTerms) {
		panic(fmt.Sprintf("FreqProxTermsEnum: ord %d out of range [0,%d)", ord, e.numTerms))
	}
	e.ord = int(ord)
	e.readTermAt(e.ord)
	e.seekStatus = SeekStatusFound
}

// Ord returns the current sorted ordinal. -1 before the first Next call.
func (e *FreqProxTermsEnum) Ord() int64 {
	return int64(e.ord)
}

// DocFreq is not buffered in RAM; mirrors Lucene's
// UnsupportedOperationException as ErrFreqProxFieldsUnsupported.
func (e *FreqProxTermsEnum) DocFreq() (int, error) {
	return 0, ErrFreqProxFieldsUnsupported
}

// TotalTermFreq is not buffered in RAM; see DocFreq.
func (e *FreqProxTermsEnum) TotalTermFreq() (int64, error) {
	return 0, ErrFreqProxFieldsUnsupported
}

// Postings returns a PostingsEnum for the current term, configured by flags.
// flags use the Lucene PostingsEnum feature mask (FREQS=1<<3, POSITIONS=
// FREQS|1<<4, OFFSETS=POSITIONS|1<<5, PAYLOADS=POSITIONS|1<<6).
//
// Mirrors Lucene's postings(reuse, flags) without the reuse argument since
// Go's escape analysis already permits caller-side pooling around the
// returned enumerator.
func (e *FreqProxTermsEnum) Postings(flags int) (PostingsEnum, error) {
	if e.ord < 0 || e.ord >= e.numTerms {
		return nil, fmt.Errorf("FreqProxTermsEnum: not positioned")
	}
	if postingsFlagRequested(flags, postingsFlagPositions) {
		if !e.terms.hasProx {
			return nil, errors.New("FreqProxTermsEnum: did not index positions")
		}
		if !e.terms.hasOffsets && postingsFlagRequested(flags, postingsFlagOffsets) {
			return nil, errors.New("FreqProxTermsEnum: did not index offsets")
		}
		posEnum := newFreqProxPostingsEnum(e.terms, e.postingsArray)
		if err := posEnum.reset(e.sortedTermIDs[e.ord]); err != nil {
			return nil, err
		}
		return posEnum, nil
	}
	if !e.terms.hasFreq && postingsFlagRequested(flags, postingsFlagFreqs) {
		return nil, errors.New("FreqProxTermsEnum: did not index freq")
	}
	docsEnum := newFreqProxDocsEnum(e.terms, e.postingsArray)
	if err := docsEnum.reset(e.sortedTermIDs[e.ord]); err != nil {
		return nil, err
	}
	return docsEnum, nil
}

// PostingsWithLiveDocs ignores liveDocs (the in-RAM buffer pre-dates
// deletions) and delegates to Postings, matching Lucene where the FreqProx
// path is only consumed before any live-docs filter is applied.
func (e *FreqProxTermsEnum) PostingsWithLiveDocs(liveDocs util.Bits, flags int) (PostingsEnum, error) {
	return e.Postings(flags)
}

// TermState returns a placeholder TermState whose CopyFrom rejects any
// copy. Mirrors Lucene's anonymous TermState subclass.
func (e *FreqProxTermsEnum) TermState() (TermState, error) {
	return &freqProxTermState{}, nil
}

// Impacts is not supported on the in-RAM buffer; mirrors Lucene's
// UnsupportedOperationException.
func (e *FreqProxTermsEnum) Impacts(flags int) error {
	return ErrFreqProxFieldsUnsupported
}

// freqProxTermState mirrors the anonymous TermState returned by Lucene's
// FreqProxTermsEnum.termState. CopyFrom always rejects, matching Lucene.
type freqProxTermState struct{}

func (s *freqProxTermState) CopyFrom(other TermState) error {
	return ErrFreqProxFieldsUnsupported
}

func (s *freqProxTermState) String() string { return "FreqProxTermState" }

// postingsFlag* mirror the public flag constants on Lucene's PostingsEnum.
// They are package-private here because Gocene has not yet centralised the
// flag bag in PostingsEnum; the values are identical to Lucene's so any
// future migration only swaps the symbol, not the bit layout.
const (
	postingsFlagFreqs     = 1 << 3
	postingsFlagPositions = postingsFlagFreqs | 1<<4
	postingsFlagOffsets   = postingsFlagPositions | 1<<5
	postingsFlagPayloads  = postingsFlagPositions | 1<<6
	postingsFlagAll       = postingsFlagOffsets | postingsFlagPayloads //nolint:unused // exported for symmetry with Lucene; kept for future codec wiring.
)

// postingsFlagRequested mirrors PostingsEnum.featureRequested. Both flags
// and feature are widened to int for ergonomics; the bitwise operation is
// identical to the Java short-typed version.
func postingsFlagRequested(flags, feature int) bool {
	return (flags & feature) == feature
}

// freqProxDocsEnum encodes the docs-only stream (stream 0) for a single
// term. It is the Go port of FreqProxFields.FreqProxDocsEnum.
type freqProxDocsEnum struct {
	PostingsEnumBase

	terms         *FreqProxTermsWriterPerField
	postingsArray *FreqProxPostingsArray
	reader        *ByteSliceReader
	readTermFreq  bool

	freq   int
	ended  bool
	termID int
}

func newFreqProxDocsEnum(terms *FreqProxTermsWriterPerField, postingsArray *FreqProxPostingsArray) *freqProxDocsEnum {
	return &freqProxDocsEnum{
		PostingsEnumBase: PostingsEnumBase{CurrentDoc: -1},
		terms:            terms,
		postingsArray:    postingsArray,
		reader:           &ByteSliceReader{},
		readTermFreq:     terms.hasFreq,
	}
}

func (d *freqProxDocsEnum) reset(termID int) error {
	d.termID = termID
	if err := d.terms.InitReader(d.reader, termID, 0); err != nil {
		return fmt.Errorf("freqProxDocsEnum.reset: %w", err)
	}
	d.ended = false
	d.CurrentDoc = -1
	d.freq = 0
	return nil
}

func (d *freqProxDocsEnum) Freq() (int, error) {
	// Lucene throws IllegalStateException; Gocene returns an error so the
	// caller can decide whether the misuse is fatal.
	if !d.readTermFreq {
		return 0, errors.New("freqProxDocsEnum: freq was not indexed")
	}
	return d.freq, nil
}

func (d *freqProxDocsEnum) NextPosition() (int, error) {
	// Lucene returns -1 sentinel; mirror it to allow code that uses
	// NextPosition to "drain" docs without positions, but Gocene's
	// NO_MORE_POSITIONS is also -1, so callers see the same value.
	return -1, nil
}

func (d *freqProxDocsEnum) StartOffset() (int, error) { return -1, nil }
func (d *freqProxDocsEnum) EndOffset() (int, error)   { return -1, nil }
func (d *freqProxDocsEnum) GetPayload() ([]byte, error) {
	return nil, nil
}

func (d *freqProxDocsEnum) NextDoc() (int, error) {
	if d.CurrentDoc == -1 {
		d.CurrentDoc = 0
	}
	if d.reader.EOF() {
		if d.ended {
			d.CurrentDoc = NO_MORE_DOCS
			return NO_MORE_DOCS, nil
		}
		d.ended = true
		d.CurrentDoc = d.postingsArray.LastDocIDs[d.termID]
		if d.readTermFreq {
			d.freq = d.postingsArray.TermFreqs[d.termID]
		}
		return d.CurrentDoc, nil
	}

	code, err := d.reader.readVInt()
	if err != nil {
		return 0, fmt.Errorf("freqProxDocsEnum.NextDoc: %w", err)
	}
	if !d.readTermFreq {
		d.CurrentDoc += int(uint32(code))
	} else {
		d.CurrentDoc += int(uint32(code) >> 1)
		if (code & 1) != 0 {
			d.freq = 1
		} else {
			f, err := d.reader.readVInt()
			if err != nil {
				return 0, fmt.Errorf("freqProxDocsEnum.NextDoc: %w", err)
			}
			d.freq = int(f)
		}
	}
	return d.CurrentDoc, nil
}

// Advance is not supported; Lucene throws UnsupportedOperationException.
func (d *freqProxDocsEnum) Advance(target int) (int, error) {
	return 0, ErrFreqProxFieldsUnsupported
}

// Cost is not supported; Lucene throws UnsupportedOperationException.
func (d *freqProxDocsEnum) Cost() int64 {
	// Returning 0 is the closest equivalent of an Unsupported call given
	// Cost's int64 signature; callers wanting parity should treat 0 as
	// "unknown".
	return 0
}

// freqProxPostingsEnum encodes the docs + positions (+ optional offsets and
// payloads) streams for a single term. It is the Go port of
// FreqProxFields.FreqProxPostingsEnum.
type freqProxPostingsEnum struct {
	PostingsEnumBase

	terms         *FreqProxTermsWriterPerField
	postingsArray *FreqProxPostingsArray
	reader        *ByteSliceReader
	posReader     *ByteSliceReader
	readOffsets   bool

	freq        int
	pos         int
	startOffset int
	endOffset   int
	posLeft     int
	termID      int
	ended       bool
	hasPayload  bool
	payload     *util.BytesRefBuilder
}

func newFreqProxPostingsEnum(terms *FreqProxTermsWriterPerField, postingsArray *FreqProxPostingsArray) *freqProxPostingsEnum {
	return &freqProxPostingsEnum{
		PostingsEnumBase: PostingsEnumBase{CurrentDoc: -1},
		terms:            terms,
		postingsArray:    postingsArray,
		reader:           &ByteSliceReader{},
		posReader:        &ByteSliceReader{},
		readOffsets:      terms.hasOffsets,
		payload:          util.NewBytesRefBuilder(),
	}
}

func (p *freqProxPostingsEnum) reset(termID int) error {
	p.termID = termID
	if err := p.terms.InitReader(p.reader, termID, 0); err != nil {
		return fmt.Errorf("freqProxPostingsEnum.reset doc stream: %w", err)
	}
	if err := p.terms.InitReader(p.posReader, termID, 1); err != nil {
		return fmt.Errorf("freqProxPostingsEnum.reset pos stream: %w", err)
	}
	p.ended = false
	p.CurrentDoc = -1
	p.posLeft = 0
	p.startOffset = 0
	p.pos = 0
	p.freq = 0
	p.hasPayload = false
	return nil
}

func (p *freqProxPostingsEnum) Freq() (int, error) { return p.freq, nil }

func (p *freqProxPostingsEnum) NextDoc() (int, error) {
	if p.CurrentDoc == -1 {
		p.CurrentDoc = 0
	}
	// Drain any positions left over from the previous doc so the position
	// stream cursor is aligned with the next doc's prox payload prefix.
	for p.posLeft != 0 {
		if _, err := p.NextPosition(); err != nil {
			return 0, err
		}
	}

	if p.reader.EOF() {
		if p.ended {
			p.CurrentDoc = NO_MORE_DOCS
			return NO_MORE_DOCS, nil
		}
		p.ended = true
		p.CurrentDoc = p.postingsArray.LastDocIDs[p.termID]
		p.freq = p.postingsArray.TermFreqs[p.termID]
	} else {
		code, err := p.reader.readVInt()
		if err != nil {
			return 0, fmt.Errorf("freqProxPostingsEnum.NextDoc: %w", err)
		}
		p.CurrentDoc += int(uint32(code) >> 1)
		if (code & 1) != 0 {
			p.freq = 1
		} else {
			f, err := p.reader.readVInt()
			if err != nil {
				return 0, fmt.Errorf("freqProxPostingsEnum.NextDoc: %w", err)
			}
			p.freq = int(f)
		}
	}

	p.posLeft = p.freq
	p.pos = 0
	p.startOffset = 0
	return p.CurrentDoc, nil
}

// Advance is not supported; Lucene throws UnsupportedOperationException.
func (p *freqProxPostingsEnum) Advance(target int) (int, error) {
	return 0, ErrFreqProxFieldsUnsupported
}

// Cost is not supported; Lucene throws UnsupportedOperationException. See
// freqProxDocsEnum.Cost for the choice of 0.
func (p *freqProxPostingsEnum) Cost() int64 {
	return 0
}

func (p *freqProxPostingsEnum) NextPosition() (int, error) {
	if p.posLeft <= 0 {
		return NO_MORE_POSITIONS, nil
	}
	p.posLeft--
	code, err := p.posReader.readVInt()
	if err != nil {
		return 0, fmt.Errorf("freqProxPostingsEnum.NextPosition: %w", err)
	}
	p.pos += int(uint32(code) >> 1)
	if (code & 1) != 0 {
		p.hasPayload = true
		plen, err := p.posReader.readVInt()
		if err != nil {
			return 0, fmt.Errorf("freqProxPostingsEnum.NextPosition payload length: %w", err)
		}
		p.payload.SetLength(int(plen))
		p.payload.GrowNoCopy(int(plen))
		if int(plen) > 0 {
			buf := p.payload.Bytes()[:int(plen)]
			if err := p.posReader.ReadBytes(buf); err != nil {
				return 0, fmt.Errorf("freqProxPostingsEnum.NextPosition payload bytes: %w", err)
			}
		}
	} else {
		p.hasPayload = false
	}

	if p.readOffsets {
		so, err := p.posReader.readVInt()
		if err != nil {
			return 0, fmt.Errorf("freqProxPostingsEnum.NextPosition start offset: %w", err)
		}
		eo, err := p.posReader.readVInt()
		if err != nil {
			return 0, fmt.Errorf("freqProxPostingsEnum.NextPosition end offset: %w", err)
		}
		p.startOffset += int(so)
		p.endOffset = p.startOffset + int(eo)
	}
	return p.pos, nil
}

func (p *freqProxPostingsEnum) StartOffset() (int, error) {
	if !p.readOffsets {
		return 0, errors.New("freqProxPostingsEnum: offsets were not indexed")
	}
	return p.startOffset, nil
}

func (p *freqProxPostingsEnum) EndOffset() (int, error) {
	if !p.readOffsets {
		return 0, errors.New("freqProxPostingsEnum: offsets were not indexed")
	}
	return p.endOffset, nil
}

func (p *freqProxPostingsEnum) GetPayload() ([]byte, error) {
	if !p.hasPayload {
		return nil, nil
	}
	ref := p.payload.Get()
	if ref == nil || ref.Length == 0 {
		return nil, nil
	}
	// Return the live window. Callers that need to retain the payload across
	// further NextPosition calls must copy the slice; that mirrors Lucene
	// where BytesRefBuilder.get() returns a BytesRef over the live buffer.
	return ref.Bytes[ref.Offset : ref.Offset+ref.Length], nil
}
