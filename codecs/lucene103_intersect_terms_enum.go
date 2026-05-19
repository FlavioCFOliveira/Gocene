// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/automaton"
)

// Lucene103IntersectTermsEnum is the Go port of
// org.apache.lucene.codecs.lucene103.blocktree.IntersectTermsEnum
// (Apache Lucene 10.4.0). It drives a strict block-tree walk filtered
// by a [automaton.CompiledAutomaton] so only the terms accepted by the
// automaton are emitted.
//
// The Java implementation owns three families of state: (1) a typed
// stack of IntersectTermsEnumFrame instances that mirror the .tim
// block layout; (2) a TrieReader.Node[] cache that lets pushFrame
// reuse parent floor-data lookups while descending the .tip index; and
// (3) the automaton triple (ByteRunnable + TransitionAccessor +
// commonSuffix BytesRef) that prunes both block traversal and per-term
// acceptance. This struct declares the equivalent typed fields and the
// public surface required by [index.TermsEnum]; the behavioural port
// (the pushFrame / popPushNext / _next state machine, decodeMetaData
// and the postings/impacts wiring) is the deferred deep-port follow-up
// tracked by backlog task #2692.
//
// Sprint 56 deliberately ships a degraded stub: the constructor
// accepts the typed inputs and stores them, every advance returns
// end-of-enumeration, and every metadata accessor returns the zero
// value. This is sufficient to keep [Lucene103FieldReader.Intersect]
// compiling and to let downstream callers receive an [index.TermsEnum]
// rather than nil — the surface that the deep port must preserve.
type Lucene103IntersectTermsEnum struct {
	index.TermsEnumBase

	// fr is the FieldReader the enumerator is bound to. The Java
	// implementation reaches through fr.parent.termsIn for the .tim
	// IndexInput and fr.parent.postingsReader for postings/impacts.
	fr *Lucene103FieldReader

	// in is the per-instance clone of fr.parent.termsIn that the
	// behavioural port will hold. Sprint 56 stores nil because the
	// stub never reads from disk; the field is declared so the
	// deferred port does not need to re-shape the struct. Mirrors
	// IntersectTermsEnum.in in Java.
	in store.IndexInput

	// trieReader is the .tip-side trie that the behavioural port
	// walks via pushFrame to track floor-block boundaries. Sprint 56
	// leaves it nil — the production wiring lands with backlog #2692.
	trieReader *TrieReader

	// runAutomaton / automaton / commonSuffix are the automaton
	// triple Java's IntersectTermsEnum keeps as final fields. The
	// constructor unpacks them from the supplied CompiledAutomaton
	// when present; otherwise they remain nil and the deferred port
	// must guard against that. Mirrors IntersectTermsEnum.runAutomaton,
	// .automaton and .commonSuffix.
	runAutomaton automaton.ByteRunnable
	automaton    automaton.TransitionAccessor
	commonSuffix *util.BytesRef

	// compiled is the original CompiledAutomaton handed to the
	// constructor. It is retained for inspection by callers that need
	// the AUTOMATON_TYPE (e.g. ALL, NONE, SINGLE, NORMAL) without
	// reaching into the unpacked triple above.
	compiled *automaton.CompiledAutomaton

	// startTerm is the optional seek-floor target. The behavioural
	// port uses it to drive seekToStartTerm; the stub stores it for
	// later inspection. Mirrors the constructor's startTerm
	// parameter.
	startTerm *index.Term

	// stack mirrors IntersectTermsEnumFrame[] in Java. The frames are
	// preallocated and reused; the Java implementation grows the stack
	// via ArrayUtil.oversize as the trie is descended deeper than the
	// initial five-slot reservation. Sprint 56 reserves the same five
	// slots so the deferred port does not need to special-case the
	// first push.
	stack []*intersectTermsEnumFrame

	// nodes mirrors the TrieReader.Node[] cache from the Java side.
	// Slot zero is set from trieReader.root at construction time;
	// remaining slots are populated lazily by pushFrame. Sprint 56
	// preallocates the slice so the deferred port matches Java's
	// preallocation strategy exactly.
	nodes []*TrieNode

	// currentFrame is the top-of-stack frame the behavioural port is
	// scanning. Mirrors IntersectTermsEnum.currentFrame.
	currentFrame *intersectTermsEnumFrame

	// currentTransition is the live Transition the automaton is
	// pointing at; pushFrame copies it from currentFrame.transition
	// after each frame change. Mirrors IntersectTermsEnum.currentTransition.
	currentTransition *automaton.Transition

	// term is the BytesRef that backs Term() between Next() calls.
	// Sprint 56 keeps it empty because the stub never positions a
	// real term; the deferred port grows it via ArrayUtil-style
	// reslices. Mirrors IntersectTermsEnum.term.
	term *util.BytesRef

	// savedStartTerm is the assert-only deep-copy of startTerm that
	// Java's setSavedStartTerm produces. Retained on the Go side so
	// the deferred port can keep the same invariant ("term >
	// savedStartTerm") behind a build tag or test-only check.
	savedStartTerm *util.BytesRef
}

