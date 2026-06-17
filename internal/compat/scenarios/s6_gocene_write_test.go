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
	"github.com/FlavioCFOliveira/Gocene/queryparser"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/highlight/uhighlight"
)

const s6TsvName = "s6-highlights.tsv"

// s6Row represents a single TSV row.
type s6Row struct {
	queryText    string
	docID        string
	snippetIndex  int
	snippetText  string
}

// TestS6_GoceneWriteLeg generates the combined S6 index from Gocene and writes
// s6-highlights.tsv. The Java harness verifies that the Lucene-side
// re-computation matches byte-for-byte.
func TestS6_GoceneWriteLeg(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run(strconv.FormatInt(seed, 10), func(t *testing.T) {
			dir := t.TempDir()

			fsDir, err := store.NewSimpleFSDirectory(dir)
			if err != nil {
				t.Fatalf("open dir: %v", err)
			}
			defer fsDir.Close()

			analyzer := analysis.NewStandardAnalyzer()
			cfg := index.NewIndexWriterConfig(analyzer)
			cfg.SetUseCompoundFile(false)
			cfg.SetMergePolicy(index.NewNoMergePolicy())
			cfg.SetMergeScheduler(index.NewSerialMergeScheduler())
			cfg.SetCodec(newCompatCodec())

			iw, err := index.NewIndexWriter(fsDir, cfg)
			if err != nil {
				t.Fatalf("NewIndexWriter: %v", err)
			}

			for i := 0; i < 12; i++ {
				doc := s6BuildDoc(i, seed)
				if err := iw.AddDocument(doc); err != nil {
					t.Fatalf("AddDocument: %v", err)
				}
			}
			if err := iw.Close(); err != nil {
				t.Fatalf("Close: %v", err)
			}

			reader, err := index.OpenDirectoryReader(fsDir)
			if err != nil {
				t.Fatalf("OpenDirectoryReader: %v", err)
			}
			defer reader.Close()

			rows, err := s6Evaluate(reader, analyzer)
			if err != nil {
				t.Fatalf("evaluate: %v", err)
			}

			if err := s6WriteTsv(filepath.Join(dir, s6TsvName), rows); err != nil {
				t.Fatalf("writeTsv: %v", err)
			}

			if err := gcompat.Verify(scenarioS6, seed, dir); err != nil {
				t.Fatalf("harness verify: %v", err)
			}
		})
	}
}

// s6BuildDoc constructs a deterministic doc matching Java buildDoc.
func s6BuildDoc(i int, seed int64) *document.Document {
	doc := document.NewDocument()
	id := fmt.Sprintf("doc-%d", i)
	idField, _ := document.NewStoredField("id", id)
	doc.Add(idField)

	const mixMul int64 = -7046029254386353131 // 0x9E3779B97F4A7C15
	mix := (seed * mixMul) ^ int64(i)

	var body strings.Builder
	alphaN := int((mix & 0x3) + 1)
	for k := 0; k < alphaN; k++ {
		body.WriteString("alpha ")
	}
	betaN := int(((mix >> 2) & 0x3) + 1)
	for k := 0; k < betaN; k++ {
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
	ft.SetStored(true)
	ft.SetTokenized(true)
	ft.SetIndexed(true)
	ft.Freeze()

	bodyField, _ := document.NewField("body", body.String(), ft)
	doc.Add(bodyField)
	return doc
}

// s6Evaluate parses the fixed query catalogue, searches, and highlights.
func s6Evaluate(reader index.IndexReaderInterface, analyzer *analysis.StandardAnalyzer) ([]s6Row, error) {
	searcher := search.NewIndexSearcher(reader)
	searcher.SetSimilarity(search.NewBM25Similarity())
	defer searcher.Close()

	uh := uhighlight.NewUnifiedHighlighterBuilder(searcher, analyzer).
		WithMaxNoHighlightPassages(0).
		Build()

	qp := queryparser.NewQueryParser("body", analyzer)
	var rows []s6Row
	for _, qtext := range expectedS6Queries {
		q, err := qp.Parse(qtext)
		if err != nil {
			return nil, fmt.Errorf("parse %q: %w", qtext, err)
		}
		topDocs, err := searcher.Search(q, 12)
		if err != nil {
			return nil, fmt.Errorf("search %q: %w", qtext, err)
		}
		// Sort score docs by doc ID for stable iteration, matching Java.
		sorted := make([]*search.ScoreDoc, len(topDocs.ScoreDocs))
		copy(sorted, topDocs.ScoreDocs)
		sort.Slice(sorted, func(a, b int) bool {
			return sorted[a].Doc < sorted[b].Doc
		})
		topDocsSorted := search.NewTopDocs(topDocs.TotalHits, sorted)
		snippets, err := uh.Highlight("body", q, topDocsSorted, 3)
		if err != nil {
			return nil, fmt.Errorf("highlight %q: %w", qtext, err)
		}
		for i, sd := range sorted {
			if i >= len(snippets) {
				break
			}
			if snippets[i] == "" {
				continue
			}
			doc, err := searcher.Doc(sd.Doc)
			if err != nil {
				return nil, err
			}
			idField := doc.Get("id")
			if idField == nil {
				return nil, fmt.Errorf("doc %d missing id", sd.Doc)
			}
			rows = append(rows, s6Row{
				queryText:   qtext,
				docID:       idField.StringValue(),
				snippetIndex: 0,
				snippetText: snippets[i],
			})
		}
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].queryText != rows[j].queryText {
			return rows[i].queryText < rows[j].queryText
		}
		if rows[i].docID != rows[j].docID {
			return rows[i].docID < rows[j].docID
		}
		return rows[i].snippetIndex < rows[j].snippetIndex
	})
	return rows, nil
}

// s6WriteTsv writes the highlight rows in the expected format.
func s6WriteTsv(path string, rows []s6Row) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	fmt.Fprintln(f, "# query_text\tdoc_id\tsnippet_index\tsnippet")
	for _, r := range rows {
		fmt.Fprintf(f, "%s\t%s\t%d\t%s\n",
			tsvEscape(r.queryText),
			r.docID,
			r.snippetIndex,
			tsvEscape(r.snippetText),
		)
	}
	return f.Close()
}

// tsvEscape mirrors Java TsvEscape.escape for \t, \n, \r and \.
func tsvEscape(s string) string {
	if !strings.ContainsAny(s, "\t\n\\\r") {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString("\\\\")
		case '\t':
			b.WriteString("\\t")
		case '\n':
			b.WriteString("\\n")
		case '\r':
			b.WriteString("\\r")
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
