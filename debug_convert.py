#!/usr/bin/env python3
"""Debug template conversion - simulating Go regex"""
import re
from pathlib import Path

template_file = Path(__file__).parent / "database" / "templates" / "download_site.html"
content = template_file.read_text(encoding="utf-8")

# Simulate Go converter rules (using Python syntax)
rules = [
    (r'\{\{\s*random_keyword\s*\(\s*\)\s*\}\}', '{{.RandomKeyword}}'),
    (r'\{\{\s*random_hotspot\s*\(\s*\)\s*\}\}', '{{.RandomKeyword}}'),
    (r'\{\{\s*keyword_with_emoji\s*\(\s*\)\s*\}\}', '{{.RandomKeyword}}'),
    (r'\{\{\s*random_url\s*\(\s*\)\s*\}\}', '{{.RandomURL}}'),
    (r'\{\{\s*random_image\s*\(\s*\)\s*\}\}', '{{.RandomImage}}'),
    (r'\{\{\s*content\s*\(\s*\)\s*\}\}', '{{.Content}}'),
    (r'\{\{\s*content_with_pinyin\s*\(\s*\)\s*\}\}', '{{.Content}}'),
    (r'\{\{\s*now\s*\(\s*\)\s*\}\}', '{{.Now}}'),
    (r"\{\{\s*cls\s*\(\s*['\"]([^'\"]*)['\"]?\s*\)\s*\}\}", r'{{.Cls "\1"}}'),
    (r"\{\{\s*encode\s*\(\s*['\"]([^'\"]+)['\"]?\s*\)\s*\}\}", r'{{.Encode "\1"}}'),
    (r'\{\{\s*random_number\s*\(\s*(\d+)\s*,\s*(\d+)\s*\)\s*\}\}', r'{{.RandomNumber \1 \2}}'),
    (r'\{\{\s*title\s*\}\}', '{{.Title}}'),
    (r'\{\{\s*site_id\s*\}\}', '{{.SiteID}}'),
    (r'\{\{\s*analytics_code\s*\}\}', '{{.AnalyticsCode}}'),
    (r'\{\{\s*baidu_push_js\s*\}\}', '{{.BaiduPushJS}}'),
    (r'\{\{\s*article_content\s*\}\}', '{{.ArticleContent}}'),
    (r"\{\{\s*analytics_code\s+or\s+['\"]['\"]?\s*\}\}", '{{.AnalyticsCode}}'),
    (r"\{\{\s*baidu_push_js\s+or\s+['\"]['\"]?\s*\}\}", '{{.BaiduPushJS}}'),
    # Loop variable {{ i }} -> {{$i}}
    (r'\{\{\s*i\s*\}\}', '{{$i}}'),
    # For loops
    (r'\{%\s*for\s+(\w+)\s+in\s+range\s*\(\s*(\d+)\s*\)\s*%\}', r'{{range $\1 := iterate \2}}'),
    (r'\{%\s*endfor\s*%\}', '{{end}}'),
    (r'\{%\s*if\s+([^%]+)\s*%\}', r'{{if \1}}'),
    (r'\{%\s*elif\s+([^%]+)\s*%\}', r'{{else if \1}}'),
    (r'\{%\s*else\s*%\}', '{{else}}'),
    (r'\{%\s*endif\s*%\}', '{{end}}'),
    (r'\{#[^#]*#\}', ''),
]

result = content
for pattern, replacement in rules:
    result = re.sub(pattern, replacement, result)

lines = result.split('\n')

# Count range and end
range_count = sum(1 for line in lines if '{{range' in line)
end_count = sum(1 for line in lines if '{{end}}' in line)
print(f"{{{{range}}}}: {range_count}, {{{{end}}}}: {end_count}")

if range_count != end_count:
    print("\n=== Unmatched range/end ===")
    depth = 0
    for i, line in enumerate(lines, 1):
        if '{{range' in line:
            depth += 1
            print(f"  Line {i} (depth={depth}): {line.strip()[:80]}")
        if '{{end}}' in line:
            print(f"  Line {i} (depth={depth}): {line.strip()[:80]}")
            depth -= 1

print("\n=== Unique template expressions ===")
pattern = r'\{\{[^}]+\}\}'
matches = set(re.findall(pattern, result))
for m in sorted(matches):
    if 'Cls' not in m and 'Random' not in m:
        print(m)

# Write converted for inspection
Path("converted_debug.txt").write_text(result, encoding="utf-8")
print(f"\nConverted template written to converted_debug.txt ({len(result)} chars)")
