// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package scenarios

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	_ "github.com/FlavioCFOliveira/Gocene/codecs/lucene90"
	"github.com/FlavioCFOliveira/Gocene/document"
	gcompat "github.com/FlavioCFOliveira/Gocene/internal/compat"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// Constants mirroring CombinedMultiSegmentIndexSearchScenario.
const (
	s1TsvName   = "s1-hits.tsv"
	s1ScoreFmt  = "%.6f"
	s1DocsPerSeg = 6
	s1NumDocs   = s1DocsPerSeg * 3
	s1VectorDim = 4
)

// s1QueryIDs preserves the fixed catalogue order.
var s1QueryIDs = []string{
	"tq-alpha", "tq-beta", "tq-gamma", "tq-delta", "tq-epsilon",
	"ph-alpha-beta", "ph-gamma-delta",
	"bool-alpha-or-zeta",
}

// TestS1_GoceneWriteLeg generates the combined S1 index from Gocene and
// writes s1-hits.tsv. The Java harness verifies that the Lucene-side
// re-scoring matches byte-for-byte.
func TestS1_GoceneWriteLeg(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run(strconv.FormatInt(seed, 10), func(t *testing.T) {
			dir := t.TempDir()

			// --- Build index ---
			fsDir, err := store.NewSimpleFSDirectory(dir)
			if err != nil {
				t.Fatalf("open dir: %v", err)
			}
			defer fsDir.Close()

			cfg := index.NewIndexWriterConfig(analysis.NewStandardAnalyzer())
			cfg.SetUseCompoundFile(false)
			cfg.SetMergePolicy(index.NewNoMergePolicy())
			cfg.SetMergeScheduler(index.NewSerialMergeScheduler())

			iw, err := index.NewIndexWriter(fsDir, cfg)
			if err != nil {
				t.Fatalf("NewIndexWriter: %v", err)
			}

			for seg := 0; seg < 3; seg++ {
				for j := 0; j < s1DocsPerSeg; j++ {
					i := seg*s1DocsPerSeg + j
					doc, err := s1BuildDoc(i, seed)
					if err != nil {
						t.Fatalf("buildDoc(%d): %v", i, err)
					}
					if err := iw.AddDocument(doc); err != nil {
						t.Fatalf("AddDocument: %v", err)
					}
				}
				if err := iw.Commit(); err != nil {
					t.Fatalf("Commit: %v", err)
				}
			}
			iw.Close()

			// --- Evaluate queries ---
			reader, err := index.OpenDirectoryReader(fsDir)
			if err != nil {
				t.Fatalf("OpenDirectoryReader: %v", err)
			}
			defer reader.Close()

			leaves, err := reader.Leaves()
			if err != nil {
				t.Fatalf("Leaves: %v", err)
			}
			if len(leaves) != 3 {
				t.Fatalf("expected 3 segments, got %d", len(leaves))
			}

			rows, err := s1Evaluate(reader)
			if err != nil {
				t.Fatalf("evaluate: %v", err)
			}

			// --- Write TSV ---
			if err := s1WriteTsv(filepath.Join(dir, s1TsvName), rows); err != nil {
				t.Fatalf("writeTsv: %v", err)
			}

			// --- Verify with Java harness ---
			if err := gcompat.Verify(scenarioS1, seed, dir); err != nil {
				t.Fatalf("harness verify: %v", err)
			}
		})
	}
}

