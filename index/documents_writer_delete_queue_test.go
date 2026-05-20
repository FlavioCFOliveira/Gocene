// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// TestDocumentsWriterDeleteQueue
// Source: lucene/core/src/test/org/apache/lucene/index/TestDocumentsWriterDeleteQueue.java
// Purpose: Unit test for DocumentsWriterDeleteQueue - the non-blocking linked
// pending-deletes queue, its DeleteSlice views, freezing, clearing and close.

package index

import (
	"math/rand"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
)

// termKey produces the (field, bytes) identity key the Java peer obtains from
// a Term's HashSet membership / Map keySet. The bytes are the term's UTF-8
// value, matching DeletedTerms keying.
func termKey(field, text string) string {
	return field + "\x00" + text
}

// globalDeleteTermKeys returns the set of (field, text) keys currently buffered
// in the queue's global term deletes, mirroring Java's reads of the package-
// private deleteTerms via numGlobalTermDeletes and the freeze iterator.
func bufferedTermKeys(bu *BufferedUpdates) map[string]struct{} {
	out := make(map[string]struct{})
	for _, e := range bu.deleteTerms.ForEachOrdered() {
		out[termKey(e.Field, string(e.Bytes))] = struct{}{}
	}
	return out
}

// assertAllBetween mirrors the Java helper: every id in [start,end] must map
// to end as its docID upper bound inside deletes.
func assertAllBetween(t *testing.T, start, end int, deletes *BufferedUpdates, ids []int) {
	t.Helper()
	for i := start; i <= end; i++ {
		term := NewTerm("id", strconv.Itoa(ids[i]))
		if got := deletes.deleteTerms.Get(term); got != end {
			t.Fatalf("id %d: expected docID upper bound %d, got %d", ids[i], end, got)
		}
	}
}

// TestUpdateDeleteSlices is the Go port of testUpdateDeleteSlices.
func TestUpdateDeleteSlices(t *testing.T) {
	queue := NewDocumentsWriterDeleteQueue(nil)
	size := 200 + rand.Intn(500)
	ids := make([]int, size)
	for i := range ids {
		ids[i] = rand.Int()
	}
	slice1 := queue.NewSlice()
	slice2 := queue.NewSlice()
	bd1 := NewBufferedUpdates("bd1")
	bd2 := NewBufferedUpdates("bd2")
	last1, last2 := 0, 0
	uniqueValues := make(map[string]struct{})

	for j := 0; j < len(ids); j++ {
		term := NewTerm("id", strconv.Itoa(ids[j]))
		uniqueValues[termKey(term.Field, term.Text())] = struct{}{}
		if _, err := queue.AddDeleteTerms(term); err != nil {
			t.Fatalf("AddDeleteTerms: %v", err)
		}
		if rand.Intn(20) == 0 || j == len(ids)-1 {
			if _, err := queue.UpdateSlice(slice1); err != nil {
				t.Fatalf("UpdateSlice: %v", err)
			}
			slice1.Apply(bd1, j)
			assertAllBetween(t, last1, j, bd1, ids)
			last1 = j + 1
		}
		if rand.Intn(10) == 5 || j == len(ids)-1 {
			if _, err := queue.UpdateSlice(slice2); err != nil {
				t.Fatalf("UpdateSlice: %v", err)
			}
			slice2.Apply(bd2, j)
			assertAllBetween(t, last2, j, bd2, ids)
			last2 = j + 1
		}
		if got := queue.NumGlobalTermDeletes(); got != len(uniqueValues) {
			t.Fatalf("NumGlobalTermDeletes: expected %d, got %d", len(uniqueValues), got)
		}
	}
	if !sameKeySet(bufferedTermKeys(bd1), uniqueValues) {
		t.Fatalf("bd1 deleteTerms key set diverged")
	}
	if !sameKeySet(bufferedTermKeys(bd2), uniqueValues) {
		t.Fatalf("bd2 deleteTerms key set diverged")
	}

	frozen, err := queue.FreezeGlobalBuffer(nil)
	if err != nil {
		t.Fatalf("FreezeGlobalBuffer: %v", err)
	}
	frozenSet := frozenTermKeys(frozen)
	if !sameKeySet(frozenSet, uniqueValues) {
		t.Fatalf("frozen term set diverged from unique values")
	}
	if got := queue.NumGlobalTermDeletes(); got != 0 {
		t.Fatalf("num deletes must be 0 after freeze, got %d", got)
	}
}

