// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package flexible

import (
	"time"
)

// CommonQueryParserConfiguration is the interface that all standard query parser
// configuration objects must implement. It exposes the most frequently used
// parser settings as first-class methods.
// This is the Go equivalent of Lucene's CommonQueryParserConfiguration.
type CommonQueryParserConfiguration interface {
	// SetEnablePositionIncrements enables or disables position increments.
	SetEnablePositionIncrements(enable bool)
	// GetEnablePositionIncrements returns whether position increments are enabled.
	GetEnablePositionIncrements() bool

	// SetAllowLeadingWildcard enables or disables leading wildcards (* or ?).
	SetAllowLeadingWildcard(allow bool)
	// GetAllowLeadingWildcard returns whether leading wildcards are allowed.
	GetAllowLeadingWildcard() bool

	// SetLowercaseExpandedTerms controls whether expanded terms are lowercased.
	SetLowercaseExpandedTerms(lowercase bool)
	// GetLowercaseExpandedTerms returns whether expanded terms are lowercased.
	GetLowercaseExpandedTerms() bool

	// SetPhraseSlop sets the default slop for phrase queries.
	SetPhraseSlop(slop int)
	// GetPhraseSlop returns the default phrase slop.
	GetPhraseSlop() int

	// SetFuzzyMinSim sets the default minimum similarity for fuzzy queries.
	SetFuzzyMinSim(minSim float32)
	// GetFuzzyMinSim returns the default fuzzy minimum similarity.
	GetFuzzyMinSim() float32

	// SetFuzzyPrefixLength sets the prefix length for fuzzy queries.
	SetFuzzyPrefixLength(prefixLength int)
	// GetFuzzyPrefixLength returns the fuzzy prefix length.
	GetFuzzyPrefixLength() int
}

// FuzzyConfig holds fuzzy-query configuration: minimum similarity and prefix length.
// This is the Go equivalent of Lucene's FuzzyConfig.
type FuzzyConfig struct {
	prefixLength  int
	minSimilarity float32
}

// NewFuzzyConfig creates a FuzzyConfig with default values (minSim=2.0, prefixLen=0).
func NewFuzzyConfig() *FuzzyConfig {
	return &FuzzyConfig{prefixLength: 0, minSimilarity: 2.0}
}

// GetPrefixLength returns the prefix length.
func (c *FuzzyConfig) GetPrefixLength() int { return c.prefixLength }

// SetPrefixLength sets the prefix length.
func (c *FuzzyConfig) SetPrefixLength(length int) { c.prefixLength = length }

// GetMinSimilarity returns the minimum similarity threshold.
func (c *FuzzyConfig) GetMinSimilarity() float32 { return c.minSimilarity }

// SetMinSimilarity sets the minimum similarity threshold.
func (c *FuzzyConfig) SetMinSimilarity(minSim float32) { c.minSimilarity = minSim }

// NumberDateFormat is a date-formatting bridge that converts Date values to
// numeric strings suitable for range query bounds.
// This is the Go equivalent of Lucene's NumberDateFormat.
type NumberDateFormat struct {
	layout string
}

// DefaultNumberDateLayout is the default layout used when none is specified.
const DefaultNumberDateLayout = time.RFC3339

// NewNumberDateFormat creates a NumberDateFormat with the given layout.
func NewNumberDateFormat(layout string) *NumberDateFormat {
	if layout == "" {
		layout = DefaultNumberDateLayout
	}
	return &NumberDateFormat{layout: layout}
}

// Format formats the given time.Time to a numeric string.
func (f *NumberDateFormat) Format(t time.Time) string {
	return t.UTC().Format(f.layout)
}

// Parse parses a date string using the configured layout.
func (f *NumberDateFormat) Parse(s string) (time.Time, error) {
	return time.Parse(f.layout, s)
}

// GetLayout returns the date layout.
func (f *NumberDateFormat) GetLayout() string { return f.layout }

// PointsConfig holds per-field numeric configuration for point-based range queries.
// This is the Go equivalent of Lucene's PointsConfig.
type PointsConfig struct {
	type_       PointsType
	numDims     int
	bytesPerDim int
}

