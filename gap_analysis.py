import os
import re

def snake_case(name):
    # Simple camelCase to snake_case
    s1 = re.sub('(.)([A-Z][a-z]+)', r'\1_\2', name)
    return re.sub('([a-z0-9])([A-Z])', r'\1_\2', s1).lower()

def analyze():
    lucene_files_path = '/home/flavio/.claude/projects/-data-dev-github-com-FlavioCFOliveira-Gocene/c8ec1881-8547-4abd-9519-c1b5bb7556da/tool-results/bg6nes144.txt'
    gocene_files_path = '/home/flavio/.claude/projects/-data-dev-github-com-FlavioCFOliveira-Gocene/c8ec1881-8547-4abd-9519-c1b5bb7556da/tool-results/bjkqhxm3p.txt'

    with open(lucene_files_path, 'r') as f:
        lucene_files = [line.strip() for line in f if line.strip()]
    
    with open(gocene_files_path, 'r') as f:
        gocene_files = [line.strip() for line in f if line.strip()]

    # Store Gocene files in a set for O(1) lookup
    gocene_set = set(gocene_files)

    results = []
    total_lucene_classes = 0

    for lf in lucene_files:
        if lf.endswith('package-info.java'):
            continue
        
        total_lucene_classes += 1
        # Extract package and class
        # /tmp/lucene/lucene/core/src/java/org/apache/lucene/index/IndexWriter.java
        relative_path = lf.replace('/tmp/lucene/lucene/core/src/java/org/apache/lucene/', '')
        dir_name = os.path.dirname(relative_path)
        base_name = os.path.basename(relative_path)
        class_name = base_name[:-5] # remove .java

        # Expected Gocene path
        expected_gocene_path = f"./{dir_name}/{snake_case(class_name)}.go"
        
        # Try to match. Gocene might have it in a slightly different place or name.
        # We check if the expected path exists.
        # Also check for cases where the class name is just the file name (case insensitive)
        
        found = False
        if expected_gocene_path in gocene_set:
            found = True
        else:
            # Try matching just by file name in the same directory
            for gf in gocene_files:
                if gf.startswith(f"./{dir_name}/") and gf.endswith('.go'):
                    gf_base = os.path.basename(gf)[:-3]
                    if gf_base == snake_case(class_name):
                        found = True
                        break
        
        if not found:
            results.append({
                'lucene_path': lf,
                'class_name': class_name,
                'package': dir_name,
                'status': 'MISSING'
            })
        else:
            results.append({
                'lucene_path': lf,
                'class_name': class_name,
                'package': dir_name,
                'status': 'PORTED'
            })

    print(f"Total Lucene Classes: {total_lucene_classes}")
    ported_count = sum(1 for r in results if r['status'] == 'PORTED')
    print(f"Ported: {ported_count}")
    print(f"Missing: {len(results) - ported_count}")
    print(f"Percentage: {(ported_count / total_lucene_classes * 100):.2f}%")
    
    # Write missing components to a file
    with open('missing_components.txt', 'w') as f:
        for r in results:
            if r['status'] == 'MISSING':
                f.write(f"{r['package']}|{r['class_name']}|{r['lucene_path']}\n")

analyze()