// TestClear is the Go port of testClear.
func TestDeleteQueueClear(t *testing.T) {
	queue := NewDocumentsWriterDeleteQueue(nil)
	if queue.AnyChanges() {
		t.Fatal("fresh queue must have no changes")
	}
	queue.Clear()
	if queue.AnyChanges() {
		t.Fatal("cleared empty queue must have no changes")
	}
	size := 200 + rand.Intn(500)
	for i := 0; i < size; i++ {
		term := NewTerm("id", strconv.Itoa(i))
		if rand.Intn(10) == 0 {
			if _, err := queue.AddDeleteQueries(&mockQuery{id: i}); err != nil {
				t.Fatalf("AddDeleteQueries: %v", err)
			}
		} else {
			if _, err := queue.AddDeleteTerms(term); err != nil {
				t.Fatalf("AddDeleteTerms: %v", err)
			}
		}
		if !queue.AnyChanges() {
			t.Fatal("queue must have changes after add")
		}
		if rand.Intn(10) == 0 {
			queue.Clear()
			queue.TryApplyGlobalSlice()
			if queue.AnyChanges() {
				t.Fatal("cleared queue must have no changes")
			}
		}
	}
}

// TestAnyChanges is the Go port of testAnyChanges.
func TestDeleteQueueAnyChanges(t *testing.T) {
	queue := NewDocumentsWriterDeleteQueue(nil)
	size := 200 + rand.Intn(500)
	termsSinceFreeze, queriesSinceFreeze := 0, 0
	for i := 0; i < size; i++ {
		term := NewTerm("id", strconv.Itoa(i))
		if rand.Intn(10) == 0 {
			if _, err := queue.AddDeleteQueries(&mockQuery{id: i}); err != nil {
				t.Fatalf("AddDeleteQueries: %v", err)
			}
			queriesSinceFreeze++
		} else {
			if _, err := queue.AddDeleteTerms(term); err != nil {
				t.Fatalf("AddDeleteTerms: %v", err)
			}
			termsSinceFreeze++
		}
		if !queue.AnyChanges() {
			t.Fatal("queue must have changes after add")
		}
		if rand.Intn(5) == 0 {
			frozen, err := queue.FreezeGlobalBuffer(nil)
			if err != nil {
				t.Fatalf("FreezeGlobalBuffer: %v", err)
			}
			if got := int(frozen.DeleteTermsSize()); got != termsSinceFreeze {
				t.Fatalf("frozen deleteTerms size: expected %d, got %d", termsSinceFreeze, got)
			}
			if got := len(frozen.DeleteQueries()); got != queriesSinceFreeze {
				t.Fatalf("frozen deleteQueries length: expected %d, got %d", queriesSinceFreeze, got)
			}
			queriesSinceFreeze, termsSinceFreeze = 0, 0
			if queue.AnyChanges() {
				t.Fatal("queue must have no changes after freeze")
			}
		}
	}
}

// TestPartiallyAppliedGlobalSlice is the Go port of
// testPartiallyAppliedGlobalSlice.
func TestPartiallyAppliedGlobalSlice(t *testing.T) {
	queue := NewDocumentsWriterDeleteQueue(nil)
	queue.GlobalBufferLock.Lock()
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if _, err := queue.AddDeleteTerms(NewTerm("foo", "bar")); err != nil {
			t.Errorf("AddDeleteTerms: %v", err)
		}
	}()
	wg.Wait()
	queue.GlobalBufferLock.Unlock()

	if !queue.AnyChanges() {
		t.Fatal("changes in del queue but not in slice yet")
	}
	queue.TryApplyGlobalSlice()
	if !queue.AnyChanges() {
		t.Fatal("changes in global buffer")
	}
	frozen, err := queue.FreezeGlobalBuffer(nil)
	if err != nil {
		t.Fatalf("FreezeGlobalBuffer: %v", err)
	}
	if !frozen.Any() {
		t.Fatal("frozen packet must report changes")
	}
	if got := frozen.DeleteTermsSize(); got != 1 {
		t.Fatalf("frozen deleteTerms size: expected 1, got %d", got)
	}
	if queue.AnyChanges() {
		t.Fatal("all changes must be applied")
	}
}

