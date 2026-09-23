// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/index/TestMixedDocValuesUpdates.java
// (Apache Lucene 10.5.0). The @Nightly testTonsOfUpdates lives in
// mixed_doc_values_updates_monster_test.go.

package index_test

import (
	"math/rand"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// Missing production members the Java tests reach.
const (
	tryUpdateDocValueFieldsMissing = "org.apache.lucene.index.IndexWriter#tryUpdateDocValue(IndexReader, int, Field...) is not ported " +
		"(Gocene's TryUpdateDocValue takes a single field name and value)"
	numericDocValuesFieldNullMissing = "org.apache.lucene.document.NumericDocValuesField(String, Long) accepting a null value is not ported"
)

// binaryDVValue renders TestBinaryDocValuesUpdates.getValue(BinaryDocValues).
func binaryDVValue(bdv index.BinaryDocValues) (int64, error) {
	bytes, err := bdv.BinaryValue()
	if err != nil {
		return 0, err
	}
	return getValue(bytes), nil
}

func mustUpdateDocValues(t testing.TB, w *index.IndexWriter, term *index.Term, fields ...*document.Field) {
	t.Helper()
	if _, err := w.UpdateDocValues(term, fields); err != nil {
		t.Fatalf("updateDocValues: %v", err)
	}
}

func TestMixedDocValuesUpdatesManyReopensAndFields(t *testing.T) {
	dir := newDirectory()
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	lmp := newLogMergePolicy()
	lmp.SetMergeFactor(3) // merge often
	conf.SetMergePolicy(lmp)
	writer := mustNewIndexWriter(t, dir, conf)

	isNRT := rand.Intn(2) == 0
	var reader *index.DirectoryReader
	if isNRT {
		reader = openReaderFromWriter(t, writer)
	} else {
		mustCommit(t, writer)
		reader = mustOpenDirectoryReader(t, dir)
	}

	numFields := rand.Intn(4) + 3              // 3-7
	numNDVFields := rand.Intn(numFields/2) + 1 // 1-3
	fieldValues := make([]int64, numFields)
	for i := range fieldValues {
		fieldValues[i] = 1
	}

	numRounds := atLeast(15)
	docID := 0
	for i := 0; i < numRounds; i++ {
		numDocs := atLeast(5)
		for j := 0; j < numDocs; j++ {
			doc := document.NewDocument()
			doc.Add(newStringField(t, "id", "doc-"+strconv.Itoa(docID), false))
			doc.Add(newStringField(t, "key", "all", false)) // update key
			// add all fields with their current value
			for f := range fieldValues {
				if f < numNDVFields {
					doc.Add(numericDVField(t, "f"+strconv.Itoa(f), fieldValues[f]))
				} else {
					doc.Add(binaryDVField(t, "f"+strconv.Itoa(f), toBytes(fieldValues[f])))
				}
			}
			mustAddDocument(t, writer, doc)
			docID++
		}

		fieldIdx := rand.Intn(len(fieldValues))
		updateField := "f" + strconv.Itoa(fieldIdx)
		fieldValues[fieldIdx]++
		if fieldIdx < numNDVFields {
			if _, err := writer.UpdateNumericDocValue(index.NewTerm("key", "all"), updateField, fieldValues[fieldIdx]); err != nil {
				t.Fatalf("updateNumericDocValue: %v", err)
			}
		} else {
			if _, err := writer.UpdateBinaryDocValue(index.NewTerm("key", "all"), updateField, toBytes(fieldValues[fieldIdx])); err != nil {
				t.Fatalf("updateBinaryDocValue: %v", err)
			}
		}

		if rand.Float64() < 0.2 {
			deleteDoc := rand.Intn(docID) // might also delete an already deleted document, ok!
			mustDeleteTerm(t, writer, "id", "doc-"+strconv.Itoa(deleteDoc))
		}

		// verify reader
		if !isNRT {
			mustCommit(t, writer)
		}

		var newReader *index.DirectoryReader
		if isNRT {
			nr, err := index.OpenIfChangedFromWriter(reader, writer)
			if err != nil {
				t.Fatalf("openIfChanged: %v", err)
			}
			newReader = nr
		} else {
			newReader = openIfChanged(t, reader)
		}
		if newReader == nil {
			t.Fatal("assertNotNull(newReader)")
		}
		mustClose(t, reader)
		reader = newReader
		if !(reader.NumDocs() > 0) { // we delete at most one document per round
			t.Fatalf("assertTrue(reader.numDocs() > 0): %d", reader.NumDocs())
		}
		leaves, err := reader.Leaves()
		if err != nil {
			t.Fatalf("leaves: %v", err)
		}
		for _, context := range leaves {
			r := context.LeafReader()
			liveDocs := r.GetLiveDocs()
			for field := range fieldValues {
				f := "f" + strconv.Itoa(field)
				bdv, err := r.GetBinaryDocValues(f)
				if err != nil {
					t.Fatalf("getBinaryDocValues: %v", err)
				}
				ndv, err := r.GetNumericDocValues(f)
				if err != nil {
					t.Fatalf("getNumericDocValues: %v", err)
				}
				if field < numNDVFields {
					if ndv == nil || bdv != nil {
						t.Fatalf("field %s: ndv=%v bdv=%v", f, ndv, bdv)
					}
				} else if ndv != nil || bdv == nil {
					t.Fatalf("field %s: ndv=%v bdv=%v", f, ndv, bdv)
				}
				maxDoc := r.MaxDoc()
				for doc := 0; doc < maxDoc; doc++ {
					if liveDocs == nil || liveDocs.Get(doc) {
						if field < numNDVFields {
							if got, err := ndv.Advance(doc); err != nil || got != doc {
								t.Fatalf("advance(%d): %d (%v)", doc, got, err)
							}
							if v, err := ndv.LongValue(); err != nil || v != fieldValues[field] {
								t.Fatalf("invalid numeric value for doc=%d, field=%s: expected %d, got %d (%v)", doc, f, fieldValues[field], v, err)
							}
						} else {
							if got, err := bdv.Advance(doc); err != nil || got != doc {
								t.Fatalf("advance(%d): %d (%v)", doc, got, err)
							}
							if v, err := binaryDVValue(bdv); err != nil || v != fieldValues[field] {
								t.Fatalf("invalid binary value for doc=%d, field=%s: expected %d, got %d (%v)", doc, f, fieldValues[field], v, err)
							}
						}
					}
				}
			}
		}
	}

	mustClose(t, writer, reader, dir)
}

func updKeyTerm(group float64) *index.Term {
	switch {
	case group < 0.1:
		return index.NewTerm("updKey", "g0")
	case group < 0.5:
		return index.NewTerm("updKey", "g1")
	case group < 0.8:
		return index.NewTerm("updKey", "g2")
	default:
		return index.NewTerm("updKey", "g3")
	}
}

// assertControlMatchesBinary renders the final verification loop shared by
// testStressMultiThreading: the numeric control cf is always twice the
// binary field f.
func assertControlMatchesBinary(t testing.TB, r index.LeafReader, i int, withLiveDocs bool) {
	t.Helper()
	bdv, err := r.GetBinaryDocValues("f" + strconv.Itoa(i))
	if err != nil {
		t.Fatalf("getBinaryDocValues: %v", err)
	}
	control, err := r.GetNumericDocValues("cf" + strconv.Itoa(i))
	if err != nil {
		t.Fatalf("getNumericDocValues: %v", err)
	}
	var liveDocs util.Bits
	if withLiveDocs {
		liveDocs = r.GetLiveDocs()
	}
	for j := 0; j < r.MaxDoc(); j++ {
		if liveDocs == nil || liveDocs.Get(j) {
			if got, err := control.Advance(j); err != nil || got != j {
				t.Fatalf("control.advance(%d): %d (%v)", j, got, err)
			}
			ctrlValue, err := control.LongValue()
			if err != nil {
				t.Fatalf("longValue: %v", err)
			}
			if got, err := bdv.Advance(j); err != nil || got != j {
				t.Fatalf("bdv.advance(%d): %d (%v)", j, got, err)
			}
			bdvValue, err := binaryDVValue(bdv)
			if err != nil {
				t.Fatalf("getValue: %v", err)
			}
			if ctrlValue != bdvValue*2 {
				t.Fatalf("doc %d field %d: control %d != 2*bdv %d", j, i, ctrlValue, bdvValue*2)
			}
		}
	}
}

func TestMixedDocValuesUpdatesStressMultiThreading(t *testing.T) {
	dir := newDirectory()
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	writer := mustNewIndexWriter(t, dir, conf)

	// create index
	numFields := nextInt(2, 4)
	numThreads := nextInt(3, 6)
	numDocs := atLeast(2000)
	for i := 0; i < numDocs; i++ {
		doc := document.NewDocument()
		doc.Add(newStringField(t, "id", "doc"+strconv.Itoa(i), false))
		doc.Add(newStringField(t, "updKey", updKeyTerm(rand.Float64()).Text(), false))
		for j := 0; j < numFields; j++ {
			value := javaNextInt()
			doc.Add(binaryDVField(t, "f"+strconv.Itoa(j), toBytes(value)))
			doc.Add(numericDVField(t, "cf"+strconv.Itoa(j), value*2)) // control, always updated to f * 2
		}
		mustAddDocument(t, writer, doc)
	}

	var numUpdates atomic.Int64
	numUpdates.Store(int64(atLeast(100)))

	// same thread updates a field as well as reopens
	var wg sync.WaitGroup
	for i := 0; i < numThreads; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var reader *index.DirectoryReader
			defer func() {
				if reader != nil {
					if err := reader.Close(); err != nil {
						t.Errorf("close: %v", err)
					}
				}
			}()
			for numUpdates.Add(-1) >= 0 {
				term := updKeyTerm(rand.Float64())
				field := rand.Intn(numFields)
				f := "f" + strconv.Itoa(field)
				cf := "cf" + strconv.Itoa(field)
				updValue := javaNextInt()
				bf, err := document.NewBinaryDocValuesField(f, toBytes(updValue))
				if err != nil {
					t.Errorf("BinaryDocValuesField: %v", err)
					return
				}
				nf, err := document.NewNumericDocValuesField(cf, updValue*2)
				if err != nil {
					t.Errorf("NumericDocValuesField: %v", err)
					return
				}
				if _, err := writer.UpdateDocValues(term, []*document.Field{bf.Field, nf.Field}); err != nil {
					t.Errorf("updateDocValues: %v", err)
					return
				}

				if rand.Float64() < 0.2 {
					// delete a random document
					doc := rand.Intn(numDocs)
					if _, err := writer.DeleteDocuments([]index.Term{*index.NewTerm("id", "doc"+strconv.Itoa(doc))}); err != nil {
						t.Errorf("deleteDocuments: %v", err)
						return
					}
				}

				if rand.Float64() < 0.05 { // commit every 20 updates on average
					if _, err := writer.Commit(); err != nil {
						t.Errorf("commit: %v", err)
						return
					}
				}

				if rand.Float64() < 0.1 { // reopen NRT reader (apply updates), on average once every 10 updates
					if reader == nil {
						r, err := index.OpenDirectoryReaderFromWriter(writer)
						if err != nil {
							t.Errorf("DirectoryReader.open(writer): %v", err)
							return
						}
						reader = r
					} else {
						r2, err := index.OpenIfChangedFromWriter(reader, writer)
						if err != nil {
							t.Errorf("openIfChanged: %v", err)
							return
						}
						if r2 != nil {
							if err := reader.Close(); err != nil {
								t.Errorf("close: %v", err)
								return
							}
							reader = r2
						}
					}
				}
			}
		}()
	}

	wg.Wait()
	if t.Failed() {
		t.FailNow()
	}
	mustClose(t, writer)

	reader := mustOpenDirectoryReader(t, dir)
	leaves, err := reader.Leaves()
	if err != nil {
		t.Fatalf("leaves: %v", err)
	}
	for _, context := range leaves {
		for i := 0; i < numFields; i++ {
			assertControlMatchesBinary(t, context.LeafReader(), i, true)
		}
	}
	mustClose(t, reader, dir)
}

