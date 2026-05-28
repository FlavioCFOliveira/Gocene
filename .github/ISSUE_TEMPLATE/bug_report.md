---
name: Bug report
about: Report a defect, an incorrect result, or a binary-compatibility divergence from Apache Lucene 10.4.0
title: "[BUG] "
labels: bug
assignees: ''
---

## Summary

A clear and concise description of the bug.

## Affected package(s)

Which Gocene package(s) are involved (for example `index`, `store`, `codecs/lucene104`)?

## Binary-compatibility impact

Gocene must read and write byte-for-byte the same artefacts as Apache Lucene
10.4.0. If this bug concerns an on-disk format, the replicator/wire protocol, or
any other serialized artefact, please state:

- [ ] This bug affects a serialized format (codec file, `.si`/`.cfs`/`.cfe`,
      segment infos, postings, doc values, term vectors, points, vectors, FST,
      compound files, checksum framing, …).
- The Lucene 10.4.0 reference behaviour you compared against (file path under
  `/tmp/lucene` and/or a Lucene-produced fixture).

## Steps to reproduce

```go
// Minimal reproducer (a failing test is ideal).
```

1.
2.
3.

## Expected behaviour

What you expected to happen (cite the Lucene 10.4.0 reference where relevant).

## Actual behaviour

What actually happened. Include the exact error message and, if helpful, a byte
diff against the expected output.

## Environment

- Gocene version / commit:
- Go version (`go version`):
- OS / architecture:

## Additional context

Anything else that helps diagnose the problem.
