// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"sort"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// MultiTermsEnum exposes a single TermsEnum view merged, by term text, from the
// TermsEnum instances of several sub-segments. It performs a merge sort over the
// sub-readers using a priority queue keyed on the current term of each sub.
// Mirrors org.apache.lucene.index.MultiTermsEnum (Apache Lucene 10.4.0).
//
// A MultiTermsEnum is built over a list of ReaderSlice values (one per
// sub-reader) and then bound to a matching list of sub-TermsEnum via Reset.
// After Reset the enum behaves exactly like any other TermsEnum: Next walks the
// merged term space in byte order, SeekCeil/SeekExact position the merge, and
// DocFreq/TotalTermFreq/Postings aggregate over the subs that share the current
// term.
type MultiTermsEnum struct {
	TermsEnumBase

	slices []ReaderSlice

	// queue orders the sub-enumerators by their current term (least first).
	queue *termMergeQueue

	// subs holds one entry per sub-reader, indexed by sub index. Entries are
	// (re)bound on Reset.
	subs []*termsEnumWithSlice

	// currentSubs holds the subs that have at least one term for this field;
	// the first numSubs entries are valid.
	currentSubs []*termsEnumWithSlice

	// top holds the subs currently positioned on the least (current) term; the
	// first numTop entries are valid.
	top []*termsEnumWithSlice

	// subDocs is the reusable backing array of (postingsEnum, slice) pairs fed
	// to MultiPostingsEnum.Reset.
	subDocs []EnumWithSlice

	numTop  int
	numSubs int

	// current is the term the merged enum is positioned on (nil before the
	// first Next/seek, or after exhaustion).
	current *Term

	// lastSeek / lastSeekExact implement the LUCENE-2130 seek optimization.
	lastSeek      *Term
	lastSeekExact bool
}

// termsEnumWithSlice pairs a sub-TermsEnum with the ReaderSlice describing how
// its local doc-ID space maps into the composite doc-ID space. Mirrors
// MultiTermsEnum.TermsEnumWithSlice in Lucene.
type termsEnumWithSlice struct {
	subSlice ReaderSlice
	terms    TermsEnum
	current  *Term
	subIndex int

	// subDocs is the per-sub reusable (postingsEnum, slice) pair, allowing the
	// underlying sub-PostingsEnum to be recycled across term lookups.
	subDocs EnumWithSlice
}

func (t *termsEnumWithSlice) reset(terms TermsEnum) {
	t.terms = terms
	t.current = terms.Term()
}

// NewMultiTermsEnum builds a MultiTermsEnum over the supplied slice list. The
// sub-enumerators are bound via Reset. The slice order is preserved and used as
// the sub index for tie-breaking and docBase mapping.
func NewMultiTermsEnum(slices []ReaderSlice) *MultiTermsEnum {
	n := len(slices)
	m := &MultiTermsEnum{
		slices:      slices,
		queue:       newTermMergeQueue(n),
		subs:        make([]*termsEnumWithSlice, n),
		currentSubs: make([]*termsEnumWithSlice, n),
		top:         make([]*termsEnumWithSlice, n),
		subDocs:     make([]EnumWithSlice, n),
	}
	for i := 0; i < n; i++ {
		m.subs[i] = &termsEnumWithSlice{subSlice: slices[i], subIndex: i}
	}
	return m
}

