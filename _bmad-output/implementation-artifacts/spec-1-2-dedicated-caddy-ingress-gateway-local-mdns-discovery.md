---
title: 'Story 1.2: Dedicated Caddy Ingress Gateway & Local mDNS Discovery'
type: 'feature'
created: '2026-10-15'
status: 'completed'
baseline_commit: 'b6c45a75e73f000a6cfd4cce945233e531573870'
route: 'dispatch'
review_loop_iteration: 0
context:
  - '_bmad-output/planning-artifacts/architecture/architecture-sovereign-2026-10-15/ARCHITECTURE-SPINE.md'
  - '_bmad-output/implementation-artifacts/epic-1-context.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** The appliance requires zero-configuration local network discovery (`http://sovereign.local`) and unified HTTP ingress without forcing household users to lookup DHCP IP addresses or access raw internal ports.

**Approach:** Deploy a dedicated Caddy container in Compose handling ports 80/443, configure base reverse-proxying for `/`, `/drop`, and `/api/*`, implement dynamic route control via Caddy Admin API (`http://caddy:2019`) in `/internal/ingress`, and broadcast `sovereign.local` via a background mDNS responder in `sovereign-daemon`.

## Boundaries & Constraints

**Always:**
- Caddy acts as the sole external ingress gateway on ports 80 and 443.
- Caddy connects only to `sovereign-ingress` (for upstream apps) and `sovereign-control` (for daemon admin API), with strictly zero access to `/var/run/docker.sock`.
- Ingress configuration must route both `sovereign.local` hostnames and direct DHCP IP addresses identically without HTTP redirect loops or host mismatch errors.
- Forwarded headers (`X-Forwarded-For`, `X-Forwarded-Proto`, `X-Forwarded-Host`, `X-Forwarded-Prefix`) must be preserved for upstream clients.
- `sovereign-daemon` must provide an mDNS responder advertising `sovereign.local` and `_http._tcp` service within 30 seconds of network interface availability.
- Subpath routing must use Caddy `handle_path` pattern to trim prefixes before dispatching to upstreams.

**Never:**
- Never mount `/var/run/docker.sock` in the Caddy container.
- Never expose raw upstream container ports (`:3000`, `:8080`) directly on public `0.0.0.0` host interfaces.
- Never block or reject traffic when clients connect via direct IP address instead of `sovereign.local`.
- Never crash daemon startup if mDNS multicast binding fails (e.g. on restricted interfaces or subnets); degrade with a logged warning.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| mDNS Query on Local Subnet | Client resolves `sovereign.local` | mDNS responder answers with server's local IPv4 within 30s | Returns empty response if interface is down |
| Direct IP Access on Port 80 | HTTP GET `http://192.168.1.100/` | Proxies to `sovereign-web:3000` returning HTTP 200 dashboard | Proxies to error page if web container is offline |
| Subpath Ingress Access | HTTP GET `http://sovereign.local/api/v1/health` | Proxies to `sovereign-daemon:8080` preserving path | Returns HTTP 502 Bad Gateway if daemon is offline |
| Caddy Admin API Synchronization | Daemon issues `SyncRoutes()` | Posts JSON route table to `http://caddy:2019/load` | Returns structured error if Caddy is unreachable |
| Multicast Port Conflict / Restriction | mDNS UDP 5353 already in use | Logs warning and continues daemon boot | Daemon runs normally with IP access preserved |

</frozen-after-approval>

## Code Map

- `/caddy/Caddyfile` -- Default Caddy declarative configuration for `sovereign.local` and catch-all `:80` / `:443`.
- `/docker-compose.yml` -- Updated Compose configuration adding `caddy` service, binding ports 80/443, connected to `sovereign-ingress` and `sovereign-control`.
- `/internal/ingress/caddy.go` -- Implementation of Caddy Admin API client (`http://caddy:2019`) for dynamic route synchronization and status checks.
- `/internal/ingress/caddy_test.go` -- Unit tests for Caddy route configuration generation and Admin API communication.
- `/internal/ingress/mdns.go` -- mDNS responder advertising `sovereign.local` on the local network.
- `/internal/ingress/mdns_test.go` -- Unit test for mDNS service configuration and IP resolution.
- `/cmd/sovereign-daemon/main.go` -- Integrate mDNS responder startup and Caddy client initialization into daemon lifecycle.
- `/api/openapi.yaml` -- Document ingress status endpoint in OpenAPI 3.1 specification.
- `/web/src/lib/api/schema.d.ts` -- Generated TypeScript types updated with ingress endpoints.

## Tasks & Acceptance

