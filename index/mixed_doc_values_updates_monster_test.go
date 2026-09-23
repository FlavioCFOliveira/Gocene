// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build gocene_monsters

// The @Nightly test port of
// lucene/core/src/test/org/apache/lucene/index/TestMixedDocValuesUpdates.java
// (Apache Lucene 10.5.0), built only with the gocene_monsters tag as Lucene
// runs it only when nightly tests are enabled.

package index_test

import (
	"math/rand"
	"sort"
	"strconv"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

func TestMixedDocValuesUpdatesTonsOfUpdates(t *testing.T) {
	// LUCENE-5248: make sure that when there are many updates, we don't use too much RAM
	dir := newDirectory()
	r := rand.New(rand.NewSource(rand.Int63()))
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetRAMBufferSizeMB(index.DefaultRAMBufferSizeMB)
	conf.SetMaxBufferedDocs(index.DisableAutoFlush) // don't flush by doc
	writer := mustNewIndexWriter(t, dir, conf)

	// test data: lots of documents (few 10Ks) and lots of update terms (few hundreds)
	numDocs := atLeast(20000)
	numBinaryFields := atLeast(5)
	numTerms := nextInt(10, 100) // terms should affect many docs
	updateTermSet := map[string]bool{}
	for len(updateTermSet) < numTerms {
		updateTermSet[util.RandomSimpleString(r, 0, 10)] = true
	}
	updateTerms := make([]string, 0, len(updateTermSet))
	for term := range updateTermSet {
		updateTerms = append(updateTerms, term)
	}
	sort.Strings(updateTerms)
	randomFrom := func() string { return updateTerms[rand.Intn(len(updateTerms))] }

	// build a large index with many BDV fields and update terms
	for i := 0; i < numDocs; i++ {
		doc := document.NewDocument()
		numUpdateTerms := nextInt(1, numTerms/10)
		for j := 0; j < numUpdateTerms; j++ {
			doc.Add(newStringField(t, "upd", randomFrom(), false))
		}
		for j := 0; j < numBinaryFields; j++ {
			val := javaNextInt()
			doc.Add(binaryDVField(t, "f"+strconv.Itoa(j), toBytes(val)))
			doc.Add(numericDVField(t, "cf"+strconv.Itoa(j), val*2))
		}
		mustAddDocument(t, writer, doc)
	}

	mustCommit(t, writer) // commit so there's something to apply to

	// set to flush every 2048 bytes (approximately every 12 updates), so we get
	// many flushes during binary updates
	writer.GetConfig().SetRAMBufferSizeMB(2048.0 / 1024 / 1024)
	numUpdates := atLeast(100)
	for i := 0; i < numUpdates; i++ {
		field := rand.Intn(numBinaryFields)
		updateTerm := index.NewTerm("upd", randomFrom())
		value := javaNextInt()
		mustUpdateDocValues(t, writer, updateTerm,
			binaryDVField(t, "f"+strconv.Itoa(field), toBytes(value)).Field,
			numericDVField(t, "cf"+strconv.Itoa(field), value*2).Field)
	}

	mustClose(t, writer)

	reader := mustOpenDirectoryReader(t, dir)
	leaves, err := reader.Leaves()
	if err != nil {
		t.Fatalf("leaves: %v", err)
	}
	for _, context := range leaves {
		for i := 0; i < numBinaryFields; i++ {
			lr := context.LeafReader()
			f, err := lr.GetBinaryDocValues("f" + strconv.Itoa(i))
			if err != nil {
				t.Fatalf("getBinaryDocValues: %v", err)
			}
			cf, err := lr.GetNumericDocValues("cf" + strconv.Itoa(i))
			if err != nil {
				t.Fatalf("getNumericDocValues: %v", err)
			}
			for j := 0; j < lr.MaxDoc(); j++ {
				if got, err := cf.NextDoc(); err != nil || got != j {
					t.Fatalf("cf.nextDoc: expected %d, got %d (%v)", j, got, err)
				}
				if got, err := f.NextDoc(); err != nil || got != j {
					t.Fatalf("f.nextDoc: expected %d, got %d (%v)", j, got, err)
				}
				cv, err := cf.LongValue()
				if err != nil {
					t.Fatalf("longValue: %v", err)
				}
				fv, err := binaryDVValue(f)
				if err != nil {
					t.Fatalf("getValue: %v", err)
				}
				if cv != fv*2 {
					t.Fatalf("field=f%d, doc=%d: %d != %d", i, j, cv, fv*2)
				}
			}
		}
	}
	mustClose(t, reader, dir)
}
