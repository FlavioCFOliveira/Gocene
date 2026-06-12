package misc

import "sort"

// HighFreqTerms reports the top-N TermStats by document frequency. Mirrors
// org.apache.lucene.misc.HighFreqTerms.

// GetHighFreqTerms returns the top-N TermStats sorted by descending docFreq.
// When docFreq values are equal, the result is ordered by ascending Field
// then ascending Term (lexicographic), matching Lucene's DocFreqComparator
// tie-breaking.
func GetHighFreqTerms(stats []*TermStats, numTerms int) []*TermStats {
	if numTerms < 1 {
		numTerms = 10
	}
	clone := make([]*TermStats, len(stats))
	copy(clone, stats)
	sort.Slice(clone, func(i, j int) bool {
		if clone[i].DocFreq != clone[j].DocFreq {
			return clone[i].DocFreq > clone[j].DocFreq // descending
		}
		if clone[i].Field != clone[j].Field {
			return clone[i].Field < clone[j].Field // ascending
		}
		return clone[i].Term < clone[j].Term // ascending
	})
	if len(clone) > numTerms {
		clone = clone[:numTerms]
	}
	return clone
}

// GetHighFreqTermsByTotalTermFreq is the totalTermFreq-sorted variant.
// When totalTermFreq values are equal, the result is ordered by ascending
// Field then ascending Term (lexicographic), matching Lucene's
// TotalTermFreqComparator tie-breaking.
func GetHighFreqTermsByTotalTermFreq(stats []*TermStats, numTerms int) []*TermStats {
	if numTerms < 1 {
		numTerms = 10
	}
	clone := make([]*TermStats, len(stats))
	copy(clone, stats)
	sort.Slice(clone, func(i, j int) bool {
		if clone[i].TotalTermFreq != clone[j].TotalTermFreq {
			return clone[i].TotalTermFreq > clone[j].TotalTermFreq // descending
		}
		if clone[i].Field != clone[j].Field {
			return clone[i].Field < clone[j].Field // ascending
		}
		return clone[i].Term < clone[j].Term // ascending
	})
	if len(clone) > numTerms {
		clone = clone[:numTerms]
	}
	return clone
}