// intersectTermsEnumFrame is the Go counterpart of
// org.apache.lucene.codecs.lucene103.blocktree.IntersectTermsEnumFrame.
// The full field set (suffixBytes, suffixesReader, transitionCount,
// floorSuffix*, isLastInFloor, lastSubFP, fpEnd, …) lands with the
// deferred deep-port follow-up — see backlog task #2692. Sprint 56
// declares only the slots the surface exposes (ord, prefix, state,
// node, transition) so the typed stack layout matches the Java
// reference and the deferred port can grow the struct in place
// instead of rewriting the cursor type.
//
// IMPORTANT: do not add fields, methods, or constructors here without
// first updating backlog task #2692 — the surface here is the contract
// the deep port must preserve.
type intersectTermsEnumFrame struct {
	// ord is the frame's position in the stack (0 == root). Mirrors
	// IntersectTermsEnumFrame.ord.
	ord int

	// fp / fpOrig are the file pointers of the current floor sub-block
	// and the parent floor block, respectively. The behavioural port
	// uses (fp == fpOrig) to spot the parent's first emission.
	fp     int64
	fpOrig int64

	// prefix is the depth in the trie (number of bytes shared by every
	// term in the block). Mirrors IntersectTermsEnumFrame.prefix.
	prefix int

	// state is the automaton state at the entry of the frame; pushFrame
	// computes it via getState() from the parent's suffix scan. Mirrors
	// IntersectTermsEnumFrame.state.
	state int

	// lastState is the automaton state recorded just before the last
	// suffix byte was consumed; the parent uses it when popping the
	// child frame to restore the right resume point. Mirrors
	// IntersectTermsEnumFrame.lastState.
	lastState int

	// node is the cached TrieReader.Node that this frame anchors at.
	// Sprint 56 leaves it nil because the trie traversal has not
	// landed yet.
	node *TrieNode

	// transition is the per-frame Transition object the automaton
	// walks within this block. Java preallocates one per frame so the
	// inner loop avoids re-allocation; the Go port mirrors that.
	transition *automaton.Transition

	// transitionIndex / transitionCount mirror the same names in
	// Java. They are populated by setState() once the deferred port
	// lands; today both fields stay at zero.
	transitionIndex int
	transitionCount int
}

// NewLucene103IntersectTermsEnum opens an automaton-driven enumerator
// over field. The caller is expected to validate compiled before
// reaching the constructor; [Lucene103FieldReader.Intersect] performs
// that validation.
//
// Mirrors IntersectTermsEnum(FieldReader, TrieReader, TransitionAccessor,
// ByteRunnable, BytesRef, BytesRef) in Java, with two adaptations:
//
//  1. The Java constructor takes the TransitionAccessor and the
//     ByteRunnable as separate arguments because Lucene's TermsEnum
//     plumbing assembles them at the FieldReader.intersect call site.
//     Gocene threads the already-validated [automaton.CompiledAutomaton]
//     down here so the two triples (type / transitionAccessor /
//     byteRunnable / commonSuffix) are unpacked in one place. When the
//     deferred port lands, the unpack will move into a helper to match
//     the Java call-site shape.
//
//  2. The TrieReader is not yet wired through Lucene103FieldReader.
//     Sprint 56 stores nil; the deferred port (backlog #2692) plumbs
//     it through once the .tip read path is in.
func NewLucene103IntersectTermsEnum(field *Lucene103FieldReader, compiled *automaton.CompiledAutomaton, startTerm *index.Term) *Lucene103IntersectTermsEnum {
	const initialStackSize = 5
	stack := make([]*intersectTermsEnumFrame, initialStackSize)
	for i := range stack {
		stack[i] = &intersectTermsEnumFrame{
			ord:        i,
			transition: &automaton.Transition{},
		}
	}
	nodes := make([]*TrieNode, initialStackSize)
	for i := 1; i < len(nodes); i++ {
		nodes[i] = NewTrieNode()
	}

	e := &Lucene103IntersectTermsEnum{
		fr:        field,
		compiled:  compiled,
		startTerm: startTerm,
		stack:     stack,
		nodes:     nodes,
		term:      &util.BytesRef{},
	}
	if compiled != nil {
		// TODO(GOC-2692): once CompiledAutomaton exposes accessor
		// methods, unpack the triple here. The current
		// [automaton.CompiledAutomaton] in Gocene does not yet
		// surface the typed RunAutomaton / TransitionAccessor /
		// CommonSuffix fields the deep port needs, so the unpack is
		// deferred to the same backlog task as the traversal port.
	}
	return e
}

