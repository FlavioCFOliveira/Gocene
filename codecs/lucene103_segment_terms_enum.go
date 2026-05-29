// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License. You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

package codecs

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// Reference: lucene/core/src/java/org/apache/lucene/codecs/lucene103/blocktree/
// SegmentTermsEnum.java (Apache Lucene 10.4.0).
//
// Lucene103SegmentTermsEnum is the Go port of
// org.apache.lucene.codecs.lucene103.blocktree.SegmentTermsEnum — the strict
// block-tree TermsEnum returned by Lucene103FieldReader.iterator(). It walks
// the field's trie index ([TrieReader]) to seek and the .tim block file to
// scan, with full support for the three structures the previous Next()-only
// stub deferred (backlog #2692, resolved under rmp #4754):
//
//   - A frame stack (getFrame / pushFrame) so the enum descends sub-blocks of
//     a high-cardinality field instead of skipping them.
//   - seekExact / seekCeil driven by the trie index (prepareSeekExact), with
//     common-prefix re-use across successive seeks.
//   - Floor blocks (scanToFloorFrame / loadNextFloorBlock) and scanToTerm for
//     both leaf and non-leaf blocks.
//
// The per-block decode logic lives in [segmentTermsEnumFrame].
type Lucene103SegmentTermsEnum struct {
	// in is the per-enum clone of the parent reader's .tim IndexInput. Nil
	// until the first loadBlock (lazy init mirrors Java).
	in store.IndexInput

	// stack holds the active frames, indexed by frame ord. staticFrame (ord
	// -1) holds seek-by-TermState / cached-seek state. currentFrame is the
	// frame the cursor currently sits in.
	stack        []*segmentTermsEnumFrame
	staticFrame  *segmentTermsEnumFrame
	currentFrame *segmentTermsEnumFrame

	// termExists is true when the cursor sits on a real term (false when it
	// sits on a sub-block boundary or a not-found position).
	termExists bool

	fr *Lucene103FieldReader

	// targetBeforeCurrentLength records the seek-state frame ord that remains
	// valid for the in-flight seek (see prepareSeekExact).
	targetBeforeCurrentLength int

	// validIndexPrefix is the prefix length of the current term that is backed
	// by valid seek frames; 0 while only next()-ing.
	validIndexPrefix int

	eof bool

	// term is the growable buffer backing Term().
	term *util.BytesRefBuilder

	trieReader *TrieReader
	// nodes caches the trie nodes visited along the current seek path, indexed
	// by depth (nodes[0] is the root).
	nodes []*TrieNode

	// seedStart caches the optional seek term supplied at construction so the
	// first Next() call can honour the FieldReader.GetIteratorWithSeek
	// contract (seek-then-iterate). nil for a plain iterator.
	seedStart *util.BytesRef
}

// NewLucene103SegmentTermsEnum opens a strict-block-tree enumerator over field.
// startTerm is accepted for source compatibility with the previous stub and the
// FieldReader.GetIteratorWithSeek contract; when non-nil the enum performs a
// SeekCeil to it on first use. nil means "start before the first term".
//
// Mirrors SegmentTermsEnum(FieldReader, TrieReader) in Java.
func NewLucene103SegmentTermsEnum(field *Lucene103FieldReader, startTerm *util.BytesRef) *Lucene103SegmentTermsEnum {
	e := &Lucene103SegmentTermsEnum{
		fr:        field,
		term:      util.NewBytesRefBuilder(),
		nodes:     make([]*TrieNode, 1),
		seedStart: startTerm,
	}
	// staticFrame is allocated lazily in the first call that needs a frame,
	// because the postings reader (needed for state allocation) is reachable
	// only through a non-nil parent.
	return e
}

// Field returns the FieldReader the enumerator is bound to.
func (e *Lucene103SegmentTermsEnum) Field() *Lucene103FieldReader { return e.fr }

