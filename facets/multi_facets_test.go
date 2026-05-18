package facets

import "testing"

type stubFacets struct {
	dim string
}

func (s *stubFacets) GetTopChildren(_ int, dim string, _ ...string) (*FacetResult, error) {
	return &FacetResult{Dim: dim}, nil
}
func (s *stubFacets) GetAllDims(_ ...string) ([]*FacetResult, error) {
	return []*FacetResult{{Dim: s.dim}}, nil
}
func (s *stubFacets) GetSpecificValue(dim string, _ ...string) (*FacetResult, error) {
	return &FacetResult{Dim: dim}, nil
}

func TestMultiFacetsRouting(t *testing.T) {
	mf := NewMultiFacets()
	mf.AddFacets("color", &stubFacets{dim: "color"})
	mf.AddFacets("size", &stubFacets{dim: "size"})
	if mf.GetFacetsForDim("color") == nil {
		t.Error("color missing")
	}
	if mf.GetFacetsForDim("missing") != nil {
		t.Error("missing should be nil")
	}
	res, err := mf.GetTopChildren(10, "color")
	if err != nil || res == nil || res.Dim != "color" {
		t.Errorf("GetTopChildren = %v %v", res, err)
	}
}

func TestMultiFacetsFromMap(t *testing.T) {
	mf := NewMultiFacetsFromMap(nil)
	if mf.GetFacetsForDim("anything") != nil {
		t.Error("nil-init should have no facets")
	}
}
