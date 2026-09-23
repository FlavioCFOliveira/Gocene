// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of
// lucene/core/src/test/org/apache/lucene/search/similarities/TestSimilarity2.java
// (Apache Lucene 10.5.0): tests against all the similarities we have.

package search_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// The DFR, IB and DFI component arrays below render the package-private static
// arrays of lucene/core/src/test/org/apache/lucene/search/similarities/TestSimilarityBase.java
// that TestSimilarity2.setUp iterates over.

// similarityBaseBasicModels renders TestSimilarityBase.BASIC_MODELS.
func similarityBaseBasicModels() []search.LuceneDFRBasicModel {
	return []search.LuceneDFRBasicModel{
		search.NewLuceneBasicModelG(), search.NewLuceneBasicModelIF(), search.NewLuceneBasicModelIn(), search.NewLuceneBasicModelIne(),
	}
}

// similarityBaseAfterEffects renders TestSimilarityBase.AFTER_EFFECTS.
func similarityBaseAfterEffects() []search.LuceneDFRAfterEffect {
	return []search.LuceneDFRAfterEffect{search.NewLuceneAfterEffectB(), search.NewLuceneAfterEffectL()}
}

// similarityBaseNormalizations renders TestSimilarityBase.NORMALIZATIONS.
func similarityBaseNormalizations() []search.LuceneDFRNormalization {
	return []search.LuceneDFRNormalization{
		search.NewLuceneNormalizationH1(),
		search.NewLuceneNormalizationH2(),
		search.NewLuceneNormalizationH3(),
		search.NewLuceneNormalizationZ(),
		search.NewLuceneNoNormalization(),
	}
}

// similarityBaseDistributions renders TestSimilarityBase.DISTRIBUTIONS.
func similarityBaseDistributions() []search.LuceneIBDistribution {
	return []search.LuceneIBDistribution{search.NewLuceneDistributionLL(), search.NewLuceneDistributionSPL()}
}

// similarityBaseLambdas renders TestSimilarityBase.LAMBDAS.
func similarityBaseLambdas() []search.LuceneIBLambda {
	return []search.LuceneIBLambda{search.NewLuceneLambdaDF(), search.NewLuceneLambdaTTF()}
}

// similarityBaseIndependenceMeasures renders TestSimilarityBase.INDEPENDENCE_MEASURES.
func similarityBaseIndependenceMeasures() []search.LuceneDFIIndependence {
	return []search.LuceneDFIIndependence{
		search.NewLuceneIndependenceStandardized(), search.NewLuceneIndependenceSaturated(), search.NewLuceneIndependenceChiSquared(),
	}
}

// setUpSimilarity2 renders TestSimilarity2.setUp(): the sims list.
func setUpSimilarity2() []search.Similarity {
	var sims []search.Similarity
	sims = append(sims, search.NewClassicSimilarity())
	sims = append(sims, search.NewLuceneBM25Similarity())
	sims = append(sims, search.NewLuceneBooleanSimilarity())
	sims = append(sims, search.NewLuceneAxiomaticF1EXPDefault())
	sims = append(sims, search.NewLuceneAxiomaticF1LOGDefault())
	sims = append(sims, search.NewLuceneAxiomaticF2EXPDefault())
	sims = append(sims, search.NewLuceneAxiomaticF2LOGDefault())
	sims = append(sims, search.NewLuceneAxiomaticF3EXPDefault(0.25, 3))
	sims = append(sims, search.NewLuceneAxiomaticF3LOG(0.25, 3))
	// TODO: not great that we dup this all with TestSimilarityBase
	for _, basicModel := range similarityBaseBasicModels() {
		for _, afterEffect := range similarityBaseAfterEffects() {
			for _, normalization := range similarityBaseNormalizations() {
				sims = append(sims, search.NewLuceneDFRSimilarity(basicModel, afterEffect, normalization))
			}
		}
	}
	for _, distribution := range similarityBaseDistributions() {
		for _, lambda := range similarityBaseLambdas() {
			for _, normalization := range similarityBaseNormalizations() {
				sims = append(sims, search.NewLuceneIBSimilarity(distribution, lambda, normalization))
			}
		}
	}
	sims = append(sims, search.NewLuceneLMDirichletSimilarity())
	sims = append(sims, search.NewLuceneLMJelinekMercerSimilarity(0.1))
	sims = append(sims, search.NewLuceneLMJelinekMercerSimilarity(0.7))
	for _, independence := range similarityBaseIndependenceMeasures() {
		sims = append(sims, search.NewLuceneDFISimilarity(independence))
	}
	return sims
}

