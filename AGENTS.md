<!-- bmad:context -->
<!-- Verified 2026-10-25 against a69b851. Managed by bmad-project-context; edits inside this block are replaced on refresh. Keep anything you want preserved outside the markers. -->

## sovereign

Sovereign is a private, self-hosted home server appliance monorepo. Go supervisor daemon, Next.js / TypeScript web client, Caddy ingress gateway, and Docker Compose services. Planning and implementation artifacts live in `_bmad-output/`, core documentation in `docs/`.

## Policy

- Commit and push (`git push origin main` for internal dev, `git push github main` for public distribution) with clear, descriptive commit messages after every important step or completed milestone.
- Dual-remote architecture:
  - Internal development repo: `origin` (`http://git.crins:3000/jens/sovereign`)
  - Public anonymous user repo: `github` (`https://github.com/j333c/sovereign`) — holds everything for installing, updating, and using Sovereign.
- Do not commit personal configs (`*.user.toml`).
- Never hand-edit `web/src/lib/api/schema.d.ts` — run `npm --prefix web run codegen` from `api/openapi.yaml`.
- Never mount `/var/run/docker.sock` in spoke application containers; all container lifecycle operations must be mediated by the Go daemon.
- The `sovereign-ai` Docker bridge must remain air-gapped without external internet routing.

## Where things are

- Host supervisor daemon: `cmd/sovereign-daemon/` and `internal/`
- Presentation web app: `web/`
- Ingress gateway configuration: `caddy/Caddyfile`
- API specification: `api/openapi.yaml`
- BMad configuration & artifacts: `_bmad/`, `_bmad-output/planning-artifacts/`, `_bmad-output/implementation-artifacts/`
- Skill definitions: `.agent/skills/` and `.agents/skills/`

## Running and verifying

- Python commands should use `python3` (e.g. for `_bmad/scripts/...`).
- Run backend verification with `make test-daemon` or `go test -count=1 ./...`.
- Run frontend verification with `make test-web` or `npm --prefix web test`.
- Network operations against `git.crins` may require running outside the sandbox (`BypassSandbox: true`).

## Conventions that differ from defaults

- Zero external telemetry: never import remote fonts, scripts, or analytics; all client assets must be local.
- SQLite must compile without CGO using `modernc.org/sqlite` with WAL mode enabled.
- All user-facing strings in `web/src/lib/i18n.ts` must maintain complete key parity across all four locales: `en`, `de`, `es`, `fr`.
- UI microcopy and errors must provide calm, reassuring next steps instead of raw system stack traces.

## Known pitfalls

- Persistent SSE telemetry connections require disabled or extended server write timeouts in `cmd/sovereign-daemon/main.go`.
- Never import `net/http/httptest` in production Go packages; keep test utilities in `*_test.go` files.

<!-- /bmad:context -->

## Repository Architecture & Remotes

- **Internal Development Repo**: `http://git.crins:3000/jens/sovereign` (`origin`) - authoritative internal development remote.
- **Public Community & Distribution Repo**: `https://github.com/j333c/sovereign` (`github`) - public GitHub repository holding everything needed for installing, updating, and using Sovereign for anonymous users.
- **Push Workflow**:
  - Internal development: `git push origin main`
  - Public release/distribution: `git push github main`
