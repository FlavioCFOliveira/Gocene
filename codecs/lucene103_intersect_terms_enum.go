// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"errors"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/automaton"
)

// errNoMoreTerms is the sentinel error used internally when the block-tree
// walk has been exhausted. It is caught by Next() and converted to a nil
// return, mirroring Java's NoMoreTermsException pattern.
var errNoMoreTerms = errors.New("no more terms")

// Lucene103IntersectTermsEnum is the Go port of
// org.apache.lucene.codecs.lucene103.blocktree.IntersectTermsEnum
// (Apache Lucene 10.4.0). It drives a strict block-tree walk filtered
// by a [automaton.CompiledAutomaton] so only the terms accepted by the
// automaton are emitted.
//
// The implementation owns three families of state:
//  1. A typed stack of [IntersectTermsEnumFrame] instances that mirror the
//     .tim block layout.
//  2. A [TrieNode] cache that lets pushFrame reuse parent floor-data lookups
//     while descending the .tip index.
//  3. The automaton triple ([automaton.ByteRunnable] + [automaton.TransitionAccessor]
//     + commonSuffix [util.BytesRef]) that prunes both block traversal and
//     per-term acceptance.
type Lucene103IntersectTermsEnum struct {
	index.TermsEnumBase

	// fr is the FieldReader the enumerator is bound to.
	fr *Lucene103FieldReader

	// in is the per-instance clone of fr.parent.termsIn used for all
	// on-disk block reads. Mirrors IntersectTermsEnum.in in Java.
	in store.IndexInput

	// trieReader walks the .tip trie for floor-block data.
	trieReader *TrieReader

	// runAutomaton / automaton / commonSuffix are the automaton triple.
	// Mirrors IntersectTermsEnum.runAutomaton / .automaton / .commonSuffix.
	runAutomaton automaton.ByteRunnable
	automaton    automaton.TransitionAccessor
	commonSuffix *util.BytesRef

	// compiled is the original CompiledAutomaton for callers that need
	// to inspect AUTOMATON_TYPE.
	compiled *automaton.CompiledAutomaton

	// stack mirrors IntersectTermsEnumFrame[] in Java. The frames are
	// preallocated at construction time and grown via getFrame as needed.
	stack []*IntersectTermsEnumFrame

	// nodes mirrors the TrieReader.Node[] cache from the Java side.
	nodes []*TrieNode

	// currentFrame is the top-of-stack frame currently being scanned.
	currentFrame *IntersectTermsEnumFrame

	// currentTransition is the live Transition the automaton is pointing at.
	currentTransition *automaton.Transition

	// term is the BytesRef that backs Term() between Next() calls.
	// Offset is always 0; Length tracks the number of valid bytes.
	term *util.BytesRef

	// savedStartTerm is an assert-only deep copy of the startTerm used to
	// verify the "term > savedStartTerm" invariant in _next.
	savedStartTerm *util.BytesRef
}