**Execution:**
- [x] `/caddy/Caddyfile` -- Author base Caddyfile supporting `sovereign.local` and fallback `:80` routing `/` and `/drop` to `sovereign-web:3000` and `/api/*` to `sovereign-daemon:8080` -- Provides dual-path ingress configuration.
- [x] `/docker-compose.yml` -- Add `caddy` service using `caddy:2-alpine`, expose `80:80` and `443:443`, connect to `sovereign-ingress` and `sovereign-control` -- Deploys dedicated gateway container with socket isolation.
- [x] `/internal/ingress/caddy.go` -- Implement Caddy Admin API client with `SyncRoutes` and `GetStatus` methods -- Enables daemon-driven dynamic ingress management.
- [x] `/internal/ingress/caddy_test.go` -- Unit test verifying Caddy configuration payloads and client error handling -- Validates Admin API integration.
- [x] `/internal/ingress/mdns.go` -- Implement lightweight mDNS responder advertising `sovereign.local` on LAN interfaces -- Enables zero-config network discovery.
- [x] `/internal/ingress/mdns_test.go` -- Unit test verifying mDNS service initialization, network IP detection, and graceful degradation -- Validates mDNS resilience.
- [x] `/cmd/sovereign-daemon/main.go` -- Start mDNS responder and Caddy sync in daemon background routines with graceful shutdown -- Hooks ingress into supervisor lifecycle.
- [x] `/cmd/sovereign-daemon/compose_test.go` -- Update Compose verification test asserting `caddy` service presence, port bindings (80, 443), and absence of `/var/run/docker.sock` -- Enforces architectural boundary invariants.
- [x] `/api/openapi.yaml` -- Add `/api/v1/system/ingress` route status endpoint to OpenAPI contract -- Exposes ingress observability.
- [x] `/web/src/lib/api/` -- Regenerate TypeScript client and update client wrapper with `getIngressStatus` -- Extends contract-driven client.

**Acceptance Criteria:**
- Given server boot on a local network, when mDNS initializes, then `sovereign.local` is broadcast via mDNS and resolves to the server's LAN IP address within 30 seconds.
- Given the Caddy ingress gateway, when HTTP requests arrive on port 80 or 443 for `/` and `/drop`, then Caddy reverse-proxies them to `sovereign-web:3000`.
- Given `sovereign-daemon`, when managing routes, then it interacts with Caddy via `http://caddy:2019` over the `sovereign-control` Docker bridge network.
- Given a client connecting directly via DHCP IP address without mDNS, then Caddy proxies identically to `sovereign-web` and `sovereign-daemon` without errors or redirect loops.
- Given the Docker Compose specification, when inspected, then Caddy binds ports 80/443, has zero mounts for `/var/run/docker.sock`, and is connected to `sovereign-ingress` and `sovereign-control`.

## Implementation Notes

Implemented dedicated Caddy ingress gateway and mDNS responder:
1. Base Caddy configuration in `/caddy/Caddyfile` with Admin API enabled on `0.0.0.0:2019`, `auto_https disable_redirects` to prevent IP access redirect loops, and reverse proxy routing for `/api/*` (preserving path) and `/` + `/drop` to `sovereign-web:3000`.
2. Dedicated `caddy` service in Compose (`docker-compose.yml`) exposing 80/443, connected to `sovereign-ingress` and `sovereign-control`, with no Docker socket mounts, and isolated upstream ports for daemon and web to `127.0.0.1`.
3. Ingress management client in `/internal/ingress/caddy.go` implementing `SyncRoutes` and `GetStatus` interacting with Caddy Admin API (`/load` and `/config/`).
4. Zero-configuration mDNS discovery responder in `/internal/ingress/mdns.go` using `github.com/grandcat/zeroconf` advertising `sovereign.local` and `_http._tcp` with automatic LAN IPv4 detection and graceful degradation on port/multicast conflicts.
5. Ingress observability endpoint `/api/v1/system/ingress` added to OpenAPI specification, daemon HTTP server, and generated Next.js TypeScript client wrapper (`getIngressStatus`).
6. Full test coverage across Go unit tests, Compose architectural boundary verification, and Next.js test/build pipeline.

## Spec Change Log

## Review Triage Log

## Design Notes

- Caddy Admin API: Caddy runs its administrative control endpoint at `http://caddy:2019` by default. Over `sovereign-control`, the Go supervisor daemon issues HTTP requests to `/config/` or `/load` to inject routes.
- mDNS Library: Use `github.com/grandcat/zeroconf` or `github.com/hashicorp/mdns` in pure Go. It binds to UDP 5353 on `0.0.0.0` and advertises service `_http._tcp` for host `sovereign.local`.
- Port Isolation: `sovereign-web:3000` and `sovereign-daemon:8080` port publishing in Compose should be restricted to internal networks or localhost (`127.0.0.1`), directing all external LAN traffic through Caddy on ports 80/443.

## Verification

**Commands:**
- `go test -v ./...` -- expected: All unit tests pass, verifying mDNS responder, Caddy client, and Compose socket isolation.
- `docker compose config` -- expected: Valid syntax confirming Caddy service on 80/443 with no docker.sock mount.
- `npm --prefix web run build` -- expected: Next.js frontend builds cleanly with updated OpenAPI client types.
