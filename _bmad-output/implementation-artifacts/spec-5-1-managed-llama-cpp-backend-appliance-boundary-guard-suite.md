---
title: 'Story 5.1: Managed llama.cpp Backend & Appliance Boundary Guard Suite'
type: 'feature'
created: '2026-10-16'
status: 'done'
route: 'dispatch'
baseline_commit: 'e23e1b936012f49360d49dc9dad63a485361a6e5'
review_loop_iteration: 1
context:
  - '_bmad-output/planning-artifacts/architecture/architecture-sovereign-2026-10-15/ARCHITECTURE-SPINE.md'
  - '_bmad-output/planning-artifacts/ux-designs/ux-sovereign-2026-10-15/DESIGN.md'
  - '_bmad-output/planning-artifacts/ux-designs/ux-sovereign-2026-10-15/EXPERIENCE.md'
  - '_bmad-output/implementation-artifacts/epic-5-context.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Local AI inference can easily destabilize a home server if raw LLM backends are exposed on public host ports, if corrupted or truncated model files cause container crash loops, if model loading exhausts host memory or disk storage, or if CPU inference threads saturate all cores and freeze video streaming and web navigation.

**Approach:** Implement a managed `llama.cpp` inference engine (`llama-server`) with an Appliance Boundary Guard Suite:
1. **Network Isolation**: Strict placement inside the `sovereign-ai` Docker bridge network with zero published host ports (`Ports: nil`). All traffic must route through the supervisor daemon's internal AI Gateway.
2. **Hardware Acceleration Autodetection**: Automatically detect host compute capabilities (CUDA `/dev/nvidia*`, Vulkan `/dev/dri/renderD128`, or CPU fallback), configure device mounts, enable FlashAttention (`-fa`), and enforce standardized `-np 2` parallel slots. On CPU-only hosts, cap inference threads to `max(1, physical_cores - 2)`.
3. **Pre-flight GGUF Magic Header Check**: Inspect binary header bytes (`0x47, 0x47, 0x55, 0x46` = `GGUF`) and format version prior to loading models into `llama-server`, rejecting corrupted or invalid files immediately.
4. **Safe Stub Mode**: When `/data/models` contains no valid GGUF model, gracefully hold inference requests in Safe Stub Mode, returning RFC 7807 Problem Details (HTTP 503) with reassuring instructions to download a model rather than crashing or restarting containers.
5. **Storage Reservation Guard**: Enforce a strict `model_size + 5.0 GB` free disk space reservation guard on `/data/models` before initiating model downloads, protecting the appliance from storage exhaustion.

## Boundaries & Constraints

**Always:**
- Deploy official upstream `ghcr.io/ggml-org/llama.cpp:b11647` (or `:server`).
- Isolate container networking to `sovereign-ai` network with NO host ports mapped.
- Standardize execution parameters: `-np 2` (parallel request slots), `-fa` (FlashAttention), `-c 4096` default context.
- Cap CPU threads to `max(1, physical_cores - 2)` on CPU-only systems.
- Validate GGUF magic header (`0x47, 0x47, 0x55, 0x46`) before starting container or switching models.
- Enforce Safe Stub Mode returning HTTP 503 with RFC 7807 Problem Details when no model is available.
- Check free storage space with 5.0 GB headroom buffer (`free_bytes >= model_bytes + 5*1024*1024*1024`).
- Support service alias resolution: `llama`, `ai`, `llm`, `llama-server` map to `ServiceLlama`.
- Preserve model files under `/data/models` across stops, restarts, and updates.

