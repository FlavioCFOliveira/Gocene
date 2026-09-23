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
// BlockReaderOverrides is the set of protected BlockReader methods that Apache
// Lucene 10.5.0 subclasses of BlockReader override and that BlockReader's own
// bodies then invoke on `this` — BlockReader.next calls nextTerm()
// (BlockReader.java:347), seekCeil calls isBeyondLastTerm() and nextTerm()
// (BlockReader.java:161, 169), seekInBlock calls isBeyondLastTerm()
// (BlockReader.java:181), initializeBlockReadLazily calls
// createBlockLineSerializer() (BlockReader.java:416), readTermStateIfNotRead
// calls readTermState() (BlockReader.java:467), and docFreq, totalTermFreq,
// termState, postings and impacts call readTermStateIfNotRead()
// (BlockReader.java:515-539).
//
// Java resolves those calls virtually. Go embedding does not, so BlockReader
// keeps a back-pointer to the most-derived instance in [BlockReader.Overrides]
// and makes the calls through it. This is the mechanism spi.BaseDataInput
// already uses for the same purpose.
//
// Every member is declared `protected` by
// org.apache.lucene.codecs.uniformsplit.BlockReader, so its exported Go
// spelling is the rendering of `protected`: reachable by subclasses that live
// in another package, exactly as
// org.apache.lucene.codecs.uniformsplit.sharedterms.STBlockReader reaches them.
type BlockReaderOverrides interface {
	// CreateBlockLineSerializer mirrors
	// BlockReader.createBlockLineSerializer (BlockReader.java:429).
	CreateBlockLineSerializer() *BlockLineSerializer

	// IsBeyondLastTerm mirrors BlockReader.isBeyondLastTerm
	// (BlockReader.java:199).
	IsBeyondLastTerm(searchedTerm *util.BytesRef, blockStartFP int64) bool

	// NextTerm mirrors BlockReader.nextTerm (BlockReader.java:356).
	NextTerm() (*util.BytesRef, error)

	// ReadTermState mirrors BlockReader.readTermState
	// (BlockReader.java:484).
	ReadTermState() (index.TermState, error)

	// ReadTermStateIfNotRead mirrors BlockReader.readTermStateIfNotRead
	// (BlockReader.java:465).
	ReadTermStateIfNotRead() (index.TermState, error)

	// SeekCeilBytes mirrors the public BlockReader.seekCeil(BytesRef)
	// (BlockReader.java:153). It is reached virtually because the Gocene
	// spi.TermsEnum contract is answered by SeekCeil(*spi.Term), which
	// delegates to it.
	SeekCeilBytes(searchedTerm *util.BytesRef) (spi.SeekStatus, error)

	// SeekExactBytes mirrors the public BlockReader.seekExact(BytesRef)
	// (BlockReader.java:173). It is reached virtually because the Gocene
	// spi.TermsEnum contract is answered by SeekExact(*spi.Term), which
	// delegates to it.
	SeekExactBytes(searchedTerm *util.BytesRef) (bool, error)

	// CompareToMiddleAndJump mirrors the protected
	// BlockReader.compareToMiddleAndJump(BytesRef), which Java resolves
	// virtually.
	CompareToMiddleAndJump(searchedTerm *util.BytesRef) (int, error)

	// ReadLineInBlock mirrors the protected BlockReader.readLineInBlock(),
	// which Java resolves virtually.
	ReadLineInBlock() (*BlockLine, error)

	// InitializeHeader mirrors the protected
	// BlockReader.initializeHeader(BytesRef, long), which Java resolves
	// virtually.
	InitializeHeader(searchedTerm *util.BytesRef, targetBlockStartFP int64) error

	// ReadHeader mirrors the protected BlockReader.readHeader(), which Java
	// resolves virtually.
	ReadHeader() (*BlockHeader, error)
}

