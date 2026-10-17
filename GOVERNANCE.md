# Sovereign Project Governance

## 1. Overview & Project Mission

The mission of Sovereign is to empower households and individuals with a private, autonomous, durable digital home sanctuary that eliminates cloud dependency without sacrificing modern user experience.

Sovereign is governed as an open, meritocratic, community-oriented open-source project. Decisions are guided by our foundational principles:
- Absolute personal data sovereignty and privacy.
- Appliance-grade resilience and zero-maintenance operation.
- Respect for upstream open-source authors and copyleft boundaries.

---

## 2. Maintainership Model

The project maintainers oversee the technical direction, release management, and repository hygiene:

- **Lead Maintainer**: Coordinates release scheduling, core architectural reviews, and security disclosures.
- **Subsystem Maintainers**: Review contributions within specific domains:
  - *Host Daemon & Storage*: Go orchestration, SQLite WAL persistence, supervisor lifecycle.
  - *Web Experience & Accessibility*: Next.js, WCAG AA compliance, internationalization (i18n).
  - *Hardware & Profiles*: Hardware tier benchmarking, thermal/power measurements, optical drive drivers.
  - *AI & Inference*: llama.cpp integration, Smart GPU arbitration, vector RAG pipelines.

---

## 3. Decision-Making Process

- **Consensus by Default**: Architectural enhancements, API additions, and new spoke integrations are discussed openly in repository issues, discussions, and pull requests on `http://git.crins:3000/jens/sovereign` (and mirrored on GitHub).
- **Architectural Invariant Veto**: Any change that compromises zero-telemetry, mounts the Docker socket into spoke containers, breaks CGO-free portability, or drops multilingual parity cannot be merged without unanimous maintainer consensus.
- **RFC Workflow**: Large structural changes (new spoke workloads, major database schema migrations) are proposed via an architectural RFC document detailing user intent, trade-offs, and rollback safety before implementation.

---

## 4. Ethical AI Usage Disclosures

Sovereign acknowledges the role of modern AI in development and software tooling:
- **Transparent Provenance**: Commits and documentation reflect AI-assisted pair-programming contributions honestly and openly.
- **Verification Priority**: Machine-generated code is treated with healthy skepticism; every change undergoes automated test validation, adversarial code review, and human sanity checking before landing in `main`.
- **Private-by-Design Inference**: Sovereign's local AI features are designed strictly for private household empowerment, running offline on the user's physical hardware without telemetry or surveillance.
