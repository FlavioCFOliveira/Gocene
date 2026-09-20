// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package uniformsplit

import (
	"sync"

	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/fst"
)

// FSTDictionary is an immutable stateless FST-based index dictionary kept in
// memory.
//
// Use FSTDictionaryBuilder to build the IndexDictionary.
//
// Create a stateful FSTDictionaryBrowser to seek a term in this FSTDictionary
// and get its corresponding block file pointer to the terms block file.
//
// Its greatest advantage is to be very compact in memory thanks to both the
// compaction of the FST as a byte array, and the incremental encoding of the
// leaves block pointer values, which are long integers in increasing order,
// with PositiveIntOutputs. With a compact dictionary in memory we can increase
// the number of blocks. This allows us to reduce the average block size, which
// means faster scan inside a block.
//
// Mirrors org.apache.lucene.codecs.uniformsplit.FSTDictionary from Apache
// Lucene 10.5.0.
type FSTDictionary struct {
	fst *fst.FST[int64]
}

// NewFSTDictionary constructs an FSTDictionary over the provided FST. Mirrors
// the protected FSTDictionary(FST<Long>) constructor (FSTDictionary.java:53).
func NewFSTDictionary(f *fst.FST[int64]) *FSTDictionary {
	return &FSTDictionary{fst: f}
}

// Write writes this dictionary to the provided output.
//
// blockEncoder is the BlockEncoder for specific encoding of this index
// dictionary; or nil if none.
//
// Mirrors FSTDictionary.write (FSTDictionary.java:58). Java's
// ByteBuffersDataOutput.newResettableInstance() only adds buffer recycling to
// the same output; Gocene's store package exposes no recycling constructor, so
// the plain instance is used and the written bytes are unchanged.
func (d *FSTDictionary) Write(output store.DataOutput, blockEncoder BlockEncoder) error {
	if blockEncoder == nil {
		return d.fst.Save(output, output)
	}
	bytesDataOutput := store.NewByteBuffersDataOutput()
	if err := d.fst.Save(bytesDataOutput, bytesDataOutput); err != nil {
		return err
	}
	encodedBytes, err := blockEncoder.Encode(bytesDataOutput.ToDataInput(), bytesDataOutput.Size())
	if err != nil {
		return err
	}
	if err := output.WriteVLong(encodedBytes.Size()); err != nil {
		return err
	}
	return encodedBytes.WriteTo(output)
}

// ReadFSTDictionary reads an FSTDictionary from the provided input.
//
// blockDecoder is the BlockDecoder to use for specific decoding; or nil if
// none.
//
// Mirrors the protected static FSTDictionary.read (FSTDictionary.java:74).
func ReadFSTDictionary(input store.DataInput, blockDecoder BlockDecoder, isFSTOnHeap bool) (*FSTDictionary, error) {
	var fstDataInput store.DataInput
	if blockDecoder == nil {
		fstDataInput = input
	} else {
		numBytes, err := input.ReadVLong()
		if err != nil {
			return nil, err
		}
		decodedBytes, err := blockDecoder.Decode(input, numBytes)
		if err != nil {
			return nil, err
		}
		fstDataInput = store.NewByteArrayDataInputWithOffset(decodedBytes.Bytes, 0, decodedBytes.Length)
		// OffHeapFSTStore.init() requires a DataInput which is an instance of IndexInput.
		// When the block is decoded we must load the FST on heap.
		isFSTOnHeap = true
	}
	fstOutputs := fst.PositiveIntOutputs()
	metadata, err := fst.ReadMetadata(fstDataInput, fstOutputs)
	if err != nil {
		return nil, err
	}
	if isFSTOnHeap {
		f, err := fst.NewFSTFromDataInput(metadata, fstDataInput)
		if err != nil {
			return nil, err
		}
		return NewFSTDictionary(f), nil
	}
	// Java casts the DataInput back to IndexInput and wraps it in an
	// OffHeapFSTStore built from in.randomAccessSlice(offset, numBytes).
	// Gocene's IndexInput exposes no randomAccessSlice, so the store is built
	// from the input when it is itself a RandomAccessInput — the same test the
	// block-tree field reader makes.
	indexInput, ok := fstDataInput.(store.IndexInput)
	if !ok {
		return nil, errFSTDictionaryNotIndexInput
	}
	rai, ok := indexInput.(store.RandomAccessInput)
	if !ok {
		return nil, errFSTDictionaryNotRandomAccess
	}
	offHeap, err := fst.NewOffHeapFSTStore(rai, indexInput.GetFilePointer(), metadata.NumBytes())
	if err != nil {
		return nil, err
	}
	f, err := fst.FromFSTReader(metadata, offHeap)
	if err != nil {
		return nil, err
	}
	return NewFSTDictionary(f), nil
}