// Mirrors org.apache.lucene.codecs.uniformsplit.BlockReader from Apache Lucene
// 10.5.0.
type BlockReader struct {
	index.BaseTermsEnum

	// Overrides is the back-pointer to the most-derived instance, through
	// which BlockReader makes the calls Java resolves virtually. See
	// [BlockReaderOverrides]. NewBlockReader sets it to the BlockReader
	// itself; a subclass constructor overwrites it with the subclass.
	Overrides BlockReaderOverrides

	// blockInput is the IndexInput on the block file.
	BlockInput store.IndexInput

	PostingsReader codecs.PostingsReaderBase
	FieldMetadata  *FieldMetadata
	BlockDecoder   BlockDecoder

	BlockHeaderReader *BlockHeaderSerializer
	BlockLineReader   *BlockLineSerializer

	// blockReadBuffer is the in-memory read buffer for the current block.
	BlockReadBuffer *store.ByteArrayDataInput

	// termStatesReadBuffer is the in-memory read buffer for the details region
	// of the current block. It shares the same byte array as blockReadBuffer,
	// with a different position.
	TermStatesReadBuffer *store.ByteArrayDataInput

	TermStateSerializer *DeltaBaseTermStateSerializer

	// dictionaryBrowserSupplier is the IndexDictionaryBrowser supplier for lazy
	// loading.
	DictionaryBrowserSupplier IndexDictionaryBrowserSupplier

	// dictionaryBrowser holds the IndexDictionaryBrowser once loaded.
	DictionaryBrowser IndexDictionaryBrowser

	// blockStartFP is the current block start file pointer, absolute in the
	// block file.
	BlockStartFP int64

	// blockHeader is the current block header.
	BlockHeader *BlockHeader

	// blockLine is the current block line.
	BlockLine *BlockLine

	// termState holds the current block line details.
	CurrentTermState index.TermState

	// blockFirstLineStart is the offset of the start of the first line of the
	// current block (just after the header), relative to the block start.
	BlockFirstLineStart int

	// lineIndexInBlock is the current line index in the block.
	LineIndexInBlock int32

	// termStateForced tells whether the current TermState has been forced with
	// a call to SeekExactWithState.
	TermStateForced bool

	// forcedTerm is set when SeekExactWithState is called.
	//
	// This optimizes the use-case when the caller calls first
	// SeekExactWithState and then Postings. In this case we don't access the
	// terms block file (we don't seek) but directly the postings file because
	// we already have the TermState with the file pointers to the postings
	// file.
	ForcedTerm *util.BytesRefBuilder

	// Scratch objects to avoid object reallocation.
	ScratchBlockBytes *util.BytesRef
	ScratchTermState  index.TermState
	ScratchBlockLine  *BlockLine
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
	r := &BlockReader{
		DictionaryBrowserSupplier: dictionaryBrowserSupplier,
		BlockInput:                blockInput,
		PostingsReader:            postingsReader,
		FieldMetadata:             fieldMetadata,
		BlockDecoder:              blockDecoder,
		BlockStartFP:              -1,
		ScratchTermState:          postingsReader.NewTermState(),
	}
	r.Overrides = r
	return r, nil
}

// SeekCeilBytes mirrors BlockReader.seekCeil(BytesRef) (BlockReader.java:150).
func (r *BlockReader) SeekCeilBytes(searchedTerm *util.BytesRef) (spi.SeekStatus, error) {
	if r.IsCurrentTerm(searchedTerm) {
		return spi.SeekStatusFound, nil
	}
	r.ClearTermState()

	browser, err := r.GetOrCreateDictionaryBrowser()
	if err != nil {
		return spi.SeekStatusEnd, err
	}
	blockStartFP, err := browser.SeekBlock(searchedTerm)
	if err != nil {
		return spi.SeekStatusEnd, err
	}
	blockStartFP = max(blockStartFP, r.FieldMetadata.GetFirstBlockStartFP())
	if r.Overrides.IsBeyondLastTerm(searchedTerm, blockStartFP) {
		return spi.SeekStatusEnd, nil
	}
	seekStatus, err := r.SeekInBlockAt(searchedTerm, blockStartFP)
	if err != nil {
		return spi.SeekStatusEnd, err
	}
	if seekStatus != spi.SeekStatusEnd {
		return seekStatus, nil
	}
	// Go to next block.
	nextTerm, err := r.Overrides.NextTerm()
	if err != nil {
		return spi.SeekStatusEnd, err
	}
	if nextTerm == nil {
		return spi.SeekStatusEnd, nil
	}
	return spi.SeekStatusNotFound, nil
}

