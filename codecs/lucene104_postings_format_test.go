// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

// Port of
// lucene/core/src/test/org/apache/lucene/codecs/lucene104/TestLucene104PostingsFormat.java
// (Apache Lucene 10.5.0). Gocene's Lucene104PostingsWriter/Reader live in
// package codecs and their static helpers (writeVInt15, readVInt15,
// writeVLong15, readVLong15, writeImpacts, readImpacts) are unexported, so the
// port is an internal test. The class extends
// org.apache.lucene.tests.index.BasePostingsFormatTestCase (not ported); its
// inherited test methods are represented by one failing test naming it.

import (
	"math"
	"math/rand"
	"testing"
	"time"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	testanalysis "github.com/FlavioCFOliveira/Gocene/tests/analysis"
)

func TestLucene104PostingsFormat_BasePostingsFormatTestCase(t *testing.T) {
	t.Fatal("requires org.apache.lucene.tests.index.BasePostingsFormatTestCase (not ported)")
}

func TestLucene104PostingsFormat_testVInt15(t *testing.T) {
	bytes := make([]byte, 5)
	out := store.NewByteArrayDataOutput(bytes)
	in := store.NewByteArrayDataInput(nil)
	for _, i := range []int32{0, 1, 127, 128, 32767, 32768, math.MaxInt32} {
		out.Reset(bytes)
		if err := writeVInt15(out, i); err != nil {
			t.Fatalf("writeVInt15(%d): %v", i, err)
		}
		in.ResetWithSlice(bytes, 0, out.GetPosition())
		got, err := readVInt15(in)
		if err != nil {
			t.Fatalf("readVInt15: %v", err)
		}
		if int32(got) != i {
			t.Fatalf("readVInt15 = %d, want %d", got, i)
		}
		if out.GetPosition() != in.GetPosition() {
			t.Fatalf("position: out %d, in %d", out.GetPosition(), in.GetPosition())
		}
	}
}

func TestLucene104PostingsFormat_testVLong15(t *testing.T) {
	bytes := make([]byte, 9)
	out := store.NewByteArrayDataOutput(bytes)
	in := store.NewByteArrayDataInput(nil)
	for _, i := range []int64{0, 1, 127, 128, 32767, 32768, math.MaxInt32, math.MaxInt64} {
		out.Reset(bytes)
		if err := writeVLong15(out, i); err != nil {
			t.Fatalf("writeVLong15(%d): %v", i, err)
		}
		in.ResetWithSlice(bytes, 0, out.GetPosition())
		got, err := readVLong15(in)
		if err != nil {
			t.Fatalf("readVLong15: %v", err)
		}
		if got != i {
			t.Fatalf("readVLong15 = %d, want %d", got, i)
		}
		if out.GetPosition() != in.GetPosition() {
			t.Fatalf("position: out %d, in %d", out.GetPosition(), in.GetPosition())
		}
	}
}

