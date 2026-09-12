// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package uniformsplit

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// BlockReader seeks the block corresponding to a given term, reads the block bytes, and scans the block terms.
// Mirrors org.apache.lucene.codecs.uniformsplit.BlockReader from Apache Lucene 10.5.0.
type BlockReader struct {
	blockInput store.IndexInput

	postingsReader codecs.PostingsReaderBase
	fieldMetadata  *FieldMetadata
	blockDecoder   BlockDecoder

	blockHeaderReader *BlockHeaderSerializer
	blockLineReader   *BlockLineSerializer

	blockReadBuffer      *store.ByteArrayDataInput
	termStatesReadBuffer *store.ByteArrayDataInput

	termStateSerializer *DeltaBaseTermStateSerializer

	dictionaryBrowserSupplier IndexDictionaryBrowserSupplier
	dictionaryBrowser         IndexDictionary

	blockStartFP int64
	blockHeader  *BlockHeader
	blockLine    *BlockLine
	termState    *codecs.BlockTermState

	blockFirstLineStart int
	lineIndexInBlock    int
	termStateForced     bool
	forcedTerm          *util.BytesRef

	scratchBlockBytes *util.BytesRef
	scratchTermState  *codecs.BlockTermState
	scratchBlockLine  *BlockLine
}

// IndexDictionaryBrowserSupplier provides IndexDictionary.Browser instances.
type IndexDictionaryBrowserSupplier interface {
	Get() (IndexDictionary, error)
}

// NewBlockReader constructs a new BlockReader.
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
		scratchTermState:          postingsReader.NewTermState().(*codecs.BlockTermState),
	}, nil
}

func (r *BlockReader) seekCeil(searchedTerm *util.BytesRef) (spi.SeekStatus, error) {
	if r.isCurrentTerm(searchedTerm) {
		return spi.SeekStatusFound, nil
	}
	r.clearTermState()

	browser, err := r.getOrCreateDictionaryBrowser()
	if err != nil {
		return spi.SeekStatusNotFound, err
	}
	blockStartFP, err := browser.Get(searchedTerm)
	if err != nil {
		return spi.SeekStatusNotFound, err
	}
	if blockStartFP < r.fieldMetadata.firstBlockStartFP {
		blockStartFP = r.fieldMetadata.firstBlockStartFP
	}

	if r.isBeyondLastTerm(searchedTerm, blockStartFP) {
		return spi.SeekStatusEnd, nil
	}

	seekStatus, err := r.seekInBlock(searchedTerm, blockStartFP)
	if err != nil {
		return spi.SeekStatusNotFound, err
	}
	if seekStatus != spi.SeekStatusEnd {
		return seekStatus, nil
	}

	// Go to next block.
	if r.nextTerm() == nil {
		return spi.SeekStatusEnd, nil
	}
	return spi.SeekStatusNotFound, nil
}

func (r *BlockReader) seekExact(searchedTerm *util.BytesRef) (bool, error) {
	if r.isCurrentTerm(searchedTerm) {
		return true, nil
	}
	r.clearTermState()

	browser, err := r.getOrCreateDictionaryBrowser()
	if err != nil {
		return false, err
	}
	blockStartFP, err := browser.Get(searchedTerm)
	if err != nil {
		return false, err
	}
	if blockStartFP < r.fieldMetadata.firstBlockStartFP || r.isBeyondLastTerm(searchedTerm, blockStartFP) {
		return false, nil
	}

	seekStatus, err := r.seekInBlock(searchedTerm, blockStartFP)
	if err != nil {
		return false, err
	}
	return seekStatus == spi.SeekStatusFound, nil
}

func (r *BlockReader) isCurrentTerm(searchedTerm *util.BytesRef) bool {
	if r.blockLine == nil {
		return false
	}
	return util.BytesRefEquals(searchedTerm, r.blockLine.term.GetTerm())
}

func (r *BlockReader) isBeyondLastTerm(searchedTerm *util.BytesRef, blockStartFP int64) bool {
	return blockStartFP == r.fieldMetadata.lastBlockStartFP &&
		util.BytesRefCompare(searchedTerm, r.fieldMetadata.lastTerm) > 0
}

func (r *BlockReader) seekInBlock(searchedTerm *util.BytesRef, blockStartFP int64) (spi.SeekStatus, error) {
	if err := r.initializeHeader(searchedTerm, blockStartFP); err != nil {
		return spi.SeekStatusNotFound, err
	}
	if r.blockHeader == nil {
		return spi.SeekStatusEnd, nil
	}
	return r.seekInBlockInternal(searchedTerm)
}