func TestMixedDocValuesUpdatesUpdateDifferentDocsInDifferentGens(t *testing.T) {
	// update same document multiple times across generations
	dir := newDirectory()
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetMaxBufferedDocs(4)
	writer := mustNewIndexWriter(t, dir, conf)
	defer mustClose(t, writer, dir)
	numDocs := atLeast(10)
	for i := 0; i < numDocs; i++ {
		doc := document.NewDocument()
		doc.Add(newStringField(t, "id", "doc"+strconv.Itoa(i), false))
		value := javaNextInt()
		doc.Add(binaryDVField(t, "f", toBytes(value)))
		doc.Add(numericDVField(t, "cf", value*2))
		mustAddDocument(t, writer, doc)
	}

	numGens := atLeast(5)
	for i := 0; i < numGens; i++ {
		doc := rand.Intn(numDocs)
		term := index.NewTerm("id", "doc"+strconv.Itoa(doc))
		value := javaNextLong()
		if rand.Intn(2) == 0 {
			t.Fatal(tryUpdateDocValueFieldsMissing + " (doUpdate)")
		} else {
			mustUpdateDocValues(t, writer, term,
				binaryDVField(t, "f", toBytes(value)).Field,
				numericDVField(t, "cf", value*2).Field)
		}

		reader := openReaderFromWriter(t, writer)
		leaves, err := reader.Leaves()
		if err != nil {
			t.Fatalf("leaves: %v", err)
		}
		for _, context := range leaves {
			r := context.LeafReader()
			fbdv, err := r.GetBinaryDocValues("f")
			if err != nil {
				t.Fatalf("getBinaryDocValues: %v", err)
			}
			cfndv, err := r.GetNumericDocValues("cf")
			if err != nil {
				t.Fatalf("getNumericDocValues: %v", err)
			}
			for j := 0; j < r.MaxDoc(); j++ {
				if got, err := cfndv.NextDoc(); err != nil || got != j {
					t.Fatalf("cfndv.nextDoc: expected %d, got %d (%v)", j, got, err)
				}
				if got, err := fbdv.NextDoc(); err != nil || got != j {
					t.Fatalf("fbdv.nextDoc: expected %d, got %d (%v)", j, got, err)
				}
				cv, err := cfndv.LongValue()
				if err != nil {
					t.Fatalf("longValue: %v", err)
				}
				bv, err := binaryDVValue(fbdv)
				if err != nil {
					t.Fatalf("getValue: %v", err)
				}
				if cv != bv*2 {
					t.Fatalf("doc %d: cf %d != 2*f %d", j, cv, bv*2)
				}
			}
		}
		mustClose(t, reader)
	}
}

