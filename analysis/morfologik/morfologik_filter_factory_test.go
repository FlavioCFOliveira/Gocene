// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
// analysis/morfologik/src/test/org/apache/lucene/analysis/morfologik/TestMorfologikFilterFactory.java

package morfologik_test

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/analysis/morfologik"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// forbidResourcesLoader mirrors the Java-side ForbidResourcesLoader inner
// class: it panics on any resource open, so that the "default dictionary"
// path is exercised without touching real resources.
type forbidResourcesLoader struct{}

func (forbidResourcesLoader) OpenResource(name string) (io.ReadCloser, error) {
	panic("forbidResourcesLoader: OpenResource must not be called")
}

func (forbidResourcesLoader) FindFactory(name string) (util.FactoryFunc, error) {
	panic("forbidResourcesLoader: FindFactory must not be called")
}

func (forbidResourcesLoader) NewInstance(name string) (any, error) {
	panic("forbidResourcesLoader: NewInstance must not be called")
}

// noopDictionary is a Dictionary that produces an IStemmer with fixed entries.
type noopDictionary struct {
	entries map[string][]morfologik.WordData
}

func (d *noopDictionary) NewStemmer() morfologik.IStemmer {
	return &fixedStemmer{entries: d.entries}
}

type fixedStemmer struct {
	entries map[string][]morfologik.WordData
}

func (s *fixedStemmer) Lookup(token string) []morfologik.WordData {
	return s.entries[token]
}

// buildFixedDictionary creates a Dictionary backed by the given lookup table.
func buildFixedDictionary(entries map[string][]morfologik.WordData) morfologik.Dictionary {
	return &noopDictionary{entries: entries}
}

// customDictLoader implements DictionaryLoader for testing explicit dictionary
// resources. It returns a predefined dictionary for the expected resource name.
type customDictLoader struct {
	dicts map[string]morfologik.Dictionary
}

func (l *customDictLoader) LoadDictionary(loader util.ResourceLoader, name string) (morfologik.Dictionary, error) {
	if d, ok := l.dicts[name]; ok {
		return d, nil
	}
	return nil, errors.New("Resource not found: " + name)
}

// TestMorfologikFilterFactory_DefaultDictionary mirrors testDefaultDictionary.
// A factory with no "dictionary" attribute and a pre-configured default
// dictionary should emit expected lemmas.
func TestMorfologikFilterFactory_DefaultDictionary(t *testing.T) {
	factory, err := morfologik.NewMorfologikFilterFactory(map[string]string{})
	if err != nil {
		t.Fatalf("NewMorfologikFilterFactory: %v", err)
	}

	// Configure default dictionary before Inform.
	polishDefault := buildFixedDictionary(map[string][]morfologik.WordData{
		"rowery": {{Stem: "rower", Tag: ""}},
		"bilety": {{Stem: "bilet", Tag: ""}},
	})
	factory.SetDefaultDictionary(polishDefault)

	if err := factory.Inform(forbidResourcesLoader{}); err != nil {
		t.Fatalf("Inform: %v", err)
	}

	// Use a simple in-memory stream that emits "rowery" then "bilety".
	src := newMockWordStream([]string{"rowery", "bilety"})
	stream := factory.Create(src)
	defer stream.Close()

	count := 0
	for {
		ok, err := stream.IncrementToken()
		if err != nil {
			t.Fatalf("IncrementToken: %v", err)
		}
		if !ok {
			break
		}
		count++
	}
	// each word maps to exactly 1 lemma → total 2
	if count != 2 {
		t.Errorf("expected 2 tokens, got %d", count)
	}
}

// TestMorfologikFilterFactory_ExplicitDictionary mirrors testExplicitDictionary.
func TestMorfologikFilterFactory_ExplicitDictionary(t *testing.T) {
	params := map[string]string{
		morfologik.DictionaryAttribute: "custom-dictionary.dict",
	}
	factory, err := morfologik.NewMorfologikFilterFactory(params)
	if err != nil {
		t.Fatalf("NewMorfologikFilterFactory: %v", err)
	}

	customDict := buildFixedDictionary(map[string][]morfologik.WordData{
		"inflected1": {{Stem: "lemma1", Tag: ""}},
		"inflected2": {{Stem: "lemma2", Tag: ""}},
	})

	loader := &customDictLoader{dicts: map[string]morfologik.Dictionary{
		"custom-dictionary.dict": customDict,
	}}
	factory.SetDictionaryLoader(loader)

	if err := factory.Inform(forbidResourcesLoader{}); err != nil {
		t.Fatalf("Inform: %v", err)
	}

	src := newMockWordStream([]string{"inflected1", "inflected2"})
	stream := factory.Create(src)
	defer stream.Close()

	count := 0
	for {
		ok, err := stream.IncrementToken()
		if err != nil {
			t.Fatalf("IncrementToken: %v", err)
		}
		if !ok {
			break
		}
		count++
	}
	if count != 2 {
		t.Errorf("expected 2 tokens, got %d", count)
	}
}

// TestMorfologikFilterFactory_MissingDictionary mirrors testMissingDictionary.
// When the resource is not found, Inform returns an error containing
// "Resource not found".
func TestMorfologikFilterFactory_MissingDictionary(t *testing.T) {
	params := map[string]string{
		morfologik.DictionaryAttribute: "missing-dictionary-resource.dict",
	}
	factory, err := morfologik.NewMorfologikFilterFactory(params)
	if err != nil {
		t.Fatalf("NewMorfologikFilterFactory: %v", err)
	}

	loader := &customDictLoader{dicts: map[string]morfologik.Dictionary{}}
	factory.SetDictionaryLoader(loader)

	if err := factory.Inform(forbidResourcesLoader{}); err == nil {
		t.Error("expected Inform to return an error for missing dictionary, got nil")
	} else if !strings.Contains(err.Error(), "Resource not found") {
		t.Errorf("error message should contain 'Resource not found', got: %v", err)
	}
}

// TestMorfologikFilterFactory_BogusArguments mirrors testBogusArguments.
func TestMorfologikFilterFactory_BogusArguments(t *testing.T) {
	params := map[string]string{
		"bogusArg": "bogusValue",
	}
	_, err := morfologik.NewMorfologikFilterFactory(params)
	if err == nil {
		t.Error("expected error for bogus argument, got nil")
	} else if !strings.Contains(err.Error(), "unknown parameters") {
		t.Errorf("error message should contain 'unknown parameters', got: %v", err)
	}
}

// mockWordStream is a minimal TokenStream that emits pre-set words via a
// simple attribute source, sharing it with downstream filters.
type mockWordStream struct {
	analysis.BaseTokenStream
	words    []string
	idx      int
	termImpl *analysis.CharTermAttributeImpl
}

func newMockWordStream(words []string) *mockWordStream {
	s := &mockWordStream{
		BaseTokenStream: *analysis.NewBaseTokenStream(),
		words:           words,
		idx:             -1,
	}
	s.termImpl = analysis.NewCharTermAttributeImpl()
	s.GetAttributeSource().AddAttributeImpl(s.termImpl)
	return s
}

func (s *mockWordStream) IncrementToken() (bool, error) {
	s.idx++
	if s.idx >= len(s.words) {
		return false, nil
	}
	s.termImpl.SetEmpty()
	s.termImpl.AppendString(s.words[s.idx])
	return true, nil
}
