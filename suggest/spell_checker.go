// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package suggest

import (
	"fmt"
	"sort"
)

// SpellChecker is used to suggest corrections for misspelled words.
//
// This is the Go port of Lucene's org.apache.lucene.search.spell.SpellChecker.
type SpellChecker struct {
	dictionary SpellDictionary
	distance   StringDistance
}

func NewSpellChecker(dict SpellDictionary, dist StringDistance) *SpellChecker {
	return &SpellChecker{
		dictionary: dict,
		distance:   dist,
	}
}

// Suggest suggests corrections for the given word.
func (sc *SpellChecker) Suggest(word string, numSuggestions int) ([]SuggestWord, error) {
	if sc.dictionary.GetFreq(word) > 0 {
		return []SuggestWord{{Word: word, Frequency: sc.dictionary.GetFreq(word), Score: 1.0}}, nil
	}

	allWords, err := sc.dictionary.GetSuggestedWords(word, 100)
	if err != nil {
		return nil, err
	}

	var suggestions []SuggestWord
	for _, w := range allWords {
		dist := sc.distance.GetDistance(word, w)
		if dist <= 2 { // Max distance threshold
			freq := sc.dictionary.GetFreq(w)
			score := 1.0 / float64(dist+1)
			suggestions = append(suggestions, SuggestWord{
				Word:      w,
				Frequency: freq,
				Score:     score,
			})
		}
	}

	sort.Slice(suggestions, func(i, j int) bool {
		if suggestions[i].Score != suggestions[j].Score {
			return suggestions[i].Score > suggestions[j].Score
		}
		return suggestions[i].Frequency > suggestions[j].Frequency
	})

	if len(suggestions) > numSuggestions {
		suggestions = suggestions[:numSuggestions]
	}

	return suggestions, nil
}
