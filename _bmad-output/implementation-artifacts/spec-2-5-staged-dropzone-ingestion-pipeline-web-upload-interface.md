---
title: 'Story 2.5: Staged Dropzone Ingestion Pipeline & Web Upload Interface'
type: 'feature'
created: '2026-10-15'
status: 'done'
route: 'dispatch'
baseline_commit: 'e06f14458bd387bd7a4a9a457db6adee2c0e0a34'
review_loop_iteration: 1
context:
  - '_bmad-output/planning-artifacts/architecture/architecture-sovereign-2026-10-15/ARCHITECTURE-SPINE.md'
  - '_bmad-output/planning-artifacts/ux-designs/ux-sovereign-2026-10-15/DESIGN.md'
  - '_bmad-output/planning-artifacts/ux-designs/ux-sovereign-2026-10-15/EXPERIENCE.md'
  - '_bmad-output/implementation-artifacts/epic-2-context.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Sarah and household family members want to add movies, TV shows, music, and audiobooks to the server from laptops, phones, and tablets by dragging files into a simple browser dropzone or copying them to a shared network folder (`\\sovereign\drop`), without dealing with complex Linux filesystem paths, partial or corrupted uploads, or manually triggering library scans across multiple applications.

**Approach:** Implement a staged media ingestion engine in `internal/storage/dropzone.go` with atomic staging writes (`/data/drop/incoming/:category/.uploading-:uuid-:filename`), media file magic bytes validation (MP4, MKV, WebM, AVI, TS, MP3, FLAC, WAV, OGG, Opus, M4A, M4B, AAC), atomic promotion (`os.Rename`) to `/data/media/:category`, automated staging cleanup, background folder scanner for network file drops, automated library refresh triggers to Jellyfin, Navidrome, and Audiobookshelf within 5s, REST API endpoints (`POST /api/v1/drop/upload`, `GET /api/v1/drop/history`), and a standalone, accessible Next.js web interface at `/drop` matching the UX design specification.

## Boundaries & Constraints

**Always:**
- Auto-provision staging directories `/data/drop/incoming/{movies,tv,music,audiobooks}` and media targets `/data/media/{movies,tv,music,audiobooks}` with `0755` permissions.
- Stream incoming uploads into temporary staging files: `/data/drop/incoming/:category/.uploading-:uuid-:filename`.
- Perform `fsync()` on staging files before magic bytes validation.
- Inspect file magic bytes for recognized video and audio signatures (MP4, MKV, WebM, AVI, MPEG-TS, MP3, FLAC, WAV, OGG, Opus, M4A, M4B, AAC).
- Atomically move verified media files via `os.Rename` to `/data/media/:category/:filename`. If destination filename already exists, resolve collision deterministically (e.g. `filename (1).ext`).
- Clean up staging files immediately if an upload is aborted, corrupted, or rejected as non-media.
- Trigger immediate library refresh calls within 5 seconds to the corresponding active media service (Jellyfin for `movies` and `tv`, Navidrome for `music`, Audiobookshelf for `audiobooks`).
- Provide background folder ingestion scanning `/data/drop/incoming` so files copied over SMB/network shares are verified, promoted, and indexed automatically.
- Provide a responsive Next.js web UI at `/drop` with category tabs (`Movies`, `TV`, `Music`, `Audiobooks`), drag-and-drop feedback, file picker trigger, upload progress, recent history list, and copyable network path helper box (`\\sovereign\drop` / `smb://sovereign.local/drop`).
- Deliver consumer-grade error microcopy without raw Linux errors or system stack traces.

**Never:**
- Never leave orphaned partial `.uploading-*` files on disk when uploads are canceled or fail.
- Never promote unverified or non-media files into `/data/media`.
- Never overwrite existing media files without collision protection.
- Never mount the host Docker socket into ingestion or dropzone handlers.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Web Upload | `POST /api/v1/drop/upload` (multipart with category and file) | Streams to staging file, validates magic bytes, moves to `/data/media/:category`, triggers refresh; returns 200 with metadata | 400 on missing file/category; 422 on invalid magic bytes |
| Non-Media File Upload | Upload of `.exe`, `.txt`, `.iso`, or arbitrary binary | Staging file deleted; returns 422 with consumer-friendly error (*"Only media files are accepted"*); not moved to `/data/media` | Staging file cleanly unlinked |
| File Collision | Upload of `song.mp3` when `/data/media/music/song.mp3` exists | Renames atomically to `song (1).mp3` preserving both files | Increments counter until unused name found |
| Library Webhook Trigger | Media promoted to `/data/media/movies` | Dispatches `RefreshService(ctx, "jellyfin")` within 5 seconds | If service inactive, logs info and succeeds upload |
| SMB File Drop | User copies `track.flac` to `/data/drop/incoming/music/track.flac` | Ingestion scanner detects file, waits for write stability, validates magic bytes, moves to `/data/media/music/track.flac`, triggers Navidrome refresh | Logs failure if invalid; keeps drop directory clean |
| Aborted Upload Stream | Client disconnects mid-upload | Request context canceled; partial `.uploading-*` file removed | HTTP 499 / connection closed handled gracefully |

