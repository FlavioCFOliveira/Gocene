import collections

def assign_priority(package, class_name):
    # Basic heuristic for priority
    # P2: Medium (Important but not blocker for basic index/search)
    # P3: Low (Utility, edge-case, or specialized feature)
    
    if package in ['index', 'store', 'analysis', 'search', 'codecs']:
        return 'P2'
    if package in ['geo', 'util']:
        return 'P3'
    return 'P3'

with open('missing_components.txt', 'r') as f:
    lines = f.readlines()

grouped = collections.defaultdict(list)
for line in lines:
    pkg, cls, path = line.strip().split('|')
    grouped[pkg].append((cls, assign_priority(pkg, cls)))

# Print as a structured report
print("# Gocene Gap Analysis Report")
print("\n## Summary")
# I already have the numbers from the previous run
print("- Total Lucene Classes: 1175")
print("- Ported: 892")
print("- Missing: 283")
print("- Total Translation: 75.91%")

print("\n## Gaps by Package")
for pkg in sorted(grouped.keys()):
    print(f"\n### Package: `{pkg}`")
    print("| Component | Criticality | Gocene Package | Status |")
    print("|---|---|---|---|")
    for cls, prio in grouped[pkg]:
        # Gocene package usually matches Lucene package
        gocene_pkg = f"github.com/FlavioCFOliveira/Gocene/{pkg}"
        print(f"| {cls} | {prio} | {gocene_pkg} | MISSING |")

