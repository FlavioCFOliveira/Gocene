package flexible

import (
	"testing"
)

func TestAnyQueryNode_QueryNodePort(t *testing.T) {
	clauses := []QueryNode{
		NewFieldQueryNode("field1", "term1", 0, 5),
		NewFieldQueryNode("field1", "term2", 6, 11),
	}
	anyNode := NewAnyQueryNode(clauses, "field1", 1)

	expectedStr := "<any field='field1' matchelements=1>\n<field start=0 end=5 field=field1 text=term1>\n<field start=6 end=11 field=field1 text=term2>\n</any>"
	if anyNode.String() != expectedStr {
		t.Errorf("Expected String() %q, got %q", expectedStr, anyNode.String())
	}

	expectedQS := "field1:((term1 term2 ) ANY 1)"
	if anyNode.ToQueryString(NewEscapeQuerySyntaxImpl()) != expectedQS {
		t.Errorf("Expected ToQueryString() %q, got %q", expectedQS, anyNode.ToQueryString(NewEscapeQuerySyntaxImpl()))
	}

	// Test with default field
	anyNodeDefault := NewAnyQueryNode(clauses, "", 1)
	expectedQSDefault := "( term1 term2 ) ANY 1"
	if anyNodeDefault.ToQueryString(NewEscapeQuerySyntaxImpl()) != expectedQSDefault {
		t.Errorf("Expected ToQueryString() %q, got %q", expectedQSDefault, anyNodeDefault.ToQueryString(NewEscapeQuerySyntaxImpl()))
	}
}

func TestQuotedFieldQueryNode_QueryNodePort(t *testing.T) {
	node := NewQuotedFieldQueryNode("field1", "life is great", 0, 13)
	expectedStr := "<quotedfield start=0 end=13 field=field1 term=life is great>"
	if node.String() != expectedStr {
		t.Errorf("Expected String() %q, got %q", expectedStr, node.String())
	}

	expectedQS := "field1:\"life is great\""
	if node.ToQueryString(NewEscapeQuerySyntaxImpl()) != expectedQS {
		t.Errorf("Expected ToQueryString() %q, got %q", expectedQS, node.ToQueryString(NewEscapeQuerySyntaxImpl()))
	}

	nodeDefault := NewQuotedFieldQueryNode("", "life is great", 0, 13)
	expectedQSDefault := "\"life is great\""
	if nodeDefault.ToQueryString(NewEscapeQuerySyntaxImpl()) != expectedQSDefault {
		t.Errorf("Expected ToQueryString() %q, got %q", expectedQSDefault, nodeDefault.ToQueryString(NewEscapeQuerySyntaxImpl()))
	}
}

func TestBooleanQueryNodes(t *testing.T) {
	clauses := []QueryNode{
		NewFieldQueryNode("f1", "t1", 0, 3),
		NewFieldQueryNode("f1", "t2", 4, 7),
	}

	t.Run("BooleanDefault", func(t *testing.T) {
		node := NewBooleanQueryNode("default", clauses)
		expectedQS := "f1:t1 f1:t2"
		if node.ToQueryString(NewEscapeQuerySyntaxImpl()) != expectedQS {
			t.Errorf("Expected %q, got %q", expectedQS, node.ToQueryString(NewEscapeQuerySyntaxImpl()))
		}
	})

	t.Run("AndQuery", func(t *testing.T) {
		node := NewAndQueryNode(clauses)
		expectedQS := "f1:t1 AND f1:t2"
		if node.ToQueryString(NewEscapeQuerySyntaxImpl()) != expectedQS {
			t.Errorf("Expected %q, got %q", expectedQS, node.ToQueryString(NewEscapeQuerySyntaxImpl()))
		}
	})

	t.Run("OrQuery", func(t *testing.T) {
		node := NewOrQueryNode(clauses)
		expectedQS := "f1:t1 OR f1:t2"
		if node.ToQueryString(NewEscapeQuerySyntaxImpl()) != expectedQS {
			t.Errorf("Expected %q, got %q", expectedQS, node.ToQueryString(NewEscapeQuerySyntaxImpl()))
		}
	})
}

func TestBooleanQueryNodeParentheses(t *testing.T) {
	clauses := []QueryNode{
		NewFieldQueryNode("f1", "t1", 0, 3),
	}
	node := NewBooleanQueryNode("AND", clauses)

	// Root node -> no parentheses
	if node.ToQueryString(NewEscapeQuerySyntaxImpl()) != "f1:t1" {
		t.Errorf("Root should have no parentheses, got %q", node.ToQueryString(NewEscapeQuerySyntaxImpl()))
	}

	// Parent is GroupQueryNode -> no parentheses
	NewGroupQueryNode(node)
	if node.ToQueryString(NewEscapeQuerySyntaxImpl()) != "f1:t1" {
		t.Errorf("Child of GroupQueryNode should have no parentheses, got %q", node.ToQueryString(NewEscapeQuerySyntaxImpl()))
	}

	// Parent is another BooleanQueryNode -> parentheses
	NewBooleanQueryNode("OR", []QueryNode{node})
	if node.ToQueryString(NewEscapeQuerySyntaxImpl()) != "( f1:t1 )" {
		t.Errorf("Child of BooleanQueryNode should have parentheses, got %q", node.ToQueryString(NewEscapeQuerySyntaxImpl()))
	}
}
