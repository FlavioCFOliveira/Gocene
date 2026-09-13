import os
import re

lucene_files_file = "/home/flavio/.claude/projects/-data-dev-github-com-FlavioCFOliveira-Gocene/c8ec1881-8547-4abd-9519-c1b5bb7556da/tool-results/b781pzd92.txt"
gocene_files_file = "/home/flavio/.claude/projects/-data-dev-github-com-FlavioCFOliveira-Gocene/c8ec1881-8547-4abd-9519-c1b5bb7556da/tool-results/b2xhxs6sv.txt"

def camel_to_snake(name):
    s1 = re.sub('(.)([A-Z][a-z]+)', r'\1_\2', name)
    return re.sub('([a-z0-9])([A-Z])', r'\1_\2', s1).lower()

with open(lucene_files_file, 'r') as f:
    lucene_paths = [line.strip() for line in f if line.strip()]

with open(gocene_files_file, 'r') as f:
    gocene_paths = set([line.strip().lstrip('./') for line in f if line.strip()])

results = {}

for path in lucene_paths:
    parts = path.split('/')
    try:
        idx = parts.index('org')
        lucene_pkg_start = idx
    except ValueError:
        continue

    pkg_parts = parts[lucene_pkg_start + 3:]
    if not pkg_parts: continue
    
    pkg_path = "/".join(pkg_parts[:-1])
    class_full = pkg_parts[-1]
    class_name = class_full.replace('.java', '')
    
    snake_name = camel_to_snake(class_name)
    predicted_path = f"{pkg_path}/{snake_name}.go"
    
    # Global search for any file containing the class name in snake_case or camelCase
    found_path = None
    if predicted_path in gocene_paths:
        found_path = predicted_path
    else:
        # Search all gocene paths for a match
        for gp in gocene_paths:
            # Check if the file name (without .go) contains the snake_name
            gp_name = os.path.splitext(os.path.basename(gp))[0]
            if snake_name == gp_name or class_name.lower() == gp_name.lower():
                found_path = gp
                break
    
    status = "MISSING"
    if found_path:
        status = "PORTED"
        # Check for packaging deviation
        if not found_path.startswith(pkg_path + "/"):
            status = "PORTED (Pkg Dev)"
    
    results[path] = {
        "pkg": pkg_path,
        "class": class_name,
        "status": status,
        "found": found_path
    }

# Group by package
grouped = {}
for path, data in results.items():
    pkg = data["pkg"]
    if pkg not in grouped:
        grouped[pkg] = []
    grouped[pkg].append(data)

total_lucene = len(results)
ported_count = sum(1 for d in results.values() if "PORTED" in d["status"])
print(f"TOTAL_LUCENE:{total_lucene}")
print(f"PORTED_COUNT:{ported_count}")
print(f"PERCENTAGE:{(ported_count/total_lucene)*100 if total_lucene > 0 else 0:.2f}%")

for pkg, components in grouped.items():
    print(f"\nPackage: {pkg}")
    for c in components:
        print(f"  {c['class']} [{c['status']}]")
