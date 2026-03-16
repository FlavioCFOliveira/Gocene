package facets

import (
	"fmt"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/document"
)

// FacetField is a field that represents a facet value for a document.
// This is used to index facet values that can be counted and aggregated.
//
// This is the Go port of Lucene's org.apache.lucene.facet.FacetField.
type FacetField struct {
	// dim is the dimension/facet field name (e.g., "category", "author")
	dim string

	// path is the hierarchical path for this facet (e.g., ["electronics", "phones"])
	path []string

	// value is the actual facet value at the leaf of the path
	value string
}

// NewFacetField creates a new FacetField with the given dimension and value.
// For non-hierarchical facets, use this constructor.
func NewFacetField(dim string, value string) *FacetField {
	return &FacetField{
		dim:   dim,
		path:  []string{},
		value: value,
	}
}

// NewFacetFieldWithPath creates a new FacetField with a hierarchical path.
// For hierarchical facets, use this constructor.
func NewFacetFieldWithPath(dim string, path []string, value string) *FacetField {
	ff := &FacetField{
		dim:   dim,
		value: value,
	}
	if len(path) > 0 {
		ff.path = make([]string, len(path))
		copy(ff.path, path)
	}
	return ff
}

// GetDim returns the dimension of this facet field.
func (ff *FacetField) GetDim() string {
	return ff.dim
}

// GetPath returns the hierarchical path of this facet field.
func (ff *FacetField) GetPath() []string {
	return ff.path
}

// GetValue returns the leaf value of this facet field.
func (ff *FacetField) GetValue() string {
	return ff.value
}

// GetFullPath returns the complete path including the value.
func (ff *FacetField) GetFullPath() []string {
	fullPath := make([]string, len(ff.path)+1)
	copy(fullPath, ff.path)
	fullPath[len(ff.path)] = ff.value
	return fullPath
}

// GetPathString returns the path as a single string with separator.
func (ff *FacetField) GetPathString(separator string) string {
	if len(ff.path) == 0 {
		return ff.value
	}
	return strings.Join(ff.path, separator) + separator + ff.value
}

// Validate validates that this facet field is properly configured.
func (ff *FacetField) Validate() error {
	if ff.dim == "" {
		return fmt.Errorf("dimension cannot be empty")
	}
	if ff.value == "" {
		return fmt.Errorf("value cannot be empty")
	}
	return nil
}

// String returns a string representation of this FacetField.
func (ff *FacetField) String() string {
	if len(ff.path) > 0 {
		return fmt.Sprintf("%s/%s=%s", ff.dim, strings.Join(ff.path, "/"), ff.value)
	}
	return fmt.Sprintf("%s=%s", ff.dim, ff.value)
}

// ToIndexField converts this FacetField to an IndexableField for indexing.
// This creates the appropriate field type based on the facet configuration.
func (ff *FacetField) ToIndexField(config *FacetsConfig) (document.IndexableField, error) {
	if err := ff.Validate(); err != nil {
		return nil, err
	}

	// Get the index field name from config
	indexFieldName := ff.dim
	if config != nil {
		indexFieldName = config.GetIndexFieldName(ff.dim)
	}

	// Create the field value (full path for hierarchical facets)
	fieldValue := ff.value
	if len(ff.path) > 0 {
		fieldValue = ff.GetPathString("/")
	}

	// Create a string field for indexing
	field, err := document.NewStringField(indexFieldName, fieldValue, true)
	if err != nil {
		return nil, fmt.Errorf("failed to create index field: %w", err)
	}

	return field, nil
}

// ToDocValuesField converts this FacetField to a DocValuesField.
// This is used for efficient facet counting at search time.
func (ff *FacetField) ToDocValuesField(config *FacetsConfig) (document.IndexableField, error) {
	if err := ff.Validate(); err != nil {
		return nil, err
	}

	// Get the index field name from config
	indexFieldName := ff.dim
	if config != nil {
		indexFieldName = config.GetIndexFieldName(ff.dim)
	}

	// Create the field value (full path for hierarchical facets)
	fieldValue := []byte(ff.value)
	if len(ff.path) > 0 {
		fieldValue = []byte(ff.GetPathString("/"))
	}

	// Check if multi-valued
	isMultiValued := false
	if config != nil {
		isMultiValued = config.IsMultiValued(ff.dim)
	}

	var field document.IndexableField
	var err error

	if isMultiValued {
		// Use SortedSetDocValues for multi-valued facets
		field, err = document.NewSortedSetDocValuesField(indexFieldName, [][]byte{fieldValue})
	} else {
		// Use SortedDocValues for single-valued facets
		field, err = document.NewSortedDocValuesField(indexFieldName, fieldValue)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create doc values field: %w", err)
	}

	return field, nil
}

