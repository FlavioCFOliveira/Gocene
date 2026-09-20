// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package memory

import (
	"errors"
	"fmt"
	"sort"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/automaton"
	gfst "github.com/FlavioCFOliveira/Gocene/util/fst"
)

// FSTTermsReader is the FST-based terms dictionary reader.
//
// The FST directly maps each term and its metadata, it is memory resident.
//
// Port of org.apache.lucene.codecs.memory.FSTTermsReader
// (FSTTermsReader.java:66), which extends FieldsProducer.
type FSTTermsReader struct {
	// fields renders `private final TreeMap<String, TermsReader> fields`
	// (FSTTermsReader.java:67). Go has no sorted map, so the TreeMap's order is
	// applied by sorting the key set wherever Java's iteration order shows.
	fields map[string]*fstTermsReader
	// postingsReader renders `private final PostingsReaderBase postingsReader`
	// (FSTTermsReader.java:68).
	postingsReader codecs.PostingsReaderBase
	// fstTermsInput renders `private final IndexInput fstTermsInput`
	// (FSTTermsReader.java:69).
	fstTermsInput store.IndexInput
}

// NewFSTTermsReader renders FSTTermsReader(SegmentReadState, PostingsReaderBase)
// (FSTTermsReader.java:71).
func NewFSTTermsReader(state *index.SegmentReadState, postingsReader codecs.PostingsReaderBase) (*FSTTermsReader, error) {
	termsFileName := index.SegmentFileName(
		state.SegmentInfo.Name(), state.SegmentSuffix, termsExtension)

	// Java hands openInput `state.context.withHints(FileTypeHint.DATA,
	// PreloadHint.INSTANCE)`; Gocene's [spi.IOContext] carries no hint list, so
	// the context travels unchanged.
	fstTermsInput, err := state.Directory.OpenInput(termsFileName, state.Context)
	if err != nil {
		return nil, err
	}

	in := fstTermsInput
	success := false
	defer func() {
		if !success {
			_ = in.Close()
		}
	}()

	if _, err := codecs.CheckIndexHeader(
		in,
		termsCodecName,
		termsVersionStart,
		termsVersionCurrent,
		state.SegmentInfo.GetID(),
		state.SegmentSuffix,
	); err != nil {
		return nil, err
	}
	if _, err := codecs.ChecksumEntireFile(in); err != nil {
		return nil, err
	}
	if err := postingsReader.Init(in, state); err != nil {
		return nil, err
	}
	if err := fstTermsReaderSeekDir(in); err != nil {
		return nil, err
	}

	r := &FSTTermsReader{
		fields:         make(map[string]*fstTermsReader),
		postingsReader: postingsReader,
		fstTermsInput:  fstTermsInput,
	}

	fieldInfos := state.FieldInfos
	numFields, err := in.ReadVInt()
	if err != nil {
		return nil, err
	}
	for i := int32(0); i < numFields; i++ {
		// Java holds fieldNumber and docCount in ints, which is exactly what
		// readVInt yields; both are widened to Go's int only where the callee
		// takes a Java int too.
		fieldNumber, err := in.ReadVInt()
		if err != nil {
			return nil, err
		}
		fieldInfo := fieldInfos.FieldInfoByNumber(int(fieldNumber))
		numTerms, err := in.ReadVLong()
		if err != nil {
			return nil, err
		}
		sumTotalTermFreq, err := in.ReadVLong()
		if err != nil {
			return nil, err
		}
		// if frequencies are omitted, sumTotalTermFreq=sumDocFreq and we only write one value
		sumDocFreq := sumTotalTermFreq
		if fieldInfo.IndexOptions() != index.IndexOptionsDocs {
			sumDocFreq, err = in.ReadVLong()
			if err != nil {
				return nil, err
			}
		}
		docCount, err := in.ReadVInt()
		if err != nil {
			return nil, err
		}
		current, err := newFSTTermsFieldReader(
			r, fieldInfo, in, numTerms, sumTotalTermFreq, sumDocFreq, int(docCount))
		if err != nil {
			return nil, err
		}
		previous := r.fields[fieldInfo.Name()]
		r.fields[fieldInfo.Name()] = current
		if err := checkFieldSummary(state.SegmentInfo, current, previous); err != nil {
			return nil, err
		}
	}
	success = true
	return r, nil
}

// fstTermsReaderSeekDir renders FSTTermsReader.seekDir(IndexInput)
// (FSTTermsReader.java:121):
//
//	in.seek(in.length() - CodecUtil.footerLength() - 8);
//	in.seek(in.readLong());
func fstTermsReaderSeekDir(in store.IndexInput) error {
	if err := in.SetPosition(in.Length() - int64(codecs.FooterLength()) - 8); err != nil {
		return err
	}
	offset, err := in.ReadLong()
	if err != nil {
		return err
	}
	return in.SetPosition(offset)
}

// checkFieldSummary renders FSTTermsReader.checkFieldSummary(SegmentInfo,
// IndexInput, TermsReader, TermsReader) (FSTTermsReader.java:126). Java passes
// the IndexInput only to build the CorruptIndexException's resource name, which
// Gocene's error values do not carry.
func checkFieldSummary(info *index.SegmentInfo, field, previous *fstTermsReader) error {
	// #docs with field must be <= #docs
	if field.docCount < 0 || field.docCount > info.MaxDoc() {
		return fmt.Errorf("invalid docCount: %d maxDoc: %d", field.docCount, info.MaxDoc())
	}
	// #postings must be >= #docs with field
	if field.sumDocFreq < int64(field.docCount) {
		return fmt.Errorf("invalid sumDocFreq: %d docCount: %d", field.sumDocFreq, field.docCount)
	}
	// #positions must be >= #postings
	if field.sumTotalTermFreq < field.sumDocFreq {
		return fmt.Errorf("invalid sumTotalTermFreq: %d sumDocFreq: %d",
			field.sumTotalTermFreq, field.sumDocFreq)
	}
	if previous != nil {
		return fmt.Errorf("duplicate fields: %s", field.fieldInfo.Name())
	}
	return nil
}

