# PRD Quality Review — Sovereign Home Server

## Overall verdict
Sovereign's PRD is an exceptionally well-grounded, technically literate specification that establishes a clear, appliance-grade product vision backed by rigorous NFRs, single-source-of-truth RAG, and pragmatic day/night GPU arbitration. Its core architecture and trade-off decisions provide solid foundations for downstream development, with primary risks confined to ambitious MVP boundaries (commercial Blu-ray AACS decryption and automated document deadline extraction) and missing inline assumption roundtrips.

---

## 1. Decision-readiness — strong

The PRD exhibits strong decision-readiness. Rather than smoothing contentious technical choices into vague compromises, it takes definitive stances on hard product trade-offs. The decision to deliberately forego custom client applications (§5 Non-Goals) in favor of the battle-tested open-source ecosystem (Jellyfin, Paperless, Subsonic clients) avoids a catastrophic maintenance burden. The GPU arbitration strategy (§4.5 FR-17 and Addendum §4) confronts the real-world collision between greedy LLM VRAM allocation and real-time media transcoding by instituting a strict day/night reservation window, idle model unloading, and hybrid iGPU/dGPU routing.

The Open Questions (§8) target genuinely hard legal, architectural, and protocol tensions—specifically commercial Blu-ray AACS key handling, remote access networking (WireGuard/Tailscale vs. pure LAN), and smart home discovery without MFi certification—rather than rhetorical placeholders. Furthermore, deferred scope items in §6.2 (such as Matter/HomeKit integration) feature explicit `[NOTE FOR PM]` callouts tied to standard stabilization.

### Findings
*No blocking findings.*

---

## 2. Substance over theater — strong

The document is remarkably free of product theater. The persona landscape is ruthlessly constrained to two distinct household personas—Alex (tech-averse homeowner managing family entertainment and setup) and Sarah (household bill and document administrator)—both of whom actively anchor concrete, distinct user journeys (UJ-1 through UJ-4). There is zero "persona theater" or demographic filler.

Innovation theater is similarly absent: Sovereign does not claim to reinvent transcoding or LLM architectures, but candidly defines itself as a curated appliance orchestration layer over proven upstream containers (Jellyfin, Paperless-ngx, Ollama, Open WebUI). The Non-Functional Requirements (§4.7 NFR-1 to NFR-4) avoid boilerplate copy-paste; every NFR specifies product-specific, verifiable bounds—such as Admin Interface TTI < 2.0s on gigabit LAN, boot time < 60s, local RAG first-token latency < 3.5s, WAL database journaling, and 3-retry automated self-healing loops.

### Findings
*No blocking findings.*

---

## 3. Boundary discipline — adequate

Boundary discipline is bolstered by an exemplary Non-Goals section (§5) that rules out custom native client apps, mandatory cloud accounts/telemetry, manual CLI configuration editing, and restrictive re-licensing. The non-users (§2.2) explicitly bar enterprise multi-tenancy and headless cloud VPS setups.

However, MVP scope boundaries (§6.1) include several high-risk features that threaten implementation timelines. First, automated commercial optical disc ripping (FR-7) includes Blu-ray media, yet the document's own Open Question 1 admits that handling commercial decryption keys (AACS/BD+) remains legally and technically unresolved. Second, the smart home deadline alert bridge (FR-13) mandates automated parsing of due dates and payment obligations from raw OCR text without bounding the extraction complexity or specifying the parsing pipeline. Third, committing to complete translation of UI and comprehensive documentation across four languages (FR-18) in MVP introduces non-trivial localization drag before core workflows stabilize.

### Findings
- **high** Optical Media Encryption & Key Handling in MVP (§4.2 FR-7, §8 Open Question 1) — FR-7 places automated Blu-ray ripping in MVP scope ("rips movie or music content to a temporary staging volume without user intervention"), yet Open Question 1 acknowledges that AACS/BD+ decryption key distribution and open-source legal compliance are unresolved. *Fix:* Scope MVP optical disc ingestion to unencrypted DVD/Audio CD media, or explicitly define the key management pattern (e.g. user-mounted keyfile directory) before declaring Blu-ray in-scope for MVP.
- **high** Ambiguous Parsing Architecture for Smart Deadline Alerts (§4.4 FR-13) — FR-13 requires scanning OCR text and document metadata for expiration dates and financial deadlines to trigger Home Assistant webhooks and email digests. It leaves unspecified whether this relies on basic regex pattern matching or a background local LLM prompt hook (as suggested in Addendum §1), leaving performance, accuracy, and VRAM impacts unbound. *Fix:* Clarify the MVP extraction mechanism (e.g., deterministic date/amount regex engine with structured fallback) and specify acceptable error bounds.

---

## 4. Downstream fitness — strong

Downstream engineering, architecture, and UX workflows will find this PRD remarkably fit for implementation. Every functional requirement (FR-1 through FR-20) includes a dedicated "Consequences (testable)" subsection with concrete, verifiable success criteria, network port expectations, and timing thresholds (e.g., mDNS broadcast within 30s in FR-1, update health check timeout of 180s in FR-3, optical drive detection within 10s in FR-7).

The architectural interfaces are well-defined: RAG is explicitly architected as single-source-of-truth querying Paperless-ngx endpoints directly without vector duplication (FR-15), and the shared local AI service exposes an OpenAI-compatible endpoint at `http://sovereign-ai:11434/v1` (FR-16). Two minor ambiguities exist: the Landing Zone drop routing mechanism and the audio container topology in MVP.

