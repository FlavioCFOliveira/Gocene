// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Shared helpers for the block-join end-to-end tests ported from
// org.apache.lucene.search.join.TestBlockJoin (and the *KnnVectorQuery test
// cases). They build a parent/child block index with IndexWriter.AddDocuments,
// reopen it through OpenDirectoryReader, and drive ToParent/ToChild block-join
// queries via IndexSearcher.
//
// Deviations from the Lucene reference, applied uniformly here:
//   - Lucene's makeJob/makeQualification add an IntPoint("year", y) plus a
//     StoredField("year", y). Gocene's IntPoint range/set queries do not yet
//     match end-to-end through IndexWriter+OpenDirectoryReader (verified: a
//     PointRangeQuery over an indexed IntPoint returns zero hits; this is a
//     points/BKD reader gap, rmp #4755-adjacent, not a block-join defect). The
//     "year" value is therefore stored as a StringField so the year-range child
//     clauses can be expressed as the equivalent TermQuery set over the small,
//     deterministic test corpora. Where a test's result set depends only on the
//     skill/term clause this is an exact behavioural match; where it depends on
//     the year range the helper documents the substitution at the call site.

package join

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// newBlockWriter creates an on-disk directory and an IndexWriter over it,
// registering cleanup of both on the test. A SimpleFSDirectory is used (rather
// than ByteBuffersDirectory) because it round-trips the vector and points wire
// formats faithfully; see the ByteBuffersDirectory endianness caveats noted
// elsewhere in the codebase.
func newBlockWriter(t *testing.T) (store.Directory, *index.IndexWriter) {
	t.Helper()
	dir, err := store.NewSimpleFSDirectory(t.TempDir())
	if err != nil {
		t.Fatalf("NewSimpleFSDirectory: %v", err)
	}
	cfg := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	w, err := index.NewIndexWriter(dir, cfg)
	if err != nil {
		_ = dir.Close()
		t.Fatalf("NewIndexWriter: %v", err)
	}
	t.Cleanup(func() { _ = dir.Close() })
	return dir, w
}

// addBlock writes one parent/child block (children first, parent last) via
// IndexWriter.AddDocuments, which keeps the block contiguous in a single
// segment as block joins require.
func addBlock(t *testing.T, w *index.IndexWriter, docs ...index.Document) {
	t.Helper()
	if err := w.AddDocuments(docs); err != nil {
		t.Fatalf("AddDocuments: %v", err)
	}
}

// commitAndOpen commits and closes the writer, then opens a DirectoryReader and
// a searcher over the directory, registering reader cleanup on the test.
func commitAndOpen(t *testing.T, dir store.Directory, w *index.IndexWriter) (*index.DirectoryReader, *search.IndexSearcher) {
	t.Helper()
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close writer: %v", err)
	}
	r, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	t.Cleanup(func() { _ = r.Close() })
	return r, search.NewIndexSearcher(r)
}

// newQueryBitSetParents builds a QueryBitSetProducer over a TermQuery, the
// canonical parents filter used throughout TestBlockJoin.
func newQueryBitSetParents(field, value string) BitSetProducer {
	return NewQueryBitSetProducer(search.NewTermQuery(index.NewTerm(field, value)))
}

// firstLeafScorer rewrites q, creates a Weight (COMPLETE scores), and returns
// the Scorer for the first leaf, mirroring the Lucene idiom
// s.createWeight(s.rewrite(q), COMPLETE, 1).scorer(leaves().get(0)).
func firstLeafScorer(t *testing.T, searcher *search.IndexSearcher, reader *index.DirectoryReader, q search.Query) search.Scorer {
	t.Helper()
	rewritten, err := q.Rewrite(reader)
	if err != nil {
		t.Fatalf("Rewrite: %v", err)
	}
	weight, err := rewritten.CreateWeight(searcher, true, 1.0)
	if err != nil {
		t.Fatalf("CreateWeight: %v", err)
	}
	leaves, err := reader.Leaves()
	if err != nil {
		t.Fatalf("Leaves: %v", err)
	}
	if len(leaves) == 0 {
		t.Fatal("reader has no leaves")
	}
	scorer, err := weight.Scorer(leaves[0])
	if err != nil {
		t.Fatalf("Scorer: %v", err)
	}
	return scorer
}

// storedString returns the stored string value of field on doc, or "" if
// absent.
func storedString(doc *document.Document, field string) string {
	f := doc.Get(field)
	if f == nil {
		return ""
	}
	return f.StringValue()
}