// initIndexInput lazily clones the parent reader's .tim IndexInput and
// allocates the static frame + trie reader. Mirrors SegmentTermsEnum.initIndexInput.
func (e *Lucene103SegmentTermsEnum) initIndexInput() error {
	if e.in != nil {
		return nil
	}
	if e.fr == nil || e.fr.parent == nil {
		return errors.New("Lucene103SegmentTermsEnum: field reader has no parent (terms input unavailable)")
	}
	if e.fr.parent.termsIn == nil {
		return fmt.Errorf("Lucene103SegmentTermsEnum: parent reader %q has nil terms input", e.fr.parent.SegmentName())
	}
	e.in = e.fr.parent.termsIn.Clone()
	return nil
}

// ensureSetup performs the one-time, cheap setup shared by every entry point:
// it opens the trie reader, seeds nodes[0] with the trie root and allocates the
// static frame. It deliberately does NOT clone the .tim input — that is the job
// of initIndexInput, which loadBlock calls lazily. Keeping e.in nil until the
// first block load preserves the "in == nil" sentinel Next() uses to detect a
// fresh enumerator (mirrors the Java constructor, whose body sets up exactly
// these three things and leaves `in` null).
func (e *Lucene103SegmentTermsEnum) ensureSetup() error {
	if e.staticFrame != nil {
		return nil
	}
	if e.fr == nil || e.fr.parent == nil {
		return errors.New("Lucene103SegmentTermsEnum: field reader has no parent")
	}
	tr, err := e.fr.NewTrieReader()
	if err != nil {
		return fmt.Errorf("Lucene103SegmentTermsEnum: open trie: %w", err)
	}
	e.trieReader = tr
	sf, err := newSegmentTermsEnumFrame(e, -1)
	if err != nil {
		return err
	}
	e.staticFrame = sf
	e.currentFrame = sf
	e.nodes[0] = tr.Root()
	e.validIndexPrefix = 0
	return nil
}

// growTerm ensures the term builder has at least n bytes of capacity and sets
// its logical length to n, preserving any existing prefix bytes. The grow/
// set-length order is the safe order for Gocene's BytesRefBuilder, whose
// SetLength only reslices when capacity already suffices.
func (e *Lucene103SegmentTermsEnum) growTerm(n int) {
	e.term.Grow(n)
	e.term.SetLength(n)
}

// ensureTermAddressable grows the term builder so that byte positions [0, n)
// are all addressable through SetByteAt during a trie walk. Unlike the Java
// BytesRefBuilder — whose bytes() exposes the full backing array regardless of
// logical length — Gocene's BytesRefBuilder.Bytes() is resliced to its logical
// length, so the walk must extend the visible slice before writing prefix bytes
// one position at a time. The final logical length is fixed later by fillTerm /
// scanToTerm via growTerm; pre-extending here only widens the addressable
// window and never shrinks term content (it copies any existing prefix).
func (e *Lucene103SegmentTermsEnum) ensureTermAddressable(n int) {
	if n <= e.term.Length() {
		return
	}
	e.term.Grow(n)
	e.term.SetLength(n)
}

// getFrame returns the frame at the given ord, growing stack as needed.
// Mirrors SegmentTermsEnum.getFrame.
func (e *Lucene103SegmentTermsEnum) getFrame(ord int) (*segmentTermsEnumFrame, error) {
	if ord >= len(e.stack) {
		next := make([]*segmentTermsEnumFrame, util.Oversize(1+ord, 8))
		copy(next, e.stack)
		for stackOrd := len(e.stack); stackOrd < len(next); stackOrd++ {
			f, err := newSegmentTermsEnumFrame(e, stackOrd)
			if err != nil {
				return nil, err
			}
			next[stackOrd] = f
		}
		e.stack = next
	}
	return e.stack[ord], nil
}

// getNode returns the cached trie node at the given depth, growing nodes as
// needed. Mirrors SegmentTermsEnum.getNode.
func (e *Lucene103SegmentTermsEnum) getNode(ord int) *TrieNode {
	if ord >= len(e.nodes) {
		next := make([]*TrieNode, util.Oversize(1+ord, 8))
		copy(next, e.nodes)
		for nodeOrd := len(e.nodes); nodeOrd < len(next); nodeOrd++ {
			next[nodeOrd] = NewTrieNode()
		}
		e.nodes = next
	}
	if e.nodes[ord] == nil {
		e.nodes[ord] = NewTrieNode()
	}
	return e.nodes[ord]
}

