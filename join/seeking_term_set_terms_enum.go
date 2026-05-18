// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package join

import "sort"

// SeekingTermSetTermsEnum is a sorted-set view that supports seeking to the
// smallest term greater than or equal to a target. Mirrors
// org.apache.lucene.search.join.SeekingTermSetTermsEnum: the join code path
// uses it to intersect a probe term against the set of join keys discovered
// in the indexed side.
type SeekingTermSetTermsEnum struct {
	terms []string
	pos   int
}

// NewSeekingTermSetTermsEnum builds a sorted-set enum from the supplied
// terms; the input slice is copied and sorted to guarantee deterministic
// seeking.
func NewSeekingTermSetTermsEnum(terms []string) *SeekingTermSetTermsEnum {
	clone := make([]string, len(terms))
	copy(clone, terms)
	sort.Strings(clone)
	return &SeekingTermSetTermsEnum{terms: clone, pos: -1}
}

// Size returns the number of terms.
func (e *SeekingTermSetTermsEnum) Size() int { return len(e.terms) }

// SeekCeil positions the cursor on the smallest term >= target. Returns true
// when such a term exists; false when target is larger than every term.
func (e *SeekingTermSetTermsEnum) SeekCeil(target string) bool {
	idx := sort.SearchStrings(e.terms, target)
	if idx >= len(e.terms) {
		e.pos = idx
		return false
	}
	e.pos = idx
	return true
}

// SeekExact positions the cursor on target. Returns true iff the target is
// present.
func (e *SeekingTermSetTermsEnum) SeekExact(target string) bool {
	idx := sort.SearchStrings(e.terms, target)
	if idx >= len(e.terms) || e.terms[idx] != target {
		return false
	}
	e.pos = idx
	return true
}

// Next advances to the next term. Returns the term and true, or ("", false)
// when exhausted.
func (e *SeekingTermSetTermsEnum) Next() (string, bool) {
	e.pos++
	if e.pos >= len(e.terms) {
		return "", false
	}
	return e.terms[e.pos], true
}

// Term returns the term at the current cursor position, or "" when the
// cursor is before-start / past-end.
func (e *SeekingTermSetTermsEnum) Term() string {
	if e.pos < 0 || e.pos >= len(e.terms) {
		return ""
	}
	return e.terms[e.pos]
}
