package uhighlight

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// NoOpOffsetStrategy never returns offsets. It is used when the query would
// highlight nothing.
//
// This is the Go port of
// org.apache.lucene.search.uhighlight.NoOpOffsetStrategy from Apache Lucene
// 10.5.0.
type NoOpOffsetStrategy struct{ BaseFieldOffsetStrategy }

// NoOpOffsetStrategyINSTANCE renders the singleton
// NoOpOffsetStrategy.INSTANCE (NoOpOffsetStrategy.java:32). Java writes it as
// NoOpOffsetStrategy.INSTANCE at every use site; Go has no class-scoped
// constants, so the owning class is carried in the name.
var NoOpOffsetStrategyINSTANCE = newNoOpOffsetStrategy()

// newNoOpOffsetStrategy renders the private NoOpOffsetStrategy constructor
// (NoOpOffsetStrategy.java:34), which builds the placeholder UHComponents the
// singleton carries. Java passes MatchNoDocsQuery.INSTANCE; Gocene's
// MatchNoDocsQuery has only the reason-carrying constructor, whose no-reason
// form is the same query.
func newNoOpOffsetStrategy() *NoOpOffsetStrategy {
	return &NoOpOffsetStrategy{BaseFieldOffsetStrategy: NewBaseFieldOffsetStrategy(
		NewUHComponents(
			"_ignored_",
			func(string) bool { return false },
			search.NewMatchNoDocsQuery(""),
			[]*util.BytesRef{},
			NONE,
			[]*LabelledCharArrayMatcher{},
			false,
			map[HighlightFlag]struct{}{},
		))}
}

// GetOffsetSource renders NoOpOffsetStrategy.getOffsetSource()
// (NoOpOffsetStrategy.java:48).
func (s *NoOpOffsetStrategy) GetOffsetSource() OffsetSource { return OffsetSourceNoneNeeded }

// GetOffsetsEnum renders
// NoOpOffsetStrategy.getOffsetsEnum(LeafReader, int, String)
// (NoOpOffsetStrategy.java:53).
func (s *NoOpOffsetStrategy) GetOffsetsEnum(_ index.LeafReader, _ int, _ string) (OffsetsEnum, error) {
	return OffsetsEnumEMPTY, nil
}

var _ FieldOffsetStrategy = (*NoOpOffsetStrategy)(nil)
