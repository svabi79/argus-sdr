# TODO — SDR Visual Suite

## UI
- [ ] RDS RadioText (RT) Anzeige hinzufügen:
  - Overlay: 1 Zeile, sanfter Fade bei Updates, Ellipsis bei Überlänge, optional kleines „RT“-Badge.
  - Detail-Panel: 2 Zeilen Auto-Wrap; bei Überlänge Ellipsis + Expand (Modal/Zone) für Volltext.
  - Update-Logik: RT nur bei stabilem Text (z. B. 2–3 identische Blöcke), optional „RT · HH:MM“ Timestamp.

## Band Settings Profiles (v1.2)
- [ ] Backend: built-in Profile-Struktur + embedded JSON (6 Profile)
- [ ] Backend: Apply-Helper (shared mit /api/config) inkl. source/dsp/save
- [ ] Backend: Merge-Patch mit Feld-Präsenz (nur explizite Felder anwenden)
- [ ] Backend: DisallowUnknownFields + Config-Validierung → 400
- [ ] Backend: Endpoints GET /api/profiles, POST /api/profiles/apply, POST /api/profiles/undo, GET /api/profiles/suggest
- [ ] Backend: Undo-Snapshot (1 Level) + Active Profile ID (Runtime-State)
- [ ] Optional: Active Profile ID über Neustart persistieren (falls gewünscht)
- [ ] UI: Dropdown + Split-Apply (full/dsp_only) + Undo + Active-Badge
- [ ] UI: Suggest-Toast bei center_hz Wechsel, Dismiss-Schutz (>5 MHz)
- [ ] UX: Loading-Indicator während Profilwechsel (1–3s Reset)
- [ ] Tests: Patch-Semantik, dsp_only (center_hz/gain_db bleiben), Unknown Fields, Suggest-Match

## Notes
- Ab jetzt hier die Todo-Liste führen.
