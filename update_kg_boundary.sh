#!/bin/bash
ROADMAP="gocene"
COMMIT="5238f5f41257e746be9e6bc171fc9cae6dee6a15"
DATE="2026-09-01"

run_with_retry() {
    local cmd="$1"
    until eval "$cmd"; do
        echo "Graph store busy, retrying in 1s..."
        sleep 1
    done
}

queries=(
  "MERGE (f:GoFile {path: 'search/vectorhighlight/boundary_scanner.go'})"
  "MERGE (f:GoFile {path: 'search/vectorhighlight/break_iterator_boundary_scanner.go'})"
  "MERGE (f:GoFile {path: 'search/vectorhighlight/simple_boundary_scanner.go'})"
  "MATCH (lc:LuceneClass {name: 'org.apache.lucene.search.vectorhighlight.BoundaryScanner'}), (f:GoFile {path: 'search/vectorhighlight/boundary_scanner.go'}) MERGE (lc)-[:PORTED_TO]->(f)"
  "MATCH (lc:LuceneClass {name: 'org.apache.lucene.search.vectorhighlight.BreakIteratorBoundaryScanner'}), (f:GoFile {path: 'search/vectorhighlight/break_iterator_boundary_scanner.go'}) MERGE (lc)-[:PORTED_TO]->(f)"
  "MATCH (lc:LuceneClass {name: 'org.apache.lucene.search.vectorhighlight.SimpleBoundaryScanner'}), (f:GoFile {path: 'search/vectorhighlight/simple_boundary_scanner.go'}) MERGE (lc)-[:PORTED_TO]->(f)"
)

for q in "${queries[@]}"; do
  run_with_retry "rmp graph create -r '$ROADMAP' --query '$q'"
done

files=(
  "search/vectorhighlight/boundary_scanner.go"
  "search/vectorhighlight/break_iterator_boundary_scanner.go"
  "search/vectorhighlight/simple_boundary_scanner.go"
)

for f in "${files[@]}"; do
  run_with_retry "rmp graph update -r '$ROADMAP' --query \"MATCH (f:GoFile {path: '$f'}) SET f.gitCommit='$COMMIT', f.gitDate='$DATE'\""
done