// FacetFieldComparator compares two FacetFields for sorting.
type FacetFieldComparator struct{}

// Compare compares two FacetFields by dimension and path.
func (ffc *FacetFieldComparator) Compare(a, b *FacetField) int {
	// Compare dimensions first
	if a.dim != b.dim {
		if a.dim < b.dim {
			return -1
		}
		return 1
	}

	// Compare paths
	minLen := len(a.path)
	if len(b.path) < minLen {
		minLen = len(b.path)
	}

	for i := 0; i < minLen; i++ {
		if a.path[i] != b.path[i] {
			if a.path[i] < b.path[i] {
				return -1
			}
			return 1
		}
	}

	// If paths are equal up to minLen, shorter path comes first
	if len(a.path) != len(b.path) {
		if len(a.path) < len(b.path) {
			return -1
		}
		return 1
	}

	// Compare values
	if a.value != b.value {
		if a.value < b.value {
			return -1
		}
		return 1
	}

	return 0
}

// FacetFields is a collection of FacetField instances.
type FacetFields struct {
	fields []*FacetField
}

// NewFacetFields creates a new empty FacetFields collection.
func NewFacetFields() *FacetFields {
	return &FacetFields{
		fields: make([]*FacetField, 0),
	}
}

// Add adds a FacetField to this collection.
func (ffs *FacetFields) Add(field *FacetField) {
	ffs.fields = append(ffs.fields, field)
}

// Get returns the FacetField at the given index.
func (ffs *FacetFields) Get(index int) *FacetField {
	if index < 0 || index >= len(ffs.fields) {
		return nil
	}
	return ffs.fields[index]
}

// Size returns the number of facet fields in this collection.
func (ffs *FacetFields) Size() int {
	return len(ffs.fields)
}

// GetByDim returns all FacetFields for the given dimension.
func (ffs *FacetFields) GetByDim(dim string) []*FacetField {
	result := make([]*FacetField, 0)
	for _, field := range ffs.fields {
		if field.GetDim() == dim {
			result = append(result, field)
		}
	}
	return result
}

// ToIndexFields converts all FacetFields to IndexableFields.
func (ffs *FacetFields) ToIndexFields(config *FacetsConfig) ([]document.IndexableField, error) {
	fields := make([]document.IndexableField, 0, len(ffs.fields))
	for _, ff := range ffs.fields {
		field, err := ff.ToIndexField(config)
		if err != nil {
			return nil, err
		}
		fields = append(fields, field)
	}
	return fields, nil
}

// ToDocValuesFields converts all FacetFields to DocValuesFields.
func (ffs *FacetFields) ToDocValuesFields(config *FacetsConfig) ([]document.IndexableField, error) {
	fields := make([]document.IndexableField, 0, len(ffs.fields))
	for _, ff := range ffs.fields {
		field, err := ff.ToDocValuesField(config)
		if err != nil {
			return nil, err
		}
		fields = append(fields, field)
	}
	return fields, nil
}

// FacetFieldBuilder provides a fluent API for building FacetFields.
type FacetFieldBuilder struct {
	dim   string
	path  []string
	value string
}

// NewFacetFieldBuilder creates a new FacetFieldBuilder.
func NewFacetFieldBuilder() *FacetFieldBuilder {
	return &FacetFieldBuilder{
		path: make([]string, 0),
	}
}

// SetDim sets the dimension.
func (ffb *FacetFieldBuilder) SetDim(dim string) *FacetFieldBuilder {
	ffb.dim = dim
	return ffb
}

// AddPathComponent adds a component to the hierarchical path.
func (ffb *FacetFieldBuilder) AddPathComponent(component string) *FacetFieldBuilder {
	ffb.path = append(ffb.path, component)
	return ffb
}

// SetPath sets the hierarchical path.
func (ffb *FacetFieldBuilder) SetPath(path []string) *FacetFieldBuilder {
	ffb.path = make([]string, len(path))
	copy(ffb.path, path)
	return ffb
}

// SetValue sets the leaf value.
func (ffb *FacetFieldBuilder) SetValue(value string) *FacetFieldBuilder {
	ffb.value = value
	return ffb
}

// Build builds the FacetField.
func (ffb *FacetFieldBuilder) Build() (*FacetField, error) {
	ff := NewFacetFieldWithPath(ffb.dim, ffb.path, ffb.value)
	if err := ff.Validate(); err != nil {
		return nil, err
	}
	return ff, nil
}
