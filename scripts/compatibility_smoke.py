#!/usr/bin/env python3
"""Validate compatibility operations and optionally run seeded HTTP smoke checks.

The static portion runs without a database and verifies that every compatibility
operation is registered by the gateway and present in generated Swagger. Set
RUN_COMPATIBILITY_SMOKE=1, BASE_URL, and ACCESS_TOKEN to run the read-only
seeded checks against a live API. Mutating checks are opt-in and require fixture
IDs so this script cannot modify arbitrary data accidentally.
"""

import json
import os
import re
import sys
import urllib.error
import urllib.request
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
GATEWAY = (ROOT / "cmd/api-gateway/main.go").read_text()
SWAGGER = json.loads((ROOT / "api/swagger/swagger.json").read_text())

REQUIRED = {
    ("GET", "/students/roster"),
    ("GET", "/teachers/roster"),
    ("GET", "/grades/report"),
    ("PUT", "/grades/{id}"),
    ("PATCH", "/grades/{id}"),
    ("DELETE", "/grades/{id}"),
    ("PUT", "/grade-components/{id}"),
    ("DELETE", "/grade-components/{id}"),
    ("POST", "/students/import"),
    ("POST", "/teachers/import"),
    ("GET", "/export/students"),
    ("GET", "/export/grades"),
    ("GET", "/export/attendance"),
    ("GET", "/exam-events"),
    ("POST", "/exam-events"),
    ("PUT", "/exam-events/{id}"),
    ("DELETE", "/exam-events/{id}"),
    ("POST", "/attendance"),
    ("PUT", "/attendance/{id}"),
    ("PATCH", "/attendance/{id}"),
    ("POST", "/teacher-preferences"),
    ("PUT", "/teacher-preferences/{id}"),
}


def registered_operations():
    groups = {"api": ""}
    group_re = re.compile(r'(\w+)\s*:=\s*(\w+)\.Group\("([^"]*)"\)')
    route_re = re.compile(r'(\w+)\.(GET|POST|PUT|PATCH|DELETE|OPTIONS|HEAD)\("([^"]*)"')
    for line in GATEWAY.splitlines():
        match = group_re.search(line)
        if match and match.group(2) in groups:
            name, parent, suffix = match.groups()
            groups[name] = groups[parent].rstrip("/") + "/" + suffix.lstrip("/") if suffix else groups[parent]
    operations = set()
    for line in GATEWAY.splitlines():
        match = route_re.search(line)
        if not match:
            continue
        group, method, suffix = match.groups()
        if group not in groups:
            continue
        path = groups[group].rstrip("/") + "/" + suffix.lstrip("/") if suffix else groups[group]
        path = re.sub(r":([A-Za-z0-9_]+)", r"{\1}", path or "/")
        prefix = os.environ.get("API_PREFIX", "/api/v1").rstrip("/")
        if path.startswith(prefix + "/") or path == prefix:
            path = path[len(prefix) :] or "/"
        operations.add((method, path))
    return operations


def static_check():
    registered = registered_operations()
    swagger_paths = SWAGGER.get("paths", {})
    swagger_ops = {(method.upper(), path) for path, spec in swagger_paths.items() for method in spec}
    missing_gateway = sorted(REQUIRED - registered)
    missing_swagger = sorted(REQUIRED - swagger_ops)
    if missing_gateway or missing_swagger:
        if missing_gateway:
            print("Gateway is missing compatibility operations:")
            for operation in missing_gateway:
                print(f"  {operation[0]} {operation[1]}")
        if missing_swagger:
            print("Swagger is missing compatibility operations:")
            for operation in missing_swagger:
                print(f"  {operation[0]} {operation[1]}")
        return False
    print(f"Validated {len(REQUIRED)} compatibility operations in gateway and Swagger.")
    return True


def request(base_url, token, method, path, body=None, content_type="application/json", extra_headers=None):
    headers = {"Accept": "application/json", "Authorization": f"Bearer {token}"}
    if body is not None:
        headers["Content-Type"] = content_type
    if extra_headers:
        headers.update(extra_headers)
    data = body.encode() if isinstance(body, str) else body
    req = urllib.request.Request(base_url.rstrip("/") + path, data=data, headers=headers, method=method)
    try:
        with urllib.request.urlopen(req, timeout=15) as response:
            return response.status, response.read()
    except urllib.error.HTTPError as error:
        return error.code, error.read()
    except urllib.error.URLError as error:
        return 0, str(error).encode()