// PointsType represents the Java numeric type for a points field.
type PointsType int

const (
	// PointsTypeInt represents an int32 field.
	PointsTypeInt PointsType = iota
	// PointsTypeLong represents an int64 field.
	PointsTypeLong
	// PointsTypeFloat represents a float32 field.
	PointsTypeFloat
	// PointsTypeDouble represents a float64 field.
	PointsTypeDouble
)

// NewPointsConfig creates a PointsConfig for the given type and number of dimensions.
func NewPointsConfig(pointsType PointsType, numDims int) *PointsConfig {
	bytesPerDim := 4
	switch pointsType {
	case PointsTypeLong, PointsTypeDouble:
		bytesPerDim = 8
	}
	return &PointsConfig{
		type_:       pointsType,
		numDims:     numDims,
		bytesPerDim: bytesPerDim,
	}
}

// GetType returns the numeric type.
func (c *PointsConfig) GetType() PointsType { return c.type_ }

// GetNumDims returns the number of dimensions.
func (c *PointsConfig) GetNumDims() int { return c.numDims }

// GetBytesPerDim returns the bytes per dimension.
func (c *PointsConfig) GetBytesPerDim() int { return c.bytesPerDim }

// StandardQueryConfigHandler implements CommonQueryParserConfiguration and
// extends AbstractQueryConfig with standard per-field point config.
// This is the Go equivalent of Lucene's StandardQueryConfigHandler.
type StandardQueryConfigHandlerFull struct {
	AbstractQueryConfig
	enablePositionIncrements bool
	allowLeadingWildcard     bool
	lowercaseExpandedTerms   bool
	phraseSlop               int
	fuzzyMinSim              float32
	fuzzyPrefixLength        int
	pointsConfigByField      map[string]*PointsConfig
}

// NewStandardQueryConfigHandlerFull creates a new StandardQueryConfigHandlerFull.
func NewStandardQueryConfigHandlerFull() *StandardQueryConfigHandlerFull {
	return &StandardQueryConfigHandlerFull{
		AbstractQueryConfig:      newAbstractQueryConfig(),
		enablePositionIncrements: true,
		allowLeadingWildcard:     false,
		lowercaseExpandedTerms:   true,
		phraseSlop:               0,
		fuzzyMinSim:              2.0,
		fuzzyPrefixLength:        0,
		pointsConfigByField:      make(map[string]*PointsConfig),
	}
}

func (h *StandardQueryConfigHandlerFull) SetEnablePositionIncrements(enable bool) {
	h.enablePositionIncrements = enable
}
func (h *StandardQueryConfigHandlerFull) GetEnablePositionIncrements() bool {
	return h.enablePositionIncrements
}
func (h *StandardQueryConfigHandlerFull) SetAllowLeadingWildcard(allow bool) {
	h.allowLeadingWildcard = allow
}
func (h *StandardQueryConfigHandlerFull) GetAllowLeadingWildcard() bool {
	return h.allowLeadingWildcard
}
func (h *StandardQueryConfigHandlerFull) SetLowercaseExpandedTerms(lowercase bool) {
	h.lowercaseExpandedTerms = lowercase
}
func (h *StandardQueryConfigHandlerFull) GetLowercaseExpandedTerms() bool {
	return h.lowercaseExpandedTerms
}
func (h *StandardQueryConfigHandlerFull) SetPhraseSlop(slop int) { h.phraseSlop = slop }
func (h *StandardQueryConfigHandlerFull) GetPhraseSlop() int     { return h.phraseSlop }
func (h *StandardQueryConfigHandlerFull) SetFuzzyMinSim(minSim float32) {
	h.fuzzyMinSim = minSim
}
func (h *StandardQueryConfigHandlerFull) GetFuzzyMinSim() float32 { return h.fuzzyMinSim }
func (h *StandardQueryConfigHandlerFull) SetFuzzyPrefixLength(prefixLength int) {
	h.fuzzyPrefixLength = prefixLength
}
func (h *StandardQueryConfigHandlerFull) GetFuzzyPrefixLength() int { return h.fuzzyPrefixLength }

