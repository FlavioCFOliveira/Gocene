// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facet

import (
	"fmt"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// DrillDownTermsIndexing controls whether dimension and sub-path terms should be indexed.
type DrillDownTermsIndexing int

const (
	DrillDownTermsIndexingNone DrillDownTermsIndexing = iota
	DrillDownTermsIndexingFullPathOnly
	DrillDownTermsIndexingAllPathsNoDim
	DrillDownTermsIndexingDimensionAndFullPath
	DrillDownTermsIndexingAll
)

// DimConfig holds the configuration for one dimension.
type DimConfig struct {
	Hierarchical            bool
	MultiValued             bool
	RequireDimCount         bool
	DrillDownTermsIndexing  DrillDownTermsIndexing
	IndexFieldName          string
}

// FacetsConfig records per-dimension configuration.
//
// This is the Go port of Lucene's org.apache.lucene.facet.FacetsConfig.
type FacetsConfig struct {
	fieldTypes map[string]*DimConfig
	mu         sync.RWMutex
}

const DefaultIndexFieldName = "$facets"

var DefaultDimConfig = &DimConfig{
	DrillDownTermsIndexing: DrillDownTermsIndexingAll,
	IndexFieldName:         DefaultIndexFieldName,
}

// NewFacetsConfig creates a new FacetsConfig.
func NewFacetsConfig() *FacetsConfig {
	return &FacetsConfig{
		fieldTypes: make(map[string]*DimConfig),
	}
}

func (c *FacetsConfig) getDefaultDimConfig() *DimConfig {
	return DefaultDimConfig
}

// GetDimConfig returns the current configuration for a dimension.
func (c *FacetsConfig) GetDimConfig(dimName string) *DimConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if dimConfig, ok := c.fieldTypes[dimName]; ok {
		return dimConfig
	}
	return c.getDefaultDimConfig()
}

// IsDimConfigured returns true if the dimension for provided name has ever been manually configured.
func (c *FacetsConfig) IsDimConfigured(dimName string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, ok := c.fieldTypes[dimName]
	return ok
}

func (c *FacetsConfig) setDimConfig(dimName string) *DimConfig {
	if dimConfig, ok := c.fieldTypes[dimName]; ok {
		return dimConfig
	}
	dimConfig := &DimConfig{
		DrillDownTermsIndexing: DrillDownTermsIndexingAll,
		IndexFieldName:         DefaultIndexFieldName,
	}
	c.fieldTypes[dimName] = dimConfig
	return dimConfig
}

func (c *FacetsConfig) SetHierarchical(dimName string, v bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.setDimConfig(dimName).Hierarchical = v
}

func (c *FacetsConfig) SetMultiValued(dimName string, value bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.setDimConfig(dimName).MultiValued = value
}

func (c *FacetsConfig) SetRequireDimCount(dimName string, value bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.setDimConfig(dimName).RequireDimCount = value
}

func (c *FacetsConfig) SetIndexFieldName(dimName string, indexFieldName string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.setDimConfig(dimName).IndexFieldName = indexFieldName
}

func (c *FacetsConfig) SetDrillDownTermsIndexing(dimName string, drillDownTermsIndexing DrillDownTermsIndexing) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.setDimConfig(dimName).DrillDownTermsIndexing = drillDownTermsIndexing
}

func (c *FacetsConfig) GetDimConfigs() map[string]*DimConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	// Return a copy to avoid concurrent modification
	res := make(map[string]*DimConfig)
	for k, v := range c.fieldTypes {
		res[k] = v
	}
	return res
}

// PathToString turns a dim + path into an encoded string.
func PathToString(dim string, path []string) string {
	return PathToStringSlice(append([]string{dim}, path...))
}

// PathToStringSlice turns the first length elements of path into an encoded string.
func PathToStringSlice(path []string) string {
	if len(path) == 0 {
		return ""
	}
	var sb strings.Builder
	for i, s := range path {
		if s == "" {
			panic("each path component must have length > 0 (got: \"\")")
		}
		for _, ch := range s {
			if ch == DelimChar || ch == EscapeChar {
				sb.WriteRune(EscapeChar)
			}
			sb.WriteRune(ch)
		}
		if i < len(path)-1 {
			sb.WriteRune(DelimChar)
		}
	}
	return sb.String()
}

const DelimChar = ''
const EscapeChar = ''

// StringToPath turns an encoded string back into the original String slice.
func StringToPath(s string) []string {
	if s == "" {
		return []string{}
	}
	var parts []string
	var buffer strings.Builder
	lastEscape := false
	for _, ch := range s {
		if lastEscape {
			buffer.WriteRune(ch)
			lastEscape = false
		} else if ch == EscapeChar {
			lastEscape = true
		} else if ch == DelimChar {
			parts = append(parts, buffer.String())
			buffer.Reset()
		} else {
			buffer.WriteRune(ch)
		}
	}
	parts = append(parts, buffer.String())
	return parts
}
