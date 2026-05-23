// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package flexible

// Token-kind constants for the flexible standard syntax parser. These are the
// numeric IDs assigned to each terminal/regex in the JavaCC grammar and are
// used by StandardSyntaxParserTokenManager to classify scanned tokens.
//
// This is the Go equivalent of the generated interface
// org.apache.lucene.queryparser.flexible.standard.parser.StandardSyntaxParserConstants.
const (
	// SSPcEOF is the end-of-file sentinel token kind.
	SSPcEOF = 0
	// SSPcNumChar matches a single decimal digit inside a number literal.
	SSPcNumChar = 1
	// SSPcEscapedChar matches a backslash-escaped character.
	SSPcEscapedChar = 2
	// SSPcTermStartChar matches the first character of an unquoted term.
	SSPcTermStartChar = 3
	// SSPcTermChar matches a subsequent character of an unquoted term.
	SSPcTermChar = 4
	// SSPcWhitespace matches horizontal/vertical whitespace.
	SSPcWhitespace = 5
	// SSPcQuotedChar matches a character inside a quoted string literal.
	SSPcQuotedChar = 6
	// (token kind 7 is unused / internal to the generated parser)

	// SSPcAND matches the "AND" keyword.
	SSPcAND = 8
	// SSPcOR matches the "OR" keyword.
	SSPcOR = 9
	// SSPcNOT matches the "NOT" keyword.
	SSPcNOT = 10
	// SSPcFNPrefix matches the "fn:" interval-function prefix.
	SSPcFNPrefix = 11
	// SSPcPlus matches "+".
	SSPcPlus = 12
	// SSPcMinus matches "-".
	SSPcMinus = 13
	// SSPcRParen matches ")".
	SSPcRParen = 14
	// SSPcOpColon matches ":".
	SSPcOpColon = 15
	// SSPcOpEqual matches "=".
	SSPcOpEqual = 16
	// SSPcOpLessThan matches "<".
	SSPcOpLessThan = 17
	// SSPcOpLessThanEq matches "<=".
	SSPcOpLessThanEq = 18
	// SSPcOpMoreThan matches ">".
	SSPcOpMoreThan = 19
	// SSPcOpMoreThanEq matches ">=".
	SSPcOpMoreThanEq = 20
	// SSPcCarat matches "^" (boost operator).
	SSPcCarat = 21
	// SSPcTilde matches "~" (fuzzy/proximity operator).
	SSPcTilde = 22
	// SSPcQuoted matches a double-quoted string literal.
	SSPcQuoted = 23
	// SSPcNumber matches a numeric literal (integer or decimal).
	SSPcNumber = 24
	// SSPcTerm matches an unquoted term.
	SSPcTerm = 25
	// SSPcRegexpTerm matches a regexp literal enclosed in '/'.
	SSPcRegexpTerm = 26
	// SSPcRangeInStart matches "[" opening an inclusive range.
	SSPcRangeInStart = 27
	// SSPcRangeExStart matches "{" opening an exclusive range.
	SSPcRangeExStart = 28
	// SSPcLParen matches "(".
	SSPcLParen = 29
	// SSPcAtLeast matches the "AT_LEAST(<n>)" interval function keyword.
	SSPcAtLeast = 30
	// SSPcAfter matches the "after" interval function keyword.
	SSPcAfter = 31
	// SSPcBefore matches the "before" interval function keyword.
	SSPcBefore = 32
	// SSPcContainedBy matches the "contained_by" interval function keyword.
	SSPcContainedBy = 33
	// SSPcContaining matches the "containing" interval function keyword.
	SSPcContaining = 34
	// SSPcExtend matches the "extend" interval function keyword.
	SSPcExtend = 35
	// SSPcFnOr matches the "or" interval function keyword.
	SSPcFnOr = 36
	// SSPcFuzzyTerm matches the "fuzzyTerm(<params>)" interval function keyword.
	SSPcFuzzyTerm = 37
	// SSPcMaxGaps matches the "maxgaps(<n>)" interval function keyword.
	SSPcMaxGaps = 38
	// SSPcMaxWidth matches the "maxwidth(<n>)" interval function keyword.
	SSPcMaxWidth = 39
	// SSPcNonOverlapping matches the "nonOverlapping" interval function keyword.
	SSPcNonOverlapping = 40
	// SSPcNotContainedBy matches the "notContainedBy" interval function keyword.
	SSPcNotContainedBy = 41
	// SSPcNotContaining matches the "notContaining" interval function keyword.
	SSPcNotContaining = 42
	// SSPcNotWithin matches the "notWithin(<n>)" interval function keyword.
	SSPcNotWithin = 43
	// SSPcOrdered matches the "ordered" interval function keyword.
	SSPcOrdered = 44
	// SSPcOverlapping matches the "overlapping" interval function keyword.
	SSPcOverlapping = 45
	// SSPcPhrase matches the "phrase" interval function keyword.
	SSPcPhrase = 46
	// SSPcUnordered matches the "unordered" interval function keyword.
	SSPcUnordered = 47
	// SSPcUnorderedNoOverlaps matches the "unorderedNoOverlaps" keyword.
	SSPcUnorderedNoOverlaps = 48
	// SSPcWildcard matches the "wildcard(<pattern>)" interval function keyword.
	SSPcWildcard = 49
	// SSPcWithin matches the "within(<n>)" interval function keyword.
	SSPcWithin = 50
	// SSPcRangeTo matches the "TO" keyword inside a range expression.
	SSPcRangeTo = 51
	// SSPcRangeInEnd matches "]" closing an inclusive range.
	SSPcRangeInEnd = 52
	// SSPcRangeExEnd matches "}" closing an exclusive range.
	SSPcRangeExEnd = 53
	// SSPcRangeQuoted matches a quoted term inside a range expression.
	SSPcRangeQuoted = 54
	// SSPcRangeGoop matches an arbitrary unquoted token inside a range.
	SSPcRangeGoop = 55
)