func TestMixedDocValuesUpdatesTryUpdateDocValues(t *testing.T) {
	dir := newDirectory()
	conf := newIndexWriterConfig()
	writer := mustNewIndexWriter(t, dir, conf)
	defer mustClose(t, writer, dir)
	numDocs := 1 + rand.Intn(128)
	for i := 0; i < numDocs; i++ {
		doc := document.NewDocument()
		doc.Add(newStringField(t, "id", strconv.Itoa(i), true))
		doc.Add(numericDVField(t, "numericId", int64(i)))
		doc.Add(binaryDVField(t, "binaryId", []byte{byte(i)}))
		mustAddDocument(t, writer, doc)
		if rand.Intn(2) == 0 {
			mustFlush(t, writer)
		}
	}
	t.Fatal(tryUpdateDocValueFieldsMissing + " (doUpdate)")
}

func TestMixedDocValuesUpdatesTryUpdateMultiThreaded(t *testing.T) {
	dir := newDirectory()
	conf := newIndexWriterConfig()
	writer := mustNewIndexWriter(t, dir, conf)
	defer mustClose(t, writer, dir)
	locks := make([]sync.Mutex, 25+rand.Intn(50))
	values := make([]*int64, len(locks))

	for i := range locks {
		doc := document.NewDocument()
		v := javaNextLong()
		values[i] = &v
		doc.Add(newStringField(t, "id", strconv.Itoa(i), false))
		doc.Add(numericDVField(t, "value", v))
		mustAddDocument(t, writer, doc)
	}

	numThreads := 2
	if testNightly {
		numThreads = 2 + rand.Intn(3)
	}
	barrier := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < numThreads; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-barrier
			for doc := 0; doc < 1000; doc++ {
				docID := rand.Intn(len(locks))
				locks[docID].Lock()
				if rarely() { // sometimes reset it
					locks[docID].Unlock()
					t.Error(numericDocValuesFieldNullMissing)
					return
				}
				value := javaNextLong()
				if rand.Intn(2) == 0 {
					nf, err := document.NewNumericDocValuesField("value", value)
					if err == nil {
						_, err = writer.UpdateDocValues(index.NewTerm("id", strconv.Itoa(docID)), []*document.Field{nf.Field})
					}
					if err != nil {
						locks[docID].Unlock()
						t.Errorf("updateDocValues: %v", err)
						return
					}
				} else {
					locks[docID].Unlock()
					t.Error(tryUpdateDocValueFieldsMissing + " (doUpdate)")
					return
				}
				values[docID] = &value
				locks[docID].Unlock()
				if rarely() {
					if err := writer.Flush(); err != nil {
						t.Errorf("flush: %v", err)
						return
					}
				}
			}
		}()
	}

	close(barrier)
	wg.Wait()
	if t.Failed() {
		t.FailNow()
	}
	reader := openReaderFromWriter(t, writer)
	defer mustClose(t, reader)
	leaves, err := reader.Leaves()
	if err != nil {
		t.Fatalf("leaves: %v", err)
	}
	for i := range locks {
		locks[i].Lock()
		value := values[i]
		topDocs, err := search.NewIndexSearcher(reader).Search(search.NewTermQuery(index.NewTerm("id", strconv.Itoa(i))), 10)
		if err != nil {
			locks[i].Unlock()
			t.Fatalf("search: %v", err)
		}
		if topDocs.TotalHits.Value != 1 {
			locks[i].Unlock()
			t.Fatalf("totalHits: expected 1, got %d", topDocs.TotalHits.Value)
		}
		docID := topDocs.ScoreDocs[0].Doc
		subIndex := index.ReaderUtilSubIndexLeaves(docID, leaves)
		leafReader := leaves[subIndex].LeafReader()
		docID -= leaves[subIndex].DocBase
		numericDocValues, err := leafReader.GetNumericDocValues("value")
		if err != nil {
			locks[i].Unlock()
			t.Fatalf("getNumericDocValues: %v", err)
		}
		found, err := numericDocValues.AdvanceExact(docID)
		if err != nil || !found {
			locks[i].Unlock()
			t.Fatalf("docID: %d advanceExact: %v (%v)", docID, found, err)
		}
		if got, err := numericDocValues.LongValue(); err != nil || got != *value {
			locks[i].Unlock()
			t.Fatalf("value: expected %d, got %d (%v)", *value, got, err)
		}
		locks[i].Unlock()
	}
}