// mustStringField builds a StringField, failing the test on error.
func mustStringField(t *testing.T, name, value string, stored bool) document.IndexableField {
	t.Helper()
	f, err := document.NewStringField(name, value, stored)
	if err != nil {
		t.Fatalf("NewStringField(%q): %v", name, err)
	}
	return f
}

// makeResume ports TestBlockJoin.makeResume: a parent document with
// docType=resume, a stored name, and a country.
func makeResume(t *testing.T, name, country string) index.Document {
	t.Helper()
	d := document.NewDocument()
	d.Add(mustStringField(t, "docType", "resume", false))
	d.Add(mustStringField(t, "name", name, true))
	d.Add(mustStringField(t, "country", country, false))
	return d
}

// makeJob ports TestBlockJoin.makeJob: a child document with a stored skill and
// a year. See the package deviation note: year is a StringField, not IntPoint.
func makeJob(t *testing.T, skill string, year int) index.Document {
	t.Helper()
	d := document.NewDocument()
	d.Add(mustStringField(t, "skill", skill, true))
	d.Add(mustStringField(t, "year", itoa(year), true))
	return d
}

// makeQualification ports TestBlockJoin.makeQualification: a child document with
// a stored qualification and a year (year as StringField, see deviation note).
func makeQualification(t *testing.T, qualification string, year int) index.Document {
	t.Helper()
	d := document.NewDocument()
	d.Add(mustStringField(t, "qualification", qualification, true))
	d.Add(mustStringField(t, "year", itoa(year), true))
	return d
}

// makeParent ports TestBlockJoin.makeParent: a parent document with
// docType=_parent and a stored parent_id.
func makeParent(t *testing.T, parentID string) index.Document {
	t.Helper()
	d := document.NewDocument()
	d.Add(mustStringField(t, "docType", "_parent", false))
	d.Add(mustStringField(t, "parent_id", parentID, true))
	return d
}

// makeVector ports TestBlockJoin.makeVector: a child document carrying a
// KnnFloatVectorField plus a stored my_parent_id.
func makeVector(t *testing.T, vectorField, childsParent string, value []float32) index.Document {
	t.Helper()
	d := document.NewDocument()
	vf, err := document.NewKnnFloatVectorFieldEuclidean(vectorField, value)
	if err != nil {
		t.Fatalf("NewKnnFloatVectorFieldEuclidean(%q): %v", vectorField, err)
	}
	d.Add(vf)
	d.Add(mustStringField(t, "my_parent_id", childsParent, true))
	return d
}

// itoa renders a non-negative int as a fixed 4-digit zero-padded string so the
// substitute year terms sort and compare consistently.
func itoa(v int) string {
	if v < 0 {
		// The block-join corpora use positive years only.
		return "0000"
	}
	digits := []byte{
		byte('0' + (v/1000)%10),
		byte('0' + (v/100)%10),
		byte('0' + (v/10)%10),
		byte('0' + v%10),
	}
	return string(digits)
}

// getParentDoc ports TestBlockJoin.getParentDoc: given a global child docID,
// find the parent of its block (the next set parent bit at or after the child
// within the child's leaf) and return that parent's stored document. It mirrors
// Lucene's helper, which resolves the parent bitset for the child's leaf and
// reads the parent doc; here the parent leaf-doc is rebased to a global id and
// read through the searcher.
func getParentDoc(t *testing.T, searcher *search.IndexSearcher, reader *index.DirectoryReader, parents BitSetProducer, childDocID int) *document.Document {
	t.Helper()
	leaves, err := reader.Leaves()
	if err != nil {
		t.Fatalf("Leaves: %v", err)
	}
	for _, ctx := range leaves {
		base := ctx.DocBase()
		max := ctx.LeafReader().MaxDoc()
		if childDocID < base || childDocID >= base+max {
			continue
		}
		bits, err := parents.GetBitSet(ctx)
		if err != nil {
			t.Fatalf("GetBitSet: %v", err)
		}
		parentLeafDoc := bits.NextSetBit(childDocID - base)
		if parentLeafDoc < 0 {
			t.Fatalf("no parent found for child doc %d", childDocID)
		}
		doc, err := searcher.Doc(base + parentLeafDoc)
		if err != nil {
			t.Fatalf("read parent doc: %v", err)
		}
		return doc
	}
	t.Fatalf("child doc %d not found in any leaf", childDocID)
	return nil
}
