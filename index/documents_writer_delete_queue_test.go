// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/index/TestDocumentsWriterDeleteQueue.java
// (Apache Lucene 10.5.0). The Java class lives in org.apache.lucene.index and
// uses search.TermQuery and search.MatchNoDocsQuery; the port lives in the
// external index_test package to avoid the index -> search import cycle and
// reaches the package-private members through export_test.go.

package index_test

import (
	"errors"
	"math/rand"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// dwdqTermKey is the identity of a Term inside the Java HashSet<Term>
// (Term.equals compares field and bytes).
func dwdqTermKey(t index.Term) string {
	return t.Field + "\x00" + string(t.Bytes.ValidBytes())
}

func dwdqKeySet(terms []index.Term) map[string]struct{} {
	out := make(map[string]struct{}, len(terms))
	for _, t := range terms {
		out[dwdqTermKey(t)] = struct{}{}
	}
	return out
}

func dwdqSameSet(a, b map[string]struct{}) bool {
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

func dwdqFrozenSet(t *testing.T, queue *index.DocumentsWriterDeleteQueue) map[string]struct{} {
	t.Helper()
	frozen, err := queue.FreezeGlobalBuffer(nil)
	if err != nil {
		t.Fatalf("freezeGlobalBuffer: %v", err)
	}
	frozenSet := make(map[string]struct{})
	iter := frozen.FrozenTermsIterator()
	for {
		bytes := iter.Next()
		if bytes == nil {
			break
		}
		frozenSet[dwdqTermKey(*index.NewTermFromBytes(iter.Field(), append([]byte(nil), bytes...)))] = struct{}{}
	}
	return frozenSet
}

func TestDocumentsWriterDeleteQueueUpdateDeleteSlices(t *testing.T) {
	queue := index.NewDocumentsWriterDeleteQueue(nil)
	size := 200 + rand.Intn(500)
	ids := make([]int32, size)
	for i := range ids {
		ids[i] = int32(rand.Uint32())
	}
	slice1 := queue.NewSlice()
	slice2 := queue.NewSlice()
	bd1 := index.NewBufferedUpdates("bd1")
	bd2 := index.NewBufferedUpdates("bd2")
	last1, last2 := 0, 0
	uniqueValues := make(map[string]struct{})
	for j := 0; j < len(ids); j++ {
		i := ids[j]
		// create an array here since we compare identity below against tailItem
		term := []index.Term{*index.NewTerm("id", strconv.Itoa(int(i)))}
		uniqueValues[dwdqTermKey(term[0])] = struct{}{}
		if _, err := queue.AddDeleteTerms(term...); err != nil {
			t.Fatalf("addDelete: %v", err)
		}
		if rand.Intn(20) == 0 || j == len(ids)-1 {
			if _, err := queue.UpdateSlice(slice1); err != nil {
				t.Fatalf("updateSlice: %v", err)
			}
			if !slice1.IsTailItem(term) {
				t.Fatal("slice1.isTailItem(term)")
			}
			slice1.Apply(bd1, j)
			dwdqAssertAllBetween(t, last1, j, bd1, ids)
			last1 = j + 1
		}
		if rand.Intn(10) == 5 || j == len(ids)-1 {
			if _, err := queue.UpdateSlice(slice2); err != nil {
				t.Fatalf("updateSlice: %v", err)
			}
			if !slice2.IsTailItem(term) {
				t.Fatal("slice2.isTailItem(term)")
			}
			slice2.Apply(bd2, j)
			dwdqAssertAllBetween(t, last2, j, bd2, ids)
			last2 = j + 1
		}
		if got := queue.NumGlobalTermDeletes(); got != len(uniqueValues) {
			t.Fatalf("expected %d, got %d", len(uniqueValues), got)
		}
	}
	if !dwdqSameSet(uniqueValues, dwdqKeySet(bd1.DeleteTerms().KeySet())) {
		t.Fatal("uniqueValues != bd1.deleteTerms.keySet()")
	}
	if !dwdqSameSet(uniqueValues, dwdqKeySet(bd2.DeleteTerms().KeySet())) {
		t.Fatal("uniqueValues != bd2.deleteTerms.keySet()")
	}
	frozenSet := dwdqFrozenSet(t, queue)
	if !dwdqSameSet(uniqueValues, frozenSet) {
		t.Fatal("uniqueValues != frozenSet")
	}
	if got := queue.NumGlobalTermDeletes(); got != 0 {
		t.Fatalf("num deletes must be 0 after freeze, got %d", got)
	}
}

func dwdqAssertAllBetween(t *testing.T, start, end int, deletes *index.BufferedUpdates, ids []int32) {
	t.Helper()
	for i := start; i <= end; i++ {
		if got := deletes.DeleteTerms().Get(*index.NewTerm("id", strconv.Itoa(int(ids[i])))); got != end {
			t.Fatalf("expected %d, got %d", end, got)
		}
	}
}

func TestDocumentsWriterDeleteQueueClear(t *testing.T) {
	queue := index.NewDocumentsWriterDeleteQueue(nil)
	if queue.AnyChanges() {
		t.Fatal("anyChanges")
	}
	queue.Clear()
	if queue.AnyChanges() {
		t.Fatal("anyChanges")
	}
	size := 200 + rand.Intn(500)
	for i := 0; i < size; i++ {
		term := index.NewTerm("id", strconv.Itoa(i))
		var err error
		if rand.Intn(10) == 0 {
			_, err = queue.AddDelete(search.NewTermQuery(term))
		} else {
			_, err = queue.AddDeleteTerms(*term)
		}
		if err != nil {
			t.Fatalf("addDelete: %v", err)
		}
		if !queue.AnyChanges() {
			t.Fatal("anyChanges")
		}
		if rand.Intn(10) == 0 {
			queue.Clear()
			if err := queue.TryApplyGlobalSlice(); err != nil {
				t.Fatalf("tryApplyGlobalSlice: %v", err)
			}
			if queue.AnyChanges() {
				t.Fatal("anyChanges")
			}
		}
	}
}

func TestDocumentsWriterDeleteQueueAnyChanges(t *testing.T) {
	queue := index.NewDocumentsWriterDeleteQueue(nil)
	size := 200 + rand.Intn(500)
	termsSinceFreeze := 0
	queriesSinceFreeze := 0
	for i := 0; i < size; i++ {
		term := index.NewTerm("id", strconv.Itoa(i))
		var err error
		if rand.Intn(10) == 0 {
			_, err = queue.AddDelete(search.NewTermQuery(term))
			queriesSinceFreeze++
		} else {
			_, err = queue.AddDeleteTerms(*term)
			termsSinceFreeze++
		}
		if err != nil {
			t.Fatalf("addDelete: %v", err)
		}
		if !queue.AnyChanges() {
			t.Fatal("anyChanges")
		}
		if rand.Intn(5) == 0 {
			freezeGlobalBuffer, err := queue.FreezeGlobalBuffer(nil)
			if err != nil {
				t.Fatalf("freezeGlobalBuffer: %v", err)
			}
			if got := freezeGlobalBuffer.DeleteTermsSize(); got != int64(termsSinceFreeze) {
				t.Fatalf("expected %d, got %d", termsSinceFreeze, got)
			}
			if got := freezeGlobalBuffer.DeleteQueriesLength(); got != queriesSinceFreeze {
				t.Fatalf("expected %d, got %d", queriesSinceFreeze, got)
			}
			queriesSinceFreeze = 0
			termsSinceFreeze = 0
			if queue.AnyChanges() {
				t.Fatal("anyChanges")
			}
		}
	}
}

func TestDocumentsWriterDeleteQueuePartiallyAppliedGlobalSlice(t *testing.T) {
	queue := index.NewDocumentsWriterDeleteQueue(nil)
	lock := queue.GlobalBufferLock()
	lock.Lock()
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if _, err := queue.AddDeleteTerms(*index.NewTerm("foo", "bar")); err != nil {
			t.Errorf("addDelete: %v", err)
		}
	}()
	wg.Wait()
	lock.Unlock()
	if !queue.AnyChanges() {
		t.Fatal("changes in del queue but not in slice yet")
	}
	if err := queue.TryApplyGlobalSlice(); err != nil {
		t.Fatalf("tryApplyGlobalSlice: %v", err)
	}
	if !queue.AnyChanges() {
		t.Fatal("changes in global buffer")
	}
	freezeGlobalBuffer, err := queue.FreezeGlobalBuffer(nil)
	if err != nil {
		t.Fatalf("freezeGlobalBuffer: %v", err)
	}
	if !freezeGlobalBuffer.Any() {
		t.Fatal("freezeGlobalBuffer.any()")
	}
	if got := freezeGlobalBuffer.DeleteTermsSize(); got != 1 {
		t.Fatalf("expected 1, got %d", got)
	}
	if queue.AnyChanges() {
		t.Fatal("all changes applied")
	}
}

func TestDocumentsWriterDeleteQueueStressDeleteQueue(t *testing.T) {
	queue := index.NewDocumentsWriterDeleteQueue(nil)
	uniqueValues := make(map[string]struct{})
	size := 10000 + rand.Intn(500)
	ids := make([]int32, size)
	for i := range ids {
		ids[i] = int32(rand.Uint32())
		uniqueValues[dwdqTermKey(*index.NewTerm("id", strconv.Itoa(int(ids[i]))))] = struct{}{}
	}
	latch := make(chan struct{})
	var idx atomic.Int32
	numThreads := 2 + rand.Intn(5)
	threads := make([]*dwdqUpdateThread, numThreads)
	var wg sync.WaitGroup
	for i := range threads {
		threads[i] = newDWDQUpdateThread(queue, &idx, ids, latch)
		wg.Add(1)
		go func(th *dwdqUpdateThread) {
			defer wg.Done()
			th.run(t)
		}(threads[i])
	}
	close(latch)
	wg.Wait()

	for _, updateThread := range threads {
		slice := updateThread.slice
		if _, err := queue.UpdateSlice(slice); err != nil {
			t.Fatalf("updateSlice: %v", err)
		}
		deletes := updateThread.deletes
		slice.Apply(deletes, index.BufferedUpdatesMaxInt)
		if !dwdqSameSet(uniqueValues, dwdqKeySet(deletes.DeleteTerms().KeySet())) {
			t.Fatal("uniqueValues != deletes.deleteTerms.keySet()")
		}
	}
	if err := queue.TryApplyGlobalSlice(); err != nil {
		t.Fatalf("tryApplyGlobalSlice: %v", err)
	}
	frozenSet := dwdqFrozenSet(t, queue)
	if got := queue.NumGlobalTermDeletes(); got != 0 {
		t.Fatalf("num deletes must be 0 after freeze, got %d", got)
	}
	if len(uniqueValues) != len(frozenSet) {
		t.Fatalf("expected %d, got %d", len(uniqueValues), len(frozenSet))
	}
	if !dwdqSameSet(uniqueValues, frozenSet) {
		t.Fatal("uniqueValues != frozenSet")
	}
}

func dwdqExpectAlreadyClosed(t *testing.T, err error) {
	t.Helper()
	var ace *store.AlreadyClosedException
	if !errors.As(err, &ace) {
		t.Fatalf("expected AlreadyClosedException, got %v", err)
	}
}

func TestDocumentsWriterDeleteQueueClose(t *testing.T) {
	{
		queue := index.NewDocumentsWriterDeleteQueue(nil)
		if !queue.IsOpen() {
			t.Fatal("isOpen")
		}
		if err := queue.Close(); err != nil {
			t.Fatalf("close: %v", err)
		}
		if rand.Intn(2) == 0 {
			if err := queue.Close(); err != nil { // double close
				t.Fatalf("close: %v", err)
			}
		}
		_, err := queue.AddDeleteTerms(*index.NewTerm("foo", "bar"))
		dwdqExpectAlreadyClosed(t, err)
		_, err = queue.FreezeGlobalBuffer(nil)
		dwdqExpectAlreadyClosed(t, err)
		_, err = queue.AddDelete(search.MatchNoDocsQueryInstance)
		dwdqExpectAlreadyClosed(t, err)
		one := int64(1)
		_, err = queue.AddDocValuesUpdates(index.NewNumericDocValuesUpdate(index.NewTerm("foo", "bar"), "foo", &one))
		dwdqExpectAlreadyClosed(t, err)
		_, err = queue.Add(nil)
		dwdqExpectAlreadyClosed(t, err)
		if queue.MaybeFreezeGlobalBuffer() != nil { // this is fine
			t.Fatal("maybeFreezeGlobalBuffer must return null")
		}
		if queue.IsOpen() {
			t.Fatal("isOpen")
		}
	}
	{
		queue := index.NewDocumentsWriterDeleteQueue(nil)
		if _, err := queue.AddDeleteTerms(*index.NewTerm("foo", "bar")); err != nil {
			t.Fatalf("addDelete: %v", err)
		}
		if err := queue.Close(); err == nil {
			t.Fatal("expected IllegalStateException")
		}
		if !queue.IsOpen() {
			t.Fatal("isOpen")
		}
		if err := queue.TryApplyGlobalSlice(); err != nil {
			t.Fatalf("tryApplyGlobalSlice: %v", err)
		}
		if _, err := queue.FreezeGlobalBuffer(nil); err != nil {
			t.Fatalf("freezeGlobalBuffer: %v", err)
		}
		if err := queue.Close(); err != nil {
			t.Fatalf("close: %v", err)
		}
		if queue.IsOpen() {
			t.Fatal("isOpen")
		}
	}
}

// dwdqUpdateThread ports the private static UpdateThread class.
type dwdqUpdateThread struct {
	queue   *index.DocumentsWriterDeleteQueue
	index   *atomic.Int32
	ids     []int32
	slice   *index.DeleteSlice
	deletes *index.BufferedUpdates
	latch   chan struct{}
}

func newDWDQUpdateThread(queue *index.DocumentsWriterDeleteQueue, idx *atomic.Int32, ids []int32, latch chan struct{}) *dwdqUpdateThread {
	return &dwdqUpdateThread{
		queue:   queue,
		index:   idx,
		ids:     ids,
		slice:   queue.NewSlice(),
		deletes: index.NewBufferedUpdates("deletes"),
		latch:   latch,
	}
}

func (u *dwdqUpdateThread) run(t *testing.T) {
	<-u.latch
	for {
		i := int(u.index.Add(1) - 1)
		if i >= len(u.ids) {
			return
		}
		term := index.NewTerm("id", strconv.Itoa(int(u.ids[i])))
		termNode := index.NewTermNode(*term)
		if _, err := u.queue.AddWithSlice(termNode, u.slice); err != nil {
			t.Errorf("add: %v", err)
			return
		}
		if !u.slice.IsTail(termNode) {
			t.Errorf("slice.isTail(termNode)")
			return
		}
		u.slice.Apply(u.deletes, index.BufferedUpdatesMaxInt)
	}
}
