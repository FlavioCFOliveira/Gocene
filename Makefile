# Gocene — Apache Lucene 10.4.0 Go Port
# Development Makefile

.PHONY: test test-verbose race-test fuzz lint compat clean help

# Default target: run standard tests (no race detector).
test:
	go test ./... -count=1 -timeout 600s

# Verbose test output with coverage.
test-verbose:
	go test ./... -v -count=1 -timeout 600s 2>&1 | tee test-output.log

# Race detector tests. Mirrors the CI "race" job:
#   go test -race ./... -timeout 900s
#
# The Go race detector (ThreadSanitizer) requires a 48-bit virtual address
# space. On x86_64 GitHub runners (ubuntu-latest) this is always available.
# On some ARM64 hosts (47-bit VMA) the detector may fail to initialise;
# use this target on x86_64 for best results.
race-test:
	go test -race ./... -timeout 900s

# Fuzz smoke tests. Each native Go fuzz target runs for a short, bounded
# time against its seed corpus. The contract is "no crash" — a panic or
# hang fails the build. For longer local fuzzing runs, see CONTRIBUTING.md.
fuzz:
	bash scripts/fuzz-smoke.sh 2>/dev/null || \
		for target in $$(go list ./... | xargs -I{} sh -c 'go test -list='Fuzz' {} 2>/dev/null'); do \
			go test -fuzz=$$target -fuzztime=10s ./...; \
		done

# Run the compat test suite (requires GOCENE_COMPAT_HARNESS=1 and Java harness).
compat:
	GOCENE_COMPAT_HARNESS=1 go test -tags compat ./internal/compat/... -v -count=1

# Check for undocumented t.Skip / t.Fatal blocker tokens.
lint:
	bash scripts/check-skips.sh

# Clean build cache and test output artifacts.
clean:
	go clean -testcache
	rm -f test-output.log

# Show available targets.
help:
	@echo "Gocene development targets:"
	@echo "  test          Run standard tests (no race detector)"
	@echo "  test-verbose  Run tests with verbose output + coverage log"
	@echo "  race-test     Run tests with race detector (x86_64 recommended)"
	@echo "  fuzz          Run fuzz smoke tests"
	@echo "  compat        Run binary-compat test suite"
	@echo "  lint          Check for undocumented t.Fatal/t.Skip blockers"
	@echo "  clean         Remove test cache and output files"