// TestStressDeleteQueue is the Go port of testStressDeleteQueue.
func TestStressDeleteQueue(t *testing.T) {
	queue := NewDocumentsWriterDeleteQueue(nil)
	size := 10000 + rand.Intn(500)
	ids := make([]int, size)
	uniqueValues := make(map[string]struct{})
	for i := range ids {
		ids[i] = rand.Int()
		uniqueValues[termKey("id", strconv.Itoa(ids[i]))] = struct{}{}
	}

	numThreads := 2 + rand.Intn(5)
	var index atomic.Int32
	var start sync.WaitGroup
	start.Add(1)
	var done sync.WaitGroup
	type worker struct {
		slice   *DeleteSlice
		deletes *BufferedUpdates
	}
	workers := make([]worker, numThreads)
	for i := range workers {
		workers[i] = worker{slice: queue.NewSlice(), deletes: NewBufferedUpdates("deletes")}
		w := workers[i]
		done.Add(1)
		go func() {
			defer done.Done()
			start.Wait()
			for {
				i := int(index.Add(1)) - 1
				if i >= len(ids) {
					return
				}
				term := NewTerm("id", strconv.Itoa(ids[i]))
				termNode := NewTermNode(term)
				if _, err := queue.AddWithSlice(termNode, w.slice); err != nil {
					t.Errorf("AddWithSlice: %v", err)
					return
				}
				if !w.slice.IsTail(termNode) {
					t.Errorf("node must be the slice tail after AddWithSlice")
					return
				}
				w.slice.Apply(w.deletes, MaxInt)
			}
		}()
	}
	start.Done()
	done.Wait()

	for _, w := range workers {
		if _, err := queue.UpdateSlice(w.slice); err != nil {
			t.Fatalf("UpdateSlice: %v", err)
		}
		w.slice.Apply(w.deletes, MaxInt)
		if !sameKeySet(bufferedTermKeys(w.deletes), uniqueValues) {
			t.Fatalf("worker deletes key set diverged from unique values")
		}
	}
	queue.TryApplyGlobalSlice()
	frozen, err := queue.FreezeGlobalBuffer(nil)
	if err != nil {
		t.Fatalf("FreezeGlobalBuffer: %v", err)
	}
	frozenSet := frozenTermKeys(frozen)
	if got := queue.NumGlobalTermDeletes(); got != 0 {
		t.Fatalf("num deletes must be 0 after freeze, got %d", got)
	}
	if len(frozenSet) != len(uniqueValues) {
		t.Fatalf("frozen set size %d != unique values size %d", len(frozenSet), len(uniqueValues))
	}
	if !sameKeySet(frozenSet, uniqueValues) {
		t.Fatalf("frozen set diverged from unique values")
	}
}

// TestDeleteQueueClose is the Go port of testClose.
func TestDeleteQueueClose(t *testing.T) {
	// First block: close an empty queue, then assert rejection of mutators.
	{
		queue := NewDocumentsWriterDeleteQueue(nil)
		if !queue.IsOpen() {
			t.Fatal("fresh queue must be open")
		}
		if err := queue.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
		if rand.Intn(2) == 0 {
			if err := queue.Close(); err != nil { // double close
				t.Fatalf("double Close: %v", err)
			}
		}
		if _, err := queue.AddDeleteTerms(NewTerm("foo", "bar")); !isAlreadyClosed(err) {
			t.Fatalf("AddDeleteTerms on closed queue: expected AlreadyClosedException, got %v", err)
		}
		if _, err := queue.FreezeGlobalBuffer(nil); !isAlreadyClosed(err) {
			t.Fatalf("FreezeGlobalBuffer on closed queue: expected AlreadyClosedException, got %v", err)
		}
		if _, err := queue.AddDeleteQueries(&mockQuery{id: 1}); !isAlreadyClosed(err) {
			t.Fatalf("AddDeleteQueries on closed queue: expected AlreadyClosedException, got %v", err)
		}
		numUpdate := NewNumericDocValuesUpdate(NewTerm("foo", "bar"), "foo", 1)
		dvUpdate := &numUpdate.DocValuesUpdate
		dvUpdate.Value = numUpdate.NumericValue
		if _, err := queue.AddDocValuesUpdates(dvUpdate); !isAlreadyClosed(err) {
			t.Fatalf("AddDocValuesUpdates on closed queue: expected AlreadyClosedException, got %v", err)
		}
		if _, err := queue.Add(nil); !isAlreadyClosed(err) {
			t.Fatalf("Add(nil) on closed queue: expected AlreadyClosedException, got %v", err)
		}
		frozen, err := queue.MaybeFreezeGlobalBuffer() // fine on a closed queue
		if err != nil {
			t.Fatalf("MaybeFreezeGlobalBuffer: %v", err)
		}
		if frozen != nil {
			t.Fatal("MaybeFreezeGlobalBuffer on closed queue must return nil")
		}
		if queue.IsOpen() {
			t.Fatal("closed queue must not report open")
		}
	}
	// Second block: closing with unapplied changes must fail.
	{
		queue := NewDocumentsWriterDeleteQueue(nil)
		if _, err := queue.AddDeleteTerms(NewTerm("foo", "bar")); err != nil {
			t.Fatalf("AddDeleteTerms: %v", err)
		}
		if err := queue.Close(); err == nil {
			t.Fatal("Close with unapplied changes must fail")
		}
		if !queue.IsOpen() {
			t.Fatal("queue must remain open after failed Close")
		}
		queue.TryApplyGlobalSlice()
		if _, err := queue.FreezeGlobalBuffer(nil); err != nil {
			t.Fatalf("FreezeGlobalBuffer: %v", err)
		}
		if err := queue.Close(); err != nil {
			t.Fatalf("Close after freeze: %v", err)
		}
		if queue.IsOpen() {
			t.Fatal("queue must be closed")
		}
	}
}

