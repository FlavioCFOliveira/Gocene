// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package flexible

// QueryParserMessages contains message key constants used throughout the
// flexible query parser. These correspond to the string keys defined in
// Lucene's QueryParserMessages.properties resource bundle.
//
// All error and informational messages in the flexible parser reference one of
// these constants so that message text can be centralised and localised.
//
// Mirrors org.apache.lucene.queryparser.flexible.core.messages.QueryParserMessages.
const (
	// MsgNodeToQueryStringFailed is used when a QueryNode cannot be converted to a query string.
	MsgNodeToQueryStringFailed = "NODE_TO_QUERY_STRING_FAILED"

	// MsgInvalidSyntax is used for generic syntax errors.
	MsgInvalidSyntax = "INVALID_SYNTAX"

	// MsgInvalidSyntaxCannotParse is used when a query cannot be parsed.
	MsgInvalidSyntaxCannotParse = "INVALID_SYNTAX_CANNOT_PARSE"

	// MsgInvalidSyntaxCannotParseCharclass is used when a character class is malformed.
	MsgInvalidSyntaxCannotParseCharclass = "INVALID_SYNTAX_CANNOT_PARSE_CHAR_CLASS"

	// MsgInvalidSyntaxFuzzyLimits is used when a fuzzy edit distance is out of range.
	MsgInvalidSyntaxFuzzyLimits = "INVALID_SYNTAX_FUZZY_LIMITS"

	// MsgInvalidSyntaxFuzzyEdits is used when a fuzzy edit count is invalid.
	MsgInvalidSyntaxFuzzyEdits = "INVALID_SYNTAX_FUZZY_EDITS"

	// MsgInvalidSyntaxFuzzy is used for generic fuzzy syntax errors.
	MsgInvalidSyntaxFuzzy = "INVALID_SYNTAX_FUZZY"

	// MsgInvalidSyntaxEscapeUnicodeTruncation is used when a Unicode escape is truncated.
	MsgInvalidSyntaxEscapeUnicodeTruncation = "INVALID_SYNTAX_ESCAPE_UNICODE_TRUNCATION"

	// MsgInvalidSyntaxEscapeCharacter is used when an escape sequence is invalid.
	MsgInvalidSyntaxEscapeCharacter = "INVALID_SYNTAX_ESCAPE_CHARACTER"

	// MsgInvalidSyntaxEscapeNoneHexUnicode is used when a non-hex digit follows \u.
	MsgInvalidSyntaxEscapeNoneHexUnicode = "INVALID_SYNTAX_ESCAPE_NONE_HEX_UNICODE"

	// MsgNodeActionNotSupported is used when a node action is not supported.
	MsgNodeActionNotSupported = "NODE_ACTION_NOT_SUPPORTED"

	// MsgParameterValueNotSupported is used when a parameter value is not supported.
	MsgParameterValueNotSupported = "PARAMETER_VALUE_NOT_SUPPORTED"

	// MsgLuceneQueryConversionError is used when a query node cannot be converted to a Lucene query.
	MsgLuceneQueryConversionError = "LUCENE_QUERY_CONVERSION_ERROR"

	// MsgEmpty is used when the query string is empty.
	MsgEmpty = "EMPTY_MESSAGE"

	// MsgWildcardNotSupported is used when a wildcard appears where it is not allowed.
	MsgWildcardNotSupported = "WILDCARD_NOT_SUPPORTED"

	// MsgTooManyBooleanClauses is used when the boolean query clause limit is exceeded.
	MsgTooManyBooleanClauses = "TOO_MANY_BOOLEAN_CLAUSES"

	// MsgLeadingWildcardNotAllowed is used when a leading wildcard is not allowed.
	MsgLeadingWildcardNotAllowed = "LEADING_WILDCARD_NOT_ALLOWED"

	// MsgCouldNotParseNumber is used when a string cannot be parsed as a number.
	MsgCouldNotParseNumber = "COULD_NOT_PARSE_NUMBER"

	// MsgNumberClassNotSupportedByNumericRangeQuery is used when the numeric type
	// is not supported by the range query implementation.
	MsgNumberClassNotSupportedByNumericRangeQuery = "NUMBER_CLASS_NOT_SUPPORTED_BY_NUMERIC_RANGE_QUERY"

	// MsgUnsupportedNumericDataType is used when a numeric data type is not supported.
	MsgUnsupportedNumericDataType = "UNSUPPORTED_NUMERIC_DATA_TYPE"

	// MsgNumericCannotBeEmpty is used when a numeric field is empty.
	MsgNumericCannotBeEmpty = "NUMERIC_CANNOT_BE_EMPTY"

	// MsgAnalyzerRequired is used when an analyzer is required but not configured.
	MsgAnalyzerRequired = "ANALYZER_REQUIRED"

	// MsgUnsupportedQueryNodeOperation is used when a query node operation is not supported.
	MsgUnsupportedQueryNodeOperation = "UNSUPPORTED_QUERY_NODE_OPERATION"

	// MsgQueryNodeError is the default message for QueryNodeError.
	MsgQueryNodeError = "QUERY_NODE_ERROR"

	// MsgQueryNodeParseException is the default message for QueryNodeParseException.
	MsgQueryNodeParseException = "QUERY_NODE_PARSE_EXCEPTION"
)
