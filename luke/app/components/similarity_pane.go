package components

import (
	"github.com/FlavioCFOliveira/Gocene/luke/models/search"
)

// SimilarityPane implements SimilarityTabOperator.
type SimilarityPane struct {
	useClassicSimilarity bool
	discountOverlaps     bool
	k1                   float32
	b                    float32
}

func NewSimilarityPane() *SimilarityPane {
	return &SimilarityPane{
		k1: 1.2,
		b:  0.75,
	}
}

func (p *SimilarityPane) GetConfig() *search.SimilarityConfig {
	return &search.SimilarityConfig{
		UseClassicSimilarity: p.useClassicSimilarity,
		DiscountOverlaps:     p.discountOverlaps,
		K1:                   p.k1,
		B:                    p.b,
	}
}

func (p *SimilarityPane) SetUseClassicSimilarity(use bool) {
	p.useClassicSimilarity = use
}

func (p *SimilarityPane) SetDiscountOverlaps(discount bool) {
	p.discountOverlaps = discount
}

func (p *SimilarityPane) SetK1(k1 float32) {
	p.k1 = k1
}

func (p *SimilarityPane) SetB(b float32) {
	p.b = b
}