// Reset binds the supplied sub-TermsEnum instances (one per slice, in slice
// order) to this MultiTermsEnum and primes the merge by advancing each sub to
// its first term. Each TermsEnum must be freshly created, i.e. Next has not yet
// been called. Subs whose field has no terms are dropped. Mirrors
// MultiTermsEnum.reset(TermsEnumIndex[]) in Lucene.
//
// Reset returns the receiver when at least one sub has terms, or nil when every
// sub is empty (mirroring Lucene's TermsEnum.EMPTY result, which callers treat
// as "no terms").
func (m *MultiTermsEnum) Reset(termsEnums []TermsEnum) (*MultiTermsEnum, error) {
	if len(termsEnums) > len(m.top) {
		return nil, fmt.Errorf("MultiTermsEnum.Reset: got %d sub-enums but only %d slices",
			len(termsEnums), len(m.top))
	}
	m.numSubs = 0
	m.numTop = 0
	m.current = nil
	m.lastSeek = nil
	m.lastSeekExact = false
	m.queue.clear()

	for i := 0; i < len(termsEnums); i++ {
		te := termsEnums[i]
		if te == nil {
			return nil, fmt.Errorf("MultiTermsEnum.Reset: sub-enum %d is nil", i)
		}
		term, err := te.Next()
		if err != nil {
			return nil, fmt.Errorf("MultiTermsEnum.Reset: priming sub %d: %w", i, err)
		}
		if term != nil {
			entry := m.subs[i]
			entry.terms = te
			entry.current = term
			m.queue.add(entry)
			m.currentSubs[m.numSubs] = entry
			m.numSubs++
		}
		// else: field has no terms in this sub; drop it.
	}

	if m.queue.size() == 0 {
		return nil, nil
	}
	return m, nil
}

// GetMatchCount returns the number of sub-enumerators currently positioned on
// the same (current) term. Mirrors MultiTermsEnum.getMatchCount semantics by
// returning numTop.
func (m *MultiTermsEnum) GetMatchCount() int { return m.numTop }

// GetSlices returns the underlying ReaderSlice list.
func (m *MultiTermsEnum) GetSlices() []ReaderSlice { return m.slices }

// Term returns the term the merged enum is currently positioned on, or nil.
func (m *MultiTermsEnum) Term() *Term { return m.current }

// pullTop extracts from the queue every sub positioned on the least term into
// top[] and sets current. Mirrors MultiTermsEnum.pullTop.
func (m *MultiTermsEnum) pullTop() {
	// numTop must be 0 here.
	m.numTop = m.queue.fillTop(m.top)
	if m.numTop > 0 {
		m.current = m.top[0].current
	} else {
		m.current = nil
	}
}

// pushTop advances each of the numTop subs currently tied at the head of the
// queue to its next term, re-sifting it in place (updateTop) or removing it
// (pop) when its enum is exhausted. The tied subs occupy the head of the queue,
// so calling queue.top() numTop times in turn yields each of them. Mirrors
// MultiTermsEnum.pushTop.
func (m *MultiTermsEnum) pushTop() error {
	for i := 0; i < m.numTop; i++ {
		top := m.queue.top()
		next, err := top.terms.Next()
		if err != nil {
			return fmt.Errorf("MultiTermsEnum.pushTop: advancing sub %d: %w", top.subIndex, err)
		}
		top.current = next
		if next == nil {
			// Sub exhausted: drop it from the queue.
			m.queue.pop()
		} else {
			// Sub advanced to a larger term: re-sift the root.
			m.queue.updateTop()
		}
	}
	m.numTop = 0
	return nil
}

// Next advances to the next merged term in byte order, returning nil at the end.
// Mirrors MultiTermsEnum.next.
func (m *MultiTermsEnum) Next() (*Term, error) {
	if m.lastSeekExact {
		// We previously did a seekExact; before next() can work we must
		// seekCeil so the subs that lacked the term can find the following one.
		// We can't simply call Next here, since that would not let OTHER subs
		// locate the smallest term following the just-sought term. Mirrors
		// MultiTermsEnum.next() in Lucene, which seekCeils to the current term.
		if m.lastSeek == nil {
			return nil, fmt.Errorf("MultiTermsEnum.Next: lastSeekExact set but lastSeek nil")
		}
		// Disable the LUCENE-2130 seek optimization for this re-seek: after a
		// seekExact, the non-matching subs' cached current terms are stale, so
		// every sub must be genuinely re-sought. Clearing lastSeek forces the
		// non-optimized path inside SeekCeil.
		target := m.lastSeek
		m.lastSeek = nil
		if _, err := m.SeekCeil(target); err != nil {
			return nil, err
		}
	}

	// Restore the queue: advance the previous tops.
	if err := m.pushTop(); err != nil {
		return nil, err
	}

	// Gather the new equal-top subs.
	if m.queue.size() > 0 {
		m.pullTop()
	} else {
		m.current = nil
	}

	return m.current, nil
}