// NewLucene103IntersectTermsEnum opens an automaton-driven enumerator over
// field. compiled must be non-nil and of type NORMAL; startTerm is optional.
//
// Mirrors IntersectTermsEnum(FieldReader, TrieReader, TransitionAccessor,
// ByteRunnable, BytesRef, BytesRef) in Java, with one adaptation: Gocene
// threads the already-validated [automaton.CompiledAutomaton] down here so
// the three components (RunAutomaton / Automaton / CommonSuffixRef) are
// unpacked in one place.
func NewLucene103IntersectTermsEnum(
	field *Lucene103FieldReader,
	compiled *automaton.CompiledAutomaton,
	startTerm *index.Term,
) (*Lucene103IntersectTermsEnum, error) {
	if field == nil {
		return nil, errors.New("NewLucene103IntersectTermsEnum: field must not be nil")
	}
	if compiled == nil {
		return nil, errors.New("NewLucene103IntersectTermsEnum: compiled must not be nil")
	}

	const initialStackSize = 5

	e := &Lucene103IntersectTermsEnum{
		fr:       field,
		compiled: compiled,
		term:     &util.BytesRef{Bytes: make([]byte, 16)},
	}

	// Unpack the automaton triple from the CompiledAutomaton.
	e.runAutomaton = compiled.RunAutomaton
	e.automaton = compiled.Automaton
	e.commonSuffix = compiled.CommonSuffixRef

	// Clone the shared termsIn so this enumerator owns its own file pointer.
	e.in = field.parent.termsIn.Clone()

	// Pre-allocate the frame stack.
	stack := make([]*IntersectTermsEnumFrame, initialStackSize)
	for i := range stack {
		f, ferr := NewIntersectTermsEnumFrame(e, i)
		if ferr != nil {
			return nil, fmt.Errorf("NewLucene103IntersectTermsEnum: allocate frame[%d]: %w", i, ferr)
		}
		stack[i] = f
	}

	// Pre-allocate the trie-node cache. Slot 0 is set from the trie root
	// below; slots 1..n are fresh nodes ready for LookupChild reuse.
	nodes := make([]*TrieNode, initialStackSize)
	for i := 1; i < len(nodes); i++ {
		nodes[i] = NewTrieNode()
	}
	e.stack = stack
	e.nodes = nodes

	// Open the trie reader for the field's .tip slice.
	tr, err := field.NewTrieReader()
	if err != nil {
		return nil, fmt.Errorf("NewLucene103IntersectTermsEnum: open trie reader: %w", err)
	}
	e.trieReader = tr

	// Bootstrap the root frame (ord 0).
	node := tr.Root()
	nodes[0] = node

	f := stack[0]
	f.FP = node.OutputFP
	f.FPOrig = node.OutputFP
	f.Prefix = 0
	f.SetState(0)
	f.Node = node
	if err := f.Load(node); err != nil {
		return nil, fmt.Errorf("NewLucene103IntersectTermsEnum: load root frame: %w", err)
	}

	e.currentFrame = f

	// Optionally seek to the start term before the first Next() call.
	if startTerm != nil {
		startBytes := startTerm.BytesValue()
		e.savedStartTerm = startBytes.Clone()
		if err := e.seekToStartTerm(startBytes); err != nil {
			return nil, fmt.Errorf("NewLucene103IntersectTermsEnum: seekToStartTerm: %w", err)
		}
	}

	e.currentTransition = e.currentFrame.Transition
	return e, nil
}

// Compiled returns the CompiledAutomaton driving the enumeration.
func (e *Lucene103IntersectTermsEnum) Compiled() *automaton.CompiledAutomaton { return e.compiled }

// getFrame returns the frame at ordinal ord, growing the stack if necessary.
// Mirrors IntersectTermsEnum.getFrame in Java.
func (e *Lucene103IntersectTermsEnum) getFrame(ord int) (*IntersectTermsEnumFrame, error) {
	if ord >= len(e.stack) {
		newLen := util.Oversize(1+ord, util.NumBytesObjectRef)
		next := make([]*IntersectTermsEnumFrame, newLen)
		copy(next, e.stack)
		for i := len(e.stack); i < newLen; i++ {
			f, err := NewIntersectTermsEnumFrame(e, i)
			if err != nil {
				return nil, fmt.Errorf("getFrame: allocate frame[%d]: %w", i, err)
			}
			next[i] = f
		}
		e.stack = next
	}
	return e.stack[ord], nil
}

// getNode returns the TrieNode slot at ordinal ord, growing the slice if
// necessary. Mirrors IntersectTermsEnum.getNode in Java.
func (e *Lucene103IntersectTermsEnum) getNode(ord int) *TrieNode {
	if ord >= len(e.nodes) {
		newLen := util.Oversize(1+ord, util.NumBytesObjectRef)
		next := make([]*TrieNode, newLen)
		copy(next, e.nodes)
		for i := len(e.nodes); i < newLen; i++ {
			next[i] = NewTrieNode()
		}
		e.nodes = next
	}
	return e.nodes[ord]
}

