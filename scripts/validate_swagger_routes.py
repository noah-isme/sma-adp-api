#!/usr/bin/env python3
"""Ensure every route registered under the API group has a Swagger path."""

import json
import re
import sys
from pathlib import Path

gateway = Path("cmd/api-gateway/main.go").read_text()
swagger = json.loads(Path("api/swagger/swagger.json").read_text())
paths = set(swagger.get("paths", {}))

groups = {"api": ""}
group_re = re.compile(r"(\w+)\s*:=\s*(\w+)\.Group\(\"([^\"]*)\"\)")
route_re = re.compile(r"(\w+)\.(GET|POST|PUT|PATCH|DELETE|OPTIONS|HEAD)\(\"([^\"]*)\"")

for line in gateway.splitlines():
    match = group_re.search(line)
    if match and match.group(2) in groups:
        name, parent, suffix = match.groups()
        groups[name] = groups[parent].rstrip("/") + "/" + suffix.lstrip("/") if suffix else groups[parent]

missing = set()
for line in gateway.splitlines():
    match = route_re.search(line)
    if not match:
        continue
    group, _, suffix = match.groups()
    if group not in groups:
        continue
    path = groups[group].rstrip("/") + "/" + suffix.lstrip("/") if suffix else groups[group]
    path = path or "/"
    path = re.sub(r":([A-Za-z0-9_]+)", r"{\1}", path)
    if path not in paths:
        missing.add(path)

if missing:
    print("Swagger is missing gateway routes:")
    for path in sorted(missing):
        print(f"  {path}")
    sys.exit(1)

print(f"Validated {len(paths)} Swagger paths against API gateway routes.")