</frozen-after-approval>

## Code Map

- `internal/storage/dropzone.go` -- Dropzone ingestion engine: staging path management, magic bytes validation, atomic file promotion, collision resolution, and SMB folder scanner.
- `internal/storage/dropzone_test.go` -- Unit tests for magic bytes detection, staging lifecycle, file collision resolution, and folder scanner.
- `cmd/sovereign-daemon/server.go` -- REST endpoints: `POST /api/v1/drop/upload`, `GET /api/v1/drop/history`, and integration with supervisor library refresh.
- `cmd/sovereign-daemon/server_test.go` -- Integration tests for dropzone upload endpoint, magic bytes rejection, atomic promotion, and library refresh invocation.
- `web/src/app/drop/page.tsx` -- Standalone Media Dropzone page: category tabs, drag-and-drop zone, file picker, upload progress, recent uploads, copyable network share path.
- `web/src/app/drop/page.test.tsx` -- Frontend tests for category switching, file selection, drag-over styles, upload progress, error messages, and network path copying.
- `api/openapi.yaml` -- Document dropzone upload and status endpoints.

## Tasks & Acceptance

**Execution:**
- [x] `internal/storage/dropzone.go` -- Implement magic bytes validator, staging manager, file promoter, and background SMB drop folder scanner.
- [x] `internal/storage/dropzone_test.go` -- Unit tests for magic bytes validation (valid media formats vs rejected binaries), atomic rename, collision handling, and staging cleanup.
- [x] `cmd/sovereign-daemon/server.go` -- Register `POST /api/v1/drop/upload` and `GET /api/v1/drop/history` handlers with supervisor library refresh webhook integration.
- [x] `cmd/sovereign-daemon/server_test.go` -- Integration tests for upload lifecycle, validation errors, and library refresh triggers.
- [x] `web/src/app/drop/page.tsx` -- Implement standalone Media Dropzone interface adhering to UX design and WCAG accessibility standards.
- [x] `web/src/app/drop/page.test.tsx` -- Unit tests for dropzone UI: category tabs, drag-drop interactions, upload progress, error handling, and clipboard copy.
- [x] `api/openapi.yaml` -- Document `POST /api/v1/drop/upload` and `GET /api/v1/drop/history`.

**Acceptance Criteria:**
- Given a user accessing `/drop` or `\\sovereign\drop`, category selectors for Movies, TV, Music, and Audiobooks are available with drag-over visual feedback and copyable network path.
- Upload streams write to `/data/drop/incoming/:category/.uploading-:uuid-:filename`.
- Upon upload completion and `fsync()`, daemon validates media magic bytes, atomically moves file to `/data/media/:category`, and cleans staging files.
- Successful moves trigger immediate HTTP library refresh webhooks to Jellyfin, Navidrome, or Audiobookshelf within 5 seconds.
- Invalid or non-media files are rejected with in-UI explanations and never indexed into media libraries.

## Implementation Notes

- **Dropzone Engine (`internal/storage/dropzone.go`)**:
  - Implemented `DetectMedia` validating magic bytes for MP4, MKV, WebM, AVI, TS, MP3 (ID3v2 & raw MPEG frames), FLAC, WAV, OGG, Opus, M4A, M4B, AAC.
  - Implemented staging write pipeline streaming to `/data/drop/incoming/:category/.uploading-:uuid-:filename` with `fsync()`.
  - Implemented deterministic collision resolution (`file (1).ext`) preserving both original and newly uploaded files.
  - Implemented background SMB folder scanner watching `/data/drop/incoming` with write stability tracking and directory hygiene.
  - Implemented asynchronous media library scan dispatch within 5s to Jellyfin, Navidrome, or Audiobookshelf.
- **Daemon REST Endpoints (`cmd/sovereign-daemon/server.go`)**:
  - Registered `POST /api/v1/drop/upload` and `GET /api/v1/drop/history`.
  - Enforced RFC 7807 Problem Details (400 for bad input, 422 for non-media files with consumer microcopy, 200 for promoted uploads).