// SeekCeil positions the merge on the smallest term >= target, mirroring
// MultiTermsEnum.seekCeil. It returns the term landed on, or nil when target is
// past the last term (SeekStatus.END). A returned term equal to target
// corresponds to SeekStatus.FOUND; a greater term to SeekStatus.NOT_FOUND.
func (m *MultiTermsEnum) SeekCeil(target *Term) (*Term, error) {
	m.queue.clear()
	m.numTop = 0
	m.lastSeekExact = false

	seekOpt := m.lastSeek != nil && m.lastSeek.CompareTo(target) <= 0

	// Copy target so a caller mutating its bytes later cannot corrupt our
	// stored seek key.
	m.lastSeek = NewTermFromBytes(target.Field, append([]byte(nil), target.GetBytesRef().ValidBytes()...))

	for i := 0; i < m.numSubs; i++ {
		sub := m.currentSubs[i]
		var status SeekStatus
		if seekOpt {
			// LUCENE-2130: if we just seek'd, and the new seek is at or after
			// the previous one, skip re-seeking subs already beyond target.
			curTerm := sub.current
			if curTerm != nil {
				cmp := target.CompareTo(curTerm)
				if cmp == 0 {
					status = SeekStatusFound
				} else if cmp < 0 {
					status = SeekStatusNotFound
				} else {
					s, err := seekCeilStatus(sub.terms, target)
					if err != nil {
						return nil, err
					}
					status = s
				}
			} else {
				status = SeekStatusEnd
			}
		} else {
			s, err := seekCeilStatus(sub.terms, target)
			if err != nil {
				return nil, err
			}
			status = s
		}

		switch status {
		case SeekStatusFound:
			sub.current = sub.terms.Term()
			m.top[m.numTop] = sub
			m.numTop++
			m.current = sub.current
			m.queue.add(sub)
		case SeekStatusNotFound:
			sub.current = sub.terms.Term()
			if sub.current == nil {
				return nil, fmt.Errorf("MultiTermsEnum.SeekCeil: sub %d returned NOT_FOUND but nil term", sub.subIndex)
			}
			m.queue.add(sub)
		default: // SeekStatusEnd
			sub.current = nil
		}
	}

	if m.numTop > 0 {
		// At least one sub had an exact match for target.
		return m.current, nil
	} else if m.queue.size() > 0 {
		// No exact match, but some sub found a term after target: advance to it.
		m.pullTop()
		return m.current, nil
	}
	m.current = nil
	return nil, nil
}

// SeekExact positions the merge exactly on target, returning true when at least
// one sub contains it. Mirrors MultiTermsEnum.seekExact.
func (m *MultiTermsEnum) SeekExact(target *Term) (bool, error) {
	m.queue.clear()
	m.numTop = 0

	seekOpt := m.lastSeek != nil && m.lastSeek.CompareTo(target) <= 0

	m.lastSeek = nil
	m.lastSeekExact = true

	for i := 0; i < m.numSubs; i++ {
		sub := m.currentSubs[i]
		var found bool
		if seekOpt {
			curTerm := sub.current
			if curTerm != nil {
				cmp := target.CompareTo(curTerm)
				if cmp == 0 {
					found = true
				} else if cmp < 0 {
					found = false
				} else {
					ok, err := sub.terms.SeekExact(target)
					if err != nil {
						return false, fmt.Errorf("MultiTermsEnum.SeekExact: sub %d: %w", sub.subIndex, err)
					}
					found = ok
				}
			} else {
				found = false
			}
		} else {
			ok, err := sub.terms.SeekExact(target)
			if err != nil {
				return false, fmt.Errorf("MultiTermsEnum.SeekExact: sub %d: %w", sub.subIndex, err)
			}
			found = ok
		}

		if found {
			sub.current = sub.terms.Term()
			m.top[m.numTop] = sub
			m.numTop++
			m.current = sub.current
		}
	}

	// lastSeek must hold the sought term so a following Next can seekCeil to let
	// subs that lacked the term find the following one. We copy the bytes so a
	// caller mutating target later cannot corrupt our stored seek key.
	m.lastSeek = NewTermFromBytes(target.Field, append([]byte(nil), target.GetBytesRef().ValidBytes()...))

	return m.numTop > 0, nil
}