func TestMixedDocValuesUpdatesResetValue(t *testing.T) {
	dir := newDirectory()
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	writer := mustNewIndexWriter(t, dir, conf)
	doc := document.NewDocument()
	doc.Add(newStringField(t, "id", "0", false))
	doc.Add(numericDVField(t, "val", 5))
	doc.Add(binaryDVField(t, "val-bin", []byte{5}))
	mustAddDocument(t, writer, doc)

	if rand.Intn(2) == 0 {
		mustCommit(t, writer)
	}
	checkReader := func(expectBinary bool) {
		t.Helper()
		reader := openReaderFromWriter(t, writer)
		defer mustClose(t, reader)
		assertLeafCount(t, 1, reader)
		leaves, _ := reader.Leaves()
		r := leaves[0].LeafReader()
		ndv, err := r.GetNumericDocValues("val")
		if err != nil {
			t.Fatalf("getNumericDocValues: %v", err)
		}
		if got, err := ndv.NextDoc(); err != nil || got != 0 {
			t.Fatalf("ndv.nextDoc: %d (%v)", got, err)
		}
		if v, err := ndv.LongValue(); err != nil || v != 5 {
			t.Fatalf("ndv.longValue: %d (%v)", v, err)
		}
		if got, err := ndv.NextDoc(); err != nil || got != spi.NO_MORE_DOCS {
			t.Fatalf("ndv.nextDoc: %d (%v)", got, err)
		}

		bdv, err := r.GetBinaryDocValues("val-bin")
		if err != nil {
			t.Fatalf("getBinaryDocValues: %v", err)
		}
		if expectBinary {
			if got, err := bdv.NextDoc(); err != nil || got != 0 {
				t.Fatalf("bdv.nextDoc: %d (%v)", got, err)
			}
			if v, err := bdv.BinaryValue(); err != nil || string(v) != string([]byte{5}) {
				t.Fatalf("bdv.binaryValue: %v (%v)", v, err)
			}
		}
		if got, err := bdv.NextDoc(); err != nil || got != spi.NO_MORE_DOCS {
			t.Fatalf("bdv.nextDoc: expected NO_MORE_DOCS, got %d (%v)", got, err)
		}
	}
	checkReader(true)

	nullBinary, err := document.NewBinaryDocValuesField("val-bin", nil)
	if err != nil {
		t.Fatalf("new BinaryDocValuesField(\"val-bin\", null): %v", err)
	}
	mustUpdateDocValues(t, writer, index.NewTerm("id", "0"), nullBinary.Field)
	checkReader(false)
	mustClose(t, writer, dir)
}