### Findings
- **medium** Undefined Ingestion Sorting Logic for Media Landing Zone (§4.2 FR-6) — FR-6 specifies that files dropped into the network share or web upload are "automatically sorted into appropriate media directories (Movies, TV Shows, Music, Audiobooks)". It does not define how files are classified (e.g., mandatory subdirectories vs. media container inspection vs. filename parsing). *Fix:* Specify that the landing zone provides designated subfolders (`drop/movies`, `drop/music`, etc.) or define the heuristic inspection rules.
- **medium** Audio Streaming Container Topology Ambiguity (§4.3 FR-10, §6.1) — FR-10 covers music libraries, party playlists, and audiobooks, referencing Navidrome and Audiobookshelf. Section 6.1 lists "Navidrome/Audiobookshelf (Audio)". Downstream container orchestration requires clarity on whether Sovereign deploys both containers side-by-side or selects a single unified audio backend. *Fix:* Explicitly state the MVP container topology (e.g., dual-container deployment: Navidrome for music/playlists and Audiobookshelf for audiobooks/podcasts).

---

## 5. Metric honesty — strong

Metric design in Section 7 is honest, thoughtful, and tightly aligned with the product's value proposition. Primary metrics (SM-1 to SM-6) measure meaningful user outcomes rather than vanity activity: First-Run Wizard completion without CLI in under 10 minutes (SM-1), 100% zero-terminal operations (SM-2), and 20 consecutive clean boots after ungraceful power disconnections (SM-4).

Most impressively, the PRD defines explicit Counter-Metrics (SM-C1, SM-C2, SM-C3) that establish principled design boundaries: preventing app catalog bloat (SM-C1), refusing to sacrifice disk verification for cold boot speed (SM-C2), and preventing LLM VRAM monopolization at the cost of video transcoding (SM-C3). A slight tension exists between the zero-telemetry policy and the measurement protocol for consumer metrics.

### Findings
- **medium** Clarify Measurement Methodology Under Zero Telemetry (§4.7 NFR-3, §7 SM-1, SM-5) — SM-1 (>=90% setup completion) and SM-5 (>=85% optical rip success) read like telemetry metrics, but NFR-3 strictly forbids outbound data collection. *Fix:* Explicitly note in §7 that SM-1 through SM-6 are measured through synthetic test suites, automated CI failure injection, and structured opt-in usability lab sessions.

---

## 6. Voice and tone integrity — strong

The tone across both `prd.md` and `addendum.md` is disciplined, pragmatic, and refreshingly candid. It treats local hardware as a set of physical constraints subject to power outages, thermal throttling, dirty database writes, and memory contention. Rather than promising frictionless AI miracles, it details model keep-alive timeouts (`OLLAMA_KEEP_ALIVE=0`), VRAM overhead reserves (`OLLAMA_GPU_OVERHEAD`), and database WAL modes. The writing is precise, unambiguous, and oriented around real engineering trade-offs.

### Findings
*No blocking findings.*

---

## 7. Assumption hygiene — adequate

The document demonstrates strong awareness of its operational premises. Section 9 compiles an Assumptions Index that captures five critical technical dependencies, including mDNS broadcast allowances on consumer routers, optical drive hardware availability, IMAP email access, Paperless API search scalability (< 10,000 documents) without an external vector DB, and hardware transcoding chipsets (Intel QuickSync / Nvidia NVENC).

However, the PRD fails the mechanical Assumptions Index roundtrip check: all five assumptions are collected in §9 with references like `[ASSUMPTION: §4.1 FR-1]`, but the actual tag `[ASSUMPTION]` never appears inline in Section 4. Readers reviewing FR-1, FR-7, FR-12, FR-15, or FR-17 directly in the text are not alerted to these underlying assumptions.

### Findings
- **medium** Assumptions Index Inline Roundtrip Failure (§4, §9) — Five assumptions are indexed in Section 9 referencing §4 subsections, but none of the tags appear inline within FR-1, FR-7, FR-12, FR-15, or FR-17. *Fix:* Insert inline `[ASSUMPTION: ...]` tags directly into the body text of the corresponding functional requirements.

---

## Mechanical notes

- **Glossary drift**: The primary administrative interface is variously referred to as "Admin Interface" (§3, §4.1), "Admin UI" (§4.5), "admin web server" (§4.1 FR-4), "admin dashboard" (§2.3 UJ-1, §2.3 UJ-5, §4.2 FR-6), and "admin frontend" (§4.5 FR-17). *Fix:* Normalize references to the Glossary term "Admin Interface".
- **Realizes cross-references**: FR-6 (Landing Zone), FR-16 (Shared AI API), and FR-17 (GPU Arbitration) omit explicit "Realizes UJ-X" headers. *Fix:* Add `Realizes UJ-1, UJ-3` to FR-6, `Realizes UJ-4` to FR-16, and `Realizes UJ-3, UJ-4` to FR-17.
- **ID continuity**: Contiguity across FR-1 through FR-20, UJ-1 through UJ-5, NFR-1 through NFR-4, SM-1 through SM-6, and SM-C1 through SM-C3 is 100% complete with no gaps or orphan IDs.