// DocFreq sums the document frequency of the current term across the subs that
// share it. Mirrors MultiTermsEnum.docFreq.
func (m *MultiTermsEnum) DocFreq() (int, error) {
	sum := 0
	for i := 0; i < m.numTop; i++ {
		df, err := m.top[i].terms.DocFreq()
		if err != nil {
			return 0, fmt.Errorf("MultiTermsEnum.DocFreq: sub %d: %w", m.top[i].subIndex, err)
		}
		sum += df
	}
	return sum, nil
}

// TotalTermFreq sums the total term frequency of the current term across the
// subs that share it. Returns -1 as soon as any sub reports -1 (frequencies not
// available), mirroring MultiTermsEnum.totalTermFreq.
func (m *MultiTermsEnum) TotalTermFreq() (int64, error) {
	var sum int64
	for i := 0; i < m.numTop; i++ {
		v, err := m.top[i].terms.TotalTermFreq()
		if err != nil {
			return 0, fmt.Errorf("MultiTermsEnum.TotalTermFreq: sub %d: %w", m.top[i].subIndex, err)
		}
		if v == -1 {
			return -1, nil
		}
		sum += v
	}
	return sum, nil
}

// Postings returns a MultiPostingsEnum union over the subs sharing the current
// term, mapping each sub's local doc IDs into the composite doc-ID space via its
// slice base. Mirrors MultiTermsEnum.postings.
func (m *MultiTermsEnum) Postings(flags int) (PostingsEnum, error) {
	return m.postings(nil, flags)
}

// PostingsWithLiveDocs returns a MultiPostingsEnum union over the subs sharing
// the current term. The liveDocs argument is accepted for interface
// compatibility; per-sub liveDocs filtering is applied upstream by the sub
// readers, so this delegates to the same union as Postings (mirroring Lucene,
// where liveDocs filtering for the merged enum is handled by the leaf readers).
func (m *MultiTermsEnum) PostingsWithLiveDocs(_ util.Bits, flags int) (PostingsEnum, error) {
	return m.postings(nil, flags)
}

func (m *MultiTermsEnum) postings(reuse PostingsEnum, flags int) (PostingsEnum, error) {
	var docsEnum *MultiPostingsEnum
	if mpe, ok := reuse.(*MultiPostingsEnum); ok && mpe.CanReuse(m) {
		docsEnum = mpe
	} else {
		docsEnum = NewMultiPostingsEnum(m, len(m.subs))
	}

	// Order the top subs by sub index so composite doc IDs are produced in
	// ascending order (slice bases increase with sub index). Mirrors Lucene's
	// timSort on INDEX_COMPARATOR.
	sort.SliceStable(m.top[:m.numTop], func(a, b int) bool {
		return m.top[a].subIndex < m.top[b].subIndex
	})

	upto := 0
	for i := 0; i < m.numTop; i++ {
		entry := m.top[i]
		subPostings, err := entry.terms.Postings(flags)
		if err != nil {
			return nil, fmt.Errorf("MultiTermsEnum.postings: sub %d: %w", entry.subIndex, err)
		}
		if subPostings == nil {
			return nil, fmt.Errorf("MultiTermsEnum.postings: sub %d returned nil postings", entry.subIndex)
		}
		entry.subDocs.PostingsEnum = subPostings
		entry.subDocs.Slice = entry.subSlice
		m.subDocs[upto] = entry.subDocs
		upto++
	}

	return docsEnum.Reset(m.subDocs, upto), nil
}

// seekCeilStatus adapts a TermsEnum.SeekCeil (which returns the landed term or
// nil) to the SeekStatus tri-state used by the merge logic.
func seekCeilStatus(te TermsEnum, target *Term) (SeekStatus, error) {
	landed, err := te.SeekCeil(target)
	if err != nil {
		return SeekStatusEnd, fmt.Errorf("seekCeil: %w", err)
	}
	if landed == nil {
		return SeekStatusEnd, nil
	}
	if target.CompareTo(landed) == 0 {
		return SeekStatusFound, nil
	}
	return SeekStatusNotFound, nil
}

// Compile-time assertion that MultiTermsEnum satisfies TermsEnum.
var _ TermsEnum = (*MultiTermsEnum)(nil)
