---
title: 'Story 3.2: Automated Media Metadata & Artwork Enrichment'
type: 'feature'
created: '2026-10-16'
status: 'review'
route: 'dispatch'
baseline_commit: 'd3b3bae27431a7c4fd31ffeb9c6646c4a2cfa0c5'
review_loop_iteration: 0
context:
  - '_bmad-output/planning-artifacts/architecture/architecture-sovereign-2026-10-15/ARCHITECTURE-SPINE.md'
  - '_bmad-output/planning-artifacts/ux-designs/ux-sovereign-2026-10-15/DESIGN.md'
  - '_bmad-output/planning-artifacts/ux-designs/ux-sovereign-2026-10-15/EXPERIENCE.md'
  - '_bmad-output/implementation-artifacts/epic-3-context.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Physical disc rips in `/data/media/staging/discs/` have raw, cryptic volume labels (e.g., `THE_MATRIX_1999`, `S1_DISC1`, `AUDIO_CD_8F3A`) without cover art, posters, release years, or track titles. Ripped media looks unorganized and bare in Jellyfin and Navidrome unless manually enriched and moved.

**Approach:** Implement an automated metadata and artwork enrichment engine in `internal/enrichment/` that monitors `/data/media/staging/discs/`, parses volume labels and TOC signatures, queries movie and music databases (TMDb, IMDb, MusicBrainz with offline fallback and mock support), downloads and embeds cover art posters and NFO files, atomically promotes enriched files into canonical library structures (`/data/media/movies`, `/data/media/tv`, `/data/media/music`), cleans up staging disc directories, dispatches immediate HTTP library refresh triggers to Jellyfin and Navidrome within 5 seconds, and updates live SSE telemetry (`Matching metadata`).

## Boundaries & Constraints

**Always:**
- Monitor `/data/media/staging/discs/` for completed disc rips.
- Parse volume labels, disc fingerprints, and file structures to identify movie titles, TV seasons, or music albums and tracks.
- Fetch metadata: title, year, genres, plot overview, track listings, and high-resolution cover art / poster images (`poster.jpg`, `cover.jpg`, `movie.nfo`).
- Support offline / air-gapped fallback: if external APIs are unreachable or in test mode, generate clean local metadata from volume labels and directory tags without failing or blocking promotion.
- Atomically move processed media using `os.Rename` (with cross-device link copy fallback) to:
  - Movies: `/data/media/movies/<Title> (<Year>)/`
  - TV: `/data/media/tv/<Title>/Season <N>/`
  - Music: `/data/media/music/<Artist>/<Album> (<Year>)/`
- Handle filename collisions cleanly without overwriting existing files in the media library.
- Clean up staging disc directories upon successful promotion.
- Dispatch immediate HTTP library refresh webhooks within 5 seconds to Jellyfin (`/Library/Refresh`) for movies/TV and Navidrome for music.
- Provide a `MetadataClient` interface with deterministic mock implementations for unit and integration testing.

**Never:**
- Never block indefinitely on slow or unresponsive third-party metadata API calls (enforce HTTP client timeouts <= 5s).
- Never discard or destroy media files if a metadata query fails to find a match; fall back to volume label titles and promote safely.
- Never mount the host Docker socket into enrichment workers.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Movie DVD/Blu-ray Rip | Staged disc with label `THE_MATRIX_1999` | Matched with TMDb/IMDb; downloads `poster.jpg`, generates `movie.nfo`, promotes to `/data/media/movies/The Matrix (1999)/`, triggers Jellyfin refresh | Fallback to sanitized label if API offline |
| TV Series Disc Rip | Staged disc with label `BREAKING_BAD_S01D01` | Identifies show & season; creates `/data/media/tv/Breaking Bad/Season 01/`, promotes episodes, triggers Jellyfin refresh | Retains original track naming on parse error |
| Audio CD Rip | Staged disc with CD-DA tracks | Queries MusicBrainz; fetches album title, artist, track names, `cover.jpg`; promotes to `/data/media/music/<Artist>/<Album>/`, triggers Navidrome refresh | Generic album fallback if unmatched |
| Metadata API Offline / Timeout | External API network failure | Generates clean fallback metadata from volume label; promotes media; logs warning | Promotes successfully without data loss |
| Target Directory Collision | Target movie folder already exists | Merges files or appends disambiguated filename without overwriting existing files | Preserves both files |
| Library Webhook Trigger | Files promoted to `/data/media/{movies,tv,music}` | Triggers supervisor refresh within 5s | Ignores if service inactive; does not fail promotion |

