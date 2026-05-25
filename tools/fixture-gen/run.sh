#!/usr/bin/env bash
# Generates Lucene 10.4.0 golden fixtures using Docker (no local Java required).
#
# Usage:
#   ./tools/fixture-gen/run.sh [output-dir]
#
# Default output-dir (relative to repo root): testdata/lucene-10.4.0-fixtures
#
# Requirements:
#   - Docker must be installed and running (use sudo if needed)
#   - Internet access to pull maven:3.9-eclipse-temurin-21 on first run
#
# To regenerate fixtures after changing FixtureGen.java:
#   cd <repo-root> && ./tools/fixture-gen/run.sh

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
OUT_DIR="${1:-testdata/lucene-10.4.0-fixtures}"
ABS_OUT_DIR="$REPO_ROOT/$OUT_DIR"
TOOL_DIR="$REPO_ROOT/tools/fixture-gen"

echo "==> Generating Lucene 10.4.0 fixtures"
echo "    Output: $ABS_OUT_DIR"

mkdir -p "$ABS_OUT_DIR"

# Determine docker command (use sudo if plain docker is not accessible)
DOCKER="docker"
if ! docker info >/dev/null 2>&1; then
  if sudo docker info >/dev/null 2>&1; then
    DOCKER="sudo docker"
  else
    echo "ERROR: Docker is not accessible. Please ensure Docker is running."
    exit 1
  fi
fi

# Build uber-jar and run the fixture generator in a single container.
# Maven local repo is cached in a named volume to speed up subsequent runs.
$DOCKER run --rm \
  -v "$TOOL_DIR:/workspace" \
  -v "$ABS_OUT_DIR:/output" \
  -v "gocene-maven-cache:/root/.m2" \
  -w /workspace \
  maven:3.9-eclipse-temurin-21 \
  bash -c "mvn -q package -DskipTests && java -jar target/fixture-gen.jar /output"

echo "==> Done. Fixtures written to $ABS_OUT_DIR"
ls -lh "$ABS_OUT_DIR"
