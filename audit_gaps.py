import os
import re

# Paths to results files
lucene_files_file = "/home/flavio/.claude/projects/-data-dev-github-com-FlavioCFOliveira-Gocene/c8ec1881-8547-4abd-9519-c1b5bb7556da/tool-results/b781pzd92.txt"
gocene_files_file = "/home/flavio/.claude/projects/-data-dev-github-com-FlavioCFOliveira-Gocene/c8ec1881-8547-4abd-9519-c1b5bb7556da/tool-results/b2xhxs6sv.txt"

def camel_to_snake(name):
    s1 = re.sub('(.)([A-Z][a-z]+)', r'\1_\2', name)
    return re.sub('([a-z0-9])([A-Z])', r'\1_\2', s1).lower()

with open(lucene_files_file, 'r') as f:
    lucene_paths = [line.strip() for line in f if line.strip()]

with open(gocene_files_file, 'r') as f:
    gocene_paths = set([line.strip().lstrip('./') for line in f if line.strip()])

# To mark partials, read the audit file
partials = set()
try:
    with open("docs/skipped-tests-audit.md", 'r') as f:
        content = f.read()
        # Simple heuristic: if the class name is in the audit file and associated with a blocker
        # We'll look for common patterns like "IndexWriter" in the blocker reason
except:
    pass

results = {}

for path in lucene_paths:
    # Extract package and class
    # /tmp/lucene/lucene/core/src/java/org/apache/lucene/index/IndexWriter.java
    parts = path.split('/')
    # Find where org/apache/lucene starts
    try:
        idx = parts.index('org')
        lucene_pkg_start = idx
    except ValueError:
        continue

    pkg_parts = parts[lucene_pkg_start + 3:] # skip org, apache, lucene
    if not pkg_parts: continue
    
    pkg_path = "/".join(pkg_parts[:-1])
    class_full = pkg_parts[-1]
    class_name = class_full.replace('.java', '')
    
    # Predicted Gocene path
    snake_name = camel_to_snake(class_name)
    predicted_path = f"{pkg_path}/{snake_name}.go"
    
    # Check existence
    # We check for the exact predicted path OR any .go file in that package containing the class name (case insensitive)
    found = False
    status = "MISSING"
    
    if predicted_path in gocene_paths:
        found = True
        status = "PORTED"
    else:
        # Check for any file in the package that might be it
        for gp in gocene_paths:
            if gp.startswith(pkg_path + "/") and class_name.lower() in gp.lower():
                found = True
                status = "PORTED"
                break
    
    # Check for PARTIAL status from audit file or known blockers
    # This is a coarse check; better to refine later
    # For now, if it's in the audit file as a blocker, it's PARTIAL
    # (I'll implement this by searching the audit content for the class name)
    # But only if it was found as PORTED.
    
    results[path] = {
        "pkg": pkg_path,
        "class": class_name,
        "status": status,
        "predicted": predicted_path
    }

# Group by package
grouped = {}
for path, data in results.items():
    pkg = data["pkg"]
    if pkg not in grouped:
        grouped[pkg] = []
    grouped[pkg].append(data)

# Calculate total
total_lucene = len(results)
ported_count = sum(1 for d in results.values() if d["status"] == "PORTED")

print(f"TOTAL_LUCENE:{total_lucene}")
print(f"PORTED_COUNT:{ported_count}")
print(f"PERCENTAGE:{(ported_count/total_lucene)*100 if total_lucene > 0 else 0:.2f}%")

for pkg, components in grouped.items():
    print(f"\nPackage: {pkg}")
    for c in components:
        print(f"  {c['class']} [{c['status']}]")