// pushFrame pushes a frame we seek'd to: it copies hasTerms / isFloor / floor
// data from node, then delegates to pushFrameFP with node.OutputFP. Mirrors
// the two-arg SegmentTermsEnum.pushFrame(node, length).
func (e *Lucene103SegmentTermsEnum) pushFrame(node *TrieNode, length int) (*segmentTermsEnumFrame, error) {
	f, err := e.getFrame(1 + e.currentFrame.ord)
	if err != nil {
		return nil, err
	}
	f.hasTerms = node.HasTerms
	f.hasTermsOrig = f.hasTerms
	f.isFloor = node.IsFloor()
	if f.isFloor {
		fin, err := e.trieReader.FloorData(node)
		if err != nil {
			return nil, fmt.Errorf("pushFrame: open floor data: %w", err)
		}
		if err := f.setFloorData(fin); err != nil {
			return nil, err
		}
	}
	return e.pushFrameFP(node, node.OutputFP, length)
}

// pushFrameFP pushes a next'd or seek'd frame at the given file pointer,
// re-using the frame slot when it already points at the same block. Mirrors the
// three-arg SegmentTermsEnum.pushFrame(node, fp, length).
func (e *Lucene103SegmentTermsEnum) pushFrameFP(node *TrieNode, fp int64, length int) (*segmentTermsEnumFrame, error) {
	f, err := e.getFrame(1 + e.currentFrame.ord)
	if err != nil {
		return nil, err
	}
	f.node = node
	if f.fpOrig == fp && f.nextEnt != -1 {
		if f.ord > e.targetBeforeCurrentLength {
			if err := f.rewind(); err != nil {
				return nil, err
			}
		}
		if length != f.prefixLength {
			return nil, fmt.Errorf("pushFrameFP: length=%d != prefixLength=%d on reused frame", length, f.prefixLength)
		}
	} else {
		f.nextEnt = -1
		f.prefixLength = length
		f.state.TermBlockOrd = 0
		f.fpOrig = fp
		f.fp = fp
		f.lastSubFP = -1
	}
	return f, nil
}

// Term returns the current cursor position, or nil at EOF.
// Mirrors SegmentTermsEnum.term() in Java.
func (e *Lucene103SegmentTermsEnum) Term() *index.Term {
	if e.eof || e.currentFrame == nil {
		return nil
	}
	return index.NewTermFromBytesRef(e.fr.fieldInfo.Name(), e.term.Get())
}

