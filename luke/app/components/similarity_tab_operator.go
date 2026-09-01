package components

import (
	"github.com/FlavioCFOliveira/Gocene/luke/models/search"
)

// SimilarityTabOperator is the operator for the Similarity tab.
type SimilarityTabOperator interface {
	ComponentOperator

	GetConfig() *search.SimilarityConfig
}