**Never:**
- Never expose `llama-server` port directly on the host interface.
- Never mount the host Docker socket (`/var/run/docker.sock`) into the AI container.
- Never crash or restart loop when `/data/models` is empty or holds corrupted files.
- Never allow CPU inference threads to utilize 100% of host CPU cores.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Activation with Model | Valid `.gguf` file in `/data/models` | Pre-flight check passes, container starts on `sovereign-ai` with `-fa -np 2`, no host ports exposed | 502 on image pull failure |
| Activation without Model | `/data/models` is empty | Backend enters Safe Stub Mode; queries return HTTP 503 ProblemDetails directing user to Settings | RFC 7807 Problem Details response |
| Corrupt Model Header | File lacks `GGUF` magic header bytes | `ValidateGGUFHeader` returns `ErrInvalidGGUFMagic`; model rejected before container startup | Error logged, container not launched |
| Storage Reservation Check | Free space < model_size + 5 GB | `CheckModelStorageSpace` returns `ErrInsufficientStorageSpace` with available and required bytes | Rejects download before writing to disk |
| Storage Reservation OK | Free space >= model_size + 5 GB | `CheckModelStorageSpace` returns `nil` | Proceeds with atomic download |
| CPU-Only Host | No NVIDIA or DRI devices | Threads capped at `max(1, physical_cores - 2)`; environment / args set `-t <threads>` | Prevents core starvation |
| GPU-Accelerated Host | `/dev/dri/renderD128` or NVIDIA present | Mounts device nodes into container spec, enables GPU offloading (`-ngl 99`) | Falls back to CPU if devices fail |

</frozen-after-approval>

## Code Map

- `internal/ai/gguf.go` -- Binary header reader, `ValidateGGUFHeader`, and model scanning utilities.
- `internal/ai/storage.go` -- Storage reservation guard (`CheckModelStorageSpace`) with 5 GB safety buffer.
- `internal/ai/stub.go` -- Safe Stub Mode handler returning RFC 7807 Problem Details (HTTP 503).
- `internal/supervisor/llama.go` -- `LlamaProvisioner`, hardware autodetection, thread capping, container spec synthesis.
- `internal/supervisor/supervisor.go` -- Wire `ServiceLlama` into canonical service definitions and lifecycle.
- `cmd/sovereign-daemon/server.go` -- Ingress and route registration for AI Gateway and safe stub fallback.

## Implementation Plan

1. **GGUF Validation & Storage Guard (`internal/ai`)**:
   - Implement `ValidateGGUFHeader(path string)` and `ValidateGGUF(r io.Reader)`.
   - Implement `CheckModelStorageSpace(path string, requiredBytes int64)`.
   - Implement `ScanModels(dir string)` returning verified model metadata.
   - Implement `SafeStubHandler` for HTTP 503 problem responses.
2. **Llama Provisioner (`internal/supervisor/llama.go`)**:
   - Hardware detection: inspect `/dev/nvidia*` and `/dev/dri/renderD128`.
   - Thread calculation: `max(1, physical_cores - 2)`.
   - ContainerSpec synthesis: image `ghcr.io/ggml-org/llama.cpp:b11647`, network `sovereign-ai`, `Ports: nil`, `/data/models:/models:ro`.
3. **Supervisor Integration**:
   - Register `ServiceLlama` in `internal/supervisor/supervisor.go`.
   - Support aliases `llama`, `ai`, `llm`, `llama-server`.
4. **Daemon Integration (`cmd/sovereign-daemon`)**:
   - Register AI Gateway routes `/v1/*` with Safe Stub Mode fallback when no model is active.
5. **Testing & Verification**:
   - Unit tests for GGUF magic verification, storage guard, and Llama provisioner.
   - Run `go test ./...` and `npm --prefix web test`.

## Review Triage Log

### Review Loop 1 (3-Layer Parallel Reviews: Blind Hunter, Edge Case Hunter, Verification Gap)

1. **Ingress routing for `/v1` and `/v1/*` in Caddy (`caddy/Caddyfile` and `internal/ingress/caddy.go`)**:
   - *Finding*: Caddy reverse proxy only matched `/api` and `/api/*`, causing external consumer requests on `/v1/*` to drop through to web UI port 3000 (HTTP 404).
   - *Fix*: Added `@v1 path /v1 /v1/*` proxying to `sovereign-daemon:8080` in `caddy/Caddyfile` and `route-v1` in `BuildCaddyConfig`, verified in `internal/ingress/caddy_test.go`.

