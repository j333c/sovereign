# Input Reconciliation: User Guidance & Boundaries

**Workspace:** `ux-sovereign-2026-10-15`  
**Date:** 2026-10-15  

---

## 1. User Inputs & Guidance Received

| # | User Input / Remark | Reconciled Location & Status |
| :--- | :--- | :--- |
| **1** | **Confirmed Sources**: `prd.md` and `addendum.md`. | Included in `EXPERIENCE.md.sources`. Fully reconciled. |
| **2** | **Working Mode**: Coaching path. | Executed step-by-step with interactive design direction previews and live feedback. |
| **3** | **Visual Direction**: "clean design, if possible no scrolling on larger screens, on mobile screens easy scrolling if necessary, less distraction, use clean ui library". Direction A selected. | Codified in `DESIGN.md.Brand & Style` (Nordic/Braun clean minimalism), `DESIGN.md.Layout & Spacing` (1080p+ single viewport zero-scroll), and demonstrated in `mockups/dashboard.html`. |
| **4** | **Component & Radius Defaults**: 8px standard radius, semantic traffic light status colors. | Recorded as `{rounded.lg}` (`8px`) in `DESIGN.md.rounded` and semantic hexes in `DESIGN.md.colors`. |
| **5** | **Central Settings View & Extensibility**: "there needs to be a place to enter the central settings view, have extensibility in mind, as more features could be added". | Header includes prominent Settings gear button (`/settings`); `mockups/settings.html` and `EXPERIENCE.md.Information Architecture` detail the extensible tabbed settings hub; dashboard grid includes an extensible modular layout. |
| **6** | **Local AI Scope Boundary**: "the local ai chat will be provided by the installed feature (software) - we do not need to provide this or other features covered by the installed software". | Scope boundary explicitly recorded in `EXPERIENCE.md.Foundation`. Local AI card in Sovereign acts as an appliance launcher to Open WebUI in a new tab; no custom consumer chat bubbles authored. |
| **7** | **Sub-Admin Layer Boundary**: "no need to create more user interaction below the admin layer, as the feature software comes with own, or the home assistant (or other home automation system) will take care of this". | Codified in `EXPERIENCE.md.Foundation` and `EXPERIENCE.md.Information Architecture`. Sovereign provides the appliance management layer; ambient notifications and tablet cards are delegated to Home Assistant via webhooks. |
| **8** | **Standalone Media Dropzone**: "the media dropzone should be a stand alone page and kept very simple for only dropping files. it should not be part of the admin interface". | Removed dropzone from the main admin dashboard to give service cards single-viewport breathing room; authored standalone dedicated page at `mockups/dropzone.html` and detailed in `EXPERIENCE.md.Information Architecture.Standalone Media Dropzone`. |
| **9** | **Overall Principle**: "strict separation of features provided by the installed software and the admin-area of our sovereign server". | Formally adopted as the foundational architecture principle in both `DESIGN.md` and `EXPERIENCE.md`. |

---

## 2. Qualitative Ideas & Drops

Zero user suggestions were dropped or altered. All architectural boundaries and visual constraints are 100% incorporated into the finalized spines and companion mockups.