// SeekExactBytes mirrors BlockReader.seekExact(BytesRef) (BlockReader.java:168).
func (r *BlockReader) SeekExactBytes(searchedTerm *util.BytesRef) (bool, error) {
	if r.IsCurrentTerm(searchedTerm) {
		return true, nil
	}
	r.ClearTermState()

	browser, err := r.GetOrCreateDictionaryBrowser()
	if err != nil {
		return false, err
	}
	blockStartFP, err := browser.SeekBlock(searchedTerm)
	if err != nil {
		return false, err
	}
	if blockStartFP < r.FieldMetadata.GetFirstBlockStartFP() || r.Overrides.IsBeyondLastTerm(searchedTerm, blockStartFP) {
		return false, nil
	}
	seekStatus, err := r.SeekInBlockAt(searchedTerm, blockStartFP)
	if err != nil {
		return false, err
	}
	return seekStatus == spi.SeekStatusFound, nil
}

// IsCurrentTerm mirrors BlockReader.isCurrentTerm (BlockReader.java:182).
func (r *BlockReader) IsCurrentTerm(searchedTerm *util.BytesRef) bool {
	// Optimization and also required to not search with the same BytesRef
	// instance as the BytesRef used to read the block line (BlockLineSerializer).
	// Indeed term() is allowed to return the same BytesRef instance.
	return util.BytesRefEquals(searchedTerm, r.TermBytes())
}

// IsBeyondLastTerm indicates whether the searched term is beyond the last term
// of the field. blockStartFP is the current block start file pointer.
//
// Mirrors BlockReader.isBeyondLastTerm (BlockReader.java:193).
func (r *BlockReader) IsBeyondLastTerm(searchedTerm *util.BytesRef, blockStartFP int64) bool {
	return blockStartFP == r.FieldMetadata.GetLastBlockStartFP() &&
		util.BytesRefCompare(searchedTerm, r.FieldMetadata.GetLastTerm()) > 0
}

// SeekInBlockAt seeks to the provided term in the block starting at the
// provided file pointer. Does not exceed the block.
//
// Mirrors the overload BlockReader.seekInBlock(BytesRef, long)
// (BlockReader.java:202).
func (r *BlockReader) SeekInBlockAt(searchedTerm *util.BytesRef, blockStartFP int64) (spi.SeekStatus, error) {
	if err := r.Overrides.InitializeHeader(searchedTerm, blockStartFP); err != nil {
		return spi.SeekStatusEnd, err
	}
	if r.BlockHeader == nil {
		return spi.SeekStatusEnd, r.NewCorruptIndexError("Illegal absence of block", &blockStartFP)
	}
	return r.SeekInBlock(searchedTerm)
}

