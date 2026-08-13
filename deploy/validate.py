#!/usr/bin/env python3
"""Static safety checks for the production deployment bundle.

This validator does not contact Docker, Cloudflare, PostgreSQL, or Redis. It
only checks that the checked-in deployment contract cannot accidentally omit
immutable images, container hardening, health checks, managed-service wiring,
or the Cloudflare-to-Nginx cutover boundary.
"""

from __future__ import annotations

import re
import sys
from pathlib import Path
from typing import Any


ROOT = Path(__file__).resolve().parent
COMPOSE_PATH = ROOT / "docker-compose.production.yml"
NGINX_PATH = ROOT / "nginx" / "sma-api.conf.template"
NGINX_MAIN_PATH = ROOT / "nginx" / "nginx.conf"
RELEASE_ENV_PATH = ROOT / "env.production.example"
BACKUP_PATH = ROOT / "backup.sh"


class ValidationError(RuntimeError):
    """Raised when a deployment safety invariant is absent."""


def require(condition: bool, message: str) -> None:
    if not condition:
        raise ValidationError(message)


def load_compose() -> dict[str, Any]:
    try:
        import yaml
    except ImportError as exc:  # pragma: no cover - environment-dependent guard
        raise ValidationError(f"PyYAML is required: {exc}") from exc
    try:
        document = yaml.safe_load(COMPOSE_PATH.read_text(encoding="utf-8"))
    except yaml.YAMLError as exc:
        raise ValidationError(f"invalid compose YAML: {exc}") from exc
    require(isinstance(document, dict), "compose must be a mapping")
    return document


def validate_image_input(value: str, name: str) -> None:
    require("@sha256:" in value, f"{name} must require an image@sha256 digest")
    require(":?" in value, f"{name} must fail closed when the digest variable is unset")


def validate_resources(service: dict[str, Any], name: str) -> None:
    limits = service.get("deploy", {}).get("resources", {}).get("limits", {})
    require(isinstance(limits, dict), f"{name} needs deploy resource limits")
    require(limits.get("cpus"), f"{name} needs a CPU limit")
    require(limits.get("memory"), f"{name} needs a memory limit")


def validate_hardening(service: dict[str, Any], name: str) -> None:
    require(service.get("read_only") is True, f"{name} must use a read-only root filesystem")
    require(str(service.get("user", "")).split(":", 1)[0] not in {"", "0"}, f"{name} must run as non-root")
    require("no-new-privileges:true" in service.get("security_opt", []), f"{name} needs no-new-privileges")
    require("ALL" in service.get("cap_drop", []), f"{name} must drop all Linux capabilities")
    require(service.get("healthcheck"), f"{name} needs a healthcheck")
    validate_resources(service, name)


def validate_compose(document: dict[str, Any] | None = None) -> None:
    document = load_compose() if document is None else document
    services = document.get("services")
    require(isinstance(services, dict), "compose needs services")
    require({"api", "worker", "nginx", "prometheus", "alertmanager", "grafana"} <= set(services), "missing production service")
    require(not ({"postgres", "redis"} & set(services)), "PostgreSQL and Redis must remain managed external services")

    for service_name in ("api", "worker", "nginx", "prometheus", "alertmanager", "grafana"):
        service = services[service_name]
        require(isinstance(service, dict), f"{service_name} must be a mapping")
        validate_resources(service, service_name)

    for service_name in ("api", "worker", "nginx"):
        validate_hardening(services[service_name], service_name)

    for service_name in ("api", "worker", "nginx", "prometheus", "alertmanager", "grafana"):
        validate_image_input(str(services[service_name].get("image", "")), f"{service_name}.image")

    api_environment = services["api"].get("environment", {})
    require(api_environment.get("ENV") == "production", "api must set ENV=production")
    require(api_environment.get("DB_SSL_MODE") == "require", "api must require PostgreSQL TLS")
    require(api_environment.get("REDIS_TLS") == "true", "api must require Redis TLS")

    nginx = services["nginx"]
    ports = {str(port) for port in nginx.get("ports", [])}
    require('80:8080' in ports and '443:8443' in ports, "Nginx must expose HTTP/HTTPS through high container ports")
    require("./nginx/proxy_params:/etc/nginx/proxy_params:ro" in nginx.get("volumes", []), "Nginx proxy headers must be mounted")
    require("api" in nginx.get("depends_on", {}), "Nginx must wait for the Go API")
    require("app" in nginx.get("networks", []) and "edge" in nginx.get("networks", []), "Nginx needs edge and app networks")
    healthcheck = nginx.get("healthcheck", {}).get("test", [])
    healthcheck_text = " ".join(str(item) for item in healthcheck)
    require("127.0.0.1:8080/health" in healthcheck_text, "Nginx healthcheck must probe its 8080 listener")

    worker_profiles = services["worker"].get("profiles", [])
    require("worker" in worker_profiles, "worker profile must be explicit while the queue is in-process")

    networks = document.get("networks", {})
    require(networks.get("app", {}).get("internal") is True, "app network must be internal")
    require(networks.get("observability", {}).get("internal") is True, "observability network must be internal")


