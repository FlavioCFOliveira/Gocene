package misc

import "sort"

// HighFreqTerms reports the top-N TermStats by document frequency. Mirrors
// org.apache.lucene.misc.HighFreqTerms.

// GetHighFreqTerms returns the top-N TermStats sorted by descending docFreq.
func GetHighFreqTerms(stats []*TermStats, numTerms int) []*TermStats {
	if numTerms < 1 {
		numTerms = 10
	}
	clone := make([]*TermStats, len(stats))
	copy(clone, stats)
	sort.SliceStable(clone, func(i, j int) bool { return clone[i].DocFreq > clone[j].DocFreq })
	if len(clone) > numTerms {
		clone = clone[:numTerms]
	}
	return clone
}

// GetHighFreqTermsByTotalTermFreq is the totalTermFreq-sorted variant.
func GetHighFreqTermsByTotalTermFreq(stats []*TermStats, numTerms int) []*TermStats {
	if numTerms < 1 {
		numTerms = 10
	}
	clone := make([]*TermStats, len(stats))
	copy(clone, stats)
	sort.SliceStable(clone, func(i, j int) bool { return clone[i].TotalTermFreq > clone[j].TotalTermFreq })
	if len(clone) > numTerms {
		clone = clone[:numTerms]
	}
	return clone
}
