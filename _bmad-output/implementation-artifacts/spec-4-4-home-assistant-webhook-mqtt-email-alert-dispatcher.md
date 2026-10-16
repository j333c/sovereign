---
title: 'Story 4.4: Home Assistant Webhook, MQTT & Email Alert Dispatcher'
type: 'feature'
created: '2026-10-16'
status: 'review'
route: 'dispatch'
baseline_commit: '35fb2d40a3c46dd9f018e819427c24c12b81daf4'
review_loop_iteration: 0
context:
  - '_bmad-output/planning-artifacts/architecture/architecture-sovereign-2026-10-15/ARCHITECTURE-SPINE.md'
  - '_bmad-output/planning-artifacts/ux-designs/ux-sovereign-2026-10-15/DESIGN.md'
  - '_bmad-output/planning-artifacts/ux-designs/ux-sovereign-2026-10-15/EXPERIENCE.md'
  - '_bmad-output/implementation-artifacts/epic-4-context.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Impending payment deadlines, utility bills, and contractual cancellation terms detected in household paperwork are only valuable if they reach family members proactively. Without ambient smart home notifications and consolidated email digests, users must remember to manually check the dashboard, risking missed payments or automatic contract renewals.

**Approach:** Implement a multi-channel alert dispatch engine in `internal/notify` that monitors detected document deadlines from `DeadlineStore` (within the 14-day alert threshold). The dispatcher delivers alerts across three integrated channels: (1) Home Assistant automation webhooks for smart home triggers, (2) MQTT publish events to topic `sovereign/alerts/documents` (QoS 1) for ambient kitchen wall tablet glance cards, and (3) SMTP consolidated email digests with direct document links (`http://sovereign.local/docs/documents/:id`). The engine prevents alert fatigue by tracking notification status (`notified`) and provides administrative test endpoints and settings in the UI.

## Boundaries & Constraints

**Always:**
- Implement multi-channel alert dispatching in `internal/notify`:
  - Home Assistant Webhook: HTTP POST JSON payload (`event: "document_deadline"`, `id`, `document_id`, `title`, `due_date`, `days_remaining`, `amount`, `currency`, `url`, `snippet`).
  - MQTT Publisher: Standard MQTT 3.1.1 protocol client (pure Go, CGO-free) publishing to configurable topic (default: `sovereign/alerts/documents`), QoS 1, non-retained JSON payload.
  - SMTP Email Digest: Standard library `net/smtp` client delivering HTML and text digests with document direct links, supporting TLS/STARTTLS authentication.
- Persist notification state: Once successfully dispatched to enabled channels, update deadline status from `pending` to `notified` in `document_deadlines` to prevent repetitive alert spamming.
- Configurable settings stored in SQLite `settings` table: `ha_webhook_url`, `mqtt_broker_host`, `mqtt_broker_port`, `mqtt_topic`, `mqtt_username`, `mqtt_password`, `alert_email`, `smtp_host`, `smtp_port`, `smtp_username`, `smtp_password`, `smtp_from`, `deadline_alert_threshold`.
- Provide REST endpoints in `sovereign-daemon`:
  - `GET /api/v1/alerts/config`: Get alert channel configurations with passwords redacted.
  - `PUT /api/v1/alerts/config`: Update alert channel configurations.
  - `POST /api/v1/alerts/test/webhook`: Test Home Assistant webhook delivery.
  - `POST /api/v1/alerts/test/mqtt`: Test MQTT broker connection and publish.
  - `POST /api/v1/alerts/test/email`: Test SMTP email sending.
  - `POST /api/v1/alerts/dispatch`: Trigger an immediate alert evaluation and dispatch cycle.
- Background integration: Integrate `AlertDispatcher` with `ObligationScanner` so that newly identified pending deadlines or nightly promoted obligations automatically trigger alert dispatch.
- Consumer microcopy: Strictly eliminate raw network errors, socket traces, or technical jargon in UI responses and email bodies, using reassuring messaging (e.g., *"Upcoming Bill: Insurance payment of $120.00 is due in 7 days."*).