// SetPointsConfig sets the PointsConfig for the given field.
func (h *StandardQueryConfigHandlerFull) SetPointsConfig(field string, config *PointsConfig) {
	h.pointsConfigByField[field] = config
}

// GetPointsConfig returns the PointsConfig for the given field, or nil.
func (h *StandardQueryConfigHandlerFull) GetPointsConfig(field string) *PointsConfig {
	return h.pointsConfigByField[field]
}

// Ensure compile-time interface satisfaction.
var _ CommonQueryParserConfiguration = (*StandardQueryConfigHandlerFull)(nil)

// StandardSyntaxParserToken is a token type for the standard syntax parser.
// This is the Go equivalent of Lucene's standard.parser.Token.
type StandardSyntaxParserToken struct {
	Kind      int
	BeginLine int
	BeginCol  int
	EndLine   int
	EndCol    int
	Image     string
	Next      *StandardSyntaxParserToken
}

// NewStandardSyntaxParserToken creates a new token.
func NewStandardSyntaxParserToken(kind int, image string) *StandardSyntaxParserToken {
	return &StandardSyntaxParserToken{Kind: kind, Image: image}
}

// StandardSyntaxParserTokenMgrError is thrown by the standard syntax parser token manager.
// This is the Go equivalent of Lucene's standard.parser.TokenMgrError.
type StandardSyntaxParserTokenMgrError struct {
	Message   string
	ErrorCode int
}

// Error implements error.
func (e *StandardSyntaxParserTokenMgrError) Error() string { return e.Message }

// NewStandardSyntaxParserTokenMgrError creates a new StandardSyntaxParserTokenMgrError.
func NewStandardSyntaxParserTokenMgrError(message string, code int) *StandardSyntaxParserTokenMgrError {
	return &StandardSyntaxParserTokenMgrError{Message: message, ErrorCode: code}
}

// StandardSyntaxParserParseException is thrown by the standard syntax parser.
// This is the Go equivalent of Lucene's standard.parser.ParseException.
type StandardSyntaxParserParseException struct {
	Message string
	Cause   error
}

// Error implements error.
func (e *StandardSyntaxParserParseException) Error() string { return e.Message }

// Unwrap returns the cause.
func (e *StandardSyntaxParserParseException) Unwrap() error { return e.Cause }

// NewStandardSyntaxParserParseException creates a new exception.
func NewStandardSyntaxParserParseException(message string, cause error) *StandardSyntaxParserParseException {
	return &StandardSyntaxParserParseException{Message: message, Cause: cause}
}

// StandardSyntaxParserConstants defines token-kind constants for the standard syntax parser.
// This is the Go equivalent of Lucene's StandardSyntaxParserConstants.
const (
	SSPKindEOF        = 0
	SSPKindAND        = 1
	SSPKindOR         = 2
	SSPKindNOT        = 3
	SSPKindPlus       = 4
	SSPKindMinus      = 5
	SSPKindLParen     = 6
	SSPKindRParen     = 7
	SSPKindColon      = 8
	SSPKindStar       = 9
	SSPKindCaret      = 10
	SSPKindTilde      = 11
	SSPKindQuoted     = 12
	SSPKindTerm       = 13
	SSPKindFuzzySlop  = 14
	SSPKindPrefixTerm = 15
	SSPKindWildTerm   = 16
	SSPKindRegExp     = 17
	SSPKindNumber     = 18
)

// SSPTokenImage provides string representations of standard syntax parser token kinds.
var SSPTokenImage = []string{
	"<EOF>",
	"\"AND\"",
	"\"OR\"",
	"\"NOT\"",
	"\"+\"",
	"\"-\"",
	"\"(\"",
	"\")\"",
	"\":\"",
	"\"*\"",
	"\"^\"",
	"\"~\"",
	"<QUOTED>",
	"<TERM>",
	"<FUZZY_SLOP>",
	"<PREFIXTERM>",
	"<WILDTERM>",
	"<REGEXPTERM>",
	"<NUMBER>",
}