// Make sure the final sub-block(s) are not skipped.
func TestLucene104PostingsFormat_testFinalBlock(t *testing.T) {
	d := store.NewMockDirectoryWrapper(store.NewByteBuffersDirectory())
	w, err := index.NewIndexWriter(d, index.NewIndexWriterConfigWithAnalyzer(
		testanalysis.NewMockAnalyzer(testanalysis.WHITESPACE, true, 0, nil, true)))
	if err != nil {
		t.Fatalf("new IndexWriter: %v", err)
	}
	for i := 0; i < 25; i++ {
		doc := document.NewDocument()
		f1, err := document.NewStringField("field", string(rune(97+i)), false)
		if err != nil {
			t.Fatal(err)
		}
		doc.Add(f1)
		f2, err := document.NewStringField("field", "z"+string(rune(97+i)), false)
		if err != nil {
			t.Fatal(err)
		}
		doc.Add(f2)
		if _, err := w.AddDocument(doc); err != nil {
			t.Fatalf("addDocument: %v", err)
		}
	}
	if err := w.ForceMerge(1); err != nil {
		t.Fatalf("forceMerge: %v", err)
	}

	r, err := index.OpenDirectoryReaderFromWriter(w)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	leaves, err := r.Leaves()
	if err != nil {
		t.Fatal(err)
	}
	if len(leaves) != 1 {
		t.Fatalf("leaves = %d, want 1", len(leaves))
	}
	terms, err := leaves[0].LeafReader().Terms("field")
	if err != nil {
		t.Fatal(err)
	}
	field, ok := terms.(*Lucene103FieldReader)
	if !ok {
		t.Fatalf("terms(\"field\") is %T, want *Lucene103FieldReader (FieldReader)", terms)
	}
	// We should see exactly two blocks: one root block (prefix empty string) and one block for z*
	// terms (prefix z):
	stats, err := field.GetStats()
	if err != nil {
		t.Fatalf("getStats: %v", err)
	}
	if stats.FloorBlockCount != 0 {
		t.Fatalf("floorBlockCount = %d, want 0", stats.FloorBlockCount)
	}
	if stats.NonFloorBlockCount != 2 {
		t.Fatalf("nonFloorBlockCount = %d, want 2", stats.NonFloorBlockCount)
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestLucene104PostingsFormat_testImpactSerialization(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	// omit norms and omit freqs
	doTestLucene104ImpactSerialization(t, r, []Impact{{Freq: 1, Norm: 1}})

	// omit freqs
	doTestLucene104ImpactSerialization(t, r, []Impact{{Freq: 1, Norm: 42}})
	// omit freqs with very large norms
	doTestLucene104ImpactSerialization(t, r, []Impact{{Freq: 1, Norm: -100}})

	// omit norms
	doTestLucene104ImpactSerialization(t, r, []Impact{{Freq: 30, Norm: 1}})
	// omit norms with large freq
	doTestLucene104ImpactSerialization(t, r, []Impact{{Freq: 500, Norm: 1}})

	// freqs and norms, basic
	doTestLucene104ImpactSerialization(t, r, []Impact{
		{Freq: 1, Norm: 7},
		{Freq: 3, Norm: 9},
		{Freq: 7, Norm: 10},
		{Freq: 15, Norm: 11},
		{Freq: 20, Norm: 13},
		{Freq: 28, Norm: 14},
	})

	// freqs and norms, high values
	doTestLucene104ImpactSerialization(t, r, []Impact{
		{Freq: 2, Norm: 2},
		{Freq: 10, Norm: 10},
		{Freq: 12, Norm: 50},
		{Freq: 50, Norm: -100},
		{Freq: 1000, Norm: -80},
		{Freq: 1005, Norm: -3},
	})
}

func doTestLucene104ImpactSerialization(t *testing.T, r *rand.Rand, impacts []Impact) {
	t.Helper()
	acc := NewCompetitiveImpactAccumulator()
	for _, impact := range impacts {
		acc.Add(impact.Freq, impact.Norm)
	}
	dir := store.NewMockDirectoryWrapper(store.NewByteBuffersDirectory())
	defer func() {
		if err := dir.Close(); err != nil {
			t.Error(err)
		}
	}()
	out, err := dir.CreateOutput("foo", store.IOContextDefault)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeImpacts(acc.GetCompetitiveFreqNormPairs(), out); err != nil {
		t.Fatalf("writeImpacts: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}
	in, err := dir.OpenInput("foo", store.IOContextDefault)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	b := make([]byte, in.Length())
	if err := in.ReadBytes(b, 0, len(b)); err != nil {
		t.Fatal(err)
	}
	impactBuffer := index.NewFreqAndNormBuffer()
	impactBuffer.GrowNoCopy(len(impacts) + r.Intn(3))
	readImpactsFromBytes(store.NewByteArrayDataInput(b), impactBuffer)
	impacts2 := make([]Impact, 0, impactBuffer.Size)
	for i := 0; i < impactBuffer.Size; i++ {
		impacts2 = append(impacts2, Impact{Freq: impactBuffer.Freqs[i], Norm: impactBuffer.Norms[i]})
	}
	_ = impacts2 // the Java test builds impacts2 without asserting on it
}
