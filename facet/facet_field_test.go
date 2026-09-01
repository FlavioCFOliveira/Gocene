package facet

import (
	"testing"
)

func TestNewFacetField(t *testing.T) {
	tests := []struct {
		name    string
		dim     string
		path    []string
		wantErr bool
	}{
		{
			name:    "valid field",
			dim:     "category",
			path:    []string{"electronics", "phones"},
			wantErr: false,
		},
		{
			name:    "empty dim",
			dim:     "",
			path:    []string{"electronics"},
			wantErr: true,
		},
		{
			name:    "empty path element",
			dim:     "category",
			path:    []string{"electronics", ""},
			wantErr: true,
		},
		{
			name:    "empty path",
			dim:     "category",
			path:    []string{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ff, err := NewFacetField(tt.dim, tt.path...)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewFacetField() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if ff.Dim() != tt.dim {
					t.Errorf("expected Dim %s, got %s", tt.dim, ff.Dim())
				}
				if len(ff.Path()) != len(tt.path) {
					t.Errorf("expected Path length %d, got %d", len(tt.path), len(ff.Path()))
				}
				for i, p := range tt.path {
					if ff.Path()[i] != p {
						t.Errorf("expected Path[%d] %s, got %s", i, p, ff.Path()[i])
					}
				}
			}
		})
	}
}

func TestFacetField_String(t *testing.T) {
	ff, _ := NewFacetField("category", "electronics", "phones")
	want := "FacetField(dim=category path=[electronics phones])"
	if got := ff.String(); got != want {
		t.Errorf("FacetField.String() = %v, want %v", got, want)
	}
}

func TestVerifyLabel(t *testing.T) {
	tests := []struct {
		label   string
		wantErr bool
	}{
		{"valid", false},
		{"", true},
	}
	for _, tt := range tests {
		err := VerifyLabel(tt.label)
		if (err != nil) != tt.wantErr {
			t.Errorf("VerifyLabel(%q) error = %v, wantErr %v", tt.label, err, tt.wantErr)
		}
	}
}
