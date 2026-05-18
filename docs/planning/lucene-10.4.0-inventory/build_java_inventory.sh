#!/usr/bin/env bash
# Build TSV listing every Java production and test file in the in-scope
# Lucene 10.4.0 modules.
#
# Columns: module \t kind \t java_path \t package \t classname \t fqdn \t loc

set -euo pipefail

LUCENE_ROOT=/tmp/lucene/lucene
SCOPE_FILE=/tmp/gocene-inventory/scope_modules.txt
OUT=/tmp/gocene-inventory/java_inventory.tsv

: > "$OUT"

while IFS= read -r mod; do
  [ -z "$mod" ] && continue

  # Production sources live under src/java and (rarely) src/java21.
  for sub in java java21; do
    src_dir="$LUCENE_ROOT/$mod/src/$sub"
    [ -d "$src_dir" ] || continue
    while IFS= read -r f; do
      classname=$(basename "$f" .java)
      [ "$classname" = "package-info" ] && continue
      [ "$classname" = "module-info" ] && continue
      pkg=$(awk '/^package / {sub(/^package /,""); sub(/;.*/,""); print; exit}' "$f")
      fqdn="${pkg}.${classname}"
      loc=$(wc -l < "$f" | tr -d ' ')
      rel="${f#$LUCENE_ROOT/}"
      printf '%s\tprod\t%s\t%s\t%s\t%s\t%s\n' "$mod" "$rel" "$pkg" "$classname" "$fqdn" "$loc" >> "$OUT"
    done < <(find "$src_dir" -name '*.java' -type f)
  done

  # Test sources live under src/test.
  src_dir="$LUCENE_ROOT/$mod/src/test"
  if [ -d "$src_dir" ]; then
    while IFS= read -r f; do
      classname=$(basename "$f" .java)
      [ "$classname" = "package-info" ] && continue
      [ "$classname" = "module-info" ] && continue
      pkg=$(awk '/^package / {sub(/^package /,""); sub(/;.*/,""); print; exit}' "$f")
      fqdn="${pkg}.${classname}"
      loc=$(wc -l < "$f" | tr -d ' ')
      rel="${f#$LUCENE_ROOT/}"
      printf '%s\ttest\t%s\t%s\t%s\t%s\t%s\n' "$mod" "$rel" "$pkg" "$classname" "$fqdn" "$loc" >> "$OUT"
    done < <(find "$src_dir" -name '*.java' -type f)
  fi
done < "$SCOPE_FILE"

echo "Done. Lines: $(wc -l < "$OUT")"
echo "Prod:  $(awk -F'\t' '$2=="prod"' "$OUT" | wc -l)"
echo "Test:  $(awk -F'\t' '$2=="test"' "$OUT" | wc -l)"