// prepareSeekExact walks the trie index to position the cursor for target.
// It returns frameReady=true when a candidate block was found that still needs
// loadBlock + scanToTerm (the caller then performs those), found=true when the
// target equals the already-positioned current term, and ok=false when the
// target is provably absent (outside [min,max] or a fast NOT_FOUND on a node
// with no terms).
//
// Mirrors SegmentTermsEnum.prepareSeekExact (with prefetch elided — Gocene's
// IndexInput has no prefetch hook on this path).
func (e *Lucene103SegmentTermsEnum) prepareSeekExact(target *util.BytesRef) (frameReady bool, found bool, ok bool, err error) {
	if err := e.ensureSetup(); err != nil {
		return false, false, false, err
	}

	// Reject targets outside [min, max] when the field is non-empty.
	if e.fr.NumTerms() > 0 {
		if e.fr.minTerm != nil && util.BytesRefCompare(target, e.fr.minTerm) < 0 {
			return false, false, false, nil
		}
		if e.fr.maxTerm != nil && util.BytesRefCompare(target, e.fr.maxTerm) > 0 {
			return false, false, false, nil
		}
	}

	e.ensureTermAddressable(1 + target.Length)
	e.eof = false

	var node *TrieNode
	var targetUpto int
	e.targetBeforeCurrentLength = e.currentFrame.ord

	if e.currentFrame != e.staticFrame {
		// Re-use the common prefix of the previous seek.
		node = e.nodes[0]
		targetUpto = 0
		lastFrame := e.stack[0]
		targetLimit := target.Length
		if e.validIndexPrefix < targetLimit {
			targetLimit = e.validIndexPrefix
		}
		cmp := 0
		for targetUpto < targetLimit {
			cmp = int(e.term.ByteAt(targetUpto)&0xff) - int(target.Bytes[target.Offset+targetUpto]&0xff)
			if cmp != 0 {
				break
			}
			node = e.nodes[1+targetUpto]
			if node.HasOutput() {
				lastFrame = e.stack[1+lastFrame.ord]
			}
			targetUpto++
		}
		if cmp == 0 {
			cmp = compareBytes(
				e.term.Bytes(), targetUpto, e.term.Length(),
				target.Bytes, target.Offset+targetUpto, target.Offset+target.Length,
			)
		}
		if cmp < 0 {
			e.currentFrame = lastFrame
		} else if cmp > 0 {
			e.targetBeforeCurrentLength = lastFrame.ord
			e.currentFrame = lastFrame
			if err := e.currentFrame.rewind(); err != nil {
				return false, false, false, err
			}
		} else {
			// Target equals current term.
			if e.termExists {
				return false, true, true, nil
			}
		}
	} else {
		e.targetBeforeCurrentLength = -1
		node = e.trieReader.Root()
		e.nodes[0] = node
		e.currentFrame = e.staticFrame
		targetUpto = 0
		cf, err := e.pushFrame(node, 0)
		if err != nil {
			return false, false, false, err
		}
		e.currentFrame = cf
	}

	for targetUpto < target.Length {
		targetLabel := int(target.Bytes[target.Offset+targetUpto]) & 0xff
		nextNode, err := e.trieReader.LookupChild(targetLabel, node, e.getNode(1+targetUpto))
		if err != nil {
			return false, false, false, err
		}
		if nextNode == nil {
			// Index exhausted.
			e.validIndexPrefix = e.currentFrame.prefixLength
			if err := e.currentFrame.scanToFloorFrame(target); err != nil {
				return false, false, false, err
			}
			if !e.currentFrame.hasTerms {
				e.termExists = false
				e.growTerm(1 + targetUpto)
				e.term.SetByteAt(targetUpto, byte(targetLabel))
				return false, false, false, nil
			}
			return true, false, true, nil
		}
		node = nextNode
		e.term.SetByteAt(targetUpto, byte(targetLabel))
		targetUpto++
		if node.HasOutput() {
			cf, err := e.pushFrame(node, targetUpto)
			if err != nil {
				return false, false, false, err
			}
			e.currentFrame = cf
		}
	}

	e.validIndexPrefix = e.currentFrame.prefixLength
	if err := e.currentFrame.scanToFloorFrame(target); err != nil {
		return false, false, false, err
	}
	if !e.currentFrame.hasTerms {
		e.termExists = false
		e.growTerm(targetUpto)
		return false, false, false, nil
	}
	return true, false, true, nil
}

// SeekExact positions the enumerator on term, returning true if found.
// Mirrors SegmentTermsEnum.seekExact(BytesRef).
func (e *Lucene103SegmentTermsEnum) SeekExact(term *index.Term) (bool, error) {
	if term == nil {
		return false, errors.New("Lucene103SegmentTermsEnum.SeekExact: term must not be nil")
	}
	target := term.BytesValue()
	frameReady, found, ok, err := e.prepareSeekExact(target)
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}
	if found {
		return true, nil
	}
	if !frameReady {
		return false, nil
	}
	if err := e.currentFrame.loadBlock(); err != nil {
		return false, err
	}
	status, err := e.currentFrame.scanToTerm(target, true)
	if err != nil {
		return false, err
	}
	return status == index.SeekStatusFound, nil
}