// because of stupid things like querynorm, it's possible we computeStats on a
// field that doesnt exist at all test this against a totally empty index, to
// make sure sims handle it.
func TestSimilarity2EmptyIndex(t *testing.T) {
	sims := setUpSimilarity2()
	dir := newDirectory()
	iw := newRandomIndexWriter(t, dir)
	ir := mustGetReader(t, iw)
	mustClose(t, iw)
	is := newSearcher(t, ir)

	for _, sim := range sims {
		is.SetSimilarity(sim)
		if got := mustSearch(t, is, search.NewTermQuery(index.NewTerm("foo", "bar")), 10).TotalHits.Value; got != 0 {
			t.Fatalf("%v: expected 0, got %d", sim, got)
		}
	}
	mustClose(t, ir, dir)
}

// similar to the above, but ORs the query with a real field.
func TestSimilarity2EmptyField(t *testing.T) {
	sims := setUpSimilarity2()
	dir := newDirectory()
	iw := newRandomIndexWriter(t, dir)
	doc := newTestDocument(newTextField(t, "foo", "bar", false))
	mustAddDocument(t, iw, doc)
	ir := mustGetReader(t, iw)
	mustClose(t, iw)
	is := newSearcher(t, ir)

	for _, sim := range sims {
		is.SetSimilarity(sim)
		query := search.NewBooleanQueryBuilder()
		query.Add(search.NewTermQuery(index.NewTerm("foo", "bar")), search.SHOULD)
		query.Add(search.NewTermQuery(index.NewTerm("bar", "baz")), search.SHOULD)
		if got := mustSearch(t, is, query.Build(), 10).TotalHits.Value; got != 1 {
			t.Fatalf("%v: expected 1, got %d", sim, got)
		}
	}
	mustClose(t, ir, dir)
}

// similar to the above, however the field exists, but we query with a term
// that doesnt exist too.
func TestSimilarity2EmptyTerm(t *testing.T) {
	sims := setUpSimilarity2()
	dir := newDirectory()
	iw := newRandomIndexWriter(t, dir)
	doc := newTestDocument(newTextField(t, "foo", "bar", false))
	mustAddDocument(t, iw, doc)
	ir := mustGetReader(t, iw)
	mustClose(t, iw)
	is := newSearcher(t, ir)

	for _, sim := range sims {
		is.SetSimilarity(sim)
		query := search.NewBooleanQueryBuilder()
		query.Add(search.NewTermQuery(index.NewTerm("foo", "bar")), search.SHOULD)
		query.Add(search.NewTermQuery(index.NewTerm("foo", "baz")), search.SHOULD)
		if got := mustSearch(t, is, query.Build(), 10).TotalHits.Value; got != 1 {
			t.Fatalf("%v: expected 1, got %d", sim, got)
		}
	}
	mustClose(t, ir, dir)
}

// make sure we can retrieve when norms are disabled.
func TestSimilarity2NoNorms(t *testing.T) {
	sims := setUpSimilarity2()
	dir := newDirectory()
	iw := newRandomIndexWriter(t, dir)
	ft := document.NewFieldTypeFrom(document.TextFieldTypeNotStored)
	ft.SetOmitNorms(true)
	ft.Freeze()
	doc := newTestDocument(newField(t, "foo", "bar", ft))
	mustAddDocument(t, iw, doc)
	ir := mustGetReader(t, iw)
	mustClose(t, iw)
	is := newSearcher(t, ir)

	for _, sim := range sims {
		is.SetSimilarity(sim)
		query := search.NewBooleanQueryBuilder()
		query.Add(search.NewTermQuery(index.NewTerm("foo", "bar")), search.SHOULD)
		if got := mustSearch(t, is, query.Build(), 10).TotalHits.Value; got != 1 {
			t.Fatalf("%v: expected 1, got %d", sim, got)
		}
	}
	mustClose(t, ir, dir)
}

