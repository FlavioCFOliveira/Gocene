// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package uniformsplit

import (
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// IndexDictionary is an immutable stateless index dictionary kept in RAM.
//
// Implementations must be immutable.
//
// Use IndexDictionaryBuilder to build the IndexDictionary.
//
// Create a stateful IndexDictionaryBrowser to seek a term in this IndexDictionary
// and get its corresponding block file pointer to the terms block file.
//
// There is a single implementation of this interface, FSTDictionary. However this
// interface allows you to plug easily a new kind of index dictionary to experiment
// and improve the existing one.
type IndexDictionary interface {
	// Write writes this dictionary to the provided output.
	//
	// blockEncoder is the BlockEncoder for specific encoding of this index dictionary;
	// or nil if none.
	Write(output store.DataOutput, blockEncoder BlockEncoder) error

	// Browser creates a new IndexDictionaryBrowser.
	Browser() (IndexDictionaryBrowser, error)
}

// IndexDictionaryBuilder builds an immutable IndexDictionary.
type IndexDictionaryBuilder interface {
	// Add adds a [block key - block file pointer] entry to the dictionary.
	//
	// The Uniform Split technique adds block keys in the dictionary. See BlockReader
	// and TermBytes for more info about block key and minimal distinguishing prefix (MDP).
	//
	// All block keys are added in strictly increasing order of the block file pointers,
	// this allows long encoding optimizations such as with
	// org.apache.lucene.util.fst.PositiveIntOutputs for org.apache.lucene.util.fst.FST.
	//
	// blockKey is the block key which is the minimal distinguishing prefix (MDP) of the
	// first term of a block.
	// blockFilePointer is a non-negative file pointer to the start of the block in the
	// block file.
	Add(blockKey *util.BytesRef, blockFilePointer int64) error

	// Build builds the immutable IndexDictionary for the added entries.
	Build() (IndexDictionary, error)
}

// IndexDictionaryBrowser is a stateful browser to seek a term in an IndexDictionary
// and get its corresponding block file pointer in the block file.
type IndexDictionaryBrowser interface {
	// SeekBlock seeks the given term in the IndexDictionary and returns its
	// corresponding block file pointer.
	//
	// Returns the block file pointer corresponding to the term if it matches exactly
	// a block key in the dictionary. Otherwise the floor block key, which is the
	// greatest block key present in the dictionary that is alphabetically
	// preceding the searched term. Otherwise -1 if there is no floor block key
	// because the searched term precedes alphabetically the first block key of
	// the dictionary.
	SeekBlock(term *util.BytesRef) (int64, error)
}

// IndexDictionaryBrowserSupplier is a supplier for a new stateful IndexDictionaryBrowser
// created on the immutable IndexDictionary.
//
// The immutable IndexDictionary is lazy loaded thread safely. This lazy loading allows
// us to load it only when org.apache.lucene.index.TermsEnum.SeekCeil or
// org.apache.lucene.index.TermsEnum.SeekExact are called (it is not loaded for a
// direct all-terms enumeration).
type IndexDictionaryBrowserSupplier interface {
	GetBrowser() (IndexDictionaryBrowser, error)
}
