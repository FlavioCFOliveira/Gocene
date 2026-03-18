package flexible

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/search"
)

func TestStandardQueryConfigHandler_New(t *testing.T) {
	config := NewStandardQueryConfigHandler()

	if config == nil {
		t.Fatal("NewStandardQueryConfigHandler should not return nil")
	}

	if config.GetDefaultOperator() != "OR" {
		t.Errorf("Default operator should be OR, got %s", config.GetDefaultOperator())
	}

	if config.GetPhraseSlop() != 0 {
		t.Errorf("Default phrase slop should be 0, got %d", config.GetPhraseSlop())
	}

	if config.IsAllowLeadingWildcard() {
		t.Error("Allow leading wildcard should be false by default")
	}

	if !config.IsLowercaseExpandedTerms() {
		t.Error("Lowercase expanded terms should be true by default")
	}
}

func TestStandardQueryConfigHandler_Setters(t *testing.T) {
	config := NewStandardQueryConfigHandler()

	config.SetDefaultField("title")
	if config.GetDefaultField() != "title" {
		t.Errorf("SetDefaultField failed, got %s", config.GetDefaultField())
	}

	config.SetDefaultOperator("AND")
	if config.GetDefaultOperator() != "AND" {
		t.Errorf("SetDefaultOperator failed, got %s", config.GetDefaultOperator())
	}

	config.SetPhraseSlop(2)
	if config.GetPhraseSlop() != 2 {
		t.Errorf("SetPhraseSlop failed, got %d", config.GetPhraseSlop())
	}

	config.SetAllowLeadingWildcard(true)
	if !config.IsAllowLeadingWildcard() {
		t.Error("SetAllowLeadingWildcard failed")
	}

	config.SetLowercaseExpandedTerms(false)
	if config.IsLowercaseExpandedTerms() {
		t.Error("SetLowercaseExpandedTerms failed")
	}
}

func TestStandardSyntaxParser_Parse(t *testing.T) {
	config := NewStandardQueryConfigHandler()
	config.SetDefaultField("content")
	parser := NewStandardSyntaxParser(config)

	tests := []struct {
		name    string
		query   string
		wantErr bool
	}{
		{
			name:    "simple term",
			query:   "test",
			wantErr: false,
		},
		{
			name:    "empty query",
			query:   "",
			wantErr: true,
		},
		{
			name:    "fielded query",
			query:   "title:hello",
			wantErr: false,
		},
		{
			name:    "quoted phrase",
			query:   `"hello world"`,
			wantErr: false,
		},
		{
			name:    "grouped query",
			query:   "(a AND b)",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node, err := parser.Parse(tt.query)
			if tt.wantErr {
				if err == nil {
					t.Errorf("Parse() expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Parse() unexpected error: %v", err)
				}
				if node == nil {
					t.Error("Parse() returned nil node")
				}
			}
		})
	}
}

func TestStandardSyntaxParser_ParseTerm(t *testing.T) {
	config := NewStandardQueryConfigHandler()
	config.SetDefaultField("content")
	parser := NewStandardSyntaxParser(config)

	node, err := parser.Parse("hello")
	if err != nil {
		t.Errorf("Parse() error: %v", err)
	}

	if node == nil {
		t.Fatal("Parse() returned nil")
	}

	// Should be a FieldQueryNode
	if fieldNode, ok := node.(*FieldQueryNode); ok {
		if fieldNode.GetText() != "hello" {
			t.Errorf("Expected text 'hello', got %s", fieldNode.GetText())
		}
		if fieldNode.GetField() != "content" {
			t.Errorf("Expected field 'content', got %s", fieldNode.GetField())
		}
	} else {
		t.Errorf("Expected FieldQueryNode, got %T", node)
	}
}

func TestStandardSyntaxParser_ParseFieldedQuery(t *testing.T) {
	config := NewStandardQueryConfigHandler()
	parser := NewStandardSyntaxParser(config)

	node, err := parser.Parse("title:hello")
	if err != nil {
		t.Errorf("Parse() error: %v", err)
	}

	if node == nil {
		t.Fatal("Parse() returned nil")
	}

	// Should be a FieldQueryNode
	if fieldNode, ok := node.(*FieldQueryNode); ok {
		if fieldNode.GetField() != "title" {
			t.Errorf("Expected field 'title', got %s", fieldNode.GetField())
		}
		if fieldNode.GetText() != "hello" {
			t.Errorf("Expected text 'hello', got %s", fieldNode.GetText())
		}
	} else {
		t.Errorf("Expected FieldQueryNode, got %T", node)
	}
}

