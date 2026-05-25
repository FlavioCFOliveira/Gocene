// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// deferred_analysis_compat_test.go is the explicit landing pad for the
// "analysis" audit rows that Sprint 114 T10 (rmp 4618) acknowledged but
// did NOT cover with an active scenario. Each entry below cites its
// audit row verbatim from docs/compat-coverage.tsv along with the
// upstream Maven coordinates a future task would need to add.
//
// Every deferral runs as a t.Skip subtest so the row appears in the
// `go test -v` output (evidence the row was considered) without failing
// the build. The reasons are technical: the third-party language
// analyzers (Hunspell binary dictionaries, Word2Vec models, Kuromoji /
// Nori / Smartcn compiled dictionaries, OpenNLP serialised models) are
// NOT shipped inside lucene-core or lucene-analysis-common; they live in
// separate Maven artefacts (sized 10MB+) and pull in pre-trained model
// binaries that the harness pom intentionally does NOT depend on. The
// pragmatic scope of T10 ships the two tractable scenarios
// (synonym-fst, token-payload-bytes) plus the stopwords text fixture;
// the heavyweights are deferred to dedicated tasks that can bring in
// the relevant JARs.
package analysis

import "testing"

// TestAnalysisAudit_DeferredRows iterates every analysis-side audit row
// that T10 recognised but could not complete inside the budget. Each
// subtest body is a t.Skip with the row's verbatim audit citation and
// the Maven coordinates a future task will need.
func TestAnalysisAudit_DeferredRows(t *testing.T) {
	deferred := []struct {
		artefact  string // audit row "artefact" column
		luceneCls string // audit row "lucene_class" column
		gapNotes  string // audit row "gap_notes" column (verbatim)
		maven     string // Maven coordinates required to lift the deferral
		reason    string // why this is deferred from Sprint 114 T10
	}{
		{
			artefact:  "Hunspell compiled dictionary",
			luceneCls: "org.apache.lucene.analysis.hunspell.Dictionary",
			gapNotes:  "Hunspell .aff/.dic fixtures are source-level (Lucene-shared) but no binary compiled-dict round-trip vs Lucene.",
			maven:     "org.apache.lucene:lucene-analysis-common:10.4.0 (already on classpath) PLUS pre-compiled dictionaries packaged by upstream consumers (e.g., OpenOffice .aff/.dic corpora). The binary compiled-Dictionary form is the in-memory FST produced by Dictionary.<init>; there is no on-disk Lucene-defined binary format to round-trip against.",
			reason:    "Gocene ships an analysis/hunspell/ port with unit tests over the Lucene-shared text fixtures; the audit-row gap is the absence of a binary compiled-dict round-trip. That round-trip is not a Lucene file-format contract (no codec emits it), so a fixture-based byte-equality scenario is moot. Deferred until a future task defines the parity contract: either (a) cross-validate the in-memory FST against a Lucene-built Dictionary by enumerating known affix expansions, or (b) introduce a Gocene-side persisted form distinct from Lucene's.",
		},
		{
			artefact:  "Word2Vec model archive",
			luceneCls: "org.apache.lucene.analysis.synonym.word2vec.Word2VecSynonymProviderFactory",
			gapNotes:  "Fixtures exist but only validate parsing, no Lucene-side write/read parity.",
			maven:     "org.apache.lucene:lucene-analysis-common:10.4.0 (already on classpath) plus a pre-trained Word2Vec model archive (typically 100MB+; not redistributable as a test fixture).",
			reason:    "Word2VecSynonymProviderFactory consumes a third-party Word2Vec ZIP (vocab.txt + vectors.bin in the upstream tool's format). The Lucene side does NOT define a write format — it only reads. A round-trip requires either (a) producing a model with the upstream gensim/Word2Vec tool, which is out of scope, or (b) re-using Lucene's own minimal test fixture, which is < 10 vectors and not byte-stable across Lucene point releases. Deferred until a Gocene-shipped tiny model is checked in under tools/lucene-fixtures/testdata.",
		},
		{
			artefact:  "Kuromoji compiled dictionary",
			luceneCls: "org.apache.lucene.analysis.ja.dict.TokenInfoDictionary",
			gapNotes:  "No round-trip vs Lucene-compiled dictionaries; relies on Lucene-shipped binary blobs.",
			maven:     "org.apache.lucene:lucene-analysis-kuromoji:10.4.0",
			reason:    "The Kuromoji JAR bundles ~30MB of pre-compiled dictionary blobs (TokenInfoDictionary, UnknownDictionary, ConnectionCosts) generated at JAR-build time from MeCab IPADIC. The on-disk format IS Lucene-defined (BinaryDictionary.write / .read) and is therefore a legitimate compatibility target, but adding the Maven coordinate above would inflate the harness jar by 30MB+. Deferred until a dedicated sprint defines whether Gocene ships its own Kuromoji port or vendors the JAR via a side-channel.",
		},
		{
			artefact:  "Nori compiled dictionary",
			luceneCls: "org.apache.lucene.analysis.ko.dict.TokenInfoDictionary",
			gapNotes:  "No round-trip vs Lucene-compiled dictionaries; relies on Lucene-shipped binary blobs.",
			maven:     "org.apache.lucene:lucene-analysis-nori:10.4.0",
			reason:    "Same shape as Kuromoji: pre-compiled Korean dictionary blobs bundled in the JAR. Deferred for the same JAR-size and build-pipeline reasons.",
		},
		{
			artefact:  "Smartcn compiled dictionary",
			luceneCls: "org.apache.lucene.analysis.cn.smart.AnalyzerProfile",
			gapNotes:  "No round-trip vs Lucene-compiled dictionaries; relies on Lucene-shipped binary blobs.",
			maven:     "org.apache.lucene:lucene-analysis-smartcn:10.4.0",
			reason:    "Same shape as Kuromoji / Nori: pre-compiled Chinese dictionary blobs bundled in the JAR. Deferred for the same JAR-size and build-pipeline reasons.",
		},
		{
			artefact:  "OpenNLP serialised models",
			luceneCls: "org.apache.lucene.analysis.opennlp.OpenNLPTokenizerFactory",
			gapNotes:  "Port exists but no fixture or round-trip.",
			maven:     "org.apache.lucene:lucene-analysis-opennlp:10.4.0 plus pre-trained OpenNLP model binaries (.bin) supplied out-of-band.",
			reason:    "OpenNLP factories consume third-party serialised models (opennlp.tools.tokenize.TokenizerModel etc.). Lucene does NOT define the binary format — it delegates to the OpenNLP library. A Lucene-side parity check would only assert that Lucene reads the same bytes the OpenNLP library reads, which is tautological. Deferred until a Gocene-shipped OpenNLP port defines its own contract.",
		},
	}

	for _, row := range deferred {
		row := row
		t.Run(row.artefact, func(t *testing.T) {
			t.Skipf("deferred: %s (lucene_class=%q gap_notes=%q maven=%q): %s",
				row.artefact, row.luceneCls, row.gapNotes, row.maven, row.reason)
		})
	}
}
