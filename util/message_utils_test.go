package util

import (
	"testing"
)

func TestGetLocalizedMessage(t *testing.T) {
	tests := []struct {
		key      string
		expected string
	}{
		{"label.status", "Status:"},
		{"window.title", "Luke: Lucene Toolbox Project"},
		{"non.existent", "non.existent"},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			if got := GetLocalizedMessage(tt.key); got != tt.expected {
				t.Errorf("GetLocalizedMessage(%q) = %q, want %q", tt.key, got, tt.expected)
			}
		})
	}
}

func TestGetLocalizedMessageWithArgs(t *testing.T) {
	tests := []struct {
		key      string
		args     []interface{}
		expected string
	}{
		{"openindex.message.index_path_invalid", []interface{}{"/tmp/badindex"}, "Cannot open index path /tmp/badindex. Not a valid lucene index directory or corrupted?"},
		{"export.terms.label.success", []interface{}{"/out/terms", "txt"}, "<html>Terms successfully exported to: <br>/out/terms<br><br>Output format is: txt</html>"},
		{"documents.termvector.message.not_available", []interface{}{"field1", 123}, "Term vector for field1 field in doc #123 not available."},
		{"non.existent", []interface{}{"arg"}, "non.existent"},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			if got := GetLocalizedMessageWithArgs(tt.key, tt.args...); got != tt.expected {
				t.Errorf("GetLocalizedMessageWithArgs(%q, %v) = %q, want %q", tt.key, tt.args, got, tt.expected)
			}
		})
	}
}
