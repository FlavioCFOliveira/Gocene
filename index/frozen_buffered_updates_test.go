// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/index/TestFrozenBufferedUpdates.java
// (Apache Lucene 10.5.0).

package index

import (
	"math/rand"
	"reflect"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// preferSeekExactLeafReader ports the private static PreferSeekExactLeafReader
// (a FilterLeafReader whose terms enumerators prefer seekExact()).
type preferSeekExactLeafReader struct {
	*FilterLeafReader
}

func newPreferSeekExactLeafReader(in LeafReader) *preferSeekExactLeafReader {
	return &preferSeekExactLeafReader{FilterLeafReader: NewFilterLeafReader(in)}
}

func (r *preferSeekExactLeafReader) Terms(field string) (Terms, error) {
	in, err := r.in.Terms(field)
	if err != nil {
		return nil, err
	}
	if in == nil {
		// new FilterTerms(null) throws NullPointerException in Java; the terms
		// of "field" always exist in this test.
		return nil, nil
	}
	return &preferSeekExactTerms{FilterTerms: NewFilterTerms(in)}, nil
}

func (r *preferSeekExactLeafReader) GetCoreCacheHelper() CacheHelper {
	return r.in.GetCoreCacheHelper()
}

func (r *preferSeekExactLeafReader) GetReaderCacheHelper() CacheHelper {
	return r.in.GetReaderCacheHelper()
}

type preferSeekExactTerms struct {
	*FilterTerms
}

func (t *preferSeekExactTerms) Iterator() (TermsEnum, error) {
	in, err := t.in.Iterator()
	if err != nil {
		return nil, err
	}
	return &preferSeekExactTermsEnum{FilterTermsEnum: NewFilterTermsEnum(in)}, nil
}

type preferSeekExactTermsEnum struct {
	*FilterTermsEnum
}

// PreferSeekExact overrides TermsEnum.preferSeekExact().
func (e *preferSeekExactTermsEnum) PreferSeekExact() bool {
	return true
}

func TestFrozenBufferedUpdatesTermDocsIterator(t *testing.T) {
	// TermsEnum.preferSeekExact() is consulted by TermDocsIterator.nextTerm in
	// Lucene 10.5.0; the Go TermsEnum contract does not declare it yet, so
	// the override above is never consulted.
	if _, ok := reflect.TypeOf((*TermsEnum)(nil)).Elem().MethodByName("PreferSeekExact"); !ok {
		t.Error("org.apache.lucene.index.TermsEnum#preferSeekExact() is not ported: " +
			"FrozenBufferedUpdates.TermDocsIterator cannot consult it")
	}
	for j := 0; j < 5; j++ {
		func() {
			dir := newDirectory()
			defer dir.Close()
			writer, err := NewIndexWriter(dir, newIndexWriterConfig())
			if err != nil {
				t.Fatalf("new IndexWriter: %v", err)
			}
			defer writer.Close()
			duplicates := rand.Intn(2) == 0
			nonMatches := rand.Intn(2) == 0
			array := util.NewBytesRefArray(util.ByteBlockSize)
			numDocs := 10 + rand.Intn(1000)
			randomIds := make(map[string]struct{})
			var asList []string
			for i := 0; i < numDocs; i++ {
				for {
					id := util.RandomRealisticUnicodeString(rand.New(rand.NewSource(rand.Int63())), 0, 20)
					if _, dup := randomIds[id]; !dup {
						randomIds[id] = struct{}{}
						asList = append(asList, id)
						break
					}
				}
			}
			for ref := range randomIds {
				doc := document.NewDocument()
				f, err := document.NewStringFieldFromBytesRef("field", []byte(ref), false)
				if err != nil {
					t.Fatalf("new StringField: %v", err)
				}
				doc.Add(f)
				array.Append(util.NewBytesRef([]byte(ref)))
				if duplicates && rarely() {
					array.Append(util.NewBytesRef([]byte(asList[rand.Intn(len(asList))])))
				}
				if nonMatches && rarely() {
					var id string
					for {
						id = util.RandomRealisticUnicodeString(rand.New(rand.NewSource(rand.Int63())), 0, 20)
						if _, present := randomIds[id]; !present {
							break
						}
					}
					array.Append(util.NewBytesRef([]byte(id)))
				}
				if _, err := writer.AddDocument(doc); err != nil {
					t.Fatalf("addDocument: %v", err)
				}
			}
			if err := writer.ForceMerge(1); err != nil {
				t.Fatalf("forceMerge: %v", err)
			}
			if _, err := writer.Commit(); err != nil {
				t.Fatalf("commit: %v", err)
			}
			reader, err := OpenDirectoryReader(dir)
			if err != nil {
				t.Fatalf("DirectoryReader.open: %v", err)
			}
			defer reader.Close()
			sorted := rand.Intn(2) == 0
			preferSeekExact := rand.Intn(2) == 0
			var values func() ([]byte, bool)
			if sorted {
				state := array.SortByBytes()
				values = func() ([]byte, bool) {
					var spare util.BytesRef
					if !state.Next(&spare) {
						return nil, false
					}
					return append([]byte(nil), spare.ValidBytes()...), true
				}
			} else {
				it := array.Iterator()
				values = func() ([]byte, bool) {
					ref, ok := it.Next()
					if !ok {
						return nil, false
					}
					return ref.ValidBytes(), true
				}
			}
			leaves, err := reader.Leaves()
			if err != nil {
				t.Fatalf("leaves: %v", err)
			}
			if len(leaves) != 1 {
				t.Fatalf("expected 1 leaf, got %d", len(leaves))
			}
			var leafReader LeafReader = leaves[0].LeafReader()
			if preferSeekExact {
				leafReader = newPreferSeekExactLeafReader(leafReader)
			}
			iterator := NewTermDocsIteratorFromReader(leafReader, sorted)
			bitSet, err := util.NewFixedBitSet(reader.MaxDoc())
			if err != nil {
				t.Fatalf("new FixedBitSet: %v", err)
			}
			for {
				ref, ok := values()
				if !ok {
					break
				}
				docIDSetIterator, err := iterator.NextTerm("field", ref)
				if err != nil {
					t.Fatalf("nextTerm: %v", err)
				}
				if !nonMatches && docIDSetIterator == nil {
					t.Fatal("assertNotNull(docIdSetIterator)")
				}
				if docIDSetIterator != nil {
					for {
						doc, err := docIDSetIterator.NextDoc()
						if err != nil {
							t.Fatalf("nextDoc: %v", err)
						}
						if doc == NO_MORE_DOCS {
							break
						}
						if !duplicates && bitSet.Get(doc) {
							t.Fatalf("doc %d seen twice", doc)
						}
						bitSet.Set(doc)
					}
				}
			}
			if reader.MaxDoc() != bitSet.Cardinality() {
				t.Fatalf("expected %d, got %d", reader.MaxDoc(), bitSet.Cardinality())
			}
		}()
	}
}