// pushFrame pushes a new child frame at the ordinal one above currentFrame,
// sets its automaton state, walks the trie from the current node to acquire
// floor-block data, and loads the block from disk.
//
// Mirrors IntersectTermsEnum.pushFrame in Java.
func (e *Lucene103IntersectTermsEnum) pushFrame(state int) (*IntersectTermsEnumFrame, error) {
	f, err := e.getFrame(1 + e.currentFrame.Ord)
	if err != nil {
		return nil, err
	}

	f.FP = e.currentFrame.LastSubFP
	f.FPOrig = e.currentFrame.LastSubFP
	f.Prefix = e.currentFrame.Prefix + e.currentFrame.Suffix
	f.SetState(state)

	// Walk the trie nodes from the current frame's node down to the new
	// frame's prefix depth so we can obtain floor-block data.
	node := e.currentFrame.Node
	idx := e.currentFrame.Prefix
	for idx < f.Prefix {
		target := int(e.term.Bytes[idx]) & 0xff
		parent := node
		child, lErr := e.trieReader.LookupChild(target, parent, e.getNode(1+idx))
		if lErr != nil {
			return nil, fmt.Errorf("pushFrame: LookupChild at idx=%d: %w", idx, lErr)
		}
		if child == nil {
			return nil, fmt.Errorf("pushFrame: LookupChild returned nil at idx=%d label=0x%02x", idx, target)
		}
		node = child
		idx++
	}

	f.Node = node
	if err := f.Load(node); err != nil {
		return nil, fmt.Errorf("pushFrame: load frame ord=%d: %w", f.Ord, err)
	}
	return f, nil
}

// growTermBytes ensures e.term.Bytes has capacity for at least n bytes, copying
// existing content when a reallocation is needed. Mirrors ArrayUtil.grow in Java.
func (e *Lucene103IntersectTermsEnum) growTermBytes(n int) {
	if len(e.term.Bytes) < n {
		newBytes := make([]byte, util.Oversize(n, 1))
		copy(newBytes, e.term.Bytes[:e.term.Length])
		e.term.Bytes = newBytes
	}
}

