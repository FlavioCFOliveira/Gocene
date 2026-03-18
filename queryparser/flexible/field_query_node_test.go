package flexible

import (
	"testing"
)

func TestFieldQueryNode_New(t *testing.T) {
	node := NewFieldQueryNode("title", "test", 0, 10)

	if node.GetField() != "title" {
		t.Errorf("GetField() = %v, want title", node.GetField())
	}

	if node.GetText() != "test" {
		t.Errorf("GetText() = %v, want test", node.GetText())
	}

	if node.GetBegin() != 0 {
		t.Errorf("GetBegin() = %v, want 0", node.GetBegin())
	}

	if node.GetEnd() != 10 {
		t.Errorf("GetEnd() = %v, want 10", node.GetEnd())
	}
}

func TestFieldQueryNode_Setters(t *testing.T) {
	node := NewFieldQueryNode("", "", 0, 0)

	node.SetField("content")
	if node.GetField() != "content" {
		t.Errorf("SetField failed, got %v", node.GetField())
	}

	node.SetText("value")
	if node.GetText() != "value" {
		t.Errorf("SetText failed, got %v", node.GetText())
	}

	node.SetBegin(5)
	if node.GetBegin() != 5 {
		t.Errorf("SetBegin failed, got %v", node.GetBegin())
	}

	node.SetEnd(15)
	if node.GetEnd() != 15 {
		t.Errorf("SetEnd failed, got %v", node.GetEnd())
	}

	node.SetPosition(2)
	if node.GetPosition() != 2 {
		t.Errorf("SetPosition failed, got %v", node.GetPosition())
	}
}

func TestFieldQueryNode_ToQueryString(t *testing.T) {
	tests := []struct {
		name                string
		field               string
		text                string
		escapeSpecialSyntax bool
		want                string
	}{
		{
			name:                "simple field and text",
			field:               "title",
			text:                "test",
			escapeSpecialSyntax: false,
			want:                "title:test",
		},
		{
			name:                "empty field",
			field:               "",
			text:                "test",
			escapeSpecialSyntax: false,
			want:                "test",
		},
		{
			name:                "text with special chars",
			field:               "title",
			text:                "test:value",
			escapeSpecialSyntax: true,
			want:                "title:test\\:value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := NewFieldQueryNode(tt.field, tt.text, 0, 0)
			got := node.ToQueryString(tt.escapeSpecialSyntax)
			if got != tt.want {
				t.Errorf("ToQueryString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFieldQueryNode_CloneTree(t *testing.T) {
	original := NewFieldQueryNode("title", "test", 0, 10)
	original.SetTag("key", "value")

	cloned := original.CloneTree()
	clonedNode := cloned.(*FieldQueryNode)

	// Verify fields are copied
	if clonedNode.GetField() != original.GetField() {
		t.Error("Clone should have same field")
	}

	if clonedNode.GetText() != original.GetText() {
		t.Error("Clone should have same text")
	}

	if clonedNode.GetBegin() != original.GetBegin() {
		t.Error("Clone should have same begin")
	}

	if clonedNode.GetEnd() != original.GetEnd() {
		t.Error("Clone should have same end")
	}

	// Verify tags are copied
	if clonedNode.GetTag("key") != "value" {
		t.Error("Clone should have copied tags")
	}

	// Verify it's a different object
	if cloned == original {
		t.Error("Clone should be a different object")
	}

	// Verify modifications don't affect original
	clonedNode.SetField("content")
	if original.GetField() == "content" {
		t.Error("Modifying clone should not affect original")
	}
}

func TestFieldQueryNode_String(t *testing.T) {
	node := NewFieldQueryNode("title", "test", 0, 10)
	result := node.String()

	expected := "<field start=0 end=10 field=title text=test>"
	if result != expected {
		t.Errorf("String() = %v, want %v", result, expected)
	}
}
