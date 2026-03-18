// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package grouping

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// GroupFieldCommand represents a command to group by a field.
// This is used to configure grouping operations.
//
// This is the Go port of Lucene's org.apache.lucene.search.grouping.GroupFieldCommand.
type GroupFieldCommand struct {
	// field is the field to group by
	field string

	// sort is the sort for groups
	sort search.Sort

	// topN is the maximum number of groups to return
	topN int

	// includeMaxScore indicates whether to include max score per group
	includeMaxScore bool
}

// NewGroupFieldCommand creates a new GroupFieldCommand.
//
// Parameters:
//   - field: the field to group by
//
// Returns:
//   - a new GroupFieldCommand instance
func NewGroupFieldCommand(field string) *GroupFieldCommand {
	return &GroupFieldCommand{
		field:           field,
		sort:            *search.NewSortByScore(),
		topN:            10,
		includeMaxScore: false,
	}
}

// GetField returns the field to group by.
func (gfc *GroupFieldCommand) GetField() string {
	return gfc.field
}

// SetSort sets the sort for groups.
func (gfc *GroupFieldCommand) SetSort(sort search.Sort) *GroupFieldCommand {
	gfc.sort = sort
	return gfc
}

// GetSort returns the sort for groups.
func (gfc *GroupFieldCommand) GetSort() search.Sort {
	return gfc.sort
}

// SetTopN sets the maximum number of groups to return.
func (gfc *GroupFieldCommand) SetTopN(topN int) *GroupFieldCommand {
	gfc.topN = topN
	return gfc
}

// GetTopN returns the maximum number of groups to return.
func (gfc *GroupFieldCommand) GetTopN() int {
	return gfc.topN
}

// SetIncludeMaxScore sets whether to include max score per group.
func (gfc *GroupFieldCommand) SetIncludeMaxScore(include bool) *GroupFieldCommand {
	gfc.includeMaxScore = include
	return gfc
}

// IsIncludeMaxScore returns whether to include max score per group.
func (gfc *GroupFieldCommand) IsIncludeMaxScore() bool {
	return gfc.includeMaxScore
}

// String returns a string representation of this command.
func (gfc *GroupFieldCommand) String() string {
	return fmt.Sprintf("GroupFieldCommand{field=%s, topN=%d}", gfc.field, gfc.topN)
}

// GroupFacetCommand represents a command to group by facets.
// This is used to configure facet-based grouping operations.
//
// This is the Go port of Lucene's org.apache.lucene.search.grouping.GroupFacetCommand.
type GroupFacetCommand struct {
	// field is the facet field to group by
	field string

	// sort is the sort for groups
	sort search.Sort

	// topN is the maximum number of groups to return
	topN int

	// includeMaxScore indicates whether to include max score per group
	includeMaxScore bool
}

// NewGroupFacetCommand creates a new GroupFacetCommand.
//
// Parameters:
//   - field: the facet field to group by
//
// Returns:
//   - a new GroupFacetCommand instance
func NewGroupFacetCommand(field string) *GroupFacetCommand {
	return &GroupFacetCommand{
		field:           field,
		sort:            *search.NewSortByScore(),
		topN:            10,
		includeMaxScore: false,
	}
}

// GetField returns the facet field to group by.
func (gfc *GroupFacetCommand) GetField() string {
	return gfc.field
}

// SetSort sets the sort for groups.
func (gfc *GroupFacetCommand) SetSort(sort search.Sort) *GroupFacetCommand {
	gfc.sort = sort
	return gfc
}

// GetSort returns the sort for groups.
func (gfc *GroupFacetCommand) GetSort() search.Sort {
	return gfc.sort
}

// SetTopN sets the maximum number of groups to return.
func (gfc *GroupFacetCommand) SetTopN(topN int) *GroupFacetCommand {
	gfc.topN = topN
	return gfc
}

// GetTopN returns the maximum number of groups to return.
func (gfc *GroupFacetCommand) GetTopN() int {
	return gfc.topN
}

// SetIncludeMaxScore sets whether to include max score per group.
func (gfc *GroupFacetCommand) SetIncludeMaxScore(include bool) *GroupFacetCommand {
	gfc.includeMaxScore = include
	return gfc
}

// IsIncludeMaxScore returns whether to include max score per group.
func (gfc *GroupFacetCommand) IsIncludeMaxScore() bool {
	return gfc.includeMaxScore
}

// String returns a string representation of this command.
func (gfc *GroupFacetCommand) String() string {
	return fmt.Sprintf("GroupFacetCommand{field=%s, topN=%d}", gfc.field, gfc.topN)
}

// GroupCommand is the interface for all grouping commands.
type GroupCommand interface {
	// GetTopN returns the maximum number of groups to return
	GetTopN() int

	// GetSort returns the sort for groups
	GetSort() search.Sort

	// IsIncludeMaxScore returns whether to include max score per group
	IsIncludeMaxScore() bool
}

// Ensure GroupFieldCommand implements GroupCommand
var _ GroupCommand = (*GroupFieldCommand)(nil)

// Ensure GroupFacetCommand implements GroupCommand
var _ GroupCommand = (*GroupFacetCommand)(nil)

// ValueSourceCommand represents a command to group by a ValueSource.
// This is used to configure value source-based grouping operations.
//
// This is the Go port of Lucene's org.apache.lucene.search.grouping.ValueSourceCommand.
type ValueSourceCommand struct {
	// valueSource is the ValueSource to group by
	valueSource ValueSource

	// sort is the sort for groups
	sort search.Sort

	// topN is the maximum number of groups to return
	topN int

	// includeMaxScore indicates whether to include max score per group
	includeMaxScore bool
}

// NewValueSourceCommand creates a new ValueSourceCommand.
//
// Parameters:
//   - valueSource: the ValueSource to group by
//
// Returns:
//   - a new ValueSourceCommand instance
func NewValueSourceCommand(valueSource ValueSource) *ValueSourceCommand {
	return &ValueSourceCommand{
		valueSource:     valueSource,
		sort:            *search.NewSortByScore(),
		topN:            10,
		includeMaxScore: false,
	}
}

// GetValueSource returns the ValueSource to group by.
func (vsc *ValueSourceCommand) GetValueSource() ValueSource {
	return vsc.valueSource
}

// SetSort sets the sort for groups.
func (vsc *ValueSourceCommand) SetSort(sort search.Sort) *ValueSourceCommand {
	vsc.sort = sort
	return vsc
}

// GetSort returns the sort for groups.
func (vsc *ValueSourceCommand) GetSort() search.Sort {
	return vsc.sort
}

// SetTopN sets the maximum number of groups to return.
func (vsc *ValueSourceCommand) SetTopN(topN int) *ValueSourceCommand {
	vsc.topN = topN
	return vsc
}

// GetTopN returns the maximum number of groups to return.
func (vsc *ValueSourceCommand) GetTopN() int {
	return vsc.topN
}

// SetIncludeMaxScore sets whether to include max score per group.
func (vsc *ValueSourceCommand) SetIncludeMaxScore(include bool) *ValueSourceCommand {
	vsc.includeMaxScore = include
	return vsc
}

// IsIncludeMaxScore returns whether to include max score per group.
func (vsc *ValueSourceCommand) IsIncludeMaxScore() bool {
	return vsc.includeMaxScore
}

// String returns a string representation of this command.
func (vsc *ValueSourceCommand) String() string {
	return fmt.Sprintf("ValueSourceCommand{valueSource=%s, topN=%d}", vsc.valueSource.Description(), vsc.topN)
}

// Ensure ValueSourceCommand implements GroupCommand
var _ GroupCommand = (*ValueSourceCommand)(nil)