// Lexical state identifiers used by the token manager state machine.
const (
	// SSPsFunction is the lexical state entered after "fn:".
	SSPsFunction = 0
	// SSPsRange is the lexical state entered inside a range expression.
	SSPsRange = 1
	// SSPsDefault is the default top-level lexical state.
	SSPsDefault = 2
)

// SSPcTokenImage provides the canonical string representation of each token
// kind. Index n corresponds to token kind n.
var SSPcTokenImage = []string{
	"<EOF>",
	"<_NUM_CHAR>",
	"<_ESCAPED_CHAR>",
	"<_TERM_START_CHAR>",
	"<_TERM_CHAR>",
	"<_WHITESPACE>",
	"<_QUOTED_CHAR>",
	"<token of kind 7>",
	"<AND>",
	"<OR>",
	"<NOT>",
	`"fn:"`,
	`"+"`,
	`"-"`,
	`")"`,
	`":"`,
	`"="`,
	`"<"`,
	`"<="`,
	`">"`,
	`">="`,
	`"^"`,
	`"~"`,
	"<QUOTED>",
	"<NUMBER>",
	"<TERM>",
	"<REGEXPTERM>",
	`"["`,
	`"{"`,
	`"("`,
	"<ATLEAST>",
	`"after"`,
	`"before"`,
	"<CONTAINED_BY>",
	`"containing"`,
	`"extend"`,
	`"or"`,
	"<FUZZYTERM>",
	"<MAXGAPS>",
	"<MAXWIDTH>",
	"<NON_OVERLAPPING>",
	"<NOT_CONTAINED_BY>",
	"<NOT_CONTAINING>",
	"<NOT_WITHIN>",
	`"ordered"`,
	`"overlapping"`,
	`"phrase"`,
	`"unordered"`,
	"<UNORDERED_NO_OVERLAPS>",
	`"wildcard"`,
	`"within"`,
	`"TO"`,
	`"]"`,
	`"}"`,
	"<RANGE_QUOTED>",
	"<RANGE_GOOP>",
	`"@"`,
}
