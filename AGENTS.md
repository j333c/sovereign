<!-- bmad:context -->
<!-- Verified 2026-11-10 against b1bbb86. Managed by bmad-project-context; edits inside this block are replaced on refresh. Keep anything you want preserved outside the markers. -->

## sovereign

Sovereign is a private, self-hosted home server appliance monorepo. Go supervisor daemon, Next.js / TypeScript web client, Caddy ingress gateway, and Docker Compose services. Planning and implementation artifacts live in `_bmad-output/`, core documentation in `docs/`.

## Policy

- Commit and push (`git push origin main` for internal dev, `git push github main` for public distribution) with clear, descriptive commit messages after every important step or completed milestone.
- Dual-remote architecture:
  - Internal development repo: `origin` (`http://git.crins:3000/jens/sovereign`) — authoritative internal development remote.
  - Public anonymous user repo: `github` (`https://github.com/j333c/sovereign`) — holds everything for installing, updating, and using Sovereign (on installed appliances inside `/opt/sovereign`, `origin` points to `https://github.com/j333c/sovereign.git`).
- Release branching and tagging workflow:
  - `main` is production-stable and reflects the current release.
  - Milestone development occurs on planned `release/v<MAJOR>.<MINOR>` branches.
  - Hotfixes and security patches occur on `hotfix/v<MAJOR>.<MINOR>.<PATCH>`.
  - Tags must follow SemVer: `v<MAJOR>.<MINOR>.<PATCH>` for stable, `v<MAJOR>.<MINOR>.<PATCH>-rc.<N>` for preview. Never move or delete published tags.
  - Push all release tags to both remotes (`git push origin <tag> && git push github <tag>`) and verify tag parity via `scripts/verify-remotes.sh`.
- Update channel rules:
  - `install.sh` supports `--channel stable|security-only|preview` (default: `stable`) and seeds `/data/state/update_channel` for first-boot SQLite initialization.
  - Appliance update discovery must run `git ls-remote --tags --refs origin` over Smart HTTP inside `/opt/sovereign` (where `origin` points to `github`) at most once per hour to avoid GitHub REST API rate limits, using a shallow tag fetch (`git fetch --depth=1 origin tag <tag>`) for annotated changelog summaries.
  - Preview channel updates perform forward schema/state migrations; downgrades require a snapshot restore (`store.DB.RestoreSnapshot` guarded by `sync.RWMutex`), while switching back to `Stable`/`Security-only` enters a monotonic "Graduating to Stable" hold until a newer stable release ships. Both the UI (`DestructiveModal`) and backend (`POST`/`PUT /api/v1/settings`) must enforce localized `"PREVIEW"` confirmation tokens.
- Do not commit personal configs (`*.user.toml`).
- Never hand-edit `web/src/lib/api/schema.d.ts` — run `npm --prefix web run codegen` from `api/openapi.yaml`.
- Never mount `/var/run/docker.sock` in spoke application containers; all container lifecycle operations must be mediated by the Go daemon.
- The `sovereign-ai` Docker bridge must remain air-gapped without external internet routing.
- Spoke and gateway container images in Compose definitions, declarative manifests, and Go supervisor constants must use pinned `MAJOR.MINOR.PATCH` SemVer tags (or pinned `^b[0-9]{4,}$` upstream build tags for `llama.cpp`), never floating `:latest`.

## Where things are

- Host supervisor daemon: `cmd/sovereign-daemon/` and `internal/`
- Shared crash-safe filesystem & capacity utilities: `internal/fsutil/`
- Presentation web app: `web/` (settings tab panels in `web/src/components/settings/`, shared utilities and i18n in `web/src/lib/`)
- Ingress gateway configuration: `caddy/Caddyfile`
- API specification: `api/openapi.yaml`
- Turnkey host installer & maintenance scripts: `install.sh` (symlinked at `scripts/install.sh`), `scripts/verify-remotes.sh`, and shell test suites in `tests/`
- BMad configuration & artifacts: `_bmad/`, `_bmad-output/planning-artifacts/`, `_bmad-output/implementation-artifacts/`
- Canonical skill definitions: `.agents/skills/`

## Running and verifying

- Python commands should use `python3` (e.g. for `_bmad/scripts/...`).
- Run backend verification with `make test-daemon` or `go test -count=1 ./...`.
- Run frontend verification with `make test-web` or `npm --prefix web test`; `web/package.json` enumerates test files explicitly in `scripts.test`, so add any new `*.test.ts(x)` file there.
- Run shell & installer verification with `make test-scripts`, `make test-installer`, and `make verify-remotes`.
- Network operations against `git.crins` may require running outside the sandbox (`BypassSandbox: true`).

## Conventions that differ from defaults

- Zero external telemetry: never import remote fonts, scripts, or analytics; all client assets must be local.
- SQLite must compile without CGO using `modernc.org/sqlite` with WAL mode enabled.
- Use `internal/fsutil` (`DiskFree`, `DiskFreeRatio`, `DiskFreePercent`, `AtomicWriteFile`, `AtomicMoveOrCopy`) for disk space checks and atomic file mutations instead of re-implementing `syscall.Statfs` or temp-file rename helpers.
- Use shared frontend helpers in `web/src/lib/format.ts` (`formatBytes`, `formatSpeed`, `formatDuration`, `formatBytesToTB`, `formatBytesToGB`), `web/src/lib/hooks.ts` (`useLanguage`, `useModalFocusTrap`), and `web/src/lib/hardware-calculator.ts` (`calculateHardwareTier`) instead of duplicating logic in components.
- All user-facing strings in `web/src/lib/i18n/` (`en.ts`, `de.ts`, `es.ts`, `fr.ts`, `setup.ts`, `types.ts`) must maintain complete key parity across all four locales.
- UI microcopy and errors must provide calm, reassuring next steps instead of raw system stack traces.

## Known pitfalls

- Persistent SSE telemetry connections require disabled or extended server write timeouts in `cmd/sovereign-daemon/main.go`.
- Never import `net/http/httptest` in production Go packages; keep test utilities and mocks in `*_test.go` or `mock.go` files.

<!-- /bmad:context -->