// SeekCeil positions the enumerator at term, or at the next ceiling term if it
// does not exist. Returns the positioned term or nil at END.
// Mirrors SegmentTermsEnum.seekCeil(BytesRef).
func (e *Lucene103SegmentTermsEnum) SeekCeil(term *index.Term) (*index.Term, error) {
	if term == nil {
		return nil, errors.New("Lucene103SegmentTermsEnum.SeekCeil: term must not be nil")
	}
	if err := e.ensureSetup(); err != nil {
		return nil, err
	}
	target := term.BytesValue()
	e.ensureTermAddressable(1 + target.Length)
	e.eof = false

	var node *TrieNode
	var targetUpto int
	e.targetBeforeCurrentLength = e.currentFrame.ord

	if e.currentFrame != e.staticFrame {
		node = e.nodes[0]
		targetUpto = 0
		lastFrame := e.stack[0]
		targetLimit := target.Length
		if e.validIndexPrefix < targetLimit {
			targetLimit = e.validIndexPrefix
		}
		cmp := 0
		for targetUpto < targetLimit {
			cmp = int(e.term.ByteAt(targetUpto)&0xff) - int(target.Bytes[target.Offset+targetUpto]&0xff)
			if cmp != 0 {
				break
			}
			node = e.nodes[1+targetUpto]
			if node.HasOutput() {
				lastFrame = e.stack[1+lastFrame.ord]
			}
			targetUpto++
		}
		if cmp == 0 {
			cmp = compareBytes(
				e.term.Bytes(), targetUpto, e.term.Length(),
				target.Bytes, target.Offset+targetUpto, target.Offset+target.Length,
			)
		}
		if cmp < 0 {
			e.currentFrame = lastFrame
		} else if cmp > 0 {
			e.targetBeforeCurrentLength = 0
			e.currentFrame = lastFrame
			if err := e.currentFrame.rewind(); err != nil {
				return nil, err
			}
		} else {
			if e.termExists {
				return e.Term(), nil
			}
		}
	} else {
		e.targetBeforeCurrentLength = -1
		node = e.trieReader.Root()
		e.nodes[0] = node
		e.currentFrame = e.staticFrame
		targetUpto = 0
		cf, err := e.pushFrame(node, 0)
		if err != nil {
			return nil, err
		}
		e.currentFrame = cf
	}

	for targetUpto < target.Length {
		targetLabel := int(target.Bytes[target.Offset+targetUpto]) & 0xff
		nextNode, err := e.trieReader.LookupChild(targetLabel, node, e.getNode(1+targetUpto))
		if err != nil {
			return nil, err
		}
		if nextNode == nil {
			e.validIndexPrefix = e.currentFrame.prefixLength
			if err := e.currentFrame.scanToFloorFrame(target); err != nil {
				return nil, err
			}
			if err := e.currentFrame.loadBlock(); err != nil {
				return nil, err
			}
			status, err := e.currentFrame.scanToTerm(target, false)
			if err != nil {
				return nil, err
			}
			return e.finishSeekCeil(status, target)
		}
		e.term.SetByteAt(targetUpto, byte(targetLabel))
		node = nextNode
		targetUpto++
		if node.HasOutput() {
			cf, err := e.pushFrame(node, targetUpto)
			if err != nil {
				return nil, err
			}
			e.currentFrame = cf
		}
	}

	e.validIndexPrefix = e.currentFrame.prefixLength
	if err := e.currentFrame.scanToFloorFrame(target); err != nil {
		return nil, err
	}
	if err := e.currentFrame.loadBlock(); err != nil {
		return nil, err
	}
	status, err := e.currentFrame.scanToTerm(target, false)
	if err != nil {
		return nil, err
	}
	return e.finishSeekCeil(status, target)
}

