// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package uniformsplit

import (
	"errors"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// BlockReader seeks the block corresponding to a given term, reads the block
// bytes, and scans the block terms.
//
// Reads fully the block in blockReadBuffer. Then scans the block terms in
// memory. The details region is lazily decoded with termStatesReadBuffer which
// shares the same byte array with blockReadBuffer. See BlockLine for the block
// format.
//
// Mirrors org.apache.lucene.codecs.uniformsplit.BlockReader from Apache Lucene
// 10.5.0.
type BlockReader struct {
	index.BaseTermsEnum

	// blockInput is the IndexInput on the block file.
	blockInput store.IndexInput

	postingsReader codecs.PostingsReaderBase
	fieldMetadata  *FieldMetadata
	blockDecoder   BlockDecoder

	blockHeaderReader *BlockHeaderSerializer
	blockLineReader   *BlockLineSerializer

	// blockReadBuffer is the in-memory read buffer for the current block.
	blockReadBuffer *store.ByteArrayDataInput

	// termStatesReadBuffer is the in-memory read buffer for the details region
	// of the current block. It shares the same byte array as blockReadBuffer,
	// with a different position.
	termStatesReadBuffer *store.ByteArrayDataInput

	termStateSerializer *DeltaBaseTermStateSerializer

	// dictionaryBrowserSupplier is the IndexDictionaryBrowser supplier for lazy
	// loading.
	dictionaryBrowserSupplier IndexDictionaryBrowserSupplier

	// dictionaryBrowser holds the IndexDictionaryBrowser once loaded.
	dictionaryBrowser IndexDictionaryBrowser

	// blockStartFP is the current block start file pointer, absolute in the
	// block file.
	blockStartFP int64

	// blockHeader is the current block header.
	blockHeader *BlockHeader

	// blockLine is the current block line.
	blockLine *BlockLine

	// termState holds the current block line details.
	termState index.TermState

	// blockFirstLineStart is the offset of the start of the first line of the
	// current block (just after the header), relative to the block start.
	blockFirstLineStart int

	// lineIndexInBlock is the current line index in the block.
	lineIndexInBlock int32

	// termStateForced tells whether the current TermState has been forced with
	// a call to SeekExactWithState.
	termStateForced bool

	// forcedTerm is set when SeekExactWithState is called.
	//
	// This optimizes the use-case when the caller calls first
	// SeekExactWithState and then Postings. In this case we don't access the
	// terms block file (we don't seek) but directly the postings file because
	// we already have the TermState with the file pointers to the postings
	// file.
	forcedTerm *util.BytesRefBuilder

	// Scratch objects to avoid object reallocation.
	scratchBlockBytes *util.BytesRef
	scratchTermState  index.TermState
	scratchBlockLine  *BlockLine
}

// NewBlockReader constructs a BlockReader.
//
// dictionaryBrowserSupplier loads the IndexDictionaryBrowser lazily in
// SeekCeil. blockDecoder is an optional block decoder, may be nil if none; it
// can be used for decompression or decryption.
//
// Mirrors the protected BlockReader constructor (BlockReader.java:131).
func NewBlockReader(
	dictionaryBrowserSupplier IndexDictionaryBrowserSupplier,
	blockInput store.IndexInput,
	postingsReader codecs.PostingsReaderBase,
	fieldMetadata *FieldMetadata,
	blockDecoder BlockDecoder,
) (*BlockReader, error) {
	return &BlockReader{
		dictionaryBrowserSupplier: dictionaryBrowserSupplier,
		blockInput:                blockInput,
		postingsReader:            postingsReader,
		fieldMetadata:             fieldMetadata,
		blockDecoder:              blockDecoder,
		blockStartFP:              -1,
		scratchTermState:          postingsReader.NewTermState(),
	}, nil
}

// seekCeil mirrors BlockReader.seekCeil(BytesRef) (BlockReader.java:150).
func (r *BlockReader) seekCeil(searchedTerm *util.BytesRef) (spi.SeekStatus, error) {
	if r.isCurrentTerm(searchedTerm) {
		return spi.SeekStatusFound, nil
	}
	r.clearTermState()

	browser, err := r.getOrCreateDictionaryBrowser()
	if err != nil {
		return spi.SeekStatusEnd, err
	}
	blockStartFP, err := browser.SeekBlock(searchedTerm)
	if err != nil {
		return spi.SeekStatusEnd, err
	}
	blockStartFP = max(blockStartFP, r.fieldMetadata.GetFirstBlockStartFP())
	if r.isBeyondLastTerm(searchedTerm, blockStartFP) {
		return spi.SeekStatusEnd, nil
	}
	seekStatus, err := r.seekInBlockAt(searchedTerm, blockStartFP)
	if err != nil {
		return spi.SeekStatusEnd, err
	}
	if seekStatus != spi.SeekStatusEnd {
		return seekStatus, nil
	}
	// Go to next block.
	nextTerm, err := r.nextTerm()
	if err != nil {
		return spi.SeekStatusEnd, err
	}
	if nextTerm == nil {
		return spi.SeekStatusEnd, nil
	}
	return spi.SeekStatusNotFound, nil
}

// seekExact mirrors BlockReader.seekExact(BytesRef) (BlockReader.java:168).
func (r *BlockReader) seekExact(searchedTerm *util.BytesRef) (bool, error) {
	if r.isCurrentTerm(searchedTerm) {
		return true, nil
	}
	r.clearTermState()

	browser, err := r.getOrCreateDictionaryBrowser()
	if err != nil {
		return false, err
	}
	blockStartFP, err := browser.SeekBlock(searchedTerm)
	if err != nil {
		return false, err
	}
	if blockStartFP < r.fieldMetadata.GetFirstBlockStartFP() || r.isBeyondLastTerm(searchedTerm, blockStartFP) {
		return false, nil
	}
	seekStatus, err := r.seekInBlockAt(searchedTerm, blockStartFP)
	if err != nil {
		return false, err
	}
	return seekStatus == spi.SeekStatusFound, nil
}

// isCurrentTerm mirrors BlockReader.isCurrentTerm (BlockReader.java:182).
func (r *BlockReader) isCurrentTerm(searchedTerm *util.BytesRef) bool {
	// Optimization and also required to not search with the same BytesRef
	// instance as the BytesRef used to read the block line (BlockLineSerializer).
	// Indeed term() is allowed to return the same BytesRef instance.
	return util.BytesRefEquals(searchedTerm, r.term())
}

// isBeyondLastTerm indicates whether the searched term is beyond the last term
// of the field. blockStartFP is the current block start file pointer.
//
// Mirrors BlockReader.isBeyondLastTerm (BlockReader.java:193).
func (r *BlockReader) isBeyondLastTerm(searchedTerm *util.BytesRef, blockStartFP int64) bool {
	return blockStartFP == r.fieldMetadata.GetLastBlockStartFP() &&
		util.BytesRefCompare(searchedTerm, r.fieldMetadata.GetLastTerm()) > 0
}

// seekInBlockAt seeks to the provided term in the block starting at the
// provided file pointer. Does not exceed the block.
//
// Mirrors the overload BlockReader.seekInBlock(BytesRef, long)
// (BlockReader.java:202).
func (r *BlockReader) seekInBlockAt(searchedTerm *util.BytesRef, blockStartFP int64) (spi.SeekStatus, error) {
	if err := r.initializeHeader(searchedTerm, blockStartFP); err != nil {
		return spi.SeekStatusEnd, err
	}
	if r.blockHeader == nil {
		return spi.SeekStatusEnd, r.newCorruptIndexError("Illegal absence of block", &blockStartFP)
	}
	return r.seekInBlock(searchedTerm)
}

// seekInBlock seeks to the provided term in this block.
//
// Does not exceed this block; SeekStatusEnd is returned if it follows the
// block.
//
// Compares the line terms with searchedTerm, taking advantage of the
// incremental encoding properties.
//
// Scans linearly the terms. Updates the current block line with the current
// term.
//
// Mirrors the overload BlockReader.seekInBlock(BytesRef)
// (BlockReader.java:222).
func (r *BlockReader) seekInBlock(searchedTerm *util.BytesRef) (spi.SeekStatus, error) {
	compare, err := r.compareToMiddleAndJump(searchedTerm)
	if err != nil {
		return spi.SeekStatusEnd, err
	}
	if compare == 0 {
		return spi.SeekStatusFound, nil
	}
	comparisonOffset := 0
	for {
		line, err := r.readLineInBlock()
		if err != nil {
			return spi.SeekStatusEnd, err
		}
		if line == nil {
			// No more terms for the block.
			return spi.SeekStatusEnd, nil
		}
		lineTermBytes := r.blockLine.GetTermBytes()
		lineTerm := lineTermBytes.GetTerm()
		// assert lineTerm.offset == 0;

		// Equivalent to comparing with BytesRef.compareTo(),
		// but faster since we start comparing from min(comparisonOffset, suffixOffset).
		suffixOffset := lineTermBytes.GetSuffixOffset()
		start := min(comparisonOffset, suffixOffset)
		end := min(searchedTerm.Length, lineTerm.Length)
		comparison := searchedTerm.Length - lineTerm.Length
		for i := start; i < end; i++ {
			// Compare unsigned bytes.
			byteDiff := int(searchedTerm.Bytes[i+searchedTerm.Offset]) - int(lineTerm.Bytes[i])
			if byteDiff != 0 {
				comparison = byteDiff
				break
			}
			comparisonOffset = i + 1
		}
		if comparison == 0 {
			return spi.SeekStatusFound, nil
		} else if comparison < 0 {
			return spi.SeekStatusNotFound, nil
		}
	}
}

// compareToMiddleAndJump compares the searched term to the middle term of the
// block. If the searched term is lexicographically equal or after the middle
// term then jumps to the second half of the block directly.
//
// Returns the comparison between the searched term and the middle term.
//
// Mirrors BlockReader.compareToMiddleAndJump (BlockReader.java:272).
func (r *BlockReader) compareToMiddleAndJump(searchedTerm *util.BytesRef) (int, error) {
	if r.lineIndexInBlock != 0 {
		// Don't try to compare and jump if we are not positioned at the first line.
		// This can happen if we seek in the same current block and we continue
		// scanning from the current line (see initializeHeader()).
		return -1, nil
	}
	if err := r.blockReadBuffer.SkipBytes(int64(r.blockHeader.MiddleLineOffset())); err != nil {
		return 0, err
	}
	r.lineIndexInBlock = r.blockHeader.MiddleLineIndex()
	if _, err := r.readLineInBlock(); err != nil {
		return 0, err
	}
	if r.blockLine == nil {
		return 0, r.newCorruptIndexError("Illegal absence of line at the middle of the block", nil)
	}
	compare := util.BytesRefCompare(searchedTerm, r.term())
	if compare < 0 {
		r.blockReadBuffer.SetPosition(r.blockFirstLineStart)
		r.lineIndexInBlock = 0
	}
	return compare, nil
}

// readLineInBlock reads the current block line. Sets blockLine and increments
// lineIndexInBlock. Returns the BlockLine; or nil if there is no more line in
// the block.
//
// Mirrors BlockReader.readLineInBlock (BlockReader.java:297).
func (r *BlockReader) readLineInBlock() (*BlockLine, error) {
	if r.lineIndexInBlock >= r.blockHeader.LinesCount() {
		r.blockLine = nil
		return nil, nil
	}
	isIncrementalEncodingSeed := r.lineIndexInBlock == 0 || r.lineIndexInBlock == r.blockHeader.MiddleLineIndex()
	r.lineIndexInBlock++
	blockLine, err := r.blockLineReader.ReadLine(r.blockReadBuffer, isIncrementalEncodingSeed, r.scratchBlockLine)
	if err != nil {
		return nil, err
	}
	r.blockLine = blockLine
	return r.blockLine, nil
}

// nextTerm moves to the next term line and reads it, it may be in the next
// block. The term details are not read yet. They will be read only when needed
// with readTermStateIfNotRead.
//
// Returns the read term bytes; or nil if there is no more term for the field.
//
// Mirrors BlockReader.nextTerm (BlockReader.java:350).
func (r *BlockReader) nextTerm() (*util.BytesRef, error) {
	if r.blockHeader == nil {
		// Read the first block for the field.
		if err := r.initializeHeader(nil, r.fieldMetadata.GetFirstBlockStartFP()); err != nil {
			return nil, err
		}
		if r.blockHeader == nil {
			firstBlockStartFP := r.fieldMetadata.GetFirstBlockStartFP()
			return nil, r.newCorruptIndexError("Illegal absence of first block", &firstBlockStartFP)
		}
	}
	line, err := r.readLineInBlock()
	if err != nil {
		return nil, err
	}
	if line == nil {
		// No more line in the current block.
		// Read the next block starting at the current file pointer in the block file.
		if err := r.initializeHeader(nil, r.blockInput.GetFilePointer()); err != nil {
			return nil, err
		}
		if r.blockHeader == nil {
			// No more block for the field.
			return nil, nil
		}
		if _, err := r.readLineInBlock(); err != nil {
			return nil, err
		}
	}
	return r.term(), nil
}

// initializeHeader reads and sets blockHeader. Sets nil if there is no block
// for the field anymore.
//
// searchedTerm is the searched term, or nil if none. targetBlockStartFP is the
// file pointer of the block to read.
//
// Mirrors BlockReader.initializeHeader (BlockReader.java:376).
func (r *BlockReader) initializeHeader(searchedTerm *util.BytesRef, targetBlockStartFP int64) error {
	if err := r.initializeBlockReadLazily(); err != nil {
		return err
	}
	if r.blockStartFP == targetBlockStartFP {
		// Optimization: If the block to read is already the current block, then
		// reuse it directly without reading nor decoding the block bytes.
		if r.blockHeader == nil {
			return r.newCorruptIndexError("Illegal absence of block", &r.blockStartFP)
		}
		if searchedTerm == nil || r.blockLine == nil ||
			util.BytesRefCompare(searchedTerm, r.blockLine.GetTermBytes().GetTerm()) <= 0 {
			// If the searched term precedes lexicographically the current term,
			// then reset the position to the first term line of the block.
			// If the searched term equals the current term, we also need to reset
			// to scan again the current line.
			r.blockReadBuffer.SetPosition(r.blockFirstLineStart)
			r.lineIndexInBlock = 0
		}
	} else {
		if err := r.blockInput.SetPosition(targetBlockStartFP); err != nil {
			return err
		}
		r.blockStartFP = targetBlockStartFP
		if _, err := r.readHeader(); err != nil {
			return err
		}
		r.blockFirstLineStart = r.blockReadBuffer.GetPosition()
		r.lineIndexInBlock = 0
	}
	return nil
}

// initializeBlockReadLazily mirrors BlockReader.initializeBlockReadLazily
// (BlockReader.java:403).
func (r *BlockReader) initializeBlockReadLazily() error {
	if r.blockStartFP == -1 {
		r.blockInput = r.blockInput.Clone()
		r.blockHeaderReader = r.createBlockHeaderSerializer()
		r.blockLineReader = r.createBlockLineSerializer()
		r.blockReadBuffer = store.NewByteArrayDataInput(nil)
		r.termStatesReadBuffer = store.NewByteArrayDataInput(nil)
		r.termStateSerializer = r.createDeltaBaseTermStateSerializer()
		r.scratchBlockBytes = util.NewBytesRefEmpty()
		r.scratchBlockLine = NewBlockLine(NewTermBytes(0, r.scratchBlockBytes), 0)
	}
	return nil
}

// createBlockHeaderSerializer mirrors
// BlockReader.createBlockHeaderSerializer (BlockReader.java:415).
func (r *BlockReader) createBlockHeaderSerializer() *BlockHeaderSerializer {
	return &BlockHeaderSerializer{}
}

// createBlockLineSerializer mirrors BlockReader.createBlockLineSerializer
// (BlockReader.java:419).
func (r *BlockReader) createBlockLineSerializer() *BlockLineSerializer {
	return NewBlockLineSerializer()
}

// createDeltaBaseTermStateSerializer mirrors
// BlockReader.createDeltaBaseTermStateSerializer (BlockReader.java:423).
func (r *BlockReader) createDeltaBaseTermStateSerializer() *DeltaBaseTermStateSerializer {
	return NewDeltaBaseTermStateSerializer()
}

// readHeader reads the block header and sets blockHeader. Returns the block
// header; or nil if there is no block for the field anymore.
//
// Mirrors BlockReader.readHeader (BlockReader.java:432).
func (r *BlockReader) readHeader() (*BlockHeader, error) {
	if r.blockInput.GetFilePointer() > r.fieldMetadata.GetLastBlockStartFP() {
		r.blockHeader = nil
		return nil, nil
	}
	numBlockBytes, err := r.blockInput.ReadVInt()
	if err != nil {
		return nil, err
	}
	blockBytesRef, err := r.decodeBlockBytesIfNeeded(int(numBlockBytes))
	if err != nil {
		return nil, err
	}
	r.blockReadBuffer.ResetWithSlice(blockBytesRef.Bytes, blockBytesRef.Offset, blockBytesRef.Length)
	r.termStatesReadBuffer.ResetWithSlice(blockBytesRef.Bytes, blockBytesRef.Offset, blockBytesRef.Length)
	blockHeader, err := r.blockHeaderReader.Read(r.blockReadBuffer, r.blockHeader)
	if err != nil {
		return nil, err
	}
	r.blockHeader = blockHeader
	return r.blockHeader, nil
}

// decodeBlockBytesIfNeeded mirrors BlockReader.decodeBlockBytesIfNeeded
// (BlockReader.java:444).
func (r *BlockReader) decodeBlockBytesIfNeeded(numBlockBytes int) (*util.BytesRef, error) {
	r.scratchBlockBytes.Bytes = util.GrowByte(r.scratchBlockBytes.Bytes, numBlockBytes)
	if err := r.blockInput.ReadBytes(r.scratchBlockBytes.Bytes, 0, numBlockBytes); err != nil {
		return nil, err
	}
	r.scratchBlockBytes.Length = numBlockBytes
	if r.blockDecoder == nil {
		return r.scratchBlockBytes, nil
	}
	r.blockReadBuffer.ResetWithSlice(r.scratchBlockBytes.Bytes, 0, numBlockBytes)
	return r.blockDecoder.Decode(r.blockReadBuffer, int64(numBlockBytes))
}

// readTermStateIfNotRead reads the BlockTermState if it is not already set.
// Sets termState.
//
// Mirrors BlockReader.readTermStateIfNotRead (BlockReader.java:455).
func (r *BlockReader) readTermStateIfNotRead() (index.TermState, error) {
	if r.termState == nil {
		ts, err := r.readTermState()
		if err != nil {
			return nil, err
		}
		r.termState = ts
		if r.termState != nil {
			base := codecs.BaseState(r.termState)
			base.TermBlockOrd = int(r.lineIndexInBlock)
			base.BlockFilePointer = r.blockStartFP
		}
	}
	return r.termState, nil
}

// readTermState reads the BlockTermState on the current line. Sets termState.
//
// Mirrors BlockReader.readTermState (BlockReader.java:474).
func (r *BlockReader) readTermState() (index.TermState, error) {
	// We reuse scratchTermState safely as the read TermState is cloned in the TermState method.
	r.termStatesReadBuffer.SetPosition(
		r.blockFirstLineStart +
			int(r.blockHeader.TermStatesBaseOffset()) +
			int(r.blockLine.GetTermStateRelativeOffset()))

	termState, err := r.termStateSerializer.ReadTermState(
		r.blockHeader.BaseDocsFP(),
		r.blockHeader.BasePositionsFP(),
		r.blockHeader.BasePayloadsFP(),
		r.termStatesReadBuffer,
		r.fieldMetadata.GetFieldInfo(),
		r.scratchTermState,
	)
	if err != nil {
		return nil, err
	}
	r.termState = termState
	return r.termState, nil
}

// term mirrors BlockReader.term() (BlockReader.java:488), which returns the
// raw term bytes of the current line.
func (r *BlockReader) term() *util.BytesRef {
	if r.termStateForced {
		return r.forcedTerm.Get()
	}
	if r.blockLine == nil {
		return nil
	}
	return r.blockLine.GetTermBytes().GetTerm()
}

// getOrCreateDictionaryBrowser mirrors
// BlockReader.getOrCreateDictionaryBrowser (BlockReader.java:556).
func (r *BlockReader) getOrCreateDictionaryBrowser() (IndexDictionaryBrowser, error) {
	if r.dictionaryBrowser == nil {
		browser, err := r.dictionaryBrowserSupplier.Get()
		if err != nil {
			return nil, err
		}
		r.dictionaryBrowser = browser
	}
	return r.dictionaryBrowser, nil
}

// clearTermState is called by the primary TermsEnum methods to clear the
// previous TermState.
//
// Mirrors BlockReader.clearTermState (BlockReader.java:564).
func (r *BlockReader) clearTermState() {
	r.termState = nil
	r.termStateForced = false
}

// newCorruptIndexError mirrors BlockReader.newCorruptIndexException
// (BlockReader.java:570).
func (r *BlockReader) newCorruptIndexError(msg string, fp *int64) error {
	at := ""
	if fp != nil {
		at = fmt.Sprintf(" at FP %d", *fp)
	}
	return index.NewCorruptIndexException(
		fmt.Sprintf("%s%s for field \"%s\"", msg, at, r.fieldMetadata.GetFieldInfo().Name()),
		fmt.Sprint(r.blockInput))
}

// --- TermsEnum surface ---
//
// Gocene's spi.TermsEnum carries the field name alongside the term bytes in
// *spi.Term, where Java's org.apache.lucene.index.TermsEnum exchanges a bare
// BytesRef. The methods below are the Java methods of the same name, adapted to
// that signature; the term bytes they read and return are unchanged.

// Next advances to the next term. Mirrors BlockReader.next
// (BlockReader.java:333).
func (r *BlockReader) Next() (*spi.Term, error) {
	if r.termStateForced {
		blockFilePointer := codecs.BaseState(r.termState).BlockFilePointer
		if err := r.initializeHeader(r.forcedTerm.Get(), blockFilePointer); err != nil {
			return nil, err
		}
		if r.blockHeader == nil {
			return nil, r.newCorruptIndexError("Illegal absence of block for TermState", &blockFilePointer)
		}
		for i := r.lineIndexInBlock; i < int32(codecs.BaseState(r.termState).TermBlockOrd); i++ {
			if _, err := r.readLineInBlock(); err != nil {
				return nil, err
			}
		}
		// assert blockLine.getTermBytes().getTerm().equals(forcedTerm.get());
	}
	r.clearTermState()
	termBytes, err := r.nextTerm()
	if err != nil {
		return nil, err
	}
	if termBytes == nil {
		return nil, nil
	}
	return spi.NewTermFromBytesRef(r.fieldMetadata.GetFieldInfo().Name(), termBytes), nil
}

// SeekCeil seeks to term or to the next term after it. Mirrors
// BlockReader.seekCeil (BlockReader.java:150), adapted to the Gocene SPI, which
// returns the positioned term rather than a SeekStatus.
func (r *BlockReader) SeekCeil(term *spi.Term) (*spi.Term, error) {
	status, err := r.seekCeil(term.BytesValue())
	if err != nil {
		return nil, err
	}
	if status == spi.SeekStatusEnd {
		return nil, nil
	}
	if status == spi.SeekStatusNotFound {
		// Go to next term.
		termBytes, err := r.nextTerm()
		if err != nil {
			return nil, err
		}
		if termBytes == nil {
			return nil, nil
		}
		return spi.NewTermFromBytesRef(r.fieldMetadata.GetFieldInfo().Name(), termBytes), nil
	}
	return spi.NewTermFromBytesRef(r.fieldMetadata.GetFieldInfo().Name(), r.term()), nil
}

// SeekExact seeks to term. Mirrors BlockReader.seekExact(BytesRef)
// (BlockReader.java:168).
func (r *BlockReader) SeekExact(term *spi.Term) (bool, error) {
	return r.seekExact(term.BytesValue())
}

// SeekExactWithState positions this BlockReader without re-seeking the term
// dictionary.
//
// The block containing the term is not read by this method. It will be read
// lazily only if needed, for example if Next is called. Calling Postings after
// this method does require the block to be read.
//
// Mirrors BlockReader.seekExact(BytesRef, TermState)
// (BlockReader.java:318).
func (r *BlockReader) SeekExactWithState(term *spi.Term, state index.TermState) error {
	r.termStateForced = true
	r.termState = r.scratchTermState
	if err := r.termState.CopyFrom(state); err != nil {
		return err
	}
	if r.forcedTerm == nil {
		r.forcedTerm = util.NewBytesRefBuilder()
	}
	// Java calls forcedTerm.copyBytes(term). util.BytesRefBuilder carries no
	// CopyBytes, so its body — ref.length = len; growNoCopy(len);
	// arraycopy(b, off, ref.bytes, 0, len) — is inlined here.
	forcedBytes := term.BytesValue()
	r.forcedTerm.SetLength(forcedBytes.Length)
	r.forcedTerm.GrowNoCopy(forcedBytes.Length)
	copy(r.forcedTerm.Bytes(), forcedBytes.ValidBytes())
	return nil
}

// Term returns the current term. Mirrors BlockReader.term
// (BlockReader.java:488).
func (r *BlockReader) Term() *spi.Term {
	termBytes := r.term()
	if termBytes == nil {
		return nil
	}
	return spi.NewTermFromBytesRef(r.fieldMetadata.GetFieldInfo().Name(), termBytes)
}

// Ord is not supported. Mirrors BlockReader.ord (BlockReader.java:496), whose
// body is `throw new UnsupportedOperationException()`. Ord carries no error in
// the TermsEnum contract (Java's ord() declares no checked exception), so the
// unsupported call panics, mirroring the unchecked Java exception.
func (r *BlockReader) Ord() int64 {
	panic(errBlockReaderOrdUnsupported)
}

var errBlockReaderOrdUnsupported = errors.New("BlockReader: ord is not supported")

// DocFreq mirrors BlockReader.docFreq (BlockReader.java:501).
func (r *BlockReader) DocFreq() (int, error) {
	ts, err := r.readTermStateIfNotRead()
	if err != nil {
		return 0, err
	}
	return codecs.BaseState(ts).DocFreq, nil
}

// TotalTermFreq mirrors BlockReader.totalTermFreq (BlockReader.java:507).
func (r *BlockReader) TotalTermFreq() (int64, error) {
	ts, err := r.readTermStateIfNotRead()
	if err != nil {
		return 0, err
	}
	return codecs.BaseState(ts).TotalTermFreq, nil
}

// TermState mirrors BlockReader.termState (BlockReader.java:513), which
// returns a clone of the read state.
func (r *BlockReader) TermState() (index.TermState, error) {
	ts, err := r.readTermStateIfNotRead()
	if err != nil {
		return nil, err
	}
	return codecs.BaseState(ts).Clone(), nil
}

// Postings mirrors BlockReader.postings (BlockReader.java:519).
func (r *BlockReader) Postings(flags int) (spi.PostingsEnum, error) {
	ts, err := r.readTermStateIfNotRead()
	if err != nil {
		return nil, err
	}
	return r.postingsReader.Postings(r.fieldMetadata.GetFieldInfo(), ts, nil, flags)
}

// Impacts mirrors BlockReader.impacts (BlockReader.java:525).
func (r *BlockReader) Impacts(flags int) (spi.ImpactsEnum, error) {
	ts, err := r.readTermStateIfNotRead()
	if err != nil {
		return nil, err
	}
	return r.postingsReader.Impacts(r.fieldMetadata.GetFieldInfo(), ts, flags)
}

// PostingsWithLiveDocs forwards to Postings; live-docs filtering is applied by
// callers at a higher layer, matching how Lucene threads liveDocs through the
// leaf reader. Java's BlockReader.postings (BlockReader.java:519) takes no live
// docs.
func (r *BlockReader) PostingsWithLiveDocs(_ util.Bits, flags int) (spi.PostingsEnum, error) {
	return r.Postings(flags)
}

var _ spi.TermsEnum = (*BlockReader)(nil)

// blockReaderBaseRAMUsage renders the private static BASE_RAM_USAGE
// (BlockReader.java:49):
//
//	shallowSizeOfInstance(BlockReader.class)
//	    + shallowSizeOfInstance(IndexInput.class)
//	    + shallowSizeOfInstance(ByteArrayDataInput.class) * 2
//
// Java's IndexInput is an abstract class with its own field layout. Gocene's
// store.IndexInput is an interface, so the value standing in its place is an
// interface header; util.ShallowSizeOf reports 0 for a nil interface, so the
// header is measured through a one-field struct instead.
var blockReaderBaseRAMUsage = util.ShallowSizeOf(BlockReader{}) +
	util.ShallowSizeOf(struct{ blockInput store.IndexInput }{}) +
	util.ShallowSizeOf(store.ByteArrayDataInput{})*2

// RamBytesUsed mirrors BlockReader.ramBytesUsed (BlockReader.java:543).
func (r *BlockReader) RamBytesUsed() int64 {
	total := blockReaderBaseRAMUsage
	if r.blockLineReader != nil {
		total += r.blockLineReader.RamBytesUsed()
	}
	if r.blockReadBuffer != nil {
		total += RamBytesUsedByByteArrayOfLength(r.blockReadBuffer.Length())
	}
	if r.termStateSerializer != nil {
		total += r.termStateSerializer.RamBytesUsed()
	}
	if r.forcedTerm != nil {
		total += RamBytesUsedByBytesRefBuilder(r.forcedTerm)
	}
	if r.blockHeader != nil {
		total += r.blockHeader.RamBytesUsed()
	}
	if r.blockLine != nil {
		total += r.blockLine.RamBytesUsed()
	}
	if r.termState != nil {
		total += RamBytesUsedByTermState(r.termState)
	}
	return total
}

// BlockReader implements Accountable (BlockReader.java:47).
var _ util.Accountable = (*BlockReader)(nil)