func TestStandardSyntaxParser_ParsePhrase(t *testing.T) {
	config := NewStandardQueryConfigHandler()
	config.SetDefaultField("content")
	parser := NewStandardSyntaxParser(config)

	node, err := parser.Parse(`"hello world"`)
	if err != nil {
		t.Errorf("Parse() error: %v", err)
	}

	if node == nil {
		t.Fatal("Parse() returned nil")
	}

	// Should be a PhraseSlopQueryNode
	if phraseNode, ok := node.(*PhraseSlopQueryNode); ok {
		if phraseNode.GetText() != "hello world" {
			t.Errorf("Expected text 'hello world', got %s", phraseNode.GetText())
		}
	} else {
		t.Errorf("Expected PhraseSlopQueryNode, got %T", node)
	}
}

func TestStandardSyntaxParser_ParseAnd(t *testing.T) {
	config := NewStandardQueryConfigHandler()
	config.SetDefaultField("content")
	parser := NewStandardSyntaxParser(config)

	node, err := parser.Parse("a AND b")
	if err != nil {
		t.Errorf("Parse() error: %v", err)
	}

	if node == nil {
		t.Fatal("Parse() returned nil")
	}

	// Should be an AndQueryNode
	if andNode, ok := node.(*AndQueryNode); ok {
		if len(andNode.GetChildren()) != 2 {
			t.Errorf("Expected 2 children, got %d", len(andNode.GetChildren()))
		}
	} else {
		t.Errorf("Expected AndQueryNode, got %T", node)
	}
}

func TestStandardSyntaxParser_ParseOr(t *testing.T) {
	config := NewStandardQueryConfigHandler()
	config.SetDefaultField("content")
	parser := NewStandardSyntaxParser(config)

	node, err := parser.Parse("a OR b")
	if err != nil {
		t.Errorf("Parse() error: %v", err)
	}

	if node == nil {
		t.Fatal("Parse() returned nil")
	}

	// Should be an OrQueryNode
	if orNode, ok := node.(*OrQueryNode); ok {
		if len(orNode.GetChildren()) != 2 {
			t.Errorf("Expected 2 children, got %d", len(orNode.GetChildren()))
		}
	} else {
		t.Errorf("Expected OrQueryNode, got %T", node)
	}
}

func TestStandardSyntaxParser_ParseNot(t *testing.T) {
	config := NewStandardQueryConfigHandler()
	config.SetDefaultField("content")
	parser := NewStandardSyntaxParser(config)

	node, err := parser.Parse("NOT a")
	if err != nil {
		t.Errorf("Parse() error: %v", err)
	}

	if node == nil {
		t.Fatal("Parse() returned nil")
	}

	// Should be a ModifierQueryNode with prohibited modifier
	if modNode, ok := node.(*ModifierQueryNode); ok {
		if modNode.GetModifier() != ModifierProhibited {
			t.Errorf("Expected ModifierProhibited, got %v", modNode.GetModifier())
		}
	} else {
		t.Errorf("Expected ModifierQueryNode, got %T", node)
	}
}

func TestStandardSyntaxParser_ParseRequired(t *testing.T) {
	config := NewStandardQueryConfigHandler()
	config.SetDefaultField("content")
	parser := NewStandardSyntaxParser(config)

	node, err := parser.Parse("+a")
	if err != nil {
		t.Errorf("Parse() error: %v", err)
	}

	if node == nil {
		t.Fatal("Parse() returned nil")
	}

	// Should be a ModifierQueryNode with required modifier
	if modNode, ok := node.(*ModifierQueryNode); ok {
		if modNode.GetModifier() != ModifierRequired {
			t.Errorf("Expected ModifierRequired, got %v", modNode.GetModifier())
		}
	} else {
		t.Errorf("Expected ModifierQueryNode, got %T", node)
	}
}

func TestStandardSyntaxParser_ParseProhibited(t *testing.T) {
	config := NewStandardQueryConfigHandler()
	config.SetDefaultField("content")
	parser := NewStandardSyntaxParser(config)

	node, err := parser.Parse("-a")
	if err != nil {
		t.Errorf("Parse() error: %v", err)
	}

	if node == nil {
		t.Fatal("Parse() returned nil")
	}

	// Should be a ModifierQueryNode with prohibited modifier
	if modNode, ok := node.(*ModifierQueryNode); ok {
		if modNode.GetModifier() != ModifierProhibited {
			t.Errorf("Expected ModifierProhibited, got %v", modNode.GetModifier())
		}
	} else {
		t.Errorf("Expected ModifierQueryNode, got %T", node)
	}
}