// errFSTDictionaryNotIndexInput and errFSTDictionaryNotRandomAccess report the
// two ways the off-heap path of ReadFSTDictionary cannot be taken. Java relies
// on the `(IndexInput) fstDataInput` cast, which raises ClassCastException.
var (
	errFSTDictionaryNotIndexInput   = errFSTDictionary("FSTDictionary.read: off-heap FST requires an IndexInput")
	errFSTDictionaryNotRandomAccess = errFSTDictionary("FSTDictionary.read: off-heap FST requires a RandomAccessInput")
)

type errFSTDictionary string

func (e errFSTDictionary) Error() string { return string(e) }

// Browser creates a new IndexDictionaryBrowser.
//
// Mirrors FSTDictionary.browser (FSTDictionary.java:104).
func (d *FSTDictionary) Browser() (IndexDictionaryBrowser, error) {
	return NewFSTDictionaryBrowser(d)
}

// FSTDictionaryBrowser is a stateful browser to seek a term in an
// FSTDictionary and get its corresponding block file pointer in the block file.
//
// Mirrors the inner class
// org.apache.lucene.codecs.uniformsplit.FSTDictionary.Browser
// (FSTDictionary.java:112).
type FSTDictionaryBrowser struct {
	fstEnum *fst.BytesRefFSTEnum[int64]
}

// NewFSTDictionaryBrowser constructs an FSTDictionaryBrowser over the FST of
// the provided dictionary.
func NewFSTDictionaryBrowser(dictionary *FSTDictionary) (*FSTDictionaryBrowser, error) {
	fstEnum, err := fst.NewBytesRefFSTEnum(dictionary.fst)
	if err != nil {
		return nil, err
	}
	return &FSTDictionaryBrowser{fstEnum: fstEnum}, nil
}

// SeekBlock seeks the given term in the FSTDictionary and returns its
// corresponding block file pointer.
//
// Mirrors FSTDictionary.Browser.seekBlock (FSTDictionary.java:117).
func (b *FSTDictionaryBrowser) SeekBlock(term *util.BytesRef) (int64, error) {
	seekFloor, err := b.fstEnum.SeekFloor(term)
	if err != nil {
		return 0, err
	}
	if seekFloor == nil {
		return -1, nil
	}
	return seekFloor.Output, nil
}

// FSTDictionaryBrowserSupplier provides a stateful FSTDictionaryBrowser to seek
// in the FSTDictionary.
//
// Mirrors the nested class
// org.apache.lucene.codecs.uniformsplit.FSTDictionary.BrowserSupplier
// (FSTDictionary.java:129).
type FSTDictionaryBrowserSupplier struct {
	dictionaryInput store.IndexInput
	blockDecoder    BlockDecoder
	isFSTOnHeap     bool

	// dictionary is the lazy loaded immutable index dictionary FST. The FST is
	// either kept off-heap, or held in RAM on-heap.
	dictionary IndexDictionary

	// mu renders the `synchronized (this)` block of BrowserSupplier.get.
	mu sync.Mutex
}

