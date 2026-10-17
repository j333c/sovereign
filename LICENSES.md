# Open-Source Software Licenses & Attribution Notices

Sovereign is built atop extraordinary open-source software created by global developer communities. We are deeply indebted to these projects and believe in strict compliance, transparency, and honoring original copyrights without proprietary encumbrance.

This document details third-party components, licenses, copyright holders, source repositories, and our architectural boundary guarantees.

---

## 1. Architectural Isolation & Legal Boundary Statement

Sovereign operates as a modular home appliance orchestrator. The host daemon and web interface (licensed under Apache License 2.0) run independently of containerized spoke workloads:

- **Isolated Container Workloads**: All bundled spoke applications (Jellyfin, Navidrome, Audiobookshelf, Paperless-ngx, Open WebUI, llama.cpp, ARM) execute inside independent, OCI-compliant Linux containers.
- **Network-Only Inter-Process Communication**: The host supervisor communicates with containerized services strictly via standard HTTP REST APIs, Unix domain sockets for web traffic reverse proxying, and POSIX filesystem mounts (`/data/`).
- **No Shared Memory or Static/Dynamic Linking**: Neither the host Go daemon nor the web frontend statically or dynamically links to GPL-licensed libraries or internal spoke binaries.
- **Container Boundary Integrity**: Under established open-source legal precedents and Free Software Foundation guidance, running unmodified, independently developed GPL-licensed software within container boundaries communicating across standard network interfaces does not create a combined or derivative work, ensuring the host orchestrator remains unencumbered Apache 2.0 while fully respecting and preserving all upstream GPL rights.

---

## 2. Bundled Open-Source Workloads

### 1. Jellyfin
- **Description**: The Free Software Media System (video streaming and hardware-accelerated transcoding).
- **License**: GNU General Public License v3.0 (GPL-3.0)
- **Copyright**: © 2018–2026 Jellyfin Contributors
- **Repository**: [https://github.com/jellyfin/jellyfin](https://github.com/jellyfin/jellyfin)
- **License Full Text**: [https://www.gnu.org/licenses/gpl-3.0.html](https://www.gnu.org/licenses/gpl-3.0.html)

### 2. Navidrome
- **Description**: Modern Music Server and Streamer compatible with Subsonic / Airsonic.
- **License**: GNU General Public License v3.0 (GPL-3.0)
- **Copyright**: © 2018–2026 Deluan Quintao and Navidrome Contributors
- **Repository**: [https://github.com/navidrome/navidrome](https://github.com/navidrome/navidrome)
- **License Full Text**: [https://www.gnu.org/licenses/gpl-3.0.html](https://www.gnu.org/licenses/gpl-3.0.html)

### 3. Audiobookshelf
- **Description**: Self-hosted audiobook and podcast server with multi-user sync.
- **License**: GNU General Public License v3.0 (GPL-3.0)
- **Copyright**: © 2021–2026 advplyr and Audiobookshelf Contributors
- **Repository**: [https://github.com/advplyr/audiobookshelf](https://github.com/advplyr/audiobookshelf)
- **License Full Text**: [https://www.gnu.org/licenses/gpl-3.0.html](https://www.gnu.org/licenses/gpl-3.0.html)

### 4. Paperless-ngx
- **Description**: Community-driven document management system with OCR and machine learning indexing.
- **License**: GNU General Public License v3.0 (GPL-3.0)
- **Copyright**: © 2021–2026 Paperless-ngx Contributors (based on original work by Daniel Quinn and Jonas Winkler)
- **Repository**: [https://github.com/paperless-ngx/paperless-ngx](https://github.com/paperless-ngx/paperless-ngx)
- **License Full Text**: [https://www.gnu.org/licenses/gpl-3.0.html](https://www.gnu.org/licenses/gpl-3.0.html)

### 5. Open WebUI
- **Description**: Extensible, self-hosted AI interface for LLMs and document RAG.
- **License**: MIT License
- **Copyright**: © 2023–2026 Open WebUI Community
- **Repository**: [https://github.com/open-webui/open-webui](https://github.com/open-webui/open-webui)
- **License Terms**: Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files (the "Software"), to deal in the Software without restriction, including without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software...

### 6. llama.cpp
- **Description**: High-performance, portable LLM inference in C/C++ with GGUF support.
- **License**: MIT License
- **Copyright**: © 2023–2026 Georgi Gerganov and llama.cpp Contributors
- **Repository**: [https://github.com/ggerganov/llama.cpp](https://github.com/ggerganov/llama.cpp)
- **License Terms**: Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files (the "Software"), to deal in the Software without restriction...

### 7. Caddy
- **Description**: Fast, multi-platform web server with automatic TLS reverse proxy capabilities.
- **License**: Apache License 2.0
- **Copyright**: © 2015–2026 Matthew Holt and Caddy Contributors
- **Repository**: [https://github.com/caddyserver/caddy](https://github.com/caddyserver/caddy)
- **License Full Text**: [https://www.apache.org/licenses/LICENSE-2.0](https://www.apache.org/licenses/LICENSE-2.0)

### 8. Automatic Ripping Machine (ARM)
- **Description**: Autonomous optical disc detection and transcoding pipeline for CD, DVD, and Blu-ray.
- **License**: MIT License / GNU General Public License v2.0
- **Copyright**: © 2016–2026 1337-server, automatic-ripping-machine Contributors
- **Repository**: [https://github.com/automatic-ripping-machine/automatic-ripping-machine](https://github.com/automatic-ripping-machine/automatic-ripping-machine)
- **License Full Text**: [https://github.com/automatic-ripping-machine/automatic-ripping-machine/blob/master/LICENSE](https://github.com/automatic-ripping-machine/automatic-ripping-machine/blob/master/LICENSE)

---

## 3. Host Dependencies & Tooling

The Sovereign host appliance daemon and web interface utilize open-source foundational libraries:

- **Go Standard Library**: BSD-style license (The Go Authors).
- **modernc.org/sqlite**: Pure Go CGO-free SQLite implementation (BSD-3-Clause / Public Domain).
- **github.com/grandcat/zeroconf**: Pure Go mDNS discovery implementation (BSD-3-Clause).
- **gopkg.in/yaml.v3**: YAML parsing and marshaling for Go (Apache License 2.0 / MIT).
- **React & Next.js**: MIT License (Meta Platforms, Inc. & Vercel, Inc.).
- **Tailwind CSS**: MIT License (Tailwind Labs, Inc.).

---

## 4. Spoke Container Provenance & Dynamic Synthesis

In compliance with the GNU General Public License (v2 and v3) and other copyleft licenses:
- **Upstream Source Code**: All Dockerfiles and compose manifests reference official public upstream source code repositories.
- **Dynamic Spoke Synthesis**: Spoke container specifications and image tags (Jellyfin, Navidrome, Audiobookshelf, Paperless-ngx, llama.cpp, Open WebUI) are dynamically synthesized by `sovereign-daemon` via the declarative supervisor engine in `internal/supervisor/`.
- **Optical Ingestion Engine**: Sovereign includes an autonomous, native Go optical disc detection and staging engine (`internal/optical/` and `internal/enrichment/`) designed for headless disc ingestion, complementing or integrating with ARM container workflows.
- **Redistribution Rights**: If customized or modified binaries are redistributed in container images, source code or scripts used to construct such modifications are provided directly within the project repository or via accessible upstream mirrors without proprietary barriers.