func (r *BlockReader) seekInBlockInternal(searchedTerm *util.BytesRef) (spi.SeekStatus, error) {
	if r.compareToMiddleAndJump(searchedTerm) == 0 {
		return spi.SeekStatusFound, nil
	}

	comparisonOffset := 0
	for {
		line := r.readLineInBlock()
		if line == nil {
			return spi.SeekStatusEnd, nil
		}

		lineTermBytes := line.term
		lineTerm := lineTermBytes.GetTerm()
		suffixOffset := lineTermBytes.GetSuffixOffset()

		start := comparisonOffset
		if suffixOffset > start {
			start = suffixOffset
		}
		end := lineTerm.Length()
		if searchedTerm.Length() < end {
			end = searchedTerm.Length()
		}

		comparison := searchedTerm.Length() - lineTerm.Length()
		for i := start; i < end; i++ {
			byteDiff := int(searchedTerm.Bytes()[searchedTerm.Offset+i]) - int(lineTerm.Bytes()[lineTerm.Offset+i])
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

func (r *BlockReader) compareToMiddleAndJump(searchedTerm *util.BytesRef) int {
	if r.lineIndexInBlock != 0 {
		return -1
	}
	r.blockReadBuffer.SetPosition(r.blockHeader.middleLineOffset)
	r.lineIndexInBlock = int(r.blockHeader.linesCount >> 1)
	r.readLineInBlock()
	if r.blockLine == nil {
		return -1
	}
	return util.BytesRefCompare(searchedTerm, r.blockLine.term.GetTerm())
}

func (r *BlockReader) readLineInBlock() *BlockLine {
	if r.lineIndexInBlock >= int(r.blockHeader.linesCount) {
		r.blockLine = nil
		return r.blockLine
	}

	isIncrementalEncodingSeed := r.lineIndexInBlock == 0 || r.lineIndexInBlock == int(r.blockHeader.linesCount>>1)
	r.lineIndexInBlock++

	r.blockLine = r.blockLineReader.ReadLine(r.blockReadBuffer, isIncrementalEncodingSeed, r.scratchBlockLine)
	return r.blockLine
}

func (r *BlockReader) initializeHeader(searchedTerm *util.BytesRef, targetBlockStartFP int64) error {
	if err := r.initializeBlockReadLazily(); err != nil {
		return err
	}

	if r.blockStartFP == targetBlockStartFP {
		if r.blockHeader == nil {
			return fmt.Errorf("illegal absence of block at FP %d", blockStartFP)
		}
		if searchedTerm == nil || r.blockLine == nil || util.BytesRefCompare(searchedTerm, r.blockLine.term.GetTerm()) <= 0 {
			r.blockReadBuffer.SetPosition(r.blockFirstLineStart)
			r.lineIndexInBlock = 0
		}
	} else {
		if _, err := r.blockInput.Seek(targetBlockStartFP); err != nil {
			return err
		}
		r.blockStartFP = targetBlockStartFP
		header, err := r.readHeader()
		if err != nil {
			return err
		}
		r.blockHeader = header
		r.blockFirstLineStart = r.blockReadBuffer.GetPosition()
		r.lineIndexInBlock = 0
	}
	return nil
}

func (r *BlockReader) initializeBlockReadLazily() error {
	if r.blockStartFP != -1 && r.blockHeader != nil {
		return nil
	}
	// In a real implementation, we might clone the input here.
	r.blockHeaderReader = DefaultBlockHeaderSerializer
	r.blockLineReader = NewBlockLineSerializer()
	r.blockReadBuffer = store.NewByteArrayDataInput()
	r.termStatesReadBuffer = store.NewByteArrayDataInput()
	r.termStateSerializer = NewDeltaBaseTermStateSerializer()
	r.scratchBlockBytes = util.NewBytesRef()
	r.scratchBlockLine = &BlockLine{}
	return nil
}

func (r *BlockReader) readHeader() (*BlockHeader, error) {
	numBlockBytes, err := r.blockInput.ReadVInt()
	if err != nil {
		return nil, err
	}
	blockBytesRef, err := r.decodeBlockBytesIfNeeded(int32(numBlockBytes))
	if err != nil {
		return nil, err
	}
	r.blockReadBuffer.Reset(blockBytesRef.Bytes(), blockBytesRef.Offset, blockBytesRef.Length)
	r.termStatesReadBuffer.Reset(blockBytesRef.Bytes(), blockBytesRef.Offset, blockBytesRef.Length)
	return r.blockHeaderReader.Read(r.blockReadBuffer, r.blockHeader)
}

func (r *BlockReader) decodeBlockBytesIfNeeded(numBlockBytes int32) (*util.BytesRef, error) {
	buf := make([]byte, numBlockBytes)
	if _, err := r.blockInput.ReadBytes(buf); err != nil {
		return nil, err
	}
	r.scratchBlockBytes = util.NewBytesRef(buf)
	if r.blockDecoder == nil {
		return r.scratchBlockBytes, nil
	}
	// Assume blockDecoder.Decode returns *util.BytesRef
	return r.blockDecoder.Decode(r.blockReadBuffer, int64(numBlockBytes))
}

func (r *BlockReader) readTermStateIfNotRead() (*codecs.BlockTermState, error) {
	if r.termState == nil {
		ts, err := r.readTermState()
		if err != nil {
			return nil, err
		}
		r.termState = ts
		if r.termState != nil {
			r.termState.TermBlockOrd = r.lineIndexInBlock
			r.termState.BlockFilePointer = r.blockStartFP
		}
	}
	return r.termState, nil
}

func (r *BlockReader) readTermState() (*codecs.BlockTermState, error) {
	r.termStatesReadBuffer.SetPosition(
		r.blockFirstLineStart +
			int(r.blockHeader.termStatesBaseOffset) +
			int(r.blockLine.termStateRelativeOffset))

	return r.termStateSerializer.ReadTermState(
		r.blockHeader.baseDocsFP,
		r.blockHeader.basePositionsFP,
		r.blockHeader.basePayloadsFP,
		r.termStatesReadBuffer,
		r.fieldMetadata.fieldInfo,
		r.scratchTermState,
	)
}

func (r *BlockReader) nextTerm() *util.BytesRef {
	if r.blockHeader == nil {
		if err := r.initializeHeader(nil, r.fieldMetadata.firstBlockStartFP); err != nil {
			return nil
		}
		if r.blockHeader == nil {
			return nil
		}
	}

	if r.readLineInBlock() == nil {
		if err := r.initializeHeader(nil, r.blockInput.GetFilePointer()); err != nil {
			return nil
		}
		if r.blockHeader == nil {
			return nil
		}
		r.readLineInBlock()
	}
	if r.blockLine == nil {
		return nil
	}
	return r.blockLine.term.GetTerm()
}

// TermsEnum implementation

func (r *BlockReader) Next() (*spi.Term, error) {
	termBytes := r.nextTerm()
	if termBytes == nil {
		return nil, nil
	}
	return spi.NewTermFromBytesRef(r.fieldMetadata.fieldInfo.Name(), termBytes), nil
}

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
		termBytes := r.nextTerm()
		if termBytes == nil {
			return nil, nil
		}
		return spi.NewTermFromBytesRef(r.fieldMetadata.fieldInfo.Name(), termBytes), nil
	}
	return spi.NewTermFromBytesRef(r.fieldMetadata.fieldInfo.Name(), r.term()), nil
}

func (r *BlockReader) SeekExact(term *spi.Term) (bool, error) {
	found, err := r.seekExact(term.BytesValue())
	return found, err
}

func (r *BlockReader) Term() *spi.Term {
	termBytes := r.term()
	if termBytes == nil {
		return nil
	}
	return spi.NewTermFromBytesRef(r.fieldMetadata.fieldInfo.Name(), termBytes)
}

func (r *BlockReader) DocFreq() (int, error) {
	ts, err := r.readTermStateIfNotRead()
	if err != nil {
		return 0, err
	}
	return int(ts.docFreq), nil
}

func (r *BlockReader) TotalTermFreq() (int64, error) {
	ts, err := r.readTermStateIfNotRead()
	if err != nil {
		return 0, err
	}
	return ts.totalTermFreq, nil
}

func (r *BlockReader) Postings(flags int) (spi.PostingsEnum, error) {
	ts, err := r.readTermStateIfNotRead()
	if err != nil {
		return nil, err
	}
	return r.postingsReader.Postings(r.fieldMetadata.fieldInfo, ts, nil, flags)
}

func (r *BlockReader) PostingsWithLiveDocs(liveDocs util.Bits, flags int) (spi.PostingsEnum, error) {
	// Not implemented in this port yet.
	return nil, fmt.Errorf("PostingsWithLiveDocs not implemented")
}

func (r *BlockReader) getOrCreateDictionaryBrowser() (IndexDictionary, error) {
	if r.dictionaryBrowser == nil {
		browser, err := r.dictionaryBrowserSupplier.Get()
		if err != nil {
			return nil, err
		}
		r.dictionaryBrowser = browser
	}
	return r.dictionaryBrowser, nil
}

func (r *BlockReader) clearTermState() {
	r.termState = nil
	r.termStateForced = false
}

type BlockDecoder interface {
	Decode(in store.DataInput, length int64) (*util.BytesRef, error)
}

type IndexDictionary interface {
	Get(term *util.BytesRef) (int64, error)
}

type IndexDictionaryBrowserSupplier interface {
	Get() (IndexDictionary, error)
}