// SeekInBlock seeks to the provided term in this block.
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
func (r *BlockReader) SeekInBlock(searchedTerm *util.BytesRef) (spi.SeekStatus, error) {
	compare, err := r.Overrides.CompareToMiddleAndJump(searchedTerm)
	if err != nil {
		return spi.SeekStatusEnd, err
	}
	if compare == 0 {
		return spi.SeekStatusFound, nil
	}
	comparisonOffset := 0
	for {
		line, err := r.Overrides.ReadLineInBlock()
		if err != nil {
			return spi.SeekStatusEnd, err
		}
		if line == nil {
			// No more terms for the block.
			return spi.SeekStatusEnd, nil
		}
		lineTermBytes := r.BlockLine.GetTermBytes()
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

// CompareToMiddleAndJump compares the searched term to the middle term of the
// block. If the searched term is lexicographically equal or after the middle
// term then jumps to the second half of the block directly.
//
// Returns the comparison between the searched term and the middle term.
//
// Mirrors BlockReader.compareToMiddleAndJump (BlockReader.java:272).
func (r *BlockReader) CompareToMiddleAndJump(searchedTerm *util.BytesRef) (int, error) {
	if r.LineIndexInBlock != 0 {
		// Don't try to compare and jump if we are not positioned at the first line.
		// This can happen if we seek in the same current block and we continue
		// scanning from the current line (see initializeHeader()).
		return -1, nil
	}
	if err := r.BlockReadBuffer.SkipBytes(int64(r.BlockHeader.MiddleLineOffset())); err != nil {
		return 0, err
	}
	r.LineIndexInBlock = r.BlockHeader.MiddleLineIndex()
	if _, err := r.Overrides.ReadLineInBlock(); err != nil {
		return 0, err
	}
	if r.BlockLine == nil {
		return 0, r.NewCorruptIndexError("Illegal absence of line at the middle of the block", nil)
	}
	compare := util.BytesRefCompare(searchedTerm, r.TermBytes())
	if compare < 0 {
		r.BlockReadBuffer.SetPosition(r.BlockFirstLineStart)
		r.LineIndexInBlock = 0
	}
	return compare, nil
}

// ReadLineInBlock reads the current block line. Sets blockLine and increments
// lineIndexInBlock. Returns the BlockLine; or nil if there is no more line in
// the block.
//
// Mirrors BlockReader.readLineInBlock (BlockReader.java:297).
func (r *BlockReader) ReadLineInBlock() (*BlockLine, error) {
	if r.LineIndexInBlock >= r.BlockHeader.LinesCount() {
		r.BlockLine = nil
		return nil, nil
	}
	isIncrementalEncodingSeed := r.LineIndexInBlock == 0 || r.LineIndexInBlock == r.BlockHeader.MiddleLineIndex()
	r.LineIndexInBlock++
	blockLine, err := r.BlockLineReader.ReadLine(r.BlockReadBuffer, isIncrementalEncodingSeed, r.ScratchBlockLine)
	if err != nil {
		return nil, err
	}
	r.BlockLine = blockLine
	return r.BlockLine, nil
}

// NextTerm moves to the next term line and reads it, it may be in the next
// block. The term details are not read yet. They will be read only when needed
// with readTermStateIfNotRead.
//
// Returns the read term bytes; or nil if there is no more term for the field.
//
// Mirrors BlockReader.nextTerm (BlockReader.java:350).
func (r *BlockReader) NextTerm() (*util.BytesRef, error) {
	if r.BlockHeader == nil {
		// Read the first block for the field.
		if err := r.Overrides.InitializeHeader(nil, r.FieldMetadata.GetFirstBlockStartFP()); err != nil {
			return nil, err
		}
		if r.BlockHeader == nil {
			firstBlockStartFP := r.FieldMetadata.GetFirstBlockStartFP()
			return nil, r.NewCorruptIndexError("Illegal absence of first block", &firstBlockStartFP)
		}
	}
	line, err := r.Overrides.ReadLineInBlock()
	if err != nil {
		return nil, err
	}
	if line == nil {
		// No more line in the current block.
		// Read the next block starting at the current file pointer in the block file.
		if err := r.Overrides.InitializeHeader(nil, r.BlockInput.GetFilePointer()); err != nil {
			return nil, err
		}
		if r.BlockHeader == nil {
			// No more block for the field.
			return nil, nil
		}
		if _, err := r.Overrides.ReadLineInBlock(); err != nil {
			return nil, err
		}
	}
	return r.TermBytes(), nil
}

// InitializeHeader reads and sets blockHeader. Sets nil if there is no block
// for the field anymore.
//
// searchedTerm is the searched term, or nil if none. targetBlockStartFP is the
// file pointer of the block to read.
//
// Mirrors BlockReader.initializeHeader (BlockReader.java:376).
func (r *BlockReader) InitializeHeader(searchedTerm *util.BytesRef, targetBlockStartFP int64) error {
	if err := r.InitializeBlockReadLazily(); err != nil {
		return err
	}
	if r.BlockStartFP == targetBlockStartFP {
		// Optimization: If the block to read is already the current block, then
		// reuse it directly without reading nor decoding the block bytes.
		if r.BlockHeader == nil {
			return r.NewCorruptIndexError("Illegal absence of block", &r.BlockStartFP)
		}
		if searchedTerm == nil || r.BlockLine == nil ||
			util.BytesRefCompare(searchedTerm, r.BlockLine.GetTermBytes().GetTerm()) <= 0 {
			// If the searched term precedes lexicographically the current term,
			// then reset the position to the first term line of the block.
			// If the searched term equals the current term, we also need to reset
			// to scan again the current line.
			r.BlockReadBuffer.SetPosition(r.BlockFirstLineStart)
			r.LineIndexInBlock = 0
		}
	} else {
		if err := r.BlockInput.SetPosition(targetBlockStartFP); err != nil {
			return err
		}
		r.BlockStartFP = targetBlockStartFP
		if _, err := r.Overrides.ReadHeader(); err != nil {
			return err
		}
		r.BlockFirstLineStart = r.BlockReadBuffer.GetPosition()
		r.LineIndexInBlock = 0
	}
	return nil
}

// InitializeBlockReadLazily mirrors BlockReader.initializeBlockReadLazily
// (BlockReader.java:403).
func (r *BlockReader) InitializeBlockReadLazily() error {
	if r.BlockStartFP == -1 {
		r.BlockInput = r.BlockInput.Clone()
		r.BlockHeaderReader = r.CreateBlockHeaderSerializer()
		r.BlockLineReader = r.Overrides.CreateBlockLineSerializer()
		r.BlockReadBuffer = store.NewByteArrayDataInput(nil)
		r.TermStatesReadBuffer = store.NewByteArrayDataInput(nil)
		r.TermStateSerializer = r.CreateDeltaBaseTermStateSerializer()
		r.ScratchBlockBytes = util.NewBytesRefEmpty()
		r.ScratchBlockLine = NewBlockLine(NewTermBytes(0, r.ScratchBlockBytes), 0)
	}
	return nil
}

// CreateBlockHeaderSerializer mirrors
// BlockReader.createBlockHeaderSerializer (BlockReader.java:415).
func (r *BlockReader) CreateBlockHeaderSerializer() *BlockHeaderSerializer {
	return &BlockHeaderSerializer{}
}

// CreateBlockLineSerializer mirrors BlockReader.createBlockLineSerializer
// (BlockReader.java:419).
func (r *BlockReader) CreateBlockLineSerializer() *BlockLineSerializer {
	return NewBlockLineSerializer()
}

// CreateDeltaBaseTermStateSerializer mirrors
// BlockReader.createDeltaBaseTermStateSerializer (BlockReader.java:423).
func (r *BlockReader) CreateDeltaBaseTermStateSerializer() *DeltaBaseTermStateSerializer {
	return NewDeltaBaseTermStateSerializer()
}

// ReadHeader reads the block header and sets blockHeader. Returns the block
// header; or nil if there is no block for the field anymore.
//
// Mirrors BlockReader.readHeader (BlockReader.java:432).
func (r *BlockReader) ReadHeader() (*BlockHeader, error) {
	if r.BlockInput.GetFilePointer() > r.FieldMetadata.GetLastBlockStartFP() {
		r.BlockHeader = nil
		return nil, nil
	}
	numBlockBytes, err := r.BlockInput.ReadVInt()
	if err != nil {
		return nil, err
	}
	blockBytesRef, err := r.DecodeBlockBytesIfNeeded(int(numBlockBytes))
	if err != nil {
		return nil, err
	}
	r.BlockReadBuffer.ResetWithSlice(blockBytesRef.Bytes, blockBytesRef.Offset, blockBytesRef.Length)
	r.TermStatesReadBuffer.ResetWithSlice(blockBytesRef.Bytes, blockBytesRef.Offset, blockBytesRef.Length)
	blockHeader, err := r.BlockHeaderReader.Read(r.BlockReadBuffer, r.BlockHeader)
	if err != nil {
		return nil, err
	}
	r.BlockHeader = blockHeader
	return r.BlockHeader, nil
}

// DecodeBlockBytesIfNeeded mirrors BlockReader.decodeBlockBytesIfNeeded
// (BlockReader.java:444).
func (r *BlockReader) DecodeBlockBytesIfNeeded(numBlockBytes int) (*util.BytesRef, error) {
	r.ScratchBlockBytes.Bytes = util.GrowByte(r.ScratchBlockBytes.Bytes, numBlockBytes)
	if err := r.BlockInput.ReadBytes(r.ScratchBlockBytes.Bytes, 0, numBlockBytes); err != nil {
		return nil, err
	}
	r.ScratchBlockBytes.Length = numBlockBytes
	if r.BlockDecoder == nil {
		return r.ScratchBlockBytes, nil
	}
	r.BlockReadBuffer.ResetWithSlice(r.ScratchBlockBytes.Bytes, 0, numBlockBytes)
	return r.BlockDecoder.Decode(r.BlockReadBuffer, int64(numBlockBytes))
}

// ReadTermStateIfNotRead reads the BlockTermState if it is not already set.
// Sets termState.
//
// Mirrors BlockReader.readTermStateIfNotRead (BlockReader.java:455).
func (r *BlockReader) ReadTermStateIfNotRead() (index.TermState, error) {
	if r.CurrentTermState == nil {
		ts, err := r.Overrides.ReadTermState()
		if err != nil {
			return nil, err
		}
		r.CurrentTermState = ts
		if r.CurrentTermState != nil {
			base := codecs.BaseState(r.CurrentTermState)
			base.TermBlockOrd = int(r.LineIndexInBlock)
			base.BlockFilePointer = r.BlockStartFP
		}
	}
	return r.CurrentTermState, nil
}

// ReadTermState reads the BlockTermState on the current line. Sets termState.
//
// Mirrors BlockReader.readTermState (BlockReader.java:474).
func (r *BlockReader) ReadTermState() (index.TermState, error) {
	// We reuse scratchTermState safely as the read TermState is cloned in the TermState method.
	r.TermStatesReadBuffer.SetPosition(
		r.BlockFirstLineStart +
			int(r.BlockHeader.TermStatesBaseOffset()) +
			int(r.BlockLine.GetTermStateRelativeOffset()))

	termState, err := r.TermStateSerializer.ReadTermState(
		r.BlockHeader.BaseDocsFP(),
		r.BlockHeader.BasePositionsFP(),
		r.BlockHeader.BasePayloadsFP(),
		r.TermStatesReadBuffer,
		r.FieldMetadata.GetFieldInfo(),
		r.ScratchTermState,
	)
	if err != nil {
		return nil, err
	}
	r.CurrentTermState = termState
	return r.CurrentTermState, nil
}

// TermBytes mirrors BlockReader.term() (BlockReader.java:488), which returns the
// raw term bytes of the current line.
func (r *BlockReader) TermBytes() *util.BytesRef {
	if r.TermStateForced {
		return r.ForcedTerm.Get()
	}
	if r.BlockLine == nil {
		return nil
	}
	return r.BlockLine.GetTermBytes().GetTerm()
}

// GetOrCreateDictionaryBrowser mirrors
// BlockReader.getOrCreateDictionaryBrowser (BlockReader.java:556).
func (r *BlockReader) GetOrCreateDictionaryBrowser() (IndexDictionaryBrowser, error) {
	if r.DictionaryBrowser == nil {
		browser, err := r.DictionaryBrowserSupplier.Get()
		if err != nil {
			return nil, err
		}
		r.DictionaryBrowser = browser
	}
	return r.DictionaryBrowser, nil
}

// ClearTermState is called by the primary TermsEnum methods to clear the
// previous TermState.
//
// Mirrors BlockReader.clearTermState (BlockReader.java:564).
func (r *BlockReader) ClearTermState() {
	r.CurrentTermState = nil
	r.TermStateForced = false
}

// NewCorruptIndexError mirrors BlockReader.newCorruptIndexException
// (BlockReader.java:570).
func (r *BlockReader) NewCorruptIndexError(msg string, fp *int64) error {
	at := ""
	if fp != nil {
		at = fmt.Sprintf(" at FP %d", *fp)
	}
	return index.NewCorruptIndexException(
		fmt.Sprintf("%s%s for field \"%s\"", msg, at, r.FieldMetadata.GetFieldInfo().Name()),
		fmt.Sprint(r.BlockInput))
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
	if r.TermStateForced {
		blockFilePointer := codecs.BaseState(r.CurrentTermState).BlockFilePointer
		if err := r.Overrides.InitializeHeader(r.ForcedTerm.Get(), blockFilePointer); err != nil {
			return nil, err
		}
		if r.BlockHeader == nil {
			return nil, r.NewCorruptIndexError("Illegal absence of block for TermState", &blockFilePointer)
		}
		for i := r.LineIndexInBlock; i < int32(codecs.BaseState(r.CurrentTermState).TermBlockOrd); i++ {
			if _, err := r.Overrides.ReadLineInBlock(); err != nil {
				return nil, err
			}
		}
		// assert blockLine.getTermBytes().getTerm().equals(forcedTerm.get());
	}
	r.ClearTermState()
	termBytes, err := r.Overrides.NextTerm()
	if err != nil {
		return nil, err
	}
	if termBytes == nil {
		return nil, nil
	}
	return spi.NewTermFromBytesRef(r.FieldMetadata.GetFieldInfo().Name(), termBytes), nil
}

// SeekCeil seeks to term or to the next term after it. Mirrors
// BlockReader.seekCeil (BlockReader.java:150), adapted to the Gocene SPI, which
// returns the positioned term rather than a SeekStatus.
func (r *BlockReader) SeekCeil(term *spi.Term) (*spi.Term, error) {
	status, err := r.Overrides.SeekCeilBytes(term.BytesValue())
	if err != nil {
		return nil, err
	}
	if status == spi.SeekStatusEnd {
		return nil, nil
	}
	if status == spi.SeekStatusNotFound {
		// Go to next term.
		termBytes, err := r.Overrides.NextTerm()
		if err != nil {
			return nil, err
		}
		if termBytes == nil {
			return nil, nil
		}
		return spi.NewTermFromBytesRef(r.FieldMetadata.GetFieldInfo().Name(), termBytes), nil
	}
	return spi.NewTermFromBytesRef(r.FieldMetadata.GetFieldInfo().Name(), r.TermBytes()), nil
}

// SeekExact seeks to term. Mirrors BlockReader.seekExact(BytesRef)
// (BlockReader.java:168).
func (r *BlockReader) SeekExact(term *spi.Term) (bool, error) {
	return r.Overrides.SeekExactBytes(term.BytesValue())
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
	r.TermStateForced = true
	r.CurrentTermState = r.ScratchTermState
	if err := r.CurrentTermState.CopyFrom(state); err != nil {
		return err
	}
	if r.ForcedTerm == nil {
		r.ForcedTerm = util.NewBytesRefBuilder()
	}
	// Java calls forcedTerm.copyBytes(term). util.BytesRefBuilder carries no
	// CopyBytes, so its body — ref.length = len; growNoCopy(len);
	// arraycopy(b, off, ref.bytes, 0, len) — is inlined here.
	forcedBytes := term.BytesValue()
	r.ForcedTerm.SetLength(forcedBytes.Length)
	r.ForcedTerm.GrowNoCopy(forcedBytes.Length)
	copy(r.ForcedTerm.Bytes(), forcedBytes.ValidBytes())
	return nil
}

// Term returns the current term. Mirrors BlockReader.term
// (BlockReader.java:488).
func (r *BlockReader) Term() *spi.Term {
	termBytes := r.TermBytes()
	if termBytes == nil {
		return nil
	}
	return spi.NewTermFromBytesRef(r.FieldMetadata.GetFieldInfo().Name(), termBytes)
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
	ts, err := r.Overrides.ReadTermStateIfNotRead()
	if err != nil {
		return 0, err
	}
	return codecs.BaseState(ts).DocFreq, nil
}

// TotalTermFreq mirrors BlockReader.totalTermFreq (BlockReader.java:507).
func (r *BlockReader) TotalTermFreq() (int64, error) {
	ts, err := r.Overrides.ReadTermStateIfNotRead()
	if err != nil {
		return 0, err
	}
	return codecs.BaseState(ts).TotalTermFreq, nil
}

// TermState mirrors BlockReader.termState (BlockReader.java:513), which
// returns a clone of the read state.
func (r *BlockReader) TermState() (index.TermState, error) {
	ts, err := r.Overrides.ReadTermStateIfNotRead()
	if err != nil {
		return nil, err
	}
	return codecs.BaseState(ts).Clone(), nil
}

// Postings mirrors BlockReader.postings (BlockReader.java:519).
func (r *BlockReader) Postings(flags int) (spi.PostingsEnum, error) {
	ts, err := r.Overrides.ReadTermStateIfNotRead()
	if err != nil {
		return nil, err
	}
	return r.PostingsReader.Postings(r.FieldMetadata.GetFieldInfo(), ts, nil, flags)
}

// Impacts mirrors BlockReader.impacts (BlockReader.java:525).
func (r *BlockReader) Impacts(flags int) (spi.ImpactsEnum, error) {
	ts, err := r.Overrides.ReadTermStateIfNotRead()
	if err != nil {
		return nil, err
	}
	return r.PostingsReader.Impacts(r.FieldMetadata.GetFieldInfo(), ts, flags)
}

// PostingsWithLiveDocs forwards to Postings; live-docs filtering is applied by
// callers at a higher layer, matching how Lucene threads liveDocs through the
// leaf reader. Java's BlockReader.postings (BlockReader.java:519) takes no live
// docs.
func (r *BlockReader) PostingsWithLiveDocs(_ util.Bits, flags int) (spi.PostingsEnum, error) {
	return r.Postings(flags)
}

var (
	_ spi.TermsEnum        = (*BlockReader)(nil)
	_ BlockReaderOverrides = (*BlockReader)(nil)
)

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
	if r.BlockLineReader != nil {
		total += r.BlockLineReader.RamBytesUsed()
	}
	if r.BlockReadBuffer != nil {
		total += RamBytesUsedByByteArrayOfLength(r.BlockReadBuffer.Length())
	}
	if r.TermStateSerializer != nil {
		total += r.TermStateSerializer.RamBytesUsed()
	}
	if r.ForcedTerm != nil {
		total += RamBytesUsedByBytesRefBuilder(r.ForcedTerm)
	}
	if r.BlockHeader != nil {
		total += r.BlockHeader.RamBytesUsed()
	}
	if r.BlockLine != nil {
		total += r.BlockLine.RamBytesUsed()
	}
	if r.CurrentTermState != nil {
		total += RamBytesUsedByTermState(r.CurrentTermState)
	}
	return total
}

// BlockReader implements Accountable (BlockReader.java:47).
var _ util.Accountable = (*BlockReader)(nil)
