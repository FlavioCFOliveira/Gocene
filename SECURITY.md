# Security Policy

Gocene is a Go port of Apache Lucene 10.4.0, currently **pre-1.0** and under
active development. Because it parses and produces untrusted binary artefacts
(index files, codec streams, replicator payloads) and processes arbitrary
text, we take security reports seriously. Note: as a port in progress, some
security-relevant features (NRT reader, full delete/update pipeline,
MockDirectoryWrapper) are still being implemented. See `CLAUDE.md` §Project
Status and `docs/skipped-tests-audit.md` for current coverage.

## Supported versions

Gocene is pre-1.0. Security fixes are applied to the `main` branch. Until a
stable release line exists, only `main` is supported.

| Version | Supported |
| ------- | --------- |
| `main`  | ✅        |
| older commits / tags | ❌ |

## Reporting a vulnerability

**Please do not open a public issue for security vulnerabilities.**

Report privately through GitHub's
[private vulnerability reporting](https://docs.github.com/en/code-security/security-advisories/guidance-on-reporting-and-writing-information-about-vulnerabilities/privately-reporting-a-security-vulnerability)
("Report a vulnerability" on the repository's **Security** tab). This opens a
confidential security advisory visible only to the maintainers.

Please include:

- A description of the vulnerability and its impact.
- The affected package(s) and, if known, the affected commit.
- A minimal reproducer (a failing test or a crafted input file is ideal).
- Whether the issue involves parsing attacker-controlled input (malformed index
  files, oversized documents, crafted queries) or a denial-of-service vector.

## What to expect

- We aim to acknowledge a report within a few business days.
- We will work with you on a fix and coordinate a disclosure timeline.
- We will credit reporters who wish to be named once a fix is released.

## Scope and hardening notes

Areas that are especially security-relevant in Gocene:

- **Index and codec readers** — parsing untrusted on-disk artefacts (bounds,
  checksums, length fields). Malformed input must produce a clear error, never
  a panic, infinite loop, or out-of-bounds access.
- **Directory / path handling** — filenames are validated to reject path
  traversal; never join unvalidated, caller-supplied names onto a base path.
- **Analysis pipeline** — tokenizers and char filters bound their input to
  avoid unbounded memory consumption on hostile documents.
- **Replicator / HTTP surfaces** — request bodies are size-limited and
  server-supplied filenames are sanitised before use.

When in doubt, treat any externally produced byte stream as hostile.
