// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facets

// MultiFacets fans facet lookups out to a map of dimension-named Facets
// implementations. Mirrors org.apache.lucene.facet.MultiFacets: callers
// register one Facets per dimension and queries are routed to the matching
// instance.
type MultiFacets struct {
	dimToFacets map[string]Facets
}

// NewMultiFacets builds an empty MultiFacets.
func NewMultiFacets() *MultiFacets {
	return &MultiFacets{dimToFacets: make(map[string]Facets)}
}

// NewMultiFacetsFromMap builds a MultiFacets backed by the supplied map.
// The map is referenced — not copied — so the caller retains ownership.
func NewMultiFacetsFromMap(m map[string]Facets) *MultiFacets {
	if m == nil {
		m = make(map[string]Facets)
	}
	return &MultiFacets{dimToFacets: m}
}

// AddFacets registers the Facets instance for the supplied dimension. A
// subsequent call with the same dimension replaces the entry.
func (m *MultiFacets) AddFacets(dim string, facets Facets) {
	m.dimToFacets[dim] = facets
}

// GetFacetsForDim returns the Facets instance registered for dim, or nil if
// none has been registered.
func (m *MultiFacets) GetFacetsForDim(dim string) Facets {
	return m.dimToFacets[dim]
}

// GetTopChildren delegates to the dimension-specific Facets.
func (m *MultiFacets) GetTopChildren(topN int, dim string, path ...string) (*FacetResult, error) {
	f := m.dimToFacets[dim]
	if f == nil {
		return nil, nil
	}
	return f.GetTopChildren(topN, dim, path...)
}

// GetSpecificValue delegates to the dimension-specific Facets.
func (m *MultiFacets) GetSpecificValue(dim string, path ...string) (*FacetResult, error) {
	f := m.dimToFacets[dim]
	if f == nil {
		return nil, nil
	}
	return f.GetSpecificValue(dim, path...)
}

// GetAllDims returns a FacetResult per registered dimension, optionally
// filtered to the supplied subset.
func (m *MultiFacets) GetAllDims(dims ...string) ([]*FacetResult, error) {
	wanted := make(map[string]bool, len(dims))
	for _, d := range dims {
		wanted[d] = true
	}
	out := make([]*FacetResult, 0, len(m.dimToFacets))
	for dim, f := range m.dimToFacets {
		if len(wanted) > 0 && !wanted[dim] {
			continue
		}
		results, err := f.GetAllDims()
		if err != nil {
			return nil, err
		}
		out = append(out, results...)
	}
	return out, nil
}