func TestMixedDocValuesUpdatesResetValueMultipleDocs(t *testing.T) {
	dir := newDirectory()
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	writer := mustNewIndexWriter(t, dir, conf)
	defer mustClose(t, writer, dir)
	numDocs := 10 + rand.Intn(50)
	currentSeqID := 0
	seqID := []int{-1, -1, -1, -1, -1}
	for i := 0; i < numDocs; i++ {
		doc := document.NewDocument()
		id := rand.Intn(5)
		seqID[id] = currentSeqID
		doc.Add(newStringField(t, "id", strconv.Itoa(id), true))
		doc.Add(numericDVField(t, "seqID", int64(currentSeqID)))
		currentSeqID++
		doc.Add(numericDVField(t, "is_live", 1))
		if i > 0 {
			t.Fatal(numericDocValuesFieldNullMissing)
		}
		mustAddDocument(t, writer, doc)
		if rand.Intn(2) == 0 {
			mustFlush(t, writer)
		}
	}
}

func TestMixedDocValuesUpdatesUpdateNotExistingFieldDV(t *testing.T) {
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	dir := newDirectory()
	writer := mustNewIndexWriter(t, dir, conf)
	defer mustClose(t, writer, dir)
	doc := document.NewDocument()
	doc.Add(newStringField(t, "id", "1", true))
	doc.Add(numericDVField(t, "test", 1))
	mustAddDocument(t, writer, doc)
	if rand.Intn(2) == 0 {
		mustCommit(t, writer)
	}
	mustUpdateDocValues(t, writer, index.NewTerm("id", "1"), numericDVField(t, "not_existing", 1).Field)

	doc1 := document.NewDocument()
	doc1.Add(newStringField(t, "id", "2", true))
	doc1.Add(binaryDVField(t, "not_existing", []byte{}))
	_, err := writer.AddDocument(doc1)
	if err == nil {
		t.Fatal("expected IllegalArgumentException from addDocument")
	}
	if want := "cannot change field \"not_existing\" from doc values type=NUMERIC to inconsistent doc values type=BINARY"; err.Error() != want {
		t.Fatalf("message: expected %q, got %q", want, err.Error())
	}

	_, err = writer.UpdateDocValues(index.NewTerm("id", "1"), []*document.Field{binaryDVField(t, "not_existing", []byte{}).Field})
	if err == nil {
		t.Fatal("expected IllegalArgumentException from updateDocValues")
	}
	if want := "Can't update [BINARY] doc values; the field [not_existing] has inconsistent doc values' type of [NUMERIC]."; err.Error() != want {
		t.Fatalf("message: expected %q, got %q", want, err.Error())
	}
}