</frozen-after-approval>

## Code Map

- `internal/enrichment/client.go` -- `MetadataClient` interface, TMDb/MusicBrainz API client, and `MockMetadataClient`.
- `internal/enrichment/parser.go` -- Disc volume label and title parser (identifying media type, title, release year, season/episode numbers).
- `internal/enrichment/engine.go` -- `EnrichmentEngine` / worker: staging folder watcher, metadata fetching, artwork saving, atomic library promotion, and staging cleanup.
- `internal/enrichment/engine_test.go` -- Unit tests for volume parsing, metadata enrichment, artwork downloads, fallback handling, atomic promotion, and refresh dispatch.
- `cmd/sovereign-daemon/server.go` -- Wire `EnrichmentEngine` into daemon lifecycle (`StartBackgroundServices` and `Close`), register `/api/v1/enrichment/*` endpoints, and update SSE telemetry.
- `cmd/sovereign-daemon/server_test.go` -- Integration tests for enrichment lifecycle and library refresh invocation.
- `api/openapi.yaml` -- Document enrichment endpoints (`GET /api/v1/enrichment/status`, `POST /api/v1/enrichment/scan`).

## Tasks & Acceptance

**Execution:**
- [x] `internal/enrichment/parser.go` -- Implement label and path parser for movies, TV series, and audio albums.
- [x] `internal/enrichment/client.go` -- Implement `MetadataClient` interface, TMDb/MusicBrainz client with rate-limiting, and `MockMetadataClient`.
- [x] `internal/enrichment/engine.go` -- Implement `EnrichmentEngine` with staging watcher, artwork fetcher, atomic promoter, and refresh webhook trigger.
- [x] `internal/enrichment/engine_test.go` -- Unit tests covering movie, TV, and music enrichment, offline fallback, directory collision, and refresh dispatch.
- [x] `cmd/sovereign-daemon/server.go` -- Register `/api/v1/enrichment/status` and `/api/v1/enrichment/scan` and integrate with background services.
- [x] `cmd/sovereign-daemon/server_test.go` -- Integration tests for enrichment endpoints and supervisor library refresh triggers.
- [x] `api/openapi.yaml` -- Document enrichment status and scan endpoints in OpenAPI.

**Acceptance Criteria:**
- Given a finished disc rip in `/data/media/staging/discs/`, when the metadata enrichment worker runs, it queries movie/music databases using volume labels and fingerprints.
- Matched cover art posters, release year, genres, and track tags are downloaded and saved alongside media.
- Processed media is atomically moved to `/data/media/{movies,tv,music}` and triggers an immediate library refresh webhook within 5 seconds.
- Unmatched or offline media safely promotes with sanitized volume label metadata without data loss.

## Implementation Notes

- Implemented `internal/enrichment/parser.go` parsing volume labels, directory contents, and disc IDs for Movie, TV, and Music categories, handling hex suffixes, season/episode disc numbering, and title casing with acronym preservation.
- Implemented `internal/enrichment/client.go` featuring `MetadataClient` interface, rate-limited HTTP client connecting to TMDb and MusicBrainz with bounded <= 5s timeouts, and deterministic `MockMetadataClient`.
- Implemented `internal/enrichment/engine.go` providing `EnrichmentEngine` with staging watcher, offline metadata fallback, NFO XML generation (`movie.nfo`, `tvshow.nfo`), poster/cover artwork saving (`poster.jpg`, `cover.jpg`), 10% disk capacity safety headroom enforcement, zero-copy atomic promotion (`os.Rename` with copy+sync+delete fallback), filename collision disambiguation, and non-blocking 5s library refresh webhook dispatch to Jellyfin and Navidrome.
- Integrated `EnrichmentEngine` with `cmd/sovereign-daemon/server.go`: wired into daemon background services, registered `GET /api/v1/enrichment/status` and `POST /api/v1/enrichment/scan`, hooked optical rip completion events to automatically trigger enrichment, and updated SSE telemetry to broadcast `Matching metadata`.
- Documented enrichment endpoints and schemas in `api/openapi.yaml`.
- Added comprehensive unit and integration test matrices in `internal/enrichment/engine_test.go` and `cmd/sovereign-daemon/server_test.go`.

## Spec Change Log

*(No changes yet)*

## Review Triage Log

*(To be populated during code review)*