// s1BuildDoc constructs a deterministic doc matching Java buildDoc.
func s1BuildDoc(i int, seed int64) (*document.Document, error) {
	doc := document.NewDocument()
	id := fmt.Sprintf("doc-%d", i)
	const s1MixMul int64 = -7046029254386353131 // int64(0x9E3779B97F4A7C15) Java constant
	mix := (seed * s1MixMul) ^ int64(i)

	// Stored id
	sf, err := document.NewStoredField("id", id)
	if err != nil {
		return nil, err
	}
	doc.Add(sf)

	// String id (indexed, not stored)
	strf, err := document.NewStringField("id", id, false)
	if err != nil {
		return nil, err
	}
	doc.Add(strf)

	// NumericDocValues
	ndv, err := document.NewNumericDocValuesField("rank_dv", int64(i))
	if err != nil {
		return nil, err
	}
	doc.Add(ndv)

	// IntPoint
	ip, err := document.NewIntPointLucene("rank_pt", int32(i))
	if err != nil {
		return nil, err
	}
	doc.Add(ip)

	// KnnFloatVectorField
	vec := make([]float32, s1VectorDim)
	for k := 0; k < s1VectorDim; k++ {
		vec[k] = float32(((uint64(mix) >> (k * 8)) & 0xFF) / 255.0) + 1e-3
	}
	knn, err := document.NewKnnFloatVectorFieldEuclidean("vec", vec)
	if err != nil {
		return nil, err
	}
	doc.Add(knn)

	// Body text with term vectors
	repAlpha := int((uint64(mix) & 0x3) + 1)
	repBeta := int(((uint64(mix) >> 2) & 0x3) + 1)
	var body strings.Builder
	for k := 0; k < repAlpha; k++ {
		body.WriteString("alpha ")
	}
	for k := 0; k < repBeta; k++ {
		body.WriteString("beta ")
	}
	body.WriteString("gamma delta ")
	if (i % 3) == 0 {
		body.WriteString("epsilon ")
	}
	if (i % 4) == 0 {
		body.WriteString("zeta ")
	}
	body.WriteString("pivot ")
	body.WriteString(id)

	ft := document.NewFieldType()
	ft.SetIndexOptions(index.IndexOptionsDocsAndFreqsAndPositionsAndOffsets)
	ft.SetStoreTermVectors(true)
	ft.SetStoreTermVectorPositions(true)
	ft.SetStoreTermVectorOffsets(true)
	ft.SetTokenized(true)
	ft.SetIndexed(true)
	ft.Freeze()

	bodyField, err := document.NewField("body", body.String(), ft)
	if err != nil {
		return nil, err
	}
	doc.Add(bodyField)

	return doc, nil
}

// s1BuildQueries returns the fixed query catalogue.
func s1BuildQueries() map[string]search.Query {
	q := make(map[string]search.Query)
	q["tq-alpha"] = search.NewTermQuery(index.NewTerm("body", "alpha"))
	q["tq-beta"] = search.NewTermQuery(index.NewTerm("body", "beta"))
	q["tq-gamma"] = search.NewTermQuery(index.NewTerm("body", "gamma"))
	q["tq-delta"] = search.NewTermQuery(index.NewTerm("body", "delta"))
	q["tq-epsilon"] = search.NewTermQuery(index.NewTerm("body", "epsilon"))
	q["ph-alpha-beta"] = search.NewPhraseQuery("body", index.NewTerm("body", "alpha"), index.NewTerm("body", "beta"))
	q["ph-gamma-delta"] = search.NewPhraseQuery("body", index.NewTerm("body", "gamma"), index.NewTerm("body", "delta"))
	boolQ := search.NewBooleanQuery()
	boolQ.Add(search.NewTermQuery(index.NewTerm("body", "alpha")), search.SHOULD)
	boolQ.Add(search.NewTermQuery(index.NewTerm("body", "zeta")), search.SHOULD)
	q["bool-alpha-or-zeta"] = boolQ
	return q
}

// s1Row represents a single TSV row.
type s1Row struct {
	queryID string
	rank    int
	docID   string
	score   float64
}

// s1Evaluate executes the query catalogue and returns sorted rows.
func s1Evaluate(reader index.IndexReaderInterface) ([]s1Row, error) {
	searcher := search.NewIndexSearcher(reader)
	searcher.SetSimilarity(search.NewBM25Similarity())
	defer searcher.Close()

	var rows []s1Row
	for _, qid := range s1QueryIDs {
		query := s1BuildQueries()[qid]
		topDocs, err := searcher.Search(query, s1NumDocs)
		if err != nil {
			return nil, err
		}
		for rank, sd := range topDocs.ScoreDocs {
			doc, err := searcher.Doc(sd.Doc)
			if err != nil {
				return nil, err
			}
			idField := doc.Get("id")
			if idField == nil {
				return nil, fmt.Errorf("doc %d missing id field", sd.Doc)
			}
			rows = append(rows, s1Row{
				queryID: qid,
				rank:    rank,
				docID:   idField.StringValue(),
				score:   float64(sd.Score),
			})
		}
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].queryID != rows[j].queryID {
			return rows[i].queryID < rows[j].queryID
		}
		return rows[i].rank < rows[j].rank
	})
	return rows, nil
}

// s1WriteTsv writes the hit rows in the expected format.
func s1WriteTsv(path string, rows []s1Row) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	fmt.Fprintln(f, "# query_id\trank\tdoc_id\tscore")
	for _, r := range rows {
		fmt.Fprintf(f, "%s\t%d\t%s\t%s\n", r.queryID, r.rank, r.docID, fmt.Sprintf(s1ScoreFmt, r.score))
	}
	return f.Close()
}