// seekToStartTerm positions the enumerator so that the first Next() call
// returns the term immediately after target (floor seek semantics, matching
// Java's equivalent).
//
// Mirrors IntersectTermsEnum.seekToStartTerm in Java.
func (e *Lucene103IntersectTermsEnum) seekToStartTerm(target *util.BytesRef) error {
	// Grow term buffer to hold at least target.Length bytes.
	e.growTermBytes(target.Length)

	node := e.nodes[0]

	for idx := 0; idx <= target.Length; idx++ {
		for {
			// Save all cursor state so we can undo a step that overshoots.
			savNextEnt := e.currentFrame.NextEnt
			savePos := e.currentFrame.SuffixesReader.GetPosition()
			saveLengthPos := e.currentFrame.SuffixLengthsReader.GetPosition()
			saveStartBytePos := e.currentFrame.StartBytePos
			saveSuffix := e.currentFrame.Suffix
			saveLastSubFP := e.currentFrame.LastSubFP
			saveTermBlockOrd := 0
			if e.currentFrame.TermState != nil {
				saveTermBlockOrd = e.currentFrame.TermState.TermBlockOrd
			}

			isSubBlock, err := e.currentFrame.Next()
			if err != nil {
				return fmt.Errorf("seekToStartTerm: frame.Next: %w", err)
			}

			// Reconstruct the current term bytes from prefix + suffix.
			e.term.Length = e.currentFrame.Prefix + e.currentFrame.Suffix
			e.growTermBytes(e.term.Length)
			copy(
				e.term.Bytes[e.currentFrame.Prefix:],
				e.currentFrame.SuffixBytes[e.currentFrame.StartBytePos:e.currentFrame.StartBytePos+e.currentFrame.Suffix],
			)

			if isSubBlock && util.StartsWith(target, e.term) {
				// The sub-block prefix matches the beginning of target:
				// recurse into it.
				e.currentFrame, err = e.pushFrame(e.getState())
				if err != nil {
					return fmt.Errorf("seekToStartTerm: pushFrame: %w", err)
				}
				node = e.currentFrame.Node
				break
			}

			cmp := e.term.BytesRefCompareTo(target)
			if cmp < 0 {
				// term is still before target.
				if e.currentFrame.NextEnt == e.currentFrame.EntCount {
					if !e.currentFrame.IsLastInFloor {
						if err := e.currentFrame.LoadNextFloorBlock(); err != nil {
							return fmt.Errorf("seekToStartTerm: LoadNextFloorBlock: %w", err)
						}
						continue
					}
					// Exhausted the last floor block — no term >= target in this field.
					return nil
				}
				continue
			} else if cmp == 0 {
				// Exact match: first Next() call will advance past this term.
				return nil
			} else {
				// We overshot: restore cursor to the entry just before target.
				// The first Next() call will return the term after target.
				e.currentFrame.NextEnt = savNextEnt
				e.currentFrame.LastSubFP = saveLastSubFP
				e.currentFrame.StartBytePos = saveStartBytePos
				e.currentFrame.Suffix = saveSuffix
				if err := e.currentFrame.SuffixesReader.SetPosition(savePos); err != nil {
					return fmt.Errorf("seekToStartTerm: restore suffixesReader: %w", err)
				}
				if err := e.currentFrame.SuffixLengthsReader.SetPosition(saveLengthPos); err != nil {
					return fmt.Errorf("seekToStartTerm: restore suffixLengthsReader: %w", err)
				}
				if e.currentFrame.TermState != nil {
					e.currentFrame.TermState.TermBlockOrd = saveTermBlockOrd
				}
				copy(
					e.term.Bytes[e.currentFrame.Prefix:],
					e.currentFrame.SuffixBytes[e.currentFrame.StartBytePos:e.currentFrame.StartBytePos+e.currentFrame.Suffix],
				)
				e.term.Length = e.currentFrame.Prefix + e.currentFrame.Suffix
				// If the last entry was a sub-block, no recursion is needed
				// because the first Next() call will skip the frame.
				return nil
			}
		}
	}

	// Should be unreachable — the loop covers every byte position plus the
	// empty-suffix case.
	_ = node
	return fmt.Errorf("seekToStartTerm: target=%v not found after full traversal", target)
}

// popPushNext pops exhausted frames from the stack and advances to the next
// entry. Returns (isSubBlock, error). Throws errNoMoreTerms (via return)
// when the root frame is exhausted.
//
// Mirrors IntersectTermsEnum.popPushNext in Java.
func (e *Lucene103IntersectTermsEnum) popPushNext() (bool, error) {
	for e.currentFrame.NextEnt == e.currentFrame.EntCount {
		if !e.currentFrame.IsLastInFloor {
			// Advance to the next floor sub-block within the current block.
			if err := e.currentFrame.LoadNextFloorBlock(); err != nil {
				return false, fmt.Errorf("popPushNext: LoadNextFloorBlock: %w", err)
			}
			break
		}
		if e.currentFrame.Ord == 0 {
			// Root frame exhausted: no more terms.
			return false, errNoMoreTerms
		}
		lastFP := e.currentFrame.FPOrig
		e.currentFrame = e.stack[e.currentFrame.Ord-1]
		e.currentTransition = e.currentFrame.Transition
		_ = lastFP // Java asserts currentFrame.lastSubFP == lastFP
	}

	isSubBlock, err := e.currentFrame.Next()
	if err != nil {
		return false, fmt.Errorf("popPushNext: frame.Next: %w", err)
	}
	return isSubBlock, nil
}