**Never:**
- Never block document ingestion, OCR processing, or interactive API calls on external webhook or MQTT timeouts (enforce strict 5-second HTTP/network timeouts).
- Never send duplicate email notifications for deadlines that have already been marked as `notified` unless explicitly triggered via test endpoints.
- Never expose plaintext SMTP or MQTT passwords in API responses (`GET /api/v1/alerts/config` must redact passwords).

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Home Assistant Webhook | Impending deadline within threshold (e.g. 7 days remaining) | POST to `ha_webhook_url` with JSON payload `{ event: "document_deadline", title: "...", due_date: "...", days_remaining: 7 }` | 5s timeout, logs warning if HA unreachable, does not fail scanner |
| MQTT Publication | Configured MQTT broker host and port | Publishes QoS 1 message to `sovereign/alerts/documents` | Graceful connection error logging; if broker offline, skips without crash |
| Consolidated Email Digest | One or more unnotified pending deadlines | SMTP email sent to `alert_email` with formatted HTML table and plain-text fallback | Logs delivery failure; retains `pending` status so next cycle retries |
| Webhook Test Endpoint | `POST /api/v1/alerts/test/webhook` with URL | Dispatches test payload to target webhook URL, returns HTTP 200 with delivery status | Returns 400 Bad Request or ProblemDetails if webhook unreachable |
| MQTT Test Endpoint | `POST /api/v1/alerts/test/mqtt` | Connects to configured/payload broker, publishes test packet, returns status | Returns ProblemDetails with sanitized error message |
| Email Test Endpoint | `POST /api/v1/alerts/test/email` | Sends test email via configured SMTP server | Returns ProblemDetails if authentication or host unreachable |
| No Channels Configured | Impending deadline detected, all alert targets blank | Deadlines stored and tracked; dispatcher logs debug notice without errors | No-op, system operates normally |
| Channel Credentials Missing | Webhook configured but MQTT credentials invalid | Webhook succeeds, MQTT fails with logged warning, email succeeds if configured | Partial channel failure does not block other channels |

</frozen-after-approval>

## Code Map

- `internal/notify/dispatcher.go` -- `AlertDispatcher` orchestrating webhook, MQTT, and SMTP delivery.
- `internal/notify/webhook.go` -- Home Assistant webhook HTTP client.
- `internal/notify/mqtt.go` -- Pure-Go MQTT 3.1.1 client for publishing document alert packets.
- `internal/notify/email.go` -- SMTP client for sending consolidated HTML and plain text digests.
- `internal/notify/config.go` -- Alert configuration data structures, persistence, and password redaction.
- `internal/store/deadlines.go` -- Extended with `DeadlineStatusNotified` and `MarkNotified` method.
- `cmd/sovereign-daemon/server.go` -- REST endpoints for alert configuration, manual dispatch, and channel tests.
- `api/openapi.yaml` -- OpenAPI schema updates for alert configuration and test endpoints.
- `web/src/lib/api/` -- TypeScript SDK methods for alert configuration and testing.

## Implementation Plan

1. **Storage & Models**: Add `DeadlineStatusNotified` and `MarkNotified` in `internal/store/deadlines.go`.
2. **Notification Subsystem (`internal/notify`)**:
   - `webhook.go`: HTTP POST client sending Home Assistant webhook payload with 5s timeout.
   - `mqtt.go`: Lightweight pure-Go MQTT 3.1.1 client with CONNECT, PUBLISH (QoS 0/1), and DISCONNECT support.
   - `email.go`: SMTP email sender with HTML and text multipart template and direct document links.
   - `dispatcher.go`: `AlertDispatcher` coordinating multi-channel delivery and updating `DeadlineStore`.
3. **Daemon & API Integration**:
   - Initialize `AlertDispatcher` in `cmd/sovereign-daemon/server.go`.
   - Register endpoints: `GET /api/v1/alerts/config`, `PUT /api/v1/alerts/config`, `POST /api/v1/alerts/test/webhook`, `POST /api/v1/alerts/test/mqtt`, `POST /api/v1/alerts/test/email`, `POST /api/v1/alerts/dispatch`.
   - Wire `AlertDispatcher` into `ObligationScanner` event callback.
4. **OpenAPI & Web SDK**:
   - Update `api/openapi.yaml` with alert config and test routes.
   - Update TypeScript client SDK in `web/src/lib/api/client.ts`.
5. **Testing & Verification**:
   - Unit tests in `internal/notify/notify_test.go` (webhook, MQTT mock broker, SMTP mock server, dispatcher).
   - Integration tests in `cmd/sovereign-daemon/server_test.go`.
   - Full test run (`go test ./...` and `npm --prefix web test`).

## Acceptance Criteria

- Given an impending document deadline within the alert threshold, dispatcher delivers webhook payload to configured Home Assistant endpoint.
- Dispatcher publishes JSON alert payload to MQTT broker on topic `sovereign/alerts/documents` (QoS 1).
- Dispatcher formats and transmits consolidated email digest via SMTP with document URLs.
- Once successfully dispatched, deadline status updates to `notified` to prevent duplicate notifications.
- Administrative test endpoints allow testing webhook, MQTT, and SMTP channels with sanitized error feedback.
