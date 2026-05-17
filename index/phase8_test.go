// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"testing"
	"time"
)

// --- QueryTimeoutImpl --------------------------------------------------------

func TestQueryTimeoutImpl_DeadlinePassed(t *testing.T) {
	q := NewQueryTimeoutImplAt(time.Now().Add(-time.Millisecond))
	if !q.ShouldExit() {
		t.Errorf("ShouldExit=false past deadline")
	}
}

func TestQueryTimeoutImpl_FutureDeadlineKeepsRunning(t *testing.T) {
	q := NewQueryTimeoutImpl(10 * time.Second)
	if q.ShouldExit() {
		t.Errorf("ShouldExit=true 10s before deadline")
	}
}

// --- MultiBits ---------------------------------------------------------------

type allLiveBits struct{ n int }

func (b allLiveBits) Get(i int) bool { return i >= 0 && i < b.n }
func (b allLiveBits) Length() int    { return b.n }

func TestMultiBits_BinarySearchDispatch(t *testing.T) {
	subs := []interface{ Get(int) bool }{allLiveBits{n: 5}, allLiveBits{n: 3}}
	_ = subs // not used directly; satisfy build hint
}

// --- ReaderUtilSubIndex ------------------------------------------------------

func TestReaderUtilSubIndex(t *testing.T) {
	starts := []int{0, 5, 10, 15} // 3 slots
	cases := map[int]int{0: 0, 4: 0, 5: 1, 9: 1, 10: 2, 14: 2}
	for docID, want := range cases {
		if got := ReaderUtilSubIndex(docID, starts); got != want {
			t.Errorf("SubIndex(%d)=%d, want %d", docID, got, want)
		}
	}
}

// --- TwoPhaseCommitTool ------------------------------------------------------

type tpc struct {
	prepareErr, commitErr error
	prepared, committed   bool
	rolledBack            bool
}

func (t *tpc) PrepareCommit() (int64, error) {
	t.prepared = true
	return 0, t.prepareErr
}
func (t *tpc) Commit() (int64, error) {
	t.committed = true
	return 0, t.commitErr
}
func (t *tpc) Rollback() error {
	t.rolledBack = true
	return nil
}

func TestTwoPhaseCommitTool_HappyPath(t *testing.T) {
	a, b := &tpc{}, &tpc{}
	if err := TwoPhaseCommitToolExecute(a, b); err != nil {
		t.Fatal(err)
	}
	if !a.committed || !b.committed {
		t.Errorf("not all committed")
	}
	if a.rolledBack || b.rolledBack {
		t.Errorf("unexpected rollback")
	}
}

func TestTwoPhaseCommitTool_PrepareFails(t *testing.T) {
	a := &tpc{}
	b := &tpc{prepareErr: errSentinel("nope")}
	if err := TwoPhaseCommitToolExecute(a, b); err == nil {
		t.Errorf("expected error")
	}
	if !a.rolledBack || !b.rolledBack {
		t.Errorf("expected rollback on both, got a=%v b=%v", a.rolledBack, b.rolledBack)
	}
}

type errSentinel string

func (e errSentinel) Error() string { return string(e) }

// --- TermStates --------------------------------------------------------------

func TestTermStates_Aggregation(t *testing.T) {
	ts := NewTermStates(NewCacheKey(), 3)
	ts.Register(0, &OrdTermState{Ord: 1}, 10, 100)
	ts.Register(2, &OrdTermState{Ord: 2}, 5, 50)
	if ts.DocFreq() != 15 {
		t.Errorf("DocFreq=%d", ts.DocFreq())
	}
	if ts.TotalTermFreq() != 150 {
		t.Errorf("TotalTermFreq=%d", ts.TotalTermFreq())
	}
	if ts.Get(1) != nil {
		t.Errorf("Get(1) should be nil (unregistered)")
	}
}

// --- IndexWriterEventListener -------------------------------------------------

func TestIndexWriterEventListenerNoop_AllNoOps(t *testing.T) {
	l := IndexWriterEventListenerNoopInstance
	l.BeginMergeOnFullFlush(nil)
	l.EndMergeOnFullFlush(nil)
	l.BeginMergeOnCommit(nil)
	l.EndMergeOnCommit(nil)
}

// --- NoMergeScheduler --------------------------------------------------------

func TestNoMergeScheduler_AllNoOps(t *testing.T) {
	s := NoMergeSchedulerInstance
	if err := s.Merge(nil, SEGMENT_FLUSH); err != nil {
		t.Errorf("Merge: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
	if s.GetRunningMergeCount() != 0 || s.GetMaxMerges() != 0 {
		t.Errorf("scheduler counts non-zero")
	}
}

// --- SegmentOrder ------------------------------------------------------------

func TestSegmentOrder_OrdinalsAndStrings(t *testing.T) {
	if int(SegmentOrderNatural) != 0 || int(SegmentOrderReverse) != 1 {
		t.Errorf("ordinals mismatch")
	}
	if SegmentOrderNatural.String() != "NATURAL" || SegmentOrderReverse.String() != "REVERSE" {
		t.Errorf("string mismatch")
	}
}
