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
const (
	// MsgNodeToQueryStringFailed is used when a QueryNode cannot be converted to a query string.
	MsgNodeToQueryStringFailed = "NODE_TO_QUERY_STRING_FAILED"

	// MsgInvalidSyntax is used for generic syntax errors.
	MsgInvalidSyntax = "INVALID_SYNTAX"

	// MsgInvalidSyntaxCannotParseCharclass is used when a character class is malformed.
	MsgInvalidSyntaxCannotParseCharclass = "INVALID_SYNTAX_CANNOT_PARSE_CHAR_CLASS"

	// MsgInvalidSyntaxFuzzyLimits is used when a fuzzy edit distance is out of range.
	MsgInvalidSyntaxFuzzyLimits = "INVALID_SYNTAX_FUZZY_LIMITS"

	// MsgInvalidSyntaxFuzzy is used for generic fuzzy syntax errors.
	MsgInvalidSyntaxFuzzy = "INVALID_SYNTAX_FUZZY"

	// MsgEmpty is used when the query string is empty.
	MsgEmpty = "EMPTY_MESSAGE"

	// MsgUnsupportedQueryNodeOperation is used when a query node operation is not supported.
	MsgUnsupportedQueryNodeOperation = "UNSUPPORTED_QUERY_NODE_OPERATION"

	// MsgQueryNodeError is the default message for QueryNodeError.
	MsgQueryNodeError = "QUERY_NODE_ERROR"

	// MsgQueryNodeParseException is the default message for QueryNodeParseException.
	MsgQueryNodeParseException = "QUERY_NODE_PARSE_EXCEPTION"
)
