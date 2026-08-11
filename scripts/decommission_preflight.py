#!/usr/bin/env python3
"""Validate cutover prerequisites without changing deployment state.

The preflight is intentionally read-only. It checks the values that must be
verified before the legacy service is retired and, unless --skip-http is used,
probes the configured Go and legacy health endpoints. It does not toggle
ROUTE_TO_GO, rotate secrets, change ingress, or delete any legacy resources.
"""

from __future__ import annotations

import argparse
import json
import os
import sys
import urllib.error
import urllib.request
from pathlib import Path
from typing import Mapping


DEFAULTS = {
    "GO_HEALTH_URL": "http://localhost:8080/health",
    "LEGACY_HEALTH_URL": "http://localhost:3000/health",
    "CANARY_PERCENTAGE": "0",
    "ROUTE_TO_GO": "false",
    "SHADOW_TRAFFIC": "false",
    "LEGACY_READONLY": "false",
}


def load_env_file(path: Path) -> dict[str, str]:
    values: dict[str, str] = {}
    if not path.exists():
        return values
    for raw_line in path.read_text(encoding="utf-8").splitlines():
        line = raw_line.strip()
        if not line or line.startswith("#") or "=" not in line:
            continue
        key, value = line.split("=", 1)
        key = key.strip()
        value = value.strip().strip("'\"")
        if key:
            values[key] = value
    return values


def merged_environment(env_file: Path) -> dict[str, str]:
    values = load_env_file(env_file)
    # The process environment wins over the file, matching deployment behavior.
    values.update(os.environ)
    return values


def check(name: str, passed: bool, detail: str) -> dict[str, object]:
    return {"name": name, "status": "PASS" if passed else "FAIL", "detail": detail}


def health_probe(url: str, timeout: float) -> tuple[bool, str]:
    request = urllib.request.Request(url, headers={"Accept": "application/json"})
    try:
        with urllib.request.urlopen(request, timeout=timeout) as response:
            body = response.read().decode("utf-8", errors="replace")
            if response.status < 200 or response.status >= 300:
                return False, f"HTTP {response.status}"
            return True, f"HTTP {response.status} ({body[:120]})"
    except urllib.error.HTTPError as error:
        return False, f"HTTP {error.code}"
    except (urllib.error.URLError, TimeoutError, OSError) as error:
        return False, str(error)


def run_preflight(values: Mapping[str, str], skip_http: bool, timeout: float) -> list[dict[str, object]]:
    get = lambda key: values.get(key, DEFAULTS.get(key, ""))
    results = [
        check("ROUTE_TO_GO", get("ROUTE_TO_GO").lower() == "true", "must be true before legacy retirement"),
        check("LEGACY_READONLY", get("LEGACY_READONLY").lower() == "true", "must be true before legacy retirement"),
        check(
            "SHADOW_TRAFFIC",
            get("SHADOW_TRAFFIC").lower() == "false",
            "must be false before removing the legacy upstream",
        ),
        check(
            "CANARY_PERCENTAGE",
            get("CANARY_PERCENTAGE") == "100",
            "must be 100 after full cutover",
        ),
        check(
            "JWT_SECRET",
            bool(values.get("JWT_SECRET")) and values["JWT_SECRET"] not in {"change_me_in_prod", "change-me"},
            "must be set to a non-default production secret",
        ),
        check(
            "REPORTS_SIGNED_URL_SECRET",
            bool(values.get("REPORTS_SIGNED_URL_SECRET"))
            and values["REPORTS_SIGNED_URL_SECRET"] not in {"change_me_reports", "change-me"},
            "must be set to a non-default signing secret",
        ),
        check(
            "ALLOWED_ORIGINS",
            bool(values.get("ALLOWED_ORIGINS")) and "*" not in values["ALLOWED_ORIGINS"],
            "must contain explicit origins and no wildcard",
        ),
    ]

    if skip_http:
        results.append(check("health probes", True, "skipped by --skip-http; run against staging before cutover"))
        return results

    for name in ("GO_HEALTH_URL", "LEGACY_HEALTH_URL"):
        url = get(name)
        reachable, detail = health_probe(url, timeout)
        results.append(check(name, reachable, f"{url}: {detail}"))
    return results


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--env-file", type=Path, default=Path(".env"), help="dotenv file to inspect (default: .env)")
    parser.add_argument("--skip-http", action="store_true", help="skip health probes; useful for static CI validation")
    parser.add_argument("--timeout", type=float, default=2.0, help="health probe timeout in seconds")
    parser.add_argument("--json", action="store_true", dest="as_json", help="emit machine-readable results")
    args = parser.parse_args()

    values = merged_environment(args.env_file)
    results = run_preflight(values, args.skip_http, args.timeout)
    passed = all(result["status"] == "PASS" for result in results)

    if args.as_json:
        print(json.dumps({"passed": passed, "checks": results}, indent=2))
    else:
        print("Legacy decommission preflight")
        for result in results:
            print(f"[{result['status']}] {result['name']}: {result['detail']}")
        if not passed:
            print("Preflight failed; no deployment state was changed.", file=sys.stderr)

    return 0 if passed else 1


if __name__ == "__main__":
    raise SystemExit(main())
