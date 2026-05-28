<!--
Thank you for contributing to Gocene. Please read CONTRIBUTING.md before
opening this pull request. Fill in every section below; PRs that leave the
checklist unaddressed may be closed without review.
-->

## Description

What does this PR change, and why?

## Related rmp task

Gocene uses the `rmp` roadmap CLI as the single source of truth for planning.
Reference the task this PR resolves:

- rmp task: #

## Type of change

- [ ] Bug fix (non-breaking change that fixes an issue)
- [ ] New feature / Lucene port (non-breaking change that adds functionality)
- [ ] Refactor (no functional change)
- [ ] Documentation
- [ ] Chore / tooling

## Binary-compatibility checklist

The Binary Compatibility Mandate is non-negotiable: Gocene must produce
artefacts Apache Lucene 10.4.0 can read, and must read every artefact Lucene
10.4.0 produces, byte-for-byte.

- [ ] This change does **not** touch any serialized format, **or** the boxes below are completed.
- [ ] I verified the behaviour against the Lucene 10.4.0 reference at `/tmp/lucene`.
- [ ] Isolated compatibility tests (round-trip and/or golden-corpus against Lucene-produced fixtures) are included and pass.
- [ ] Combined/integration compatibility tests covering this feature alongside the features it composes with are included and pass.
- [ ] Any documented non-determinism is justified against the Lucene 10.4.0 source.

## Validation

- [ ] `go build ./...` passes.
- [ ] `go vet ./...` passes on touched packages.
- [ ] `go test` passes for the affected packages (paste the commands and results).
- [ ] `gofmt`-clean.

```
# Paste the exact test commands you ran and their output.
```

## Notes for reviewers

Anything reviewers should pay special attention to.