// finishSeekCeil resolves the seekCeil result after scanToTerm: on END it
// copies the target into the term buffer and advances via Next() to the
// ceiling. Mirrors the tail of SegmentTermsEnum.seekCeil.
func (e *Lucene103SegmentTermsEnum) finishSeekCeil(status index.SeekStatus, target *util.BytesRef) (*index.Term, error) {
	if status == index.SeekStatusEnd {
		e.term.CopyBytes(target.Bytes, target.Offset, target.Length)
		e.termExists = false
		nxt, err := e.Next()
		if err != nil {
			return nil, err
		}
		return nxt, nil // nil == END, non-nil == NOT_FOUND ceiling
	}
	// FOUND or NOT_FOUND: the cursor sits on the matching / ceiling term.
	return e.Term(), nil
}

// Next advances to the next term in the block-tree, descending sub-blocks and
// crossing floor blocks. Mirrors SegmentTermsEnum.next().
func (e *Lucene103SegmentTermsEnum) Next() (*index.Term, error) {
	if err := e.ensureSetup(); err != nil {
		return nil, err
	}

	// Honour a seek term supplied at construction (GetIteratorWithSeek).
	if e.seedPending() {
		if _, err := e.SeekCeil(index.NewTermFromBytesRef(e.fr.fieldInfo.Name(), e.seedStart)); err != nil {
			return nil, err
		}
		e.seedStart = nil
		if e.eof {
			return nil, nil
		}
		return e.Term(), nil
	}

	if e.in == nil {
		// Fresh enum: seek to first term.
		node := e.trieReader.Root()
		e.nodes[0] = node
		cf, err := e.pushFrame(node, 0)
		if err != nil {
			return nil, err
		}
		e.currentFrame = cf
		if err := e.currentFrame.loadBlock(); err != nil {
			return nil, err
		}
	}

	e.targetBeforeCurrentLength = e.currentFrame.ord

	if e.currentFrame == e.staticFrame {
		// A prior seek cached a term but did not load a frame; catch up.
		ok, err := e.SeekExact(index.NewTermFromBytesRef(e.fr.fieldInfo.Name(), e.term.Get()))
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, errors.New("Lucene103SegmentTermsEnum.Next: re-seek to pending term failed")
		}
	}

	// Pop finished blocks.
	for e.currentFrame.nextEnt == e.currentFrame.entCount {
		if !e.currentFrame.isLastInFloor {
			if err := e.currentFrame.loadNextFloorBlock(); err != nil {
				return nil, err
			}
			break
		}
		if e.currentFrame.ord == 0 {
			e.eof = true
			e.term.Clear()
			e.validIndexPrefix = 0
			if err := e.currentFrame.rewind(); err != nil {
				return nil, err
			}
			e.termExists = false
			return nil, nil
		}
		lastFP := e.currentFrame.fpOrig
		e.currentFrame = e.stack[e.currentFrame.ord-1]
		if e.currentFrame.nextEnt == -1 || e.currentFrame.lastSubFP != lastFP {
			if err := e.currentFrame.scanToFloorFrame(e.term.Get()); err != nil {
				return nil, err
			}
			if err := e.currentFrame.loadBlock(); err != nil {
				return nil, err
			}
			if err := e.currentFrame.scanToSubBlock(lastFP); err != nil {
				return nil, err
			}
		}
		if e.currentFrame.prefixLength < e.validIndexPrefix {
			e.validIndexPrefix = e.currentFrame.prefixLength
		}
	}

	for {
		isSub, err := e.currentFrame.next()
		if err != nil {
			return nil, err
		}
		if isSub {
			cf, err := e.pushFrameFP(nil, e.currentFrame.lastSubFP, e.term.Length())
			if err != nil {
				return nil, err
			}
			e.currentFrame = cf
			if err := e.currentFrame.loadBlock(); err != nil {
				return nil, err
			}
		} else {
			return e.Term(), nil
		}
	}
}

// seedPending reports whether a construction-time seek term still needs to be
// honoured before the enum starts iterating. A fresh enum has not yet loaded a
// block (e.in == nil) and still sits on the static frame.
func (e *Lucene103SegmentTermsEnum) seedPending() bool {
	return e.seedStart != nil && e.in == nil && e.currentFrame == e.staticFrame
}

