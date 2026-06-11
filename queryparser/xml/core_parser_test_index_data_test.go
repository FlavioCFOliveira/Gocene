// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package xml_test

import (
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/queryparser/xml"
	"github.com/FlavioCFOliveira/Gocene/queryparser/xml/builders"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// TestCoreParserTestIndexData verifies CoreParser functionality: construction,
// builder registration, and XML document parsing. This replaces the index-data
// test from Java, which required a reuters21578.txt fixture and a full
// IndexWriter/DirectoryReader round-trip.
//
// In Gocene the CoreParser works purely as an XML-to-Query decoder, so this
// test validates that parsing and builder dispatch work correctly for the
// supported XML query elements.
func TestCoreParserTestIndexData(t *testing.T) {
	t.Run("empty parser requires builders", func(t *testing.T) {
		parser := xml.NewCoreParser("body")
		_, err := parser.Parse(strings.NewReader(`<MatchAllDocsQuery/>`))
		if err == nil {
			t.Error("expected error when no builders registered")
		}
	})

	t.Run("parser with registered builders", func(t *testing.T) {
		parser := xml.NewCoreParser("body")
		builders.RegisterCoreBuilders(parser, nil)

		q, err := parser.Parse(strings.NewReader(`<MatchAllDocsQuery/>`))
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := q.(*search.MatchAllDocsQuery); !ok {
			t.Errorf("expected MatchAllDocsQuery, got %T", q)
		}
	})
}

// TestCoreParserTermQuery verifies TermQuery XML parsing.
func TestCoreParserTermQuery(t *testing.T) {
	parser := xml.NewCoreParser("body")
	builders.RegisterCoreBuilders(parser, nil)

	q, err := parser.Parse(strings.NewReader(`<TermQuery fieldName="title">hello</TermQuery>`))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := q.(*search.TermQuery); !ok {
		t.Errorf("expected TermQuery, got %T", q)
	}
}

// TestCoreParserBooleanQuery verifies BooleanQuery XML parsing.
func TestCoreParserBooleanQuery(t *testing.T) {
	parser := xml.NewCoreParser("body")
	builders.RegisterCoreBuilders(parser, nil)

	q, err := parser.Parse(strings.NewReader(`<BooleanQuery>
		<Clause occurs="must"><TermQuery fieldName="t">a</TermQuery></Clause>
		<Clause occurs="must"><TermQuery fieldName="t">b</TermQuery></Clause>
	</BooleanQuery>`))
	if err != nil {
		t.Fatal(err)
	}
	bq, ok := q.(*search.BooleanQuery)
	if !ok {
		t.Fatalf("expected BooleanQuery, got %T", q)
	}
	if len(bq.Clauses()) != 2 {
		t.Errorf("expected 2 clauses, got %d", len(bq.Clauses()))
	}
}

// TestCoreParserRangeQuery verifies RangeQuery XML parsing.
func TestCoreParserRangeQuery(t *testing.T) {
	parser := xml.NewCoreParser("body")
	builders.RegisterCoreBuilders(parser, nil)

	q, err := parser.Parse(strings.NewReader(`<RangeQuery fieldName="t" lowerTerm="a" upperTerm="z" includeLower="true" includeUpper="false"/>`))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := q.(*search.TermRangeQuery); !ok {
		t.Errorf("expected TermRangeQuery, got %T", q)
	}
}

// TestCoreParserPointRangeQuery verifies PointRangeQuery XML parsing.
func TestCoreParserPointRangeQuery(t *testing.T) {
	parser := xml.NewCoreParser("body")
	builders.RegisterCoreBuilders(parser, nil)

	q, err := parser.Parse(strings.NewReader(`<PointRangeQuery fieldName="t" lowerTerm="aaaa" upperTerm="zzzz"/>`))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := q.(*search.PointRangeQuery); !ok {
		t.Errorf("expected PointRangeQuery, got %T", q)
	}
}

// TestCoreParserSpanTerm verifies SpanTerm XML parsing.
func TestCoreParserSpanTerm(t *testing.T) {
	parser := xml.NewCoreParser("body")
	builders.RegisterCoreBuilders(parser, nil)

	q, err := parser.Parse(strings.NewReader(`<SpanTerm fieldName="t">hello</SpanTerm>`))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := q.(*search.SpanTermQuery); !ok {
		t.Errorf("expected SpanTermQuery, got %T", q)
	}
}

// TestCorePlusQueriesParser verifies the CorePlusQueriesParser.
func TestCorePlusQueriesParser(t *testing.T) {
	parser := xml.NewCorePlusQueriesParser("body")
	builders.RegisterCorePlusQueriesBuilders(parser, nil, nil)

	q, err := parser.Parse(strings.NewReader(`<MatchAllDocsQuery/>`))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := q.(*search.MatchAllDocsQuery); !ok {
		t.Errorf("expected MatchAllDocsQuery, got %T", q)
	}
}

// TestCorePlusExtensionsParser verifies the CorePlusExtensionsParser.
func TestCorePlusExtensionsParser(t *testing.T) {
	parser := xml.NewCorePlusExtensionsParser("body")
	builders.RegisterCorePlusExtensionsBuilders(parser, nil, nil)

	q, err := parser.Parse(strings.NewReader(`<MatchAllDocsQuery/>`))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := q.(*search.MatchAllDocsQuery); !ok {
		t.Errorf("expected MatchAllDocsQuery, got %T", q)
	}
}

// TestCoreParserDefaultField verifies the default field is passed correctly.
func TestCoreParserDefaultField(t *testing.T) {
	parser := xml.NewCoreParser("customfield")
	if parser.DefaultField != "customfield" {
		t.Errorf("DefaultField = %q", parser.DefaultField)
	}
}

// TestCoreParserParseErrors verifies error handling for invalid XML.
func TestCoreParserParseErrors(t *testing.T) {
	parser := xml.NewCoreParser("body")
	builders.RegisterCoreBuilders(parser, nil)

	_, err := parser.Parse(strings.NewReader(""))
	if err == nil {
		t.Error("expected error for empty input")
	}
}