2. **On-demand image pull before model verification (`internal/supervisor/supervisor.go`)**:
   - *Finding*: `ActivateService` called `PullImage` before `Provision`. In offline environments or when `/data/models` was empty, this failed or pulled multi-GB images unnecessarily instead of entering Safe Stub Mode.
   - *Fix*: In `ActivateService`, probe models via `Provision(ctx)` first. If `InSafeStub: true`, transition status to `StatusRunning` + `HealthDegraded` immediately without pulling images.

3. **Safe Stub Mode transitions & watchdog probe loops (`internal/supervisor/supervisor.go`)**:
   - *Finding*: If activated without a model, `ActivateService` returned early on subsequent calls without starting a container when a model was added. Calling `RestartService` failed because no container existed. Watchdog probed health on non-existent container.
   - *Fix*: Permitted re-activation from degraded safe stub mode; `RestartService` starts container if not previously created; watchdog automatically detects models placed on disk and auto-transitions the service to running + healthy.

4. **Corrupted model candidates and safe stub fallback (`internal/supervisor/llama.go`)**:
   - *Finding*: `Provision` took candidate 0 and failed on corrupt files without trying other models, and threw an error instead of entering Safe Stub Mode when all candidates were corrupt.
   - *Fix*: Iterate through candidate files, test `ai.ValidateGGUFHeader(path)`, pick first valid candidate. If no candidate is valid or all corrupt, return `LlamaProvisionResult{InSafeStub: true}, nil` without error or crash loops.

5. **Symlink resolution and container path alignment (`internal/supervisor/llama.go`)**:
   - *Finding*: `active.gguf` symlinks pointing to absolute paths failed to map cleanly inside container mount.
   - *Fix*: Evaluated symlinks using `filepath.EvalSymlinks` to determine canonical model filename.

6. **NVIDIA CUDA passthrough environment variables (`internal/supervisor/llama.go`)**:
   - *Finding*: Passing character devices `/dev/nvidia*` alone lacked runtime environment variables for container libraries.
   - *Fix*: Set `NVIDIA_VISIBLE_DEVICES=all` and `NVIDIA_DRIVER_CAPABILITIES=compute,utility` when NVIDIA GPU is detected.

7. **Proxy error handler and transport preservation (`internal/ai/gateway.go`)**:
   - *Finding*: `SetLlamaURL` recreated `ReverseProxy` without preserving transport or custom `ErrorHandler`.
   - *Fix*: Retained `Transport` and wired `ErrorHandler` to `SafeStubHandler`.

8. **Storage reservation overflow checks and absolute pathing (`internal/ai/storage.go`)**:
   - *Finding*: Potential overflow with negative or extreme values; relative non-existent paths terminated early in `defaultDiskFree`.
   - *Fix*: Added bounds check `requiredBytes < 0 || requiredBytes > math.MaxInt64 - SafetyBufferBytes` and converted path to `filepath.Abs`.

9. **OpenAPI specification completeness (`api/openapi.yaml`)**:
   - *Finding*: Missing `/v1/models` and `/v1/chat/completions` paths and RFC 7807 problem details references.
   - *Fix*: Added definitions for `/v1/models` and `/v1/chat/completions` with 503 Safe Stub problem details schema.

10. **Verification Gap Additions**:
    - Added `TestServer_AIGateway_ProxyWhenRunning` and `TestServer_AIGateway_ServiceInactiveProblem` in `cmd/sovereign-daemon/server_test.go`.
    - Added `TestSupervisor_LlamaActivation_WithModel`, `TestSupervisor_Llama_TransitionFromSafeStubToHealthy`, and `TestSupervisor_Llama_WatchdogAutoTransition` in `internal/supervisor/supervisor_test.go`.
    - Added `TestLlamaProvisioner_CorruptCandidateHandling`, `TestLlamaProvisioner_AllCorrupt_EntersSafeStub`, `TestLlamaProvisioner_SymlinkResolution`, and `TestLlamaProvisioner_SynthesizeComposeSpec` in `internal/supervisor/llama_test.go`.

### Automated Verification
- `go test ./...`: PASS (all 13 packages)
- `npm --prefix web test`: PASS (all 92 tests across 11 test suites)