func TestStandardSyntaxParser_ParseGroup(t *testing.T) {
	config := NewStandardQueryConfigHandler()
	config.SetDefaultField("content")
	parser := NewStandardSyntaxParser(config)

	node, err := parser.Parse("(a AND b)")
	if err != nil {
		t.Errorf("Parse() error: %v", err)
	}

	if node == nil {
		t.Fatal("Parse() returned nil")
	}

	// Should be a GroupQueryNode
	if _, ok := node.(*GroupQueryNode); !ok {
		t.Errorf("Expected GroupQueryNode, got %T", node)
	}
}

func TestStandardQueryNodeProcessorPipeline_New(t *testing.T) {
	pipeline := NewStandardQueryNodeProcessorPipeline()

	if pipeline == nil {
		t.Fatal("NewStandardQueryNodeProcessorPipeline should not return nil")
	}

	if len(pipeline.GetPipeline()) == 0 {
		t.Error("Pipeline should have processors")
	}
}

func TestStandardQueryTreeBuilder_New(t *testing.T) {
	builder := NewStandardQueryTreeBuilder()

	if builder == nil {
		t.Fatal("NewStandardQueryTreeBuilder should not return nil")
	}

	// Check that all builders are registered
	builders := []string{
		"FieldQueryNode",
		"BooleanQueryNode",
		"AndQueryNode",
		"OrQueryNode",
		"ModifierQueryNode",
		"BoostQueryNode",
		"FuzzyQueryNode",
		"RangeQueryNode",
		"PhraseSlopQueryNode",
		"GroupQueryNode",
		"MatchAllDocsQueryNode",
		"MatchNoDocsQueryNode",
	}

	for _, name := range builders {
		if builder.GetBuilder(name) == nil {
			t.Errorf("Builder for %s should be registered", name)
		}
	}
}

func TestStandardQueryParser_New(t *testing.T) {
	parser := NewStandardQueryParser()

	if parser == nil {
		t.Fatal("NewStandardQueryParser should not return nil")
	}

	if parser.GetConfig() == nil {
		t.Error("Parser should have config")
	}
}

func TestStandardQueryParser_Parse(t *testing.T) {
	parser := NewStandardQueryParser()
	parser.SetDefaultField("content")

	tests := []struct {
		name    string
		query   string
		wantErr bool
	}{
		{
			name:    "simple term",
			query:   "hello",
			wantErr: false,
		},
		{
			name:    "fielded query",
			query:   "title:test",
			wantErr: false,
		},
		{
			name:    "boolean AND",
			query:   "a AND b",
			wantErr: false,
		},
		{
			name:    "boolean OR",
			query:   "a OR b",
			wantErr: false,
		},
		{
			name:    "empty query",
			query:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query, err := parser.Parse(tt.query)
			if tt.wantErr {
				if err == nil {
					t.Errorf("Parse() expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Parse() unexpected error: %v", err)
				}
				if query == nil {
					t.Error("Parse() returned nil query")
				}
			}
		})
	}
}

func TestStandardQueryParser_ParseSimpleTerm(t *testing.T) {
	parser := NewStandardQueryParser()
	parser.SetDefaultField("content")

	query, err := parser.Parse("hello")
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	if query == nil {
		t.Fatal("Parse() returned nil query")
	}

	// Should be a TermQuery
	if _, ok := query.(*search.TermQuery); !ok {
		t.Errorf("Expected TermQuery, got %T", query)
	}
}

func TestStandardQueryParser_ParseBooleanQuery(t *testing.T) {
	parser := NewStandardQueryParser()
	parser.SetDefaultField("content")

	query, err := parser.Parse("a AND b")
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	if query == nil {
		t.Fatal("Parse() returned nil query")
	}

	// Should be a BooleanQuery
	if _, ok := query.(*search.BooleanQuery); !ok {
		t.Errorf("Expected BooleanQuery, got %T", query)
	}
}

func TestStandardQueryParser_Setters(t *testing.T) {
	parser := NewStandardQueryParser()

	parser.SetDefaultField("title")
	if parser.GetConfig().GetDefaultField() != "title" {
		t.Errorf("SetDefaultField failed")
	}

	parser.SetDefaultOperator("AND")
	if parser.GetConfig().GetDefaultOperator() != "AND" {
		t.Errorf("SetDefaultOperator failed")
	}
}
