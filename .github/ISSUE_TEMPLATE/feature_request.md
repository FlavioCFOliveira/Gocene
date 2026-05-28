---
name: Feature request
about: Propose porting a Lucene feature or adding an enhancement to Gocene
title: "[FEATURE] "
labels: enhancement
assignees: ''
---

## Problem statement

What problem would this feature solve? Who needs it and why?

## Proposed solution

Describe the feature you would like. If it ports a specific Apache Lucene
10.4.0 capability, name the Lucene class(es) and the module under
`/tmp/lucene/lucene/<module>/src/java/...`.

## Binary-compatibility considerations

Gocene targets byte-for-byte compatibility with Apache Lucene 10.4.0. If this
feature produces or consumes any serialized artefact, describe the exact binary
contract it must honour and how it will be verified (round-trip and/or
golden-corpus tests against Lucene-produced fixtures).

## Alternatives considered

Any alternative designs or workarounds you have weighed.

## Additional context

Links, references, or prior art (papers, Lucene issues, etc.).
