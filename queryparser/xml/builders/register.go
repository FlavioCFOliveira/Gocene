package builders

import (
	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/queryparser/xml"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// RegisterCoreBuilders attaches the standard set of XML query builders to the
// supplied CoreParser. Mirrors the wiring done in Lucene's CoreParser
// constructor.
func RegisterCoreBuilders(parser *xml.CoreParser, userInputParse func(field, queryString string) (search.Query, error)) {
	parser.AddBuilder("TermQuery", TermQueryBuilder{})
	parser.AddBuilder("TermsQuery", TermsQueryBuilder{})
	parser.AddBuilder("MatchAllDocsQuery", MatchAllDocsQueryBuilder{})
	parser.AddBuilder("ConstantScoreQuery", NewConstantScoreQueryBuilder(parser.QueryBuilderFactory))
	parser.AddBuilder("BooleanQuery", NewBooleanQueryBuilder(parser.QueryBuilderFactory))
	parser.AddBuilder("RangeQuery", RangeQueryBuilder{})
	parser.AddBuilder("PointRangeQuery", PointRangeQueryBuilder{})
	parser.AddBuilder("DisjunctionMaxQuery", NewDisjunctionMaxQueryBuilder(parser.QueryBuilderFactory))
	parser.AddBuilder("UserQuery", NewUserInputQueryBuilder(userInputParse))

	spanFactory := NewSpanQueryBuilderFactory()
	spanFactory.AddBuilder("SpanTerm", SpanTermBuilder{})
	spanFactory.AddBuilder("SpanOr", NewSpanOrBuilder(spanFactory))
	spanFactory.AddBuilder("SpanOrTerms", SpanOrTermsBuilder{})
	spanFactory.AddBuilder("SpanNot", NewSpanNotBuilder(spanFactory))
	spanFactory.AddBuilder("SpanNear", NewSpanNearBuilder(spanFactory))
	spanFactory.AddBuilder("SpanFirst", NewSpanFirstBuilder(spanFactory))
	spanFactory.AddBuilder("SpanPositionRange", NewSpanPositionRangeBuilder(spanFactory))
	spanFactory.AddBuilder("BoostingTermQuery", BoostingTermBuilder{})

	parser.AddBuilder("SpanTerm", SpanTermBuilder{})
	parser.AddBuilder("SpanOr", NewSpanOrBuilder(spanFactory))
	parser.AddBuilder("SpanOrTerms", SpanOrTermsBuilder{})
	parser.AddBuilder("SpanNot", NewSpanNotBuilder(spanFactory))
	parser.AddBuilder("SpanNear", NewSpanNearBuilder(spanFactory))
	parser.AddBuilder("SpanFirst", NewSpanFirstBuilder(spanFactory))
	parser.AddBuilder("SpanPositionRange", NewSpanPositionRangeBuilder(spanFactory))
	parser.AddBuilder("BoostingTermQuery", BoostingTermBuilder{})
}

// RegisterCorePlusQueriesBuilders attaches the optional queries-module
// builders (LikeThis, FuzzyLikeThis) on top of RegisterCoreBuilders.
func RegisterCorePlusQueriesBuilders(parser *xml.CorePlusQueriesParser, analyzer analysis.Analyzer, userInputParse func(field, queryString string) (search.Query, error)) {
	RegisterCoreBuilders(parser.CoreParser, userInputParse)
	parser.AddBuilder("LikeThisQuery", NewLikeThisQueryBuilder(analyzer))
	parser.AddBuilder("FuzzyLikeThisQuery", NewFuzzyLikeThisQueryBuilder(analyzer))
}

// RegisterCorePlusExtensionsBuilders attaches the extension-module builders.
// At the moment no extension-only builders exist beyond the queries set, so
// the wiring is a thin alias.
func RegisterCorePlusExtensionsBuilders(parser *xml.CorePlusExtensionsParser, analyzer analysis.Analyzer, userInputParse func(field, queryString string) (search.Query, error)) {
	RegisterCorePlusQueriesBuilders(parser.CorePlusQueriesParser, analyzer, userInputParse)
}