// TestDeleteSliceIsTailItem exercises DeleteSlice.IsTailItem against the
// identity-comparison contract relied upon by the Java peer.
func TestDeleteSliceIsTailItem(t *testing.T) {
	queue := NewDocumentsWriterDeleteQueue(nil)
	slice := queue.NewSlice()
	terms := []*Term{NewTerm("id", "1")}
	if _, err := queue.AddDeleteTerms(terms...); err != nil {
		t.Fatalf("AddDeleteTerms: %v", err)
	}
	if _, err := queue.UpdateSlice(slice); err != nil {
		t.Fatalf("UpdateSlice: %v", err)
	}
	if !slice.IsTailItem(terms) {
		t.Fatal("IsTailItem must report the term-array item identity")
	}
	if slice.IsTailItem([]*Term{NewTerm("id", "1")}) {
		t.Fatal("IsTailItem must compare by identity, not value")
	}
}

// TestAdvanceQueue exercises AdvanceQueue and the seqNo carry-over.
func TestAdvanceQueue(t *testing.T) {
	queue := NewDocumentsWriterDeleteQueue(nil)
	first := queue.GetNextSequenceNumber()
	if first != 1 {
		t.Fatalf("first sequence number: expected 1, got %d", first)
	}
	next, err := queue.AdvanceQueue(3)
	if err != nil {
		t.Fatalf("AdvanceQueue: %v", err)
	}
	if !queue.IsAdvanced() {
		t.Fatal("source queue must report advanced")
	}
	if _, err := queue.AdvanceQueue(3); err == nil {
		t.Fatal("second AdvanceQueue must fail")
	}
	if next.Generation != queue.Generation+1 {
		t.Fatalf("successor generation: expected %d, got %d", queue.Generation+1, next.Generation)
	}
	if next.GetMaxCompletedSeqNo() != queue.GetMaxSeqNo() {
		// successor falls back to source's last seq when not advanced.
		if next.GetMaxCompletedSeqNo() < 0 {
			t.Fatalf("successor GetMaxCompletedSeqNo invalid: %d", next.GetMaxCompletedSeqNo())
		}
	}
}

// frozenTermKeys collects (field, text) keys from a frozen packet's term
// iterator, mirroring the Java peer's freeze-iterator HashSet build.
func frozenTermKeys(frozen *FrozenBufferedUpdates) map[string]struct{} {
	out := make(map[string]struct{})
	if frozen == nil {
		return out
	}
	it := frozen.FrozenTermsIterator()
	for {
		bytes := it.Next()
		if bytes == nil {
			break
		}
		out[termKey(it.Field(), string(bytes))] = struct{}{}
	}
	return out
}

// sameKeySet reports whether two key sets are equal.
func sameKeySet(a, b map[string]struct{}) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if _, ok := b[k]; !ok {
			return false
		}
	}
	return true
}

// isAlreadyClosed reports whether err is an AlreadyClosedException.
func isAlreadyClosed(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(*AlreadyClosedException)
	return ok
}