func TestMixedDocValuesUpdatesUpdateFieldWithNoPreviousDocValuesThrowsError(t *testing.T) {
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	dir := newDirectory()
	writer := mustNewIndexWriter(t, dir, conf)
	defer mustClose(t, writer, dir)
	doc := document.NewDocument()
	doc.Add(newStringField(t, "id", "1", true))
	mustAddDocument(t, writer, doc)
	if rand.Intn(2) == 0 {
		reader := openReaderFromWriter(t, writer)
		leaves, err := reader.Leaves()
		if err != nil {
			t.Fatalf("leaves: %v", err)
		}
		id, err := leaves[0].LeafReader().GetNumericDocValues("id")
		if err != nil {
			t.Fatalf("getNumericDocValues: %v", err)
		}
		if id != nil {
			t.Fatalf("assertNull(id): got %v", id)
		}
		mustClose(t, reader)
	} else if rand.Intn(2) == 0 {
		mustCommit(t, writer)
	}
	_, err := writer.UpdateDocValues(index.NewTerm("id", "1"), []*document.Field{numericDVField(t, "id", 1).Field})
	if err == nil {
		t.Fatal("expected IllegalArgumentException from updateDocValues")
	}
	if want := "Can't update [NUMERIC] doc values; the field [id] has inconsistent doc values' type of [NONE]."; err.Error() != want {
		t.Fatalf("message: expected %q, got %q", want, err.Error())
	}
}

func TestMixedDocValuesUpdatesLongRunValuesReset(t *testing.T) {
	dir := newDirectory()
	writer := mustNewIndexWriter(t, dir, index.NewIndexWriterConfig())
	defer mustClose(t, writer, dir)
	numDocs := atLeast(65536)
	for i := 0; i < numDocs; i++ {
		doc := document.NewDocument()
		doc.Add(newStringField(t, "id", strconv.Itoa(i), false))
		if i == 0 || i == numDocs-1 || rand.Intn(2) == 0 {
			doc.Add(numericDVField(t, "numeric", 1))
		}
		mustAddDocument(t, writer, doc)
	}
	mustFlush(t, writer)

	t.Fatal(numericDocValuesFieldNullMissing)
}
