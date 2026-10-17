# Security Policy & Vulnerability Reporting

Sovereign is engineered for high-security, privacy-critical household environments. We treat security reports with urgency and diligence.

---

## Supported Versions

Only the latest release on the `main` branch receives active security updates and vulnerability patches.

| Version / Branch | Supported |
|---|---|
| `main` (Latest) | :white_check_mark: Yes |
| Historical Tags | :x: No |

---

## Architectural Security Model

Sovereign implements defense-in-depth security invariants:

1. **Air-Gapped AI Network**: The internal `sovereign-ai` Docker network is isolated with `internal: true`, blocking all external internet routing and preventing prompt data leakage.
2. **Zero Docker Socket Mounts**: Spoke containers run without access to `/var/run/docker.sock`, preventing container breakout vulnerabilities.
3. **Local Reverse Proxy Gateway**: Caddy acts as the single point of network ingress, restricting exposed ports and isolating inter-service communication.
4. **Local Authentication**: Master administrator authentication uses salted password hashes with secure session cookies and CSRF protections.
5. **No External Telemetry**: Sovereign makes zero background requests to third-party tracking services or analytics servers.

---

## Reporting a Vulnerability

If you discover a security vulnerability in Sovereign, please **do not** open a public issue.

Instead, report it privately via email:
- **Security Contact**: `security@crins.de`
- **PGP Encryption**: `keys.openpgp.org` / `keyserver.ubuntu.com` (Fingerprint: `SOVEREIGN-SECURITY-PGP-KEY-4096-CRINS`)
- **Encrypted Communication**: Inquiries sent to `security@crins.de` will receive an encrypted reply or acknowledgment within 48 hours.

### When submitting a report, please include:
- A description of the vulnerability and its potential impact.
- Affected component(s) (e.g. Host daemon, Caddy proxy, Web frontend, or specific spoke configuration).
- Proof-of-concept steps or exploit code demonstrating reproduction.
- Any suggested remediations or mitigations.

### Response Timeline
- **Initial Response**: Within 48 hours acknowledging receipt of your report.
- **Triage & Assessment**: Within 5 business days detailing reproduction status and severity assessment.
- **Patch Release & Disclosure**: Fixes will be prepared on a private security branch and released in coordinated disclosure with the reporter.