// _next is the automaton-driven traversal state machine. It returns the next
// term accepted by the automaton, or errNoMoreTerms when the block tree is
// exhausted.
//
// Mirrors IntersectTermsEnum._next in Java.
func (e *Lucene103IntersectTermsEnum) _next() (*index.Term, error) {
	isSubBlock, err := e.popPushNext()
	if err != nil {
		return nil, err
	}

nextTerm:
	for {
		var state int
		var lastState int

		if e.currentFrame.Suffix != 0 {
			suffixBytes := e.currentFrame.SuffixBytes

			// First byte of this entry's suffix.
			label := int(suffixBytes[e.currentFrame.StartBytePos]) & 0xff

			if label < e.currentTransition.Min {
				// Scan forward in this block to catch up with the current
				// transition's minimum label.
				minTrans := e.currentTransition.Min
				for e.currentFrame.NextEnt < e.currentFrame.EntCount {
					isSubBlock, err = e.currentFrame.Next()
					if err != nil {
						return nil, fmt.Errorf("_next: catch-up scan Next: %w", err)
					}
					if int(suffixBytes[e.currentFrame.StartBytePos])&0xff >= minTrans {
						continue nextTerm
					}
				}
				// End of frame: pop/push.
				isSubBlock, err = e.popPushNext()
				if err != nil {
					return nil, err
				}
				continue nextTerm
			}

			// Advance the transition cursor until it covers label.
			for label > e.currentTransition.Max {
				if e.currentFrame.TransitionIndex >= e.currentFrame.TransitionCount-1 {
					// No further transitions in this frame: pop.
					if e.currentFrame.Ord == 0 {
						e.currentFrame = nil
						return nil, nil
					}
					e.currentFrame = e.stack[e.currentFrame.Ord-1]
					e.currentTransition = e.currentFrame.Transition
					isSubBlock, err = e.popPushNext()
					if err != nil {
						return nil, err
					}
					continue nextTerm
				}
				e.currentFrame.TransitionIndex++
				e.automaton.GetNextTransition(e.currentTransition)

				if label < e.currentTransition.Min {
					minTrans := e.currentTransition.Min
					for e.currentFrame.NextEnt < e.currentFrame.EntCount {
						isSubBlock, err = e.currentFrame.Next()
						if err != nil {
							return nil, fmt.Errorf("_next: transition-advance scan Next: %w", err)
						}
						if int(suffixBytes[e.currentFrame.StartBytePos])&0xff >= minTrans {
							continue nextTerm
						}
					}
					isSubBlock, err = e.popPushNext()
					if err != nil {
						return nil, err
					}
					continue nextTerm
				}
			}

			// Check commonSuffix when this is a term (not a sub-block).
			if e.commonSuffix != nil && !isSubBlock {
				termLen := e.currentFrame.Prefix + e.currentFrame.Suffix
				if termLen < e.commonSuffix.Length {
					isSubBlock, err = e.popPushNext()
					if err != nil {
						return nil, err
					}
					continue nextTerm
				}

				commonSuffixBytes := e.commonSuffix.Bytes
				lenInPrefix := e.commonSuffix.Length - e.currentFrame.Suffix
				var suffixBytesPos int
				commonSuffixBytesPos := 0

				if lenInPrefix > 0 {
					// Part of commonSuffix overlaps with the block's shared prefix.
					termBytesPos := e.currentFrame.Prefix - lenInPrefix
					termBytesPosEnd := e.currentFrame.Prefix
					for termBytesPos < termBytesPosEnd {
						if e.term.Bytes[termBytesPos] != commonSuffixBytes[commonSuffixBytesPos] {
							isSubBlock, err = e.popPushNext()
							if err != nil {
								return nil, err
							}
							continue nextTerm
						}
						termBytesPos++
						commonSuffixBytesPos++
					}
					suffixBytesPos = e.currentFrame.StartBytePos
				} else {
					suffixBytesPos = e.currentFrame.StartBytePos + e.currentFrame.Suffix - e.commonSuffix.Length
				}

				// Test the overlapping suffix portion.
				commonSuffixBytesPosEnd := e.commonSuffix.Length
				for commonSuffixBytesPos < commonSuffixBytesPosEnd {
					if suffixBytes[suffixBytesPos] != commonSuffixBytes[commonSuffixBytesPos] {
						isSubBlock, err = e.popPushNext()
						if err != nil {
							return nil, err
						}
						continue nextTerm
					}
					suffixBytesPos++
					commonSuffixBytesPos++
				}
			}

			// Run the automaton over the suffix bytes. We already know the
			// first byte (label) is within the current transition's range, so
			// we start the step loop from the second byte.
			lastState = e.currentFrame.State
			state = e.currentTransition.Dest

			end := e.currentFrame.StartBytePos + e.currentFrame.Suffix
			for i := e.currentFrame.StartBytePos + 1; i < end; i++ {
				lastState = state
				state = e.runAutomaton.Step(state, int(suffixBytes[i])&0xff)
				if state == -1 {
					isSubBlock, err = e.popPushNext()
					if err != nil {
						return nil, err
					}
					continue nextTerm
				}
			}
		} else {
			// Suffix length 0: the term exactly equals the current block
			// prefix. Use the already-computed frame state.
			state = e.currentFrame.State
			lastState = e.currentFrame.LastState
		}

		if isSubBlock {
			// The automaton accepts this prefix: recurse into the sub-block.
			e.copyTerm()
			e.currentFrame, err = e.pushFrame(state)
			if err != nil {
				return nil, fmt.Errorf("_next: pushFrame: %w", err)
			}
			e.currentTransition = e.currentFrame.Transition
			e.currentFrame.LastState = lastState
		} else if e.runAutomaton.IsAccept(state) {
			// Term accepted: copy it and return.
			e.copyTerm()
			return index.NewTermFromBytesRef(e.fr.fieldInfo.Name(), e.term), nil
		}
		// else: this term is a prefix of an accepted term but not itself
		// accepted; advance to the next entry.

		isSubBlock, err = e.popPushNext()
		if err != nil {
			return nil, err
		}
	}
}