- **Web UI (`web/src/app/drop/page.tsx`)**:
  - Implemented responsive standalone Media Dropzone card adhering to Braun/Nordic design standards.
  - Included Movies, TV Shows, Music, and Audiobooks category selector tabs.
  - Implemented 2px dashed drag & drop target with drag-over state and file picker trigger.
  - Added active multi-file progress indicators, dismissible consumer reassurance alerts, copyable SMB network share helper (`\\sovereign\drop`), and recent upload history list with `StatusBadge` geometric micro-shapes.
  - WCAG 2.1 AA compliant (44px touch targets, `:focus-visible` rings, `aria-live="polite"`).
- **Contracts & Testing**:
  - Documented dropzone upload and history endpoints in `api/openapi.yaml`.
  - Generated TypeScript types via `openapi-typescript`.
  - Added full test suites: `internal/storage/dropzone_test.go`, `cmd/sovereign-daemon/server_test.go`, and `web/src/app/drop/page.test.tsx`.

## Spec Change Log

*(No changes yet)*

## Review Triage Log

### 1. Blind Hunter Review
- **Finding (High)**: Slice bounds panic risk during MPEG-TS detection when `len(data) == 188`. `data[188]` on a slice of length 188 panics in Go.
  - *Fix*: Guarded with `len(data) > 188 && data[188] == 0x47` and `len(data) > 204 && data[204] == 0x47`.
- **Finding (High)**: Lack of category vs media type verification (e.g. uploading audio files to movies/tv or video to music/audiobooks).
  - *Fix*: Implemented `IsCategoryCompatible(category, contentType)` and `ErrCategoryMismatch`. Daemon returns HTTP 422 ProblemDetails on category mismatch.
- **Finding (Medium)**: In-memory/temporary buffering of large multipart uploads via `r.ParseMultipartForm(100MB)`.
  - *Fix*: Replaced with direct streaming via `r.MultipartReader()`, streaming chunks directly into the staging file with O(1) memory footprint.

### 2. Edge Case Hunter Review
- **Finding (High)**: Cross-device filesystem move failure (`syscall.EXDEV`) when staging and media directories are on separate mount points or NAS storage.
  - *Fix*: Implemented `atomicMoveOrCopy` falling back to buffered file copy, fsync, and source unlink when `os.Rename` fails with `EXDEV`.
- **Finding (Medium)**: Race condition during concurrent uploads of files with identical names.
  - *Fix*: Added `promotionMu` mutex protecting `ResolveTargetFilename` and destination file creation.
- **Finding (Medium)**: Stale `.uploading-*` staging files could persist after unexpected daemon restart.
  - *Fix*: Implemented `CleanOrphanedStagingFiles` unlinking stale staging files on startup.
- **Finding (Medium)**: Zero-byte files dropped into SMB folder tracked indefinitely in stability map.
  - *Fix*: Updated `ScanOnce` to detect zero-byte files past the stability window, unlink them, and record rejection in history.
- **Finding (Medium)**: Client connection aborts during upload stream could leave orphan staging files.
  - *Fix*: Integrated `contextReader` and `context.Canceled` detection in `HandleUploadStream` and `handleDropUpload` returning HTTP 499 (Upload Aborted) and ensuring clean unlinking.
- **Finding (Low)**: Next.js SSR hydration mismatch with `toLocaleTimeString()` in dropzone history.
  - *Fix*: Added `isMounted` state and `suppressHydrationWarning` on timestamp displays in `web/src/app/drop/page.tsx`.

### 3. Verification Gap Reviewer
- **Finding**: Missing integration test asserting `RefreshService` invocation on active media service.
  - *Fix*: Added `TestServer_Dropzone_UploadValidMedia_CallsRefreshService` asserting Jellyfin refresh webhook dispatch.
- **Finding**: Missing integration test for client aborted upload stream handling.
  - *Fix*: Added `TestServer_Dropzone_UploadAborted_Returns499AndCleansStaging` asserting HTTP 499 and clean staging directory.
- **Finding**: Missing test coverage for category mismatch and category-to-service mapping.
  - *Fix*: Added `TestDropzone_ServiceForCategory`, `TestDropzone_HandleUploadStream_CategoryMismatch`, `TestDropzone_ScanOnce_CategoryMismatch`, and `TestServer_Dropzone_UploadCategoryMismatch_Rejects422AndCleans`.
- **Finding**: Missing unit tests for background scanner loop, zero-byte rejection, and orphaned staging cleanup.
  - *Fix*: Added `TestDropzone_Start_BackgroundScanner`, `TestDropzone_ScanOnce_ZeroByteRejection`, and `TestDropzone_CleanOrphanedStagingFiles`.