// DocFreq returns the document frequency of the current term.
// Mirrors SegmentTermsEnum.docFreq().
func (e *Lucene103SegmentTermsEnum) DocFreq() (int, error) {
	if e.eof || e.currentFrame == nil {
		return 0, nil
	}
	if err := e.currentFrame.decodeMetaData(); err != nil {
		return 0, err
	}
	return e.currentFrame.state.DocFreq, nil
}

// TotalTermFreq returns the total term frequency of the current term.
// Mirrors SegmentTermsEnum.totalTermFreq().
func (e *Lucene103SegmentTermsEnum) TotalTermFreq() (int64, error) {
	if e.eof || e.currentFrame == nil {
		return 0, nil
	}
	if err := e.currentFrame.decodeMetaData(); err != nil {
		return 0, err
	}
	return e.currentFrame.state.TotalTermFreq, nil
}

// Postings decodes the current term's metadata and returns a PostingsEnum.
// Mirrors SegmentTermsEnum.postings(PostingsEnum, int).
func (e *Lucene103SegmentTermsEnum) Postings(flags int) (index.PostingsEnum, error) {
	if e.eof || e.currentFrame == nil {
		return &index.EmptyPostingsEnum{}, nil
	}
	if err := e.currentFrame.decodeMetaData(); err != nil {
		return nil, fmt.Errorf("Lucene103SegmentTermsEnum.Postings: decodeMetaData: %w", err)
	}
	pe, err := e.fr.parent.postingsReader.Postings(e.fr.fieldInfo, e.currentFrame.state, nil, flags)
	if err != nil {
		return nil, fmt.Errorf("Lucene103SegmentTermsEnum.Postings: postingsReader.Postings: %w", err)
	}
	return pe, nil
}

// PostingsWithLiveDocs forwards to Postings; live-docs filtering is applied by
// callers (search.TermWeight / IndexWriter delete resolution) at a higher
// layer, matching how Lucene threads liveDocs through the leaf reader.
func (e *Lucene103SegmentTermsEnum) PostingsWithLiveDocs(_ util.Bits, flags int) (index.PostingsEnum, error) {
	return e.Postings(flags)
}

// TermState would return a snapshot of the current term's decoded state.
// Gocene's [BlockTermState] does not implement the index.TermState interface
// (its CopyFrom signature differs), and no consumer in the codebase performs a
// seek-by-TermState on the block-tree enum, so this returns (nil, nil) — the
// same contract the previous implementation honoured. decodeMetaData still runs
// to keep the cursor's stats current for the immediately following DocFreq /
// Postings call.
//
// Mirrors SegmentTermsEnum.termState() at the interface level only.
func (e *Lucene103SegmentTermsEnum) TermState() (index.TermState, error) {
	if e.eof || e.currentFrame == nil {
		return nil, nil
	}
	if err := e.currentFrame.decodeMetaData(); err != nil {
		return nil, err
	}
	return nil, nil
}

// Ord is not supported by the block-tree codec (Java throws UOE).
func (e *Lucene103SegmentTermsEnum) Ord() (int64, error) {
	return -1, errSegmentTermsEnumOrdUnsupported
}

var errSegmentTermsEnumOrdUnsupported = errors.New(
	"Lucene103SegmentTermsEnum: ord is not supported by the block-tree codec")

// ErrSegmentTermsEnumOrdUnsupported is the exported sentinel for errors.Is.
var ErrSegmentTermsEnumOrdUnsupported = errSegmentTermsEnumOrdUnsupported

// compareBytes compares a[aFrom:aTo] with b[bFrom:bTo] using unsigned byte
// ordering, mirroring java.util.Arrays.compareUnsigned. bytes.Compare yields the
// identical ordering.
func compareBytes(a []byte, aFrom, aTo int, b []byte, bFrom, bTo int) int {
	return bytes.Compare(a[aFrom:aTo], b[bFrom:bTo])
}

// Compile-time interface check.
var _ index.TermsEnum = (*Lucene103SegmentTermsEnum)(nil)