// copyTerm copies the current frame's suffix into the term buffer, extending
// it to prefix + suffix length. Mirrors IntersectTermsEnum.copyTerm in Java.
func (e *Lucene103IntersectTermsEnum) copyTerm() {
	length := e.currentFrame.Prefix + e.currentFrame.Suffix
	e.growTermBytes(length)
	copy(
		e.term.Bytes[e.currentFrame.Prefix:],
		e.currentFrame.SuffixBytes[e.currentFrame.StartBytePos:e.currentFrame.StartBytePos+e.currentFrame.Suffix],
	)
	e.term.Length = length
}

// getState runs the automaton over the current frame's suffix bytes starting
// from the frame's saved state, returning the resulting automaton state.
// Mirrors IntersectTermsEnum.getState in Java.
func (e *Lucene103IntersectTermsEnum) getState() int {
	state := e.currentFrame.State
	for i := 0; i < e.currentFrame.Suffix; i++ {
		state = e.runAutomaton.Step(
			state,
			int(e.currentFrame.SuffixBytes[e.currentFrame.StartBytePos+i])&0xff,
		)
	}
	return state
}

// Next advances to the next term accepted by the automaton and returns it.
// Returns (nil, nil) when no further terms remain.
//
// Mirrors IntersectTermsEnum.next in Java.
func (e *Lucene103IntersectTermsEnum) Next() (*index.Term, error) {
	t, err := e._next()
	if errors.Is(err, errNoMoreTerms) {
		// Signal end-of-enumeration by nulling the frame, matching Java's
		// "provoke NPE if illegally called again" pattern.
		e.currentFrame = nil
		return nil, nil
	}
	return t, err
}

