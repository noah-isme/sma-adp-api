# Cloudflare origin and edge checklist

The public request path is Cloudflare → the VPS Nginx listener. Cloudflare is
the only internet-facing origin client; PostgreSQL, Redis, Prometheus,
Alertmanager, Grafana, and the legacy upstream stay private.

## Origin TLS

1. Issue a Cloudflare Origin Certificate for `api.example.com` (and any API
   subdomain used during staging). Install the certificate and key under
   `/etc/sma/tls` with mode `0640` and `0600` respectively.
2. Set the zone SSL/TLS mode to **Full (strict)** and minimum TLS version to
   **TLS 1.2**. The following API calls are exact read-back checks; use a
   scoped API token and do not put it in a repository file:

   ```bash
   export CF_API_TOKEN='from-secret-manager'
   export CF_ZONE_ID='from-secret-manager'
   curl --fail --silent \
     "https://api.cloudflare.com/client/v4/zones/${CF_ZONE_ID}/settings/ssl" \
     -H "Authorization: Bearer ${CF_API_TOKEN}" \
     -H 'Content-Type: application/json'
   curl --fail --silent \
     "https://api.cloudflare.com/client/v4/zones/${CF_ZONE_ID}/settings/min_tls_version" \
     -H "Authorization: Bearer ${CF_API_TOKEN}" \
     -H 'Content-Type: application/json'
   ```

3. Enable **Always Use HTTPS**, HSTS only after confirming every subdomain is
   HTTPS-ready, and disable TLS 1.0/1.1. Record screenshots or JSON responses
   in the release evidence.

## DNS and origin firewall

- Create proxied (orange-cloud) A/AAAA records for `api.example.com` pointing
  to the VPS. Keep the VPS origin address out of public application docs.
- Permit inbound 80/443 only from the published Cloudflare IP ranges at the
  VPS firewall. Permit SSH only from the operations network. Do not expose
  5432, 6379, 9090, 9093, or 3000.
- Configure Nginx to trust the Cloudflare client address only after the origin
  firewall rule is active. Nginx uses `CF-Connecting-IP` for stable canary
  hashing and forwards `X-Forwarded-For` to both upstreams.

## WAF and rate limits

Enable Cloudflare Managed WAF rules and create an API rate-limit rule before
the first canary:

| Match | Threshold | Action |
| --- | ---: | --- |
| `http.host eq "api.example.com" and starts_with(http.request.uri.path, "/api/v1/auth/")` | 60 requests / 60 seconds / IP | Block for 60 seconds |
| `http.host eq "api.example.com" and http.request.uri.path eq "/api/v1/auth/login"` | 10 requests / 60 seconds / IP | Block for 10 minutes |

These Cloudflare limits are an outer guard. The Nginx template also applies
`auth_limit` to login, refresh, forgot-password, and reset-password and an
`api_limit` to the remaining API paths. Capture the rule IDs and a staging
429 response as evidence.

Use a custom WAF rule to challenge clearly automated abuse, but do not block
the frontend’s known API methods. Do not enable request mirroring: state-
changing requests must have exactly one upstream consumer.

## Verification commands

```bash
curl --fail --show-error -D /tmp/sma-headers.txt \
  https://api.example.com/health -o /tmp/sma-health.json
grep -E '^(HTTP/|cf-ray:|strict-transport-security:|x-content-type-options:)' \
  /tmp/sma-headers.txt
curl --fail --show-error https://api.example.com/ready
```

The evidence must show a Cloudflare `cf-ray`, origin security headers, Full
Strict TLS, and 200 responses from `/health` and `/ready`. `/metrics`,
`/internal/*`, and `/debug/*` must return 404 from the public hostname.
