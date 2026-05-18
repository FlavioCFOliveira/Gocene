package taxonomy

// AssociationFacetField is the abstract per-document field that ships a facet
// label together with a byte-payload association (typically an int32 or
// float32 value the aggregator interprets). Mirrors
// org.apache.lucene.facet.taxonomy.AssociationFacetField.
type AssociationFacetField struct {
	// Dim is the facet dimension.
	Dim string

	// Path is the hierarchical path under Dim.
	Path []string

	// Association is the raw byte payload attached to (Dim, Path).
	Association []byte
}

// NewAssociationFacetField builds an AssociationFacetField.
func NewAssociationFacetField(dim string, path []string, association []byte) *AssociationFacetField {
	clonePath := make([]string, len(path))
	copy(clonePath, path)
	cloneAssoc := make([]byte, len(association))
	copy(cloneAssoc, association)
	return &AssociationFacetField{
		Dim:         dim,
		Path:        clonePath,
		Association: cloneAssoc,
	}
}

// GetDim returns the dimension.
func (f *AssociationFacetField) GetDim() string { return f.Dim }

// GetPath returns the path components.
func (f *AssociationFacetField) GetPath() []string { return f.Path }

// GetAssociation returns the raw payload bytes.
func (f *AssociationFacetField) GetAssociation() []byte { return f.Association }