// Compiled returns the CompiledAutomaton driving the enumeration.
// Exposed so callers can inspect AUTOMATON_TYPE without rebuilding
// the triple from scratch.
func (e *Lucene103IntersectTermsEnum) Compiled() *automaton.CompiledAutomaton { return e.compiled }

// StartTerm returns the optional seek-floor target supplied at
// construction time. Exposed for tests; the behavioural port consults
// it through seekToStartTerm.
func (e *Lucene103IntersectTermsEnum) StartTerm() *index.Term { return e.startTerm }

// Next advances to the next term accepted by the automaton.
//
// Sprint 56: degraded stub — returns nil. The behavioural port (the
// popPushNext / _next state machine that mirrors IntersectTermsEnum.next
// in Java) is the deferred deep-port follow-up tracked by backlog
// task #2692. Wiring it up requires:
//
//   - IntersectTermsEnumFrame.next(), .loadNextFloorBlock(),
//     .decodeMetaData(), and the suffix/length/stats readers, none of
//     which are ported in Sprint 17's minimal Stats-driven cursor.
//   - The Lucene103PostingsReader.Postings / .Impacts call sites
//     (also deferred — Sprint 18 only ships the compressing helpers).
//   - The TrieReader.LookupChild fast-path used by pushFrame.
//
// Returning nil here mirrors the EmptyTermsEnum semantics rather than
// triggering a runtime panic, which keeps downstream callers (e.g.
// query rewriting) safe to invoke on an unfinished port.
func (e *Lucene103IntersectTermsEnum) Next() (*index.Term, error) {
	return nil, nil
}

// SeekCeil is not part of IntersectTermsEnum's normal contract — the
// Java implementation throws UnsupportedOperationException because the
// traversal is automaton-driven. The Go port returns nil to keep
// callers that mistakenly route here (e.g. generic TermsEnum loops)
// from panicking; this matches EmptyTermsEnum.SeekCeil in Gocene.
func (e *Lucene103IntersectTermsEnum) SeekCeil(term *index.Term) (*index.Term, error) {
	return nil, nil
}

// SeekExact behaves like SeekCeil for the same reason.
func (e *Lucene103IntersectTermsEnum) SeekExact(term *index.Term) (bool, error) {
	return false, nil
}

// DocFreq returns 0 because no term is positioned in the stub.
func (e *Lucene103IntersectTermsEnum) DocFreq() (int, error) { return 0, nil }

// TotalTermFreq returns 0 because no term is positioned in the stub.
func (e *Lucene103IntersectTermsEnum) TotalTermFreq() (int64, error) { return 0, nil }

// Postings returns an empty PostingsEnum because the behavioural port
// (and Lucene103PostingsReader.Postings) is deferred — see backlog
// task #2692.
func (e *Lucene103IntersectTermsEnum) Postings(flags int) (index.PostingsEnum, error) {
	return &index.EmptyPostingsEnum{}, nil
}

// PostingsWithLiveDocs forwards to Postings; the stub ignores liveDocs.
func (e *Lucene103IntersectTermsEnum) PostingsWithLiveDocs(liveDocs util.Bits, flags int) (index.PostingsEnum, error) {
	return e.Postings(flags)
}

// Compile-time interface check.
var _ index.TermsEnum = (*Lucene103IntersectTermsEnum)(nil)