def validate_nginx() -> None:
    template = NGINX_PATH.read_text(encoding="utf-8")
    main = NGINX_MAIN_PATH.read_text(encoding="utf-8")
    for token in (
        "upstream go_api",
        "upstream legacy_api",
        "split_clients",
        "CANARY_PERCENTAGE",
        "ROUTE_TO_GO",
        "limit_req zone=auth_limit",
        "ssl_protocols TLSv1.2 TLSv1.3",
        "location = /health",
        "location = /ready",
        "add_header Permissions-Policy",
        "add_header Content-Security-Policy",
    ):
        require(token in template, f"Nginx template missing {token}")
    require("map $http_cf_connecting_ip $sma_client_ip" in main, "Nginx must canonicalise the Cloudflare client IP")
    require("limit_req_zone $sma_client_ip" in main, "Nginx limits must use the canonical client IP")
    require("limit_req_status 429;" in main, "Nginx rate limits must return HTTP 429")
    require("X-XSS-Protection" not in template and "X-XSS-Protection" not in main, "obsolete X-XSS-Protection must not be configured")
    require("proxy_pass http://${DOLLAR}route_backend" in template, "Nginx must route through the cutover backend")
    require("include /tmp/nginx-conf/*.conf" in main, "Nginx must render templates into a writable non-root path")


def validate_dockerfiles() -> None:
    for filename in ("Dockerfile.api", "Dockerfile.worker"):
        content = (ROOT / filename).read_text(encoding="utf-8")
        require("distroless/static-debian12:nonroot" in content, f"{filename} must use a non-root runtime")
        require("USER nonroot:nonroot" in content, f"{filename} must declare non-root user")
        require("HEALTHCHECK" in content, f"{filename} must define a container healthcheck")
        require(":latest" not in content, f"{filename} must not use latest")


def validate_monitoring() -> None:
    prometheus = (ROOT / "monitoring" / "prometheus.yml").read_text(encoding="utf-8")
    alertmanager = (ROOT / "monitoring" / "alertmanager.yml").read_text(encoding="utf-8")
    datasource = (ROOT / "monitoring" / "grafana" / "provisioning" / "datasources" / "prometheus.yml").read_text(encoding="utf-8")
    dashboard = (ROOT / "monitoring" / "grafana" / "provisioning" / "dashboards" / "sma-api.yml").read_text(encoding="utf-8")
    require("/etc/prometheus/sma-api-alerts.yml" in prometheus, "Prometheus must load the existing SMA alerts")
    require("api:8080" in prometheus and "/metrics" in prometheus, "Prometheus must scrape the Go API metrics endpoint")
    require("operations-email" in alertmanager and "send_resolved: true" in alertmanager, "Alertmanager must notify operations")
    require("DS_PROMETHEUS" in datasource and "prometheus:9090" in datasource, "Grafana datasource provisioning is incomplete")
    require("/var/lib/grafana/dashboards" in dashboard, "Grafana dashboard provisioning is incomplete")


def validate_backup() -> None:
    content = BACKUP_PATH.read_text(encoding="utf-8")
    require("verify-backup.sh" in content, "backup wrapper must use the verified dump primitive")
    require("rclone copyto" in content, "backup wrapper must upload each artifact through rclone")
    require("SMA_BACKUP_ENCRYPTED" in content, "backup wrapper must require an encrypted remote")
    require("restore-integrity.json" in content, "backup wrapper must upload restore evidence when present")
    require("rclone purge" not in content and "rclone delete" not in content, "backup wrapper must not delete remote backups")


def validate_release_example() -> None:
    content = RELEASE_ENV_PATH.read_text(encoding="utf-8")
    for key in ("SMA_API_IMAGE", "SMA_WORKER_IMAGE", "NGINX_IMAGE", "PROMETHEUS_IMAGE", "ALERTMANAGER_IMAGE", "GRAFANA_IMAGE"):
        match = re.search(rf"^{key}=(.+)$", content, re.MULTILINE)
        require(match is not None and "@sha256:" in match.group(1), f"release example missing digest input: {key}")
    require("DB_SSL_MODE=require" in content, "release example must require PostgreSQL TLS")
    require("REDIS_TLS=true" in content, "release example must require Redis TLS")
    require("JWT_SECRET=REPLACE_IN_SECRET_STORE" in content, "release example must keep JWT_SECRET in the secret store")
    require("REFRESH_TOKEN_EXPIRATION=" in content, "release example must set refresh-token lifetime")
    require(re.search(r"^PASSWORD_RESET_URL=https://\S+", content, re.MULTILINE) is not None, "release example must set the password-reset URL")
    require(re.search(r"^PORTAL_PASSWORD_RESET_URL=https://\S+", content, re.MULTILINE) is not None, "release example must set the portal password-reset URL")
    require(re.search(r"^ALLOWED_ORIGINS=https://[^\s,]+", content, re.MULTILINE) is not None, "release example must pin an explicit admin CORS origin")
    require("SMTP_ENABLED=true" in content, "release example must enable SMTP password-reset delivery")
    require(re.search(r"^SMTP_TLS_MODE=(starttls|tls)$", content, re.MULTILINE) is not None, "release example must use encrypted SMTP transport")
    require("REPLACE_IN_SECRET_STORE" in content, "release example must keep secrets out of git")


def validate() -> None:
    for path in (COMPOSE_PATH, NGINX_PATH, NGINX_MAIN_PATH, RELEASE_ENV_PATH, BACKUP_PATH):
        require(path.is_file(), f"missing deployment artifact: {path}")
    validate_compose()
    validate_nginx()
    validate_dockerfiles()
    validate_monitoring()
    validate_backup()
    validate_release_example()


def main() -> int:
    try:
        validate()
    except (OSError, ValidationError) as exc:
        print(f"deployment validation failed: {exc}", file=sys.stderr)
        return 1
    print(f"validated production deployment bundle: {ROOT}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