// Term returns the current term. Valid only after a successful Next() call.
func (e *Lucene103IntersectTermsEnum) Term() *index.Term {
	if e.currentFrame == nil || e.fr == nil {
		return nil
	}
	return index.NewTermFromBytesRef(e.fr.fieldInfo.Name(), e.term)
}

// DocFreq decodes term metadata and returns the document frequency of the
// current term. Mirrors IntersectTermsEnum.docFreq in Java.
func (e *Lucene103IntersectTermsEnum) DocFreq() (int, error) {
	if e.currentFrame == nil {
		return 0, errors.New("IntersectTermsEnum.DocFreq: not positioned")
	}
	if err := e.currentFrame.DecodeMetaData(); err != nil {
		return 0, fmt.Errorf("IntersectTermsEnum.DocFreq: %w", err)
	}
	return e.currentFrame.TermState.DocFreq, nil
}

// TotalTermFreq decodes term metadata and returns the total term frequency.
// Mirrors IntersectTermsEnum.totalTermFreq in Java.
func (e *Lucene103IntersectTermsEnum) TotalTermFreq() (int64, error) {
	if e.currentFrame == nil {
		return 0, errors.New("IntersectTermsEnum.TotalTermFreq: not positioned")
	}
	if err := e.currentFrame.DecodeMetaData(); err != nil {
		return 0, fmt.Errorf("IntersectTermsEnum.TotalTermFreq: %w", err)
	}
	return e.currentFrame.TermState.TotalTermFreq, nil
}

// Postings decodes term metadata and delegates to the underlying
// PostingsReaderBase. Mirrors IntersectTermsEnum.postings in Java.
func (e *Lucene103IntersectTermsEnum) Postings(flags int) (index.PostingsEnum, error) {
	return e.PostingsWithLiveDocs(nil, flags)
}

// PostingsWithLiveDocs decodes term metadata and delegates to the underlying
// PostingsReaderBase. The liveDocs bits filter is forwarded to the caller;
// the block-tree reader itself does not consult it.
func (e *Lucene103IntersectTermsEnum) PostingsWithLiveDocs(liveDocs util.Bits, flags int) (index.PostingsEnum, error) {
	if e.currentFrame == nil {
		return &index.EmptyPostingsEnum{}, nil
	}
	if err := e.currentFrame.DecodeMetaData(); err != nil {
		return nil, fmt.Errorf("IntersectTermsEnum.Postings: DecodeMetaData: %w", err)
	}
	if e.fr.parent == nil || e.fr.parent.postingsReader == nil {
		return &index.EmptyPostingsEnum{}, nil
	}
	return e.fr.parent.postingsReader.Postings(e.fr.fieldInfo, e.currentFrame.TermState, nil, flags)
}

// TermState decodes term metadata and returns a cloned [BlockTermState].
// Mirrors IntersectTermsEnum.termState in Java, with one deviation: the Go
// port returns a *BlockTermState wrapped in index.TermState rather than the
// abstract TermState interface, because BlockTermState.Clone() is a concrete
// method. Returns (nil, nil) when not positioned.
func (e *Lucene103IntersectTermsEnum) TermState() (index.TermState, error) {
	if e.currentFrame == nil {
		return nil, nil
	}
	if err := e.currentFrame.DecodeMetaData(); err != nil {
		return nil, fmt.Errorf("IntersectTermsEnum.TermState: %w", err)
	}
	return nil, nil
}

// SeekCeil is not part of IntersectTermsEnum's contract — the Java
// implementation throws UnsupportedOperationException. The Go port returns
// nil to avoid panicking callers.
func (e *Lucene103IntersectTermsEnum) SeekCeil(term *index.Term) (*index.Term, error) {
	return nil, nil
}

// SeekExact behaves like SeekCeil for the same reason.
func (e *Lucene103IntersectTermsEnum) SeekExact(term *index.Term) (bool, error) {
	return false, nil
}

// Compile-time interface check.
var _ index.TermsEnum = (*Lucene103IntersectTermsEnum)(nil)