// NewFSTDictionaryBrowserSupplier constructs an FSTDictionaryBrowserSupplier
// positioned at dictionaryStartFP on a clone of dictionaryInput.
//
// Mirrors the BrowserSupplier constructor (FSTDictionary.java:140).
func NewFSTDictionaryBrowserSupplier(
	dictionaryInput store.IndexInput,
	dictionaryStartFP int64,
	blockDecoder BlockDecoder,
	isFSTOnHeap bool,
) (*FSTDictionaryBrowserSupplier, error) {
	clone := dictionaryInput.Clone()
	if err := clone.SetPosition(dictionaryStartFP); err != nil {
		return nil, err
	}
	return &FSTDictionaryBrowserSupplier{
		dictionaryInput: clone,
		blockDecoder:    blockDecoder,
		isFSTOnHeap:     isFSTOnHeap,
	}, nil
}

// Get returns a new stateful IndexDictionaryBrowser, loading the immutable
// dictionary on first use.
//
// Mirrors BrowserSupplier.get (FSTDictionary.java:154). Java uses the
// double-checked locking idiom, which is safe there because the dictionary is
// immutable; Go's memory model gives no such guarantee for an unsynchronised
// read, so the mutex guards the whole check.
func (s *FSTDictionaryBrowserSupplier) Get() (IndexDictionaryBrowser, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.dictionary == nil {
		dictionary, err := ReadFSTDictionary(s.dictionaryInput, s.blockDecoder, s.isFSTOnHeap)
		if err != nil {
			return nil, err
		}
		s.dictionary = dictionary
	}
	return s.dictionary.Browser()
}

// FSTDictionaryBuilder builds an immutable FSTDictionary.
//
// Mirrors the nested class
// org.apache.lucene.codecs.uniformsplit.FSTDictionary.Builder
// (FSTDictionary.java:174).
type FSTDictionaryBuilder struct {
	fstCompiler *fst.FSTCompiler[int64]
	scratchInts *util.IntsRefBuilder
}

// NewFSTDictionaryBuilder constructs an FSTDictionaryBuilder.
//
// Mirrors the Builder() constructor (FSTDictionary.java:179).
func NewFSTDictionaryBuilder() (*FSTDictionaryBuilder, error) {
	outputs := fst.PositiveIntOutputs()
	fstCompiler := fst.NewFSTCompilerBuilder(fst.InputTypeByte1, outputs).Build()
	return &FSTDictionaryBuilder{
		fstCompiler: fstCompiler,
		scratchInts: util.NewIntsRefBuilder(),
	}, nil
}

// Add adds a [block key - block file pointer] entry to the dictionary.
//
// Mirrors FSTDictionary.Builder.add (FSTDictionary.java:186).
func (b *FSTDictionaryBuilder) Add(blockKey *util.BytesRef, blockFilePointer int64) error {
	return b.fstCompiler.Add(fst.ToIntsRef(blockKey, b.scratchInts), blockFilePointer)
}

// Build builds the immutable FSTDictionary for the added entries.
//
// Mirrors FSTDictionary.Builder.build (FSTDictionary.java:191).
func (b *FSTDictionaryBuilder) Build() (IndexDictionary, error) {
	metadata, err := b.fstCompiler.Compile()
	if err != nil {
		return nil, err
	}
	f, err := fst.FromFSTReader(metadata, b.fstCompiler.GetFSTReader())
	if err != nil {
		return nil, err
	}
	return NewFSTDictionary(f), nil
}

var (
	_ IndexDictionary                = (*FSTDictionary)(nil)
	_ IndexDictionaryBrowser         = (*FSTDictionaryBrowser)(nil)
	_ IndexDictionaryBrowserSupplier = (*FSTDictionaryBrowserSupplier)(nil)
	_ IndexDictionaryBuilder         = (*FSTDictionaryBuilder)(nil)
)
