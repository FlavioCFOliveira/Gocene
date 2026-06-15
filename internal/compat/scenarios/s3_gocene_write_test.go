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
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	_ "github.com/FlavioCFOliveira/Gocene/codecs/lucene90"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/facets"
	"github.com/FlavioCFOliveira/Gocene/facets/taxonomy"
	gcompat "github.com/FlavioCFOliveira/Gocene/internal/compat"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
)

const (
	s3TsvName    = "s3-facet-counts.tsv"
	s3TaxoSubdir = "taxo"
	s3NumDocs    = 16
)

var (
	s3Colors = []string{"red", "green", "blue"}
	s3Sizes  = []string{"s", "m", "l"}
)

// TestS3_GoceneWriteLeg generates the combined S3 faceted index from Gocene
// and writes s3-facet-counts.tsv. The Java harness verifies facet counts.
func TestS3_GoceneWriteLeg(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run(strconv.FormatInt(seed, 10), func(t *testing.T) {
			dir := t.TempDir()
			taxoDir := filepath.Join(dir, s3TaxoSubdir)
			if err := os.MkdirAll(taxoDir, 0755); err != nil {
				t.Fatalf("mkdir taxo: %v", err)
			}

			mainFsDir, err := store.NewSimpleFSDirectory(dir)
			if err != nil {
				t.Fatalf("open main dir: %v", err)
			}
			defer mainFsDir.Close()

			taxoFsDir, err := store.NewSimpleFSDirectory(taxoDir)
			if err != nil {
				t.Fatalf("open taxo dir: %v", err)
			}
			defer taxoFsDir.Close()

			// Install the compat codec globally so the taxonomy writer also writes
			// Lucene-compatible stored fields.
			compat := newCompatCodec()
			origCodec := index.GetDefaultCodec()
			index.RegisterNamedCodec("Lucene104", compat)
			index.RegisterDefaultCodec(compat)
			t.Cleanup(func() {
				index.RegisterNamedCodec("Lucene104", origCodec)
				index.RegisterDefaultCodec(origCodec)
			})

			taxoWriter, err := facets.NewDirectoryTaxonomyWriter(taxoFsDir)
			if err != nil {
				t.Fatalf("NewDirectoryTaxonomyWriter: %v", err)
			}

			cfg := index.NewIndexWriterConfig(analysis.NewStandardAnalyzer())
			cfg.SetUseCompoundFile(false)
			cfg.SetMergePolicy(index.NewNoMergePolicy())
			cfg.SetMergeScheduler(index.NewSerialMergeScheduler())
			cfg.SetCodec(newCompatCodec())

			iw, err := index.NewIndexWriter(mainFsDir, cfg)
			if err != nil {
				t.Fatalf("NewIndexWriter: %v", err)
			}

			config := facets.NewFacetsConfig()

			for i := 0; i < s3NumDocs; i++ {
				doc := document.NewDocument()
				idField, err := document.NewStringField("id", fmt.Sprintf("f-%d", i), true)
				if err != nil {
					t.Fatalf("NewStringField: %v", err)
				}
				doc.Add(idField)

				bodyField, err := document.NewTextField("body", fmt.Sprintf("alpha pivot-%d", i), false)
				if err != nil {
					t.Fatalf("NewTextField: %v", err)
				}
				doc.Add(bodyField)

				colorFacet := facets.NewFacetField("color", s3PickColor(seed, i))
				sizeFacet := facets.NewFacetField("size", s3PickSize(seed, i))

				doc, err = config.BuildWithTaxonomy(taxoWriter, doc, colorFacet, sizeFacet)
				if err != nil {
					t.Fatalf("BuildWithTaxonomy: %v", err)
				}

				if err := iw.AddDocument(doc); err != nil {
					t.Fatalf("AddDocument: %v", err)
				}
			}

			if err := taxoWriter.Commit(); err != nil {
				t.Fatalf("taxo commit: %v", err)
			}
			if err := iw.Commit(); err != nil {
				t.Fatalf("writer commit: %v", err)
			}
			taxoWriter.Close()
			iw.Close()

			// --- Evaluate facets ---
			reader, err := index.OpenDirectoryReader(mainFsDir)
			if err != nil {
				t.Fatalf("OpenDirectoryReader: %v", err)
			}
			defer reader.Close()

			taxoReader, err := facets.NewDirectoryTaxonomyReader(taxoFsDir)
			if err != nil {
				t.Fatalf("NewDirectoryTaxonomyReader: %v", err)
			}
			defer taxoReader.Close()

			rows, err := s3Evaluate(reader, taxoReader, config)
			if err != nil {
				t.Fatalf("evaluate: %v", err)
			}

			if err := s3WriteTsv(filepath.Join(dir, s3TsvName), rows); err != nil {
				t.Fatalf("writeTsv: %v", err)
			}

			if err := gcompat.Verify(scenarioS3, seed, dir); err != nil {
				t.Fatalf("harness verify: %v", err)
			}
		})
	}
}

type s3Row struct {
	dim   string
	label string
	count int
}

func s3Evaluate(reader index.IndexReaderInterface, taxoReader *facets.DirectoryTaxonomyReader, config *facets.FacetsConfig) ([]s3Row, error) {
	searcher := search.NewIndexSearcher(reader)
	defer searcher.Close()

	fc := facets.NewFacetsCollector()
	if err := searcher.SearchWithCollector(search.NewTermQuery(index.NewTerm("body", "alpha")), fc); err != nil {
		return nil, err
	}
	if err := fc.Finish(); err != nil {
		return nil, err
	}

	adapter := taxonomy.NewDirectoryTaxonomyReaderAdapter(taxoReader)
	ftfc := taxonomy.NewFastTaxonomyFacetCounts("", adapter, config)
	if err := ftfc.Accumulate(fc.GetMatchingDocs()); err != nil {
		return nil, err
	}

	var rows []s3Row
	for _, dim := range []string{"color", "size"} {
		result, err := ftfc.GetTopChildren(10, dim)
		if err != nil {
			return nil, err
		}
		if result == nil {
			continue
		}
		for _, lv := range result.LabelValues {
			rows = append(rows, s3Row{dim: dim, label: lv.Label, count: int(lv.Value)})
		}
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].dim != rows[j].dim {
			return rows[i].dim < rows[j].dim
		}
		if rows[i].count != rows[j].count {
			return rows[i].count > rows[j].count // descending count
		}
		return rows[i].label < rows[j].label
	})
	return rows, nil
}

func s3WriteTsv(path string, rows []s3Row) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	fmt.Fprintln(f, "# dim\tlabel\tcount")
	for _, r := range rows {
		fmt.Fprintf(f, "%s\t%s\t%d\n", r.dim, r.label, r.count)
	}
	return f.Close()
}

func s3PickColor(seed int64, i int) string {
	const mul int64 = -7046029254386353131 // int64(0x9E3779B97F4A7C15)
	mix := (seed * mul) ^ int64(i)
	idx := int(mix % int64(len(s3Colors)))
	if idx < 0 {
		idx = -idx
	}
	return s3Colors[idx]
}

func s3PickSize(seed int64, i int) string {
	const mul int64 = -4996333548504531783 // int64(0xBF58476D1CE4E5B9)
	mix := (seed * mul) ^ (int64(i) * 31)
	idx := int(mix % int64(len(s3Sizes)))
	if idx < 0 {
		idx = -idx
	}
	return s3Sizes[idx]
}
