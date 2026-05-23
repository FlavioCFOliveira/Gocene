// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/java/org/apache/lucene/queries/intervals/Disjunctions.java

package intervals

// disjunctionsPullUp pulls disjunctions up to the top of a conjunction source tree.
//
// Given a list of sources, it expands any disjunctive sources into separate lists,
// one per combination, then applies the combiner to each list.
//
// Mirrors org.apache.lucene.queries.intervals.Disjunctions.pullUp
// (list-of-sources variant).
//
// Deviations from Java:
//   - Does not enforce IndexSearcher.getMaxClauseCount; expansion is unbounded.
func disjunctionsPullUp(sources []IntervalsSource, combiner func([]IntervalsSource) IntervalsSource) []IntervalsSource {
	// Start with one empty list.
	rewritten := [][]IntervalsSource{{}}

	for _, src := range sources {
		disjuncts := splitDisjunctions(src)
		if len(disjuncts) == 1 {
			for i := range rewritten {
				rewritten[i] = append(rewritten[i], disjuncts[0])
			}
		} else {
			var toAdd [][]IntervalsSource
			for _, disj := range disjuncts {
				for _, subList := range rewritten {
					newList := make([]IntervalsSource, len(subList)+1)
					copy(newList, subList)
					newList[len(subList)] = disj
					toAdd = append(toAdd, newList)
				}
			}
			rewritten = toAdd
		}
	}

	if len(rewritten) == 1 {
		return []IntervalsSource{combiner(rewritten[0])}
	}
	out := make([]IntervalsSource, len(rewritten))
	for i, list := range rewritten {
		out[i] = combiner(list)
	}
	return out
}

// disjunctionsPullUpSingle pulls disjunctions up for a single source with a mapping function.
func disjunctionsPullUpSingle(src IntervalsSource, mapper func(IntervalsSource) IntervalsSource) []IntervalsSource {
	disjuncts := splitDisjunctions(src)
	if len(disjuncts) == 1 {
		return []IntervalsSource{mapper(disjuncts[0])}
	}
	out := make([]IntervalsSource, len(disjuncts))
	for i, d := range disjuncts {
		out[i] = mapper(d)
	}
	return out
}

// splitDisjunctions separates the disjunctions in a source.
// Sources with minExtent==1 are grouped together as a single DisjunctionIntervalsSource.
func splitDisjunctions(src IntervalsSource) []IntervalsSource {
	var singletons []IntervalsSource
	var nonSingletons []IntervalsSource
	for _, disj := range src.PullUpDisjunctions() {
		if disj.MinExtent() == 1 {
			singletons = append(singletons, disj)
		} else {
			nonSingletons = append(nonSingletons, disj)
		}
	}
	var split []IntervalsSource
	if len(singletons) > 0 {
		split = append(split, NewDisjunctionIntervalsSource(singletons, true))
	}
	split = append(split, nonSingletons...)
	return split
}
