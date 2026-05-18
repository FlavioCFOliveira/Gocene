package ext

import (
	"errors"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/queryparser"
	"github.com/FlavioCFOliveira/Gocene/search"
)

func TestExtensionsRegistration(t *testing.T) {
	exts := NewExtensions()
	if exts.GetExtensionFieldDelimiter() != ':' {
		t.Errorf("delimiter = %q, want %q", exts.GetExtensionFieldDelimiter(), ':')
	}

	called := false
	exts.Add("myExt", ParserExtensionFunc(func(q *ExtensionQuery) (search.Query, error) {
		called = true
		return search.NewMatchAllDocsQuery(), nil
	}))

	if exts.GetExtension("missing") != nil {
		t.Error("missing extension should be nil")
	}
	ext := exts.GetExtension("myExt")
	if ext == nil {
		t.Fatal("expected extension")
	}
	if _, err := ext.Parse(&ExtensionQuery{rawQueryString: "foo"}); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Error("extension was not invoked")
	}
}

func TestSplitExtensionField(t *testing.T) {
	exts := NewExtensions()
	field, key := exts.SplitExtensionField("dflt", "myExt")
	if field != "dflt" || key != "" {
		t.Errorf("plain field: got (%q,%q)", field, key)
	}
	field, key = exts.SplitExtensionField("dflt", "foo:myExt")
	if field != "foo" || key != "myExt" {
		t.Errorf("delim: got (%q,%q)", field, key)
	}
}

func TestBuildExtensionField(t *testing.T) {
	exts := NewExtensions()
	if got := exts.BuildExtensionField("k", "f"); got != "f:k" {
		t.Errorf("got %q", got)
	}
	if got := exts.BuildExtensionField("k", ""); got != "k" {
		t.Errorf("empty field: got %q", got)
	}
}

func TestEscapeExtensionField(t *testing.T) {
	exts := NewExtensions()
	if got := exts.EscapeExtensionField("a:b:c"); got != "a\\:b\\:c" {
		t.Errorf("got %q", got)
	}
	if got := exts.EscapeExtensionField("plain"); got != "plain" {
		t.Errorf("got %q", got)
	}
}

func TestCustomDelimiter(t *testing.T) {
	exts := NewExtensionsWithDelimiter('|')
	if got := exts.BuildExtensionField("k", "f"); got != "f|k" {
		t.Errorf("got %q", got)
	}
	field, key := exts.SplitExtensionField("dflt", "f|k")
	if field != "f" || key != "k" {
		t.Errorf("got (%q,%q)", field, key)
	}
}

func TestExtensionQueryAccessors(t *testing.T) {
	parser := queryparser.NewQueryParserWithDefaultField("body")
	eq := NewExtensionQuery(parser, "title", "foo bar")
	if eq.GetField() != "title" {
		t.Error("field")
	}
	if eq.GetRawQueryString() != "foo bar" {
		t.Error("raw")
	}
	if eq.GetTopLevelParser() != parser {
		t.Error("parser ref")
	}
}

func TestExtendableQueryParserDispatchesExtension(t *testing.T) {
	exts := NewExtensions()
	var capturedField, capturedRaw string
	exts.Add("myExt", ParserExtensionFunc(func(q *ExtensionQuery) (search.Query, error) {
		capturedField = q.GetField()
		capturedRaw = q.GetRawQueryString()
		return search.NewMatchAllDocsQuery(), nil
	}))

	p := NewExtendableQueryParser("body", analysis.NewStandardAnalyzer(), exts)
	q, err := p.Parse("myExt:hello")
	if err != nil {
		t.Fatal(err)
	}
	if q == nil {
		t.Fatal("nil query")
	}
	if capturedField != "myExt" || capturedRaw != "hello" {
		t.Errorf("captured = (%q,%q)", capturedField, capturedRaw)
	}
}

func TestExtendableQueryParserFallsThroughWhenNoExtension(t *testing.T) {
	exts := NewExtensions()
	p := NewExtendableQueryParser("body", analysis.NewStandardAnalyzer(), exts)
	q, err := p.Parse("hello")
	if err != nil {
		t.Fatal(err)
	}
	if q == nil {
		t.Fatal("nil query")
	}
}

func TestExtensionFuncReturnsError(t *testing.T) {
	exts := NewExtensions()
	sentinel := errors.New("ext failure")
	exts.Add("bad", ParserExtensionFunc(func(q *ExtensionQuery) (search.Query, error) {
		return nil, sentinel
	}))
	p := NewExtendableQueryParser("body", analysis.NewStandardAnalyzer(), exts)
	_, err := p.Parse("bad:foo")
	if !errors.Is(err, sentinel) {
		t.Errorf("err = %v, want %v", err, sentinel)
	}
}

func TestExtendableQueryParserExposesExtensions(t *testing.T) {
	exts := NewExtensions()
	p := NewExtendableQueryParser("body", analysis.NewStandardAnalyzer(), exts)
	if p.GetExtensions() != exts {
		t.Error("GetExtensions should return same registry")
	}
	if p.GetExtensionFieldDelimiter() != ':' {
		t.Error("delimiter mismatch")
	}
}

func TestExtendableQueryParserNilExtensions(t *testing.T) {
	p := NewExtendableQueryParser("body", analysis.NewStandardAnalyzer(), nil)
	if p.GetExtensions() == nil {
		t.Error("nil extensions should be replaced by empty registry")
	}
}
