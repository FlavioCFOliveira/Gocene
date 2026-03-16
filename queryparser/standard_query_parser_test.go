package queryparser

import (
	"testing"
)

func TestNewStandardQueryParser(t *testing.T) {
	p := NewStandardQueryParser()

	if p == nil {
		t.Fatal("Expected StandardQueryParser to be created")
	}

	if p.GetDefaultField() != "" {
		t.Errorf("Expected empty default field, got '%s'", p.GetDefaultField())
	}

	if p.GetDefaultOperator() != OR {
		t.Error("Expected default operator OR")
	}
}

func TestStandardQueryParserSetters(t *testing.T) {
	p := NewStandardQueryParser()

	// Test SetDefaultField
	p.SetDefaultField("content")
	if p.GetDefaultField() != "content" {
		t.Errorf("Expected default field 'content', got '%s'", p.GetDefaultField())
	}

	// Test SetDefaultOperator
	p.SetDefaultOperator(AND)
	if p.GetDefaultOperator() != AND {
		t.Error("Expected default operator AND")
	}

	// Test SetAllowLeadingWildcard
	p.SetAllowLeadingWildcard(true)
	if !p.GetAllowLeadingWildcard() {
		t.Error("Expected allow leading wildcard to be true")
	}

	// Test SetPhraseSlop
	p.SetPhraseSlop(5)
	if p.GetPhraseSlop() != 5 {
		t.Errorf("Expected phrase slop 5, got %d", p.GetPhraseSlop())
	}
}

func TestStandardQueryParserParse(t *testing.T) {
	p := NewStandardQueryParser()
	p.SetDefaultField("content")

	// Test empty query
	_, err := p.Parse("")
	if err == nil {
		t.Error("Expected error for empty query")
	}

	// Test simple term
	query, err := p.Parse("test")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if query == nil {
		t.Error("Expected query to be created")
	}

	// Test phrase
	query, err = p.Parse("\"hello world\"")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if query == nil {
		t.Error("Expected query to be created")
	}

	// Test fielded query
	query, err = p.Parse("title:test")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if query == nil {
		t.Error("Expected query to be created")
	}
}

func TestStandardQueryParserParseWithField(t *testing.T) {
	p := NewStandardQueryParser()

	query, err := p.ParseWithField("title", "test")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if query == nil {
		t.Error("Expected query to be created")
	}
}

func TestBooleanOperatorString(t *testing.T) {
	if AND.String() != "AND" {
		t.Error("Expected AND.String() to return 'AND'")
	}
	if OR.String() != "OR" {
		t.Error("Expected OR.String() to return 'OR'")
	}
}