def seeded_check():
    if os.environ.get("RUN_COMPATIBILITY_SMOKE") != "1":
        print("Seeded compatibility smoke skipped (set RUN_COMPATIBILITY_SMOKE=1 to run).")
        return True
    base_url = os.environ.get("BASE_URL")
    token = os.environ.get("ACCESS_TOKEN")
    if not base_url or not token:
        print("Seeded compatibility smoke requires BASE_URL and ACCESS_TOKEN.", file=sys.stderr)
        return False
    checks = [
        ("GET", "/students/roster?limit=1"),
        ("GET", "/teachers/roster?limit=1"),
        ("GET", "/grades/report?limit=1"),
        ("GET", "/export/students"),
        ("GET", "/export/grades"),
        ("GET", "/export/attendance"),
        ("GET", "/exam-events?limit=1"),
    ]
    failures = []
    for method, path in checks:
        status, _ = request(base_url, token, method, path)
        if status != 200:
            failures.append(f"{method} {path} returned {status}")
    if failures:
        print("Seeded compatibility smoke failures:", file=sys.stderr)
        print("\n".join(f"  {failure}" for failure in failures), file=sys.stderr)
        return False

    print(f"Seeded read-only compatibility smoke passed ({len(checks)} requests).")
    if os.environ.get("RUN_MUTATING_SMOKE") != "1":
        print("Seeded mutating compatibility smoke skipped (set RUN_MUTATING_SMOKE=1 with fixture IDs/payloads).")
        return True

    mutating = []

    def add_fixture(method, path, env_name, payload=None, content_type="application/json", headers=None):
        if not os.environ.get(env_name):
            return
        status, _ = request(base_url, token, method, path, payload, content_type, headers)
        mutating.append((method, path, status))

    grade_edit_id = os.environ.get("SMOKE_GRADE_EDIT_ID")
    if grade_edit_id and os.environ.get("SMOKE_GRADE_EDIT_PAYLOAD"):
        add_fixture("PUT", f"/grades/{grade_edit_id}", "SMOKE_GRADE_EDIT_PAYLOAD", os.environ["SMOKE_GRADE_EDIT_PAYLOAD"])
    grade_delete_id = os.environ.get("SMOKE_GRADE_DELETE_ID")
    if grade_delete_id:
        add_fixture("DELETE", f"/grades/{grade_delete_id}", "SMOKE_GRADE_DELETE_ID")
    component_edit_id = os.environ.get("SMOKE_COMPONENT_EDIT_ID")
    if component_edit_id and os.environ.get("SMOKE_COMPONENT_EDIT_PAYLOAD"):
        add_fixture("PUT", f"/grade-components/{component_edit_id}", "SMOKE_COMPONENT_EDIT_PAYLOAD", os.environ["SMOKE_COMPONENT_EDIT_PAYLOAD"])
    component_delete_id = os.environ.get("SMOKE_COMPONENT_DELETE_ID")
    if component_delete_id:
        add_fixture("DELETE", f"/grade-components/{component_delete_id}", "SMOKE_COMPONENT_DELETE_ID")

    student_csv = os.environ.get("SMOKE_STUDENT_CSV_PATH")
    if student_csv and os.path.exists(student_csv):
        body = Path(student_csv).read_bytes()
        key = os.environ.get("SMOKE_STUDENT_IMPORT_KEY", "compatibility-smoke-student")
        add_fixture("POST", "/students/import", "SMOKE_STUDENT_CSV_PATH", body, "text/csv", {"Idempotency-Key": key})
        add_fixture("POST", "/students/import", "SMOKE_STUDENT_CSV_PATH", body, "text/csv", {"Idempotency-Key": key})
    teacher_csv = os.environ.get("SMOKE_TEACHER_CSV_PATH")
    if teacher_csv and os.path.exists(teacher_csv):
        body = Path(teacher_csv).read_bytes()
        key = os.environ.get("SMOKE_TEACHER_IMPORT_KEY", "compatibility-smoke-teacher")
        add_fixture("POST", "/teachers/import", "SMOKE_TEACHER_CSV_PATH", body, "text/csv", {"Idempotency-Key": key})
        add_fixture("POST", "/teachers/import", "SMOKE_TEACHER_CSV_PATH", body, "text/csv", {"Idempotency-Key": key})

    attendance_payload = os.environ.get("SMOKE_ATTENDANCE_PAYLOAD")
    if attendance_payload:
        add_fixture("POST", "/attendance", "SMOKE_ATTENDANCE_PAYLOAD", attendance_payload)
    if os.environ.get("SMOKE_ATTENDANCE_ID") and attendance_payload:
        add_fixture("PATCH", f"/attendance/{os.environ['SMOKE_ATTENDANCE_ID']}", "SMOKE_ATTENDANCE_PAYLOAD", attendance_payload)
    preference_payload = os.environ.get("SMOKE_TEACHER_PREFERENCE_PAYLOAD")
    if preference_payload:
        add_fixture("POST", "/teacher-preferences", "SMOKE_TEACHER_PREFERENCE_PAYLOAD", preference_payload)
    if os.environ.get("SMOKE_TEACHER_PREFERENCE_ID") and preference_payload:
        add_fixture("PUT", f"/teacher-preferences/{os.environ['SMOKE_TEACHER_PREFERENCE_ID']}", "SMOKE_TEACHER_PREFERENCE_PAYLOAD", preference_payload)

    mutation_failures = [f"{method} {path} returned {status}" for method, path, status in mutating if not 200 <= status < 300]
    if mutation_failures:
        print("Seeded mutating compatibility smoke failures:", file=sys.stderr)
        print("\n".join(f"  {failure}" for failure in mutation_failures), file=sys.stderr)
        return False
    print(f"Seeded mutating compatibility smoke passed ({len(mutating)} requests; only configured fixtures were exercised).")
    return True


if __name__ == "__main__":
    raise SystemExit(0 if static_check() and seeded_check() else 1)