// make sure scores are not skewed by docs not containing the field.
func TestSimilarity2NoFieldSkew(t *testing.T) {
	sims := setUpSimilarity2()
	dir := newDirectory()
	// an evil merge policy could reorder our docs for no reason
	iwConfig := newIndexWriterConfig()
	iwConfig.SetMergePolicy(newLogMergePolicy())
	iw := newRandomIndexWriterWithConfig(t, dir, iwConfig)
	doc := newTestDocument(newTextField(t, "foo", "bar baz somethingelse", false))
	mustAddDocument(t, iw, doc)
	ir := mustGetReader(t, iw)
	is := newSearcher(t, ir)

	queryBuilder := search.NewBooleanQueryBuilder()
	queryBuilder.Add(search.NewTermQuery(index.NewTerm("foo", "bar")), search.SHOULD)
	queryBuilder.Add(search.NewTermQuery(index.NewTerm("foo", "baz")), search.SHOULD)
	query := queryBuilder.Build()

	// collect scores
	var scores []search.Explanation
	for _, sim := range sims {
		is.SetSimilarity(sim)
		e, err := is.Explain(query, 0)
		if err != nil {
			t.Fatalf("explain: %v", err)
		}
		scores = append(scores, e)
	}
	mustClose(t, ir)

	// add some additional docs without the field
	numExtraDocs := nextInt(1, 1000)
	for i := 0; i < numExtraDocs; i++ {
		mustAddDocument(t, iw, document.NewDocument())
	}

	// check scores are the same
	ir = mustGetReader(t, iw)
	is = newSearcher(t, ir)
	for i := 0; i < len(sims); i++ {
		is.SetSimilarity(sims[i])
		expected := scores[i]
		actual, err := is.Explain(query, 0)
		if err != nil {
			t.Fatalf("explain: %v", err)
		}
		if expected.GetValue() != actual.GetValue() {
			t.Fatalf("%v: actual=%v,expected=%v", sims[i], actual, expected)
		}
	}

	mustClose(t, iw, ir, dir)
}

// make sure all sims work if TF is omitted.
func TestSimilarity2OmitTF(t *testing.T) {
	sims := setUpSimilarity2()
	dir := newDirectory()
	iw := newRandomIndexWriter(t, dir)
	ft := document.NewFieldTypeFrom(document.TextFieldTypeNotStored)
	ft.SetIndexOptions(index.IndexOptionsDocs)
	ft.Freeze()
	f := newField(t, "foo", "bar", ft)
	doc := newTestDocument(f)
	mustAddDocument(t, iw, doc)
	ir := mustGetReader(t, iw)
	mustClose(t, iw)
	is := newSearcher(t, ir)

	for _, sim := range sims {
		is.SetSimilarity(sim)
		query := search.NewBooleanQueryBuilder()
		query.Add(search.NewTermQuery(index.NewTerm("foo", "bar")), search.SHOULD)
		if got := mustSearch(t, is, query.Build(), 10).TotalHits.Value; got != 1 {
			t.Fatalf("%v: expected 1, got %d", sim, got)
		}
	}
	mustClose(t, ir, dir)
}

// make sure all sims work if TF and norms is omitted.
func TestSimilarity2OmitTFAndNorms(t *testing.T) {
	sims := setUpSimilarity2()
	dir := newDirectory()
	iw := newRandomIndexWriter(t, dir)
	ft := document.NewFieldTypeFrom(document.TextFieldTypeNotStored)
	ft.SetIndexOptions(index.IndexOptionsDocs)
	ft.SetOmitNorms(true)
	ft.Freeze()
	f := newField(t, "foo", "bar", ft)
	doc := newTestDocument(f)
	mustAddDocument(t, iw, doc)
	ir := mustGetReader(t, iw)
	mustClose(t, iw)
	is := newSearcher(t, ir)

	for _, sim := range sims {
		is.SetSimilarity(sim)
		query := search.NewBooleanQueryBuilder()
		query.Add(search.NewTermQuery(index.NewTerm("foo", "bar")), search.SHOULD)
		if got := mustSearch(t, is, query.Build(), 10).TotalHits.Value; got != 1 {
			t.Fatalf("%v: expected 1, got %d", sim, got)
		}
	}
	mustClose(t, ir, dir)
}