// Iterator renders FSTTermsReader.iterator() (FSTTermsReader.java:153), which
// walks the key set of the TreeMap and is therefore sorted.
func (r *FSTTermsReader) Iterator() (spi.FieldIterator, error) {
	names := make([]string, 0, len(r.fields))
	for name := range r.fields {
		names = append(names, name)
	}
	sort.Strings(names)
	return spi.NewMemoryFieldIterator(names), nil
}

// Terms renders FSTTermsReader.terms(String) (FSTTermsReader.java:158):
// `return fields.get(field)`. The leading `assert field != null` is disabled at
// runtime unless the JVM is started with -ea.
func (r *FSTTermsReader) Terms(field string) (spi.Terms, error) {
	if tr, ok := r.fields[field]; ok {
		return tr, nil
	}
	return nil, nil
}

// Size renders FSTTermsReader.size() (FSTTermsReader.java:164):
// `return fields.size()`.
func (r *FSTTermsReader) Size() int { return len(r.fields) }

// Close renders FSTTermsReader.close() (FSTTermsReader.java:169): close the
// postings reader and the terms input, then clear the field map either way.
func (r *FSTTermsReader) Close() error {
	var firstErr error
	if r.postingsReader != nil {
		if err := r.postingsReader.Close(); err != nil {
			firstErr = err
		}
	}
	if r.fstTermsInput != nil {
		if err := r.fstTermsInput.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	r.fields = make(map[string]*fstTermsReader)
	return firstErr
}

// CheckIntegrity renders FSTTermsReader.checkIntegrity()
// (FSTTermsReader.java:793): `postingsReader.checkIntegrity()`.
func (r *FSTTermsReader) CheckIntegrity() error { return r.postingsReader.CheckIntegrity() }

// GetMergeInstance returns the receiver: FSTTermsReader does not override
// FieldsProducer.getMergeInstance(), whose default returns `this`.
func (r *FSTTermsReader) GetMergeInstance() spi.FieldsProducer { return r }

// String renders FSTTermsReader.toString() (FSTTermsReader.java:783).
func (r *FSTTermsReader) String() string {
	return fmt.Sprintf("FSTTermsReader(fields=%d,delegate=%v)", len(r.fields), r.postingsReader)
}

// ─── TermsReader ─────────────────────────────────────────────────────────────

// fstTermsReader is the Terms of one field.
//
// Port of the inner class FSTTermsReader.TermsReader (FSTTermsReader.java:177),
// whose enclosing instance supplies postingsReader; Go has no implicit outer
// instance, so the back-pointer is carried explicitly in parent.
type fstTermsReader struct {
	// parent renders the implicit FSTTermsReader.this of the Java inner class.
	parent *FSTTermsReader

	// fieldInfo renders `final FieldInfo fieldInfo` (FSTTermsReader.java:179).
	fieldInfo *index.FieldInfo
	// numTerms renders `final long numTerms` (FSTTermsReader.java:180).
	numTerms int64
	// sumTotalTermFreq renders `final long sumTotalTermFreq`
	// (FSTTermsReader.java:181).
	sumTotalTermFreq int64
	// sumDocFreq renders `final long sumDocFreq` (FSTTermsReader.java:182).
	sumDocFreq int64
	// docCount renders `final int docCount` (FSTTermsReader.java:183).
	docCount int
	// dict renders `final FST<FSTTermOutputs.TermData> dict`
	// (FSTTermsReader.java:184).
	dict *gfst.FST[*TermData]
}

// newFSTTermsFieldReader renders TermsReader(FieldInfo, IndexInput, long, long,
// long, int) (FSTTermsReader.java:186).
func newFSTTermsFieldReader(
	parent *FSTTermsReader,
	fieldInfo *index.FieldInfo,
	in store.IndexInput,
	numTerms, sumTotalTermFreq, sumDocFreq int64,
	docCount int,
) (*fstTermsReader, error) {
	tr := &fstTermsReader{
		parent:           parent,
		fieldInfo:        fieldInfo,
		numTerms:         numTerms,
		sumTotalTermFreq: sumTotalTermFreq,
		sumDocFreq:       sumDocFreq,
		docCount:         docCount,
	}
	outputs := NewFSTTermOutputs(fieldInfo)
	fstMetadata, err := gfst.ReadMetadata[*TermData](in, outputs)
	if err != nil {
		return nil, err
	}
	filePointer := in.GetFilePointer()

	// Java: `new OffHeapFSTStore(in, in.getFilePointer(), fstMetadata)`, whose
	// constructor takes in.randomAccessSlice(offset, numBytes). Gocene's
	// IndexInput does not expose randomAccessSlice, so the store is built from
	// the input when it is itself a RandomAccessInput — the same test the
	// block-tree field reader makes — and the on-heap loader is used otherwise.
	if rai, ok := in.(store.RandomAccessInput); ok {
		offHeap, storeErr := gfst.NewOffHeapFSTStore(rai, filePointer, fstMetadata.NumBytes())
		if storeErr == nil {
			tr.dict, err = gfst.FromFSTReader(fstMetadata, offHeap)
			if err != nil {
				return nil, err
			}
			if err := in.SkipBytes(offHeap.Size()); err != nil {
				return nil, err
			}
			return tr, nil
		}
	}

	clone := in.Clone()
	if err := clone.SetPosition(filePointer); err != nil {
		_ = clone.Close()
		return nil, err
	}
	tr.dict, err = gfst.NewFSTFromDataInput[*TermData](fstMetadata, clone)
	if err != nil {
		_ = clone.Close()
		return nil, err
	}
	_ = clone.Close()
	if err := in.SkipBytes(fstMetadata.NumBytes()); err != nil {
		return nil, err
	}
	return tr, nil
}

// String renders TermsReader.toString() (FSTTermsReader.java:207).
func (tr *fstTermsReader) String() string {
	return fmt.Sprintf("FSTTerms(terms=%d,postings=%d,positions=%d,docs=%d)",
		tr.numTerms, tr.sumDocFreq, tr.sumTotalTermFreq, tr.docCount)
}

// Field returns the name of the field this Terms instance represents.
//
// org.apache.lucene.index.Terms declares no field() accessor, so Java reads
// TermsReader.fieldInfo directly (FSTTermsReader.java:179); Gocene's
// [spi.Terms] contract does declare one, and it is answered from the same
// FieldInfo.
func (tr *fstTermsReader) Field() string { return tr.fieldInfo.Name() }

// HasFreqs renders TermsReader.hasFreqs() (FSTTermsReader.java:220).
func (tr *fstTermsReader) HasFreqs() bool {
	return tr.fieldInfo.IndexOptions().Subsumes(index.IndexOptionsDocsAndFreqs)
}

// HasOffsets renders TermsReader.hasOffsets() (FSTTermsReader.java:225).
func (tr *fstTermsReader) HasOffsets() bool {
	return tr.fieldInfo.IndexOptions().Subsumes(index.IndexOptionsDocsAndFreqsAndPositionsAndOffsets)
}

// HasPositions renders TermsReader.hasPositions() (FSTTermsReader.java:232).
func (tr *fstTermsReader) HasPositions() bool {
	return tr.fieldInfo.IndexOptions().Subsumes(index.IndexOptionsDocsAndFreqsAndPositions)
}

// HasPayloads renders TermsReader.hasPayloads() (FSTTermsReader.java:237).
func (tr *fstTermsReader) HasPayloads() bool { return tr.fieldInfo.HasPayloads() }

// Size renders TermsReader.size() (FSTTermsReader.java:242).
func (tr *fstTermsReader) Size() int64 { return tr.numTerms }

// GetSumTotalTermFreq renders TermsReader.getSumTotalTermFreq()
// (FSTTermsReader.java:247). Java declares no checked exception on it; Gocene's
// [spi.Terms] carries the error return on every statistic.
func (tr *fstTermsReader) GetSumTotalTermFreq() (int64, error) { return tr.sumTotalTermFreq, nil }

// GetSumDocFreq renders TermsReader.getSumDocFreq() (FSTTermsReader.java:252).
func (tr *fstTermsReader) GetSumDocFreq() (int64, error) { return tr.sumDocFreq, nil }

// GetDocCount renders TermsReader.getDocCount() (FSTTermsReader.java:257).
func (tr *fstTermsReader) GetDocCount() (int, error) { return tr.docCount, nil }

// Iterator renders TermsReader.iterator() (FSTTermsReader.java:262):
// `return new SegmentTermsEnum()`.
func (tr *fstTermsReader) Iterator() (spi.TermsEnum, error) {
	return newFSTSegmentTermsEnum(tr)
}

// Intersect renders TermsReader.intersect(CompiledAutomaton, BytesRef)
// (FSTTermsReader.java:267). Java's start term is a bare BytesRef; Gocene's
// [spi.Terms] carries it as a *Term (field + bytes), so the bytes are unwrapped
// before they reach IntersectTermsEnum.
func (tr *fstTermsReader) Intersect(compiled *automaton.CompiledAutomaton, startTerm *spi.Term) (spi.TermsEnum, error) {
	if compiled.Type != automaton.AutomatonTypeNormal {
		return nil, errors.New("please use CompiledAutomaton.getTermsEnum instead")
	}
	var start *util.BytesRef
	if startTerm != nil {
		start = startTerm.BytesValue()
	}
	return newFSTIntersectTermsEnum(tr, compiled, start)
}

// GetIteratorWithSeek returns an iterator positioned at or after seekTerm.
//
// org.apache.lucene.index.Terms declares no such member, so TermsReader
// overrides nothing here; the Gocene [spi.Terms] contract does declare it, and
// it is answered with the two Lucene operations a Java caller would spell out —
// Terms.iterator() followed by TermsEnum.seekCeil(BytesRef).
func (tr *fstTermsReader) GetIteratorWithSeek(seekTerm *spi.Term) (spi.TermsEnum, error) {
	te, err := tr.Iterator()
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
func (tr *fstTermsReader) GetPostingsReader(termText string, flags int) (spi.PostingsEnum, error) {
	te, err := tr.Iterator()
	if err != nil {
		return nil, err
	}
	found, err := te.SeekExact(spi.NewTerm(tr.fieldInfo.Name(), termText))
	if err != nil || !found {
		return nil, err
	}
	return te.Postings(flags)
}

// GetMin is the default org.apache.lucene.index.Terms#getMin()
// (Terms.java:143) that TermsReader inherits: `return iterator().next()`.
func (tr *fstTermsReader) GetMin() (*spi.Term, error) {
	te, err := tr.Iterator()
	if err != nil {
		return nil, err
	}
	return te.Next()
}

// GetMax is the default org.apache.lucene.index.Terms#getMax()
// (Terms.java:153) that TermsReader inherits. Java first tries a seek-by-ord
// and falls back to a digit-by-digit binary search when the enumerator throws
// UnsupportedOperationException; TermsReader.BaseTermsEnum.seekExact(long)
// (FSTTermsReader.java:322) always does, so the ord attempt is spelled out as
// the branch it always takes: straight to the binary search.
func (tr *fstTermsReader) GetMax() (*spi.Term, error) {
	if tr.Size() == 0 {
		// empty: only possible from a FilteredTermsEnum...
		return nil, nil
	}

	// otherwise: binary search
	iterator, err := tr.Iterator()
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
			term, err := iterator.SeekCeil(spi.NewTermFromBytes(tr.fieldInfo.Name(), scratch))
			if err != nil {
				return nil, err
			}
			if term == nil {
				// Scratch was too high
				if mid == 0 {
					scratch = scratch[:len(scratch)-1]
					return spi.NewTermFromBytes(tr.fieldInfo.Name(), scratch), nil
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

// ─── BaseTermsEnum ───────────────────────────────────────────────────────────

// errFSTTermsEnumUnsupported is the sentinel behind the
// UnsupportedOperationException thrown by TermsReader.BaseTermsEnum's
// seekExact(long) and ord() (FSTTermsReader.java:322, :324).
var errFSTTermsEnumUnsupported = errors.New("memory: FSTTermsReader.BaseTermsEnum: operation is not supported")

// ErrFSTTermsEnumUnsupported is the exported sentinel for errors.Is.
var ErrFSTTermsEnumUnsupported = errFSTTermsEnumUnsupported

// fstBaseTermsEnum wraps the common operations both enumerators need in order
// to talk to the postings format.
//
// Port of the abstract inner class TermsReader.BaseTermsEnum
// (FSTTermsReader.java:275), which extends org.apache.lucene.index.BaseTermsEnum.
type fstBaseTermsEnum struct {
	// TermsEnumBase renders the `extends org.apache.lucene.index.BaseTermsEnum`
	// half of the declaration: it carries the lazily created AttributeSource
	// behind BaseTermsEnum.attributes().
	spi.TermsEnumBase

	// tr renders the implicit TermsReader.this of the Java inner class.
	tr *fstTermsReader

	// termStateRef is the same term state as PostingsReaderBase sees it: the
	// interface value whose dynamic type is the codec's own BlockTermState
	// subclass, which the codec narrows back with a type assertion. state is
	// the widened BlockTermState view of that very object.
	termStateRef index.TermState
	// state renders `final BlockTermState state` (FSTTermsReader.java:278):
	// current term stats plus decoded metadata, customized by the PBF.
	state *codecs.BlockTermState
	// meta renders `FSTTermOutputs.TermData meta` (FSTTermsReader.java:281):
	// current term stats plus undecoded metadata.
	meta *TermData
	// bytesReader renders `ByteArrayDataInput bytesReader`
	// (FSTTermsReader.java:282).
	bytesReader *store.ByteArrayDataInput

	// decodeMetaData renders the abstract `void decodeMetaData()`
	// (FSTTermsReader.java:285). Go has no abstract methods, so the concrete
	// enumerator installs its body here for the shared members below to call.
	decodeMetaData func() error
}

// newFSTBaseTermsEnum renders BaseTermsEnum() (FSTTermsReader.java:287).
// NOTE: metadata will only be initialized in child class.
func newFSTBaseTermsEnum(tr *fstTermsReader) *fstBaseTermsEnum {
	termStateRef := tr.parent.postingsReader.NewTermState()
	return &fstBaseTermsEnum{
		tr:           tr,
		termStateRef: termStateRef,
		state:        codecs.BaseState(termStateRef),
		bytesReader:  store.NewByteArrayDataInput(nil),
	}
}

// TermState renders BaseTermsEnum.termState() (FSTTermsReader.java:294):
// `decodeMetaData(); return state.clone()`.
func (e *fstBaseTermsEnum) TermState() (index.TermState, error) {
	if err := e.decodeMetaData(); err != nil {
		return nil, err
	}
	return e.state.Clone(), nil
}

// DocFreq renders BaseTermsEnum.docFreq() (FSTTermsReader.java:300):
// `return state.docFreq`.
func (e *fstBaseTermsEnum) DocFreq() (int, error) { return e.state.DocFreq, nil }

// TotalTermFreq renders BaseTermsEnum.totalTermFreq()
// (FSTTermsReader.java:305):
// `return state.totalTermFreq == -1 ? state.docFreq : state.totalTermFreq`.
func (e *fstBaseTermsEnum) TotalTermFreq() (int64, error) {
	if e.state.TotalTermFreq == -1 {
		return int64(e.state.DocFreq), nil
	}
	return e.state.TotalTermFreq, nil
}

// Postings renders BaseTermsEnum.postings(PostingsEnum, int)
// (FSTTermsReader.java:310):
// `decodeMetaData(); return postingsReader.postings(fieldInfo, state, reuse,
// flags)`. Gocene's [spi.TermsEnum] carries no reuse parameter, so nil is
// passed through to the postings reader.
func (e *fstBaseTermsEnum) Postings(flags int) (spi.PostingsEnum, error) {
	if err := e.decodeMetaData(); err != nil {
		return nil, err
	}
	return e.tr.parent.postingsReader.Postings(e.tr.fieldInfo, e.termStateRef, nil, flags)
}

// PostingsWithLiveDocs returns the same postings as Postings:
// org.apache.lucene.index.TermsEnum declares no live-docs-aware overload, so
// the live docs are applied by the caller.
func (e *fstBaseTermsEnum) PostingsWithLiveDocs(liveDocs util.Bits, flags int) (spi.PostingsEnum, error) {
	return e.Postings(flags)
}

// Impacts renders BaseTermsEnum.impacts(int) (FSTTermsReader.java:316):
// `decodeMetaData(); return postingsReader.impacts(fieldInfo, state, flags)`.
func (e *fstBaseTermsEnum) Impacts(flags int) (spi.ImpactsEnum, error) {
	if err := e.decodeMetaData(); err != nil {
		return nil, err
	}
	return e.tr.parent.postingsReader.Impacts(e.tr.fieldInfo, e.termStateRef, flags)
}

// SeekExactOrd renders BaseTermsEnum.seekExact(long)
// (FSTTermsReader.java:322), whose body is
// `throw new UnsupportedOperationException()`.
func (e *fstBaseTermsEnum) SeekExactOrd(ord int64) error { return errFSTTermsEnumUnsupported }

// Ord renders BaseTermsEnum.ord() (FSTTermsReader.java:327), whose body is
// `throw new UnsupportedOperationException()`. Ord carries no error in the
// TermsEnum contract (Java's ord() declares no checked exception), so the
// unsupported call panics, mirroring the unchecked Java exception.
func (e *fstBaseTermsEnum) Ord() int64 { panic(errFSTTermsEnumUnsupported) }

// ─── SegmentTermsEnum ────────────────────────────────────────────────────────

// fstSegmentTermsEnum iterates through all terms in one field.
//
// Port of the private inner class TermsReader.SegmentTermsEnum
// (FSTTermsReader.java:333).
type fstSegmentTermsEnum struct {
	*fstBaseTermsEnum

	// term renders `BytesRef term` (FSTTermsReader.java:335): the current term,
	// null when the enum ends or is unpositioned.
	term *util.BytesRef
	// fstEnum renders `final BytesRefFSTEnum<FSTTermOutputs.TermData> fstEnum`
	// (FSTTermsReader.java:336).
	fstEnum *gfst.BytesRefFSTEnum[*TermData]
	// decoded renders `boolean decoded` (FSTTermsReader.java:339): true when
	// the current term's metadata is decoded.
	decoded bool
	// seekPending renders `boolean seekPending` (FSTTermsReader.java:341): true
	// when the enum was positioned by seekExact(TermState).
	seekPending bool
}

// newFSTSegmentTermsEnum renders SegmentTermsEnum()
// (FSTTermsReader.java:344).
func newFSTSegmentTermsEnum(tr *fstTermsReader) (*fstSegmentTermsEnum, error) {
	fstEnum, err := gfst.NewBytesRefFSTEnum[*TermData](tr.dict)
	if err != nil {
		return nil, err
	}
	e := &fstSegmentTermsEnum{
		fstBaseTermsEnum: newFSTBaseTermsEnum(tr),
		fstEnum:          fstEnum,
		decoded:          false,
		seekPending:      false,
	}
	e.meta = nil
	e.fstBaseTermsEnum.decodeMetaData = e.decodeMetaDataImpl
	return e, nil
}

// Term renders SegmentTermsEnum.term() (FSTTermsReader.java:353):
// `return term`. Gocene's [spi.Term] carries the field name too, which Java
// reads off the enclosing TermsReader.
func (e *fstSegmentTermsEnum) Term() *spi.Term {
	if e.term == nil {
		return nil
	}
	return spi.NewTermFromBytesRef(e.tr.fieldInfo.Name(), e.term)
}

// decodeMetaDataImpl renders SegmentTermsEnum.decodeMetaData()
// (FSTTermsReader.java:359): let the PBF decode metadata from the byte blob.
func (e *fstSegmentTermsEnum) decodeMetaDataImpl() error {
	if !e.decoded && !e.seekPending {
		if e.meta.Bytes != nil {
			e.bytesReader.ResetWithSlice(e.meta.Bytes, 0, len(e.meta.Bytes))
		}
		if err := e.tr.parent.postingsReader.DecodeTerm(
			e.bytesReader, e.tr.fieldInfo, e.termStateRef, true); err != nil {
			return err
		}
		e.decoded = true
	}
	return nil
}

// updateEnum renders SegmentTermsEnum.updateEnum(InputOutput)
// (FSTTermsReader.java:370): update the current term from the FST enumerator.
func (e *fstSegmentTermsEnum) updateEnum(pair *gfst.BytesRefInputOutput[*TermData]) {
	if pair == nil {
		e.term = nil
	} else {
		e.term = pair.Input
		e.meta = pair.Output
		e.state.DocFreq = e.meta.DocFreq
		e.state.TotalTermFreq = e.meta.TotalTermFreq
	}
	e.decoded = false
	e.seekPending = false
}

// Next renders SegmentTermsEnum.next() (FSTTermsReader.java:384). The `assert
// status == SeekStatus.FOUND` Java makes after the pending seek is disabled at
// runtime unless the JVM is started with -ea.
func (e *fstSegmentTermsEnum) Next() (*spi.Term, error) {
	if e.seekPending { // previously positioned, but termOutputs not fetched
		e.seekPending = false
		if _, err := e.seekCeilStatus(e.term); err != nil {
			return nil, err
		}
	}
	pair, err := e.fstEnum.Next()
	if err != nil {
		return nil, err
	}
	e.updateEnum(pair)
	return e.Term(), nil
}

// SeekExact renders SegmentTermsEnum.seekExact(BytesRef)
// (FSTTermsReader.java:395):
// `updateEnum(fstEnum.seekExact(target)); return term != null`.
func (e *fstSegmentTermsEnum) SeekExact(target *spi.Term) (bool, error) {
	pair, err := e.fstEnum.SeekExact(target.BytesValue())
	if err != nil {
		return false, err
	}
	e.updateEnum(pair)
	return e.term != nil, nil
}

// seekCeilStatus renders SegmentTermsEnum.seekCeil(BytesRef)
// (FSTTermsReader.java:401) with Java's SeekStatus return value, which Gocene's
// [spi.TermsEnum] does not carry.
func (e *fstSegmentTermsEnum) seekCeilStatus(target *util.BytesRef) (spi.SeekStatus, error) {
	pair, err := e.fstEnum.SeekCeil(target)
	if err != nil {
		return spi.SeekStatusEnd, err
	}
	e.updateEnum(pair)
	if e.term == nil {
		return spi.SeekStatusEnd, nil
	}
	if util.BytesRefCompare(e.term, target) == 0 {
		return spi.SeekStatusFound, nil
	}
	return spi.SeekStatusNotFound, nil
}

// SeekCeil reports SeekStatus.END as a nil term, which is how Gocene's
// [spi.TermsEnum] carries it.
func (e *fstSegmentTermsEnum) SeekCeil(target *spi.Term) (*spi.Term, error) {
	status, err := e.seekCeilStatus(target.BytesValue())
	if err != nil {
		return nil, err
	}
	if status == spi.SeekStatusEnd {
		return nil, nil
	}
	return e.Term(), nil
}

// SeekExactTermState renders SegmentTermsEnum.seekExact(BytesRef, TermState)
// (FSTTermsReader.java:411).
func (e *fstSegmentTermsEnum) SeekExactTermState(target *spi.Term, otherState index.TermState) error {
	bytes := target.BytesValue()
	if e.term == nil || util.BytesRefCompare(bytes, e.term) != 0 {
		if err := e.state.CopyFrom(otherState); err != nil {
			return err
		}
		e.term = bytes.Clone()
		e.seekPending = true
	}
	return nil
}

// ─── IntersectTermsEnum ──────────────────────────────────────────────────────

// fstIntersectFrame records how the current term was constructed, so the
// enumerator can accumulate metadata or rewind the term.
//
// Port of the private inner class IntersectTermsEnum.Frame
// (FSTTermsReader.java:449).
type fstIntersectFrame struct {
	// fstArc renders `FST.Arc<FSTTermOutputs.TermData> fstArc`
	// (FSTTermsReader.java:451).
	fstArc *gfst.Arc[*TermData]
	// output renders `FSTTermOutputs.TermData output`
	// (FSTTermsReader.java:453).
	output *TermData
	// fsaState renders `int fsaState` (FSTTermsReader.java:456).
	fsaState int
}

// newFSTIntersectFrame renders Frame() (FSTTermsReader.java:458).
func newFSTIntersectFrame() *fstIntersectFrame {
	return &fstIntersectFrame{fstArc: &gfst.Arc[*TermData]{}, fsaState: -1}
}

// String renders Frame.toString() (FSTTermsReader.java:464).
func (f *fstIntersectFrame) String() string {
	return fmt.Sprintf("arc=%v state=%d", f.fstArc, f.fsaState)
}

// fstIntersectTermsEnum iterates the intersection of the field's terms with an
// automaton. It cannot seek.
//
// Port of the private inner class TermsReader.IntersectTermsEnum
// (FSTTermsReader.java:421).
type fstIntersectTermsEnum struct {
	*fstBaseTermsEnum

	// term renders `BytesRefBuilder term` (FSTTermsReader.java:423): the
	// current term, null when the enum ends or is unpositioned.
	term *util.BytesRefBuilder
	// decoded renders `boolean decoded` (FSTTermsReader.java:425).
	decoded bool
	// pending renders `boolean pending` (FSTTermsReader.java:428): true when
	// there is a pending term at the time next() is called.
	pending bool

	// stack renders `Frame[] stack` (FSTTermsReader.java:434).
	stack []*fstIntersectFrame
	// level renders `int level` (FSTTermsReader.java:435).
	level int
	// metaUpto renders `int metaUpto` (FSTTermsReader.java:439): the level up
	// to which metadata has been accumulated.
	metaUpto int

	// fst renders `final FST<FSTTermOutputs.TermData> fst`
	// (FSTTermsReader.java:442).
	fst *gfst.FST[*TermData]
	// fstReader renders `final FST.BytesReader fstReader`
	// (FSTTermsReader.java:443).
	fstReader gfst.BytesReader
	// fstOutputs renders `final Outputs<FSTTermOutputs.TermData> fstOutputs`
	// (FSTTermsReader.java:444).
	fstOutputs gfst.Outputs[*TermData]

	// fsa renders `final ByteRunnable fsa` (FSTTermsReader.java:447): the query
	// automaton to intersect with.
	fsa automaton.ByteRunnable
}

// newFSTIntersectTermsEnum renders IntersectTermsEnum(CompiledAutomaton,
// BytesRef) (FSTTermsReader.java:469).
func newFSTIntersectTermsEnum(
	tr *fstTermsReader,
	compiled *automaton.CompiledAutomaton,
	startTerm *util.BytesRef,
) (*fstIntersectTermsEnum, error) {
	e := &fstIntersectTermsEnum{
		fstBaseTermsEnum: newFSTBaseTermsEnum(tr),
		fst:              tr.dict,
		fstOutputs:       tr.dict.Outputs(),
		fsa:              compiled.GetByteRunnable(),
		level:            -1,
		stack:            make([]*fstIntersectFrame, 16),
	}
	e.fstReader = e.fst.GetBytesReader()
	for i := range e.stack {
		e.stack[i] = newFSTIntersectFrame()
	}
	e.fstBaseTermsEnum.decodeMetaData = e.decodeMetaDataImpl

	e.loadVirtualFrame(e.newFrame())
	e.level++
	first, err := e.loadFirstFrame(e.newFrame())
	if err != nil {
		return nil, err
	}
	e.pushFrame(first)

	e.meta = nil
	e.metaUpto = 1
	e.decoded = false
	e.pending = false

	if startTerm == nil {
		e.pending = e.isAccept(e.topFrame())
	} else {
		if _, err := e.doSeekCeil(startTerm); err != nil {
			return nil, err
		}
		e.pending = (e.term == nil || util.BytesRefCompare(startTerm, e.term.Get()) != 0) &&
			e.isValid(e.topFrame()) && e.isAccept(e.topFrame())
	}
	return e, nil
}

// Term renders IntersectTermsEnum.term() (FSTTermsReader.java:503):
// `return term == null ? null : term.get()`. Gocene's [spi.Term] carries the
// field name too, which Java reads off the enclosing TermsReader.
func (e *fstIntersectTermsEnum) Term() *spi.Term {
	if e.term == nil {
		return nil
	}
	return spi.NewTermFromBytesRef(e.tr.fieldInfo.Name(), e.term.Get())
}

// decodeMetaDataImpl renders IntersectTermsEnum.decodeMetaData()
// (FSTTermsReader.java:508). The leading `assert term != null` is disabled at
// runtime unless the JVM is started with -ea.
func (e *fstIntersectTermsEnum) decodeMetaDataImpl() error {
	if !e.decoded {
		if e.meta.Bytes != nil {
			e.bytesReader.ResetWithSlice(e.meta.Bytes, 0, len(e.meta.Bytes))
		}
		if err := e.tr.parent.postingsReader.DecodeTerm(
			e.bytesReader, e.tr.fieldInfo, e.termStateRef, true); err != nil {
			return err
		}
		e.decoded = true
	}
	return nil
}

// loadMetaData renders IntersectTermsEnum.loadMetaData()
// (FSTTermsReader.java:520): lazily accumulate metadata once an accepted term
// has been reached.
func (e *fstIntersectTermsEnum) loadMetaData() {
	last := e.stack[e.metaUpto]
	for e.metaUpto != e.level {
		e.metaUpto++
		next := e.stack[e.metaUpto]
		next.output = e.fstOutputs.Add(next.output, last.output)
		last = next
	}
	if last.fstArc.IsFinal() {
		e.meta = e.fstOutputs.Add(last.output, last.fstArc.NextFinalOutput())
	} else {
		e.meta = last.output
	}
	e.state.DocFreq = e.meta.DocFreq
	e.state.TotalTermFreq = e.meta.TotalTermFreq
}

// seekCeilStatus renders IntersectTermsEnum.seekCeil(BytesRef)
// (FSTTermsReader.java:539) with Java's SeekStatus return value.
func (e *fstIntersectTermsEnum) seekCeilStatus(target *util.BytesRef) (spi.SeekStatus, error) {
	e.decoded = false
	if _, err := e.doSeekCeil(target); err != nil {
		return spi.SeekStatusEnd, err
	}
	e.loadMetaData()
	if e.term == nil {
		return spi.SeekStatusEnd, nil
	}
	if util.BytesRefCompare(e.term.Get(), target) == 0 {
		return spi.SeekStatusFound, nil
	}
	return spi.SeekStatusNotFound, nil
}

// SeekCeil reports SeekStatus.END as a nil term, which is how Gocene's
// [spi.TermsEnum] carries it.
func (e *fstIntersectTermsEnum) SeekCeil(target *spi.Term) (*spi.Term, error) {
	status, err := e.seekCeilStatus(target.BytesValue())
	if err != nil {
		return nil, err
	}
	if status == spi.SeekStatusEnd {
		return nil, nil
	}
	return e.Term(), nil
}

// SeekExact renders the default BaseTermsEnum.seekExact(BytesRef)
// (BaseTermsEnum.java:57): `return seekCeil(text) == SeekStatus.FOUND`.
// IntersectTermsEnum does not override it.
func (e *fstIntersectTermsEnum) SeekExact(target *spi.Term) (bool, error) {
	status, err := e.seekCeilStatus(target.BytesValue())
	if err != nil {
		return false, err
	}
	return status == spi.SeekStatusFound, nil
}

// Next renders IntersectTermsEnum.next() (FSTTermsReader.java:551): a
// depth-first walk of the FST restricted by the automaton.
func (e *fstIntersectTermsEnum) Next() (*spi.Term, error) {
	if e.pending {
		e.pending = false
		e.loadMetaData()
		return e.Term(), nil
	}
	e.decoded = false
	// DFS:
	for e.level > 0 {
		frame := e.newFrame()
		expanded, err := e.loadExpandFrame(e.topFrame(), frame)
		if err != nil {
			return nil, err
		}
		if expanded != nil { // has valid target
			e.pushFrame(frame)
			if e.isAccept(frame) { // gotcha
				break
			}
			continue // check next target
		}
		frame = e.popFrame()
		broke := false
		for e.level > 0 {
			next, err := e.loadNextFrame(e.topFrame(), frame)
			if err != nil {
				return nil, err
			}
			if next != nil { // has valid sibling
				e.pushFrame(frame)
				if e.isAccept(frame) { // gotcha — break DFS
					broke = true
					break
				}
				broke = true // continue DFS — check next target
				break
			}
			frame = e.popFrame()
		}
		if broke {
			if e.isAccept(e.topFrame()) {
				break
			}
			continue
		}
		return nil, nil
	}
	e.loadMetaData()
	return e.Term(), nil
}

// doSeekCeil renders IntersectTermsEnum.doSeekCeil(BytesRef)
// (FSTTermsReader.java:586). The `assert isValid(frame)` inside the prefix walk
// is disabled at runtime unless the JVM is started with -ea.
func (e *fstIntersectTermsEnum) doSeekCeil(target *util.BytesRef) (*spi.Term, error) {
	var frame *fstIntersectFrame
	upto := 0
	limit := target.Length
	for upto < limit { // to target prefix, or ceil label (rewind prefix)
		frame = e.newFrame()
		label := int(target.Bytes[target.Offset+upto]) & 0xff
		var err error
		frame, err = e.loadCeilFrame(label, e.topFrame(), frame)
		if err != nil {
			return nil, err
		}
		if frame == nil || frame.fstArc.Label() != label {
			break
		}
		e.pushFrame(frame)
		upto++
	}
	if upto == limit { // got target
		return e.Term(), nil
	}
	if frame != nil { // got larger term('s prefix)
		e.pushFrame(frame)
		if e.isAccept(frame) {
			return e.Term(), nil
		}
		return e.Next()
	}
	for e.level > 0 { // got target's prefix, advance to larger term
		frame = e.popFrame()
		for e.level > 0 && !e.canRewind(frame) {
			frame = e.popFrame()
		}
		next, err := e.loadNextFrame(e.topFrame(), frame)
		if err != nil {
			return nil, err
		}
		if next != nil {
			e.pushFrame(frame)
			if e.isAccept(frame) {
				return e.Term(), nil
			}
			return e.Next()
		}
	}
	return nil, nil
}

// loadVirtualFrame renders IntersectTermsEnum.loadVirtualFrame(Frame)
// (FSTTermsReader.java:622): the virtual frame, never popped.
func (e *fstIntersectTermsEnum) loadVirtualFrame(frame *fstIntersectFrame) *fstIntersectFrame {
	frame.output = e.fstOutputs.GetNoOutput()
	frame.fsaState = -1
	return frame
}

// loadFirstFrame renders IntersectTermsEnum.loadFirstFrame(Frame)
// (FSTTermsReader.java:629): the frame for the start arc (node) on the FST.
func (e *fstIntersectTermsEnum) loadFirstFrame(frame *fstIntersectFrame) (*fstIntersectFrame, error) {
	frame.fstArc = e.fst.GetFirstArc(frame.fstArc)
	frame.output = frame.fstArc.Output()
	frame.fsaState = 0
	return frame, nil
}

// loadExpandFrame renders IntersectTermsEnum.loadExpandFrame(Frame, Frame)
// (FSTTermsReader.java:637): the frame for the target arc (node) on the FST.
func (e *fstIntersectTermsEnum) loadExpandFrame(top, frame *fstIntersectFrame) (*fstIntersectFrame, error) {
	if !e.canGrow(top) {
		return nil, nil
	}
	arc, err := e.fst.ReadFirstRealTargetArc(top.fstArc.Target(), frame.fstArc, e.fstReader)
	if err != nil {
		return nil, err
	}
	frame.fstArc = arc
	frame.fsaState = e.fsa.Step(top.fsaState, frame.fstArc.Label())
	if frame.fsaState == -1 {
		return e.loadNextFrame(top, frame)
	}
	frame.output = frame.fstArc.Output()
	return frame, nil
}

// loadNextFrame renders IntersectTermsEnum.loadNextFrame(Frame, Frame)
// (FSTTermsReader.java:652): the frame for the sibling arc (node) on the FST.
func (e *fstIntersectTermsEnum) loadNextFrame(top, frame *fstIntersectFrame) (*fstIntersectFrame, error) {
	if !e.canRewind(frame) {
		return nil, nil
	}
	for !frame.fstArc.IsLast() {
		arc, err := e.fst.ReadNextRealArc(frame.fstArc, e.fstReader)
		if err != nil {
			return nil, err
		}
		frame.fstArc = arc
		frame.fsaState = e.fsa.Step(top.fsaState, frame.fstArc.Label())
		if frame.fsaState != -1 {
			break
		}
	}
	if frame.fsaState == -1 {
		return nil, nil
	}
	frame.output = frame.fstArc.Output()
	return frame, nil
}

// loadCeilFrame renders IntersectTermsEnum.loadCeilFrame(int, Frame, Frame)
// (FSTTermsReader.java:675): the frame for the target arc (node) on the FST,
// such that arc.label >= label and !fsa.reject(arc.label).
func (e *fstIntersectTermsEnum) loadCeilFrame(label int, top, frame *fstIntersectFrame) (*fstIntersectFrame, error) {
	arc, err := gfst.ReadCeilArc(label, e.fst, top.fstArc, frame.fstArc, e.fstReader)
	if err != nil {
		return nil, err
	}
	if arc == nil {
		return nil, nil
	}
	frame.fstArc = arc
	frame.fsaState = e.fsa.Step(top.fsaState, arc.Label())
	if frame.fsaState == -1 {
		return e.loadNextFrame(top, frame)
	}
	frame.output = frame.fstArc.Output()
	return frame, nil
}

// isAccept renders IntersectTermsEnum.isAccept(Frame)
// (FSTTermsReader.java:690): a term both fst and fsa accept.
func (e *fstIntersectTermsEnum) isAccept(frame *fstIntersectFrame) bool {
	return e.fsa.IsAccept(frame.fsaState) && frame.fstArc.IsFinal()
}

// isValid renders IntersectTermsEnum.isValid(Frame)
// (FSTTermsReader.java:694): a prefix neither fst nor fsa rejects.
func (e *fstIntersectTermsEnum) isValid(frame *fstIntersectFrame) bool {
	return frame.fsaState != -1
}

// canGrow renders IntersectTermsEnum.canGrow(Frame)
// (FSTTermsReader.java:698): can walk forward on both fst and fsa.
func (e *fstIntersectTermsEnum) canGrow(frame *fstIntersectFrame) bool {
	return frame.fsaState != -1 && gfst.TargetHasArcs(frame.fstArc)
}

// canRewind renders IntersectTermsEnum.canRewind(Frame)
// (FSTTermsReader.java:702): can jump to a sibling.
func (e *fstIntersectTermsEnum) canRewind(frame *fstIntersectFrame) bool {
	return !frame.fstArc.IsLast()
}

// pushFrame renders IntersectTermsEnum.pushFrame(Frame)
// (FSTTermsReader.java:706).
func (e *fstIntersectTermsEnum) pushFrame(frame *fstIntersectFrame) {
	e.term = e.grow(frame.fstArc.Label())
	e.level++
}

// popFrame renders IntersectTermsEnum.popFrame()
// (FSTTermsReader.java:712).
func (e *fstIntersectTermsEnum) popFrame() *fstIntersectFrame {
	e.term = e.shrink()
	e.level--
	if e.metaUpto > e.level {
		e.metaUpto = e.level
	}
	return e.stack[e.level+1]
}

// newFrame renders IntersectTermsEnum.newFrame()
// (FSTTermsReader.java:720).
func (e *fstIntersectTermsEnum) newFrame() *fstIntersectFrame {
	if e.level+1 == len(e.stack) {
		temp := make([]*fstIntersectFrame, util.Oversize(e.level+2, util.NumBytesObjectRef))
		copy(temp, e.stack)
		for i := len(e.stack); i < len(temp); i++ {
			temp[i] = newFSTIntersectFrame()
		}
		e.stack = temp
	}
	return e.stack[e.level+1]
}

// topFrame renders IntersectTermsEnum.topFrame()
// (FSTTermsReader.java:733).
func (e *fstIntersectTermsEnum) topFrame() *fstIntersectFrame { return e.stack[e.level] }

// grow renders IntersectTermsEnum.grow(int) (FSTTermsReader.java:737).
func (e *fstIntersectTermsEnum) grow(label int) *util.BytesRefBuilder {
	if e.term == nil {
		e.term = util.NewBytesRefBuilder()
	} else {
		e.term.AppendByte(byte(label))
	}
	return e.term
}

// shrink renders IntersectTermsEnum.shrink() (FSTTermsReader.java:746).
func (e *fstIntersectTermsEnum) shrink() *util.BytesRefBuilder {
	if e.term.Length() == 0 {
		e.term = nil
	} else {
		e.term.SetLength(e.term.Length() - 1)
	}
	return e.term
}

// walk breadth-first traverses every reachable arc of fst.
//
// Port of the package-private static helper FSTTermsReader.walk(FST<T>)
// (FSTTermsReader.java:757). Nothing in Apache Lucene 10.5.0 calls it — it is a
// debugging aid kept in the class — so it is carried across unchanged rather
// than dropped.
//
// Java's `new BitSet()` grows on demand; the growable []uint64 below is the
// rendering this repository already uses for java.util.BitSet (see
// automaton.Automaton.isAccept).
func walk[T any](f *gfst.FST[T]) error {
	queue := make([]*gfst.Arc[T], 0)
	var seen []uint64
	seenGet := func(node int64) bool {
		word := int(node >> 6)
		return word < len(seen) && seen[word]&(1<<uint(node&63)) != 0
	}
	seenSet := func(node int64) {
		word := int(node >> 6)
		if word >= len(seen) {
			grown := make([]uint64, word+1)
			copy(grown, seen)
			seen = grown
		}
		seen[word] |= 1 << uint(node&63)
	}

	reader := f.GetBytesReader()
	startArc := f.GetFirstArc(&gfst.Arc[T]{})
	queue = append(queue, startArc)
	for len(queue) > 0 {
		arc := queue[0]
		queue = queue[1:]
		node := arc.Target()
		if gfst.TargetHasArcs(arc) && !seenGet(node) {
			seenSet(node)
			var err error
			arc, err = f.ReadFirstRealTargetArc(node, arc, reader)
			if err != nil {
				return err
			}
			for {
				queue = append(queue, (&gfst.Arc[T]{}).CopyFrom(arc))
				if arc.IsLast() {
					break
				}
				arc, err = f.ReadNextRealArc(arc, reader)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// interface compliance
var (
	_ spi.FieldsProducer = (*FSTTermsReader)(nil)
	_ spi.Terms          = (*fstTermsReader)(nil)
	_ spi.TermsEnum      = (*fstSegmentTermsEnum)(nil)
	_ spi.TermsEnum      = (*fstIntersectTermsEnum)(nil)
)
