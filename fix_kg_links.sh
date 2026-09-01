#!/bin/bash
ROADMAP="gocene"

declare -A mapping
mapping["LongRange"]="search/grouping/long_range.go"
mapping["LongRangeFactory"]="search/grouping/long_range_factory.go"
mapping["BoundaryScanner"]="search/vectorhighlight/boundary_scanner.go"
mapping["BreakIteratorBoundaryScanner"]="search/vectorhighlight/break_iterator_boundary_scanner.go"
mapping["SimpleBoundaryScanner"]="search/vectorhighlight/simple_boundary_scanner.go"
mapping["FieldQuery"]="search/vectorhighlight/field_query.go"
mapping["FieldTermStack"]="search/vectorhighlight/field_term_stack.go"
mapping["FieldPhraseList"]="search/vectorhighlight/field_phrase_list.go"
mapping["FieldFragList"]="search/vectorhighlight/field_frag_list.go"
mapping["SimpleFieldFragList"]="search/vectorhighlight/simple_field_frag_list.go"
mapping["WeightedFieldFragList"]="search/vectorhighlight/weighted_field_frag_list.go"
mapping["FragListBuilder"]="search/vectorhighlight/frag_list_builder.go"
mapping["BaseFragListBuilder"]="search/vectorhighlight/base_frag_list_builder.go"
mapping["SimpleFragListBuilder"]="search/vectorhighlight/simple_frag_list_builder.go"
mapping["SingleFragListBuilder"]="search/vectorhighlight/single_frag_list_builder.go"
mapping["WeightedFragListBuilder"]="search/vectorhighlight/weighted_frag_list_builder.go"
mapping["FastVectorHighlighter"]="search/vectorhighlight/fast_vector_highlighter.go"
mapping["FragmentsBuilder"]="search/vectorhighlight/fragments_builder.go"
mapping["BaseFragmentsBuilder"]="search/vectorhighlight/base_fragments_builder.go"
mapping["SimpleFragmentsBuilder"]="search/vectorhighlight/simple_fragments_builder.go"
mapping["ScoreOrderFragmentsBuilder"]="search/vectorhighlight/score_order_fragments_builder.go"

for lc in "${!mapping[@]}"; do
    f=${mapping[$lc]}
    rmp graph create -r "$ROADMAP" --query "MATCH (lc:LuceneClass {name: '$lc'}), (f:GoFile {path: '$f'}) MERGE (lc)-[:PORTED_TO]->(f)"
done
