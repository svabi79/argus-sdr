# SDR Wideband Suite -- Umbauplan

## Zielbild

Aus `sdr-visual-suite` wird eine **skalierbare, policy-gesteuerte Wideband-SDR-Engine**.

Ziel ist:
- gleiche Grundfunktionen wie heute: Live-Spectrum, Waterfall, Events, Tracking, Classification, Live-Listen, Recording, Decoder
- aber deutlich flexibler und zukunftsfähig
- Bandbreite soll konfigurierbar skalieren: von ~2.5 MHz bis zu den Grenzen von SDR, I/O, GPU und gewähltem Modus
- spätere Nutzung soll über **Konfiguration/Profiles/Policies** erfolgen, nicht über Code-Umbau

## Leitprinzipien

1. **UI entkoppelt von Analysequalität**
   - Anzeigeparameter dürfen nicht mehr direkt die Kernanalyse verschlechtern/ändern.
2. **Multi-Resolution statt Ein-FFT-für-alles**
   - Globaler Surveillance-Layer mittelfein
   - lokale Refinement-Layer hoch / sehr hoch aufgelöst
3. **Candidate-driven Processing**
   - Detectoren erzeugen Kandidaten
   - Refiner verfeinert Kandidaten lokal
   - Classifier/PLL/Decoder arbeiten auf verfeinerten Kandidaten
4. **Policy-driven Operation**
   - gewünschtes Verhalten über Betriebsprofile steuerbar
5. **Ressourcenbewusst**
   - GPU/CPU/Storage/Latency-Budgets sind Teil der Architektur

## Nicht-Ziele in Phase 1

- Keine vollständige 20-80 MHz-Endlösung in einem Schritt
- Keine perfekte neue GPU-Pipeline über Nacht
- Kein Big-Bang-Delete der bisherigen Pipeline

Phase 1 baut die **Architekturgrundlage**, so dass spätere Skalierung ohne erneuten Komplettumbau möglich ist.

---

## Zielarchitektur

### 1. Acquisition Layer
Verantwortung:
- IQ aus Quelle lesen
- Source-Config anwenden
- Ringbuffer/Chunking verwalten

### 2. Spectrum Engine
Verantwortung:
- Surveillance-Spectrum erzeugen
- später mehrere Resolution-Levels erzeugen
- UI-geeignete decimierte Views ableiten

### 3. Candidate Detection Layer
Verantwortung:
- breitbandig Aktivität/Kandidaten finden
- coarse estimates liefern: center/bw/snr/type-hints

### 4. Refinement Layer
Verantwortung:
- Kandidaten lokal höher aufgelöst nachanalysieren
- center/bw/snr stabilisieren
- IQ-/Feature-Snippets für Classifier vorbereiten

### 5. Classification + Decode Layer
Verantwortung:
- Signaltypen klassifizieren
- PLL / Stereo / RDS / Decoder anwenden
- hochwertige Signalmetadaten erzeugen

### 6. Tracking/Event Layer
Verantwortung:
- Kandidaten über Zeit stabil tracken
- Events erzeugen/schließen
- UI und Recorder mit stabilen Signalen füttern

### 7. Presentation Layer
Verantwortung:
- Overview Spectrum
- decimierte WS-Frames
- Detail-Views
- UI-State ohne Einfluss auf Kernanalyse

---

## Phase-1-Umbau

### Phase-1 Status
- Architekturgrundlage, Config-Modell und Arbitration/Admission-Surface sind umgesetzt.
- Phase 1 ist als Meilenstein abgeschlossen.

### Kernpunkte
- Projektname auf `sdr-wideband-suite` umgestellt
- neues Konfigurationsmodell für `pipeline`, `surveillance`, `refinement`, `resources`, `profiles`
- Analyse klarer von Presentation getrennt
- explizites Candidate-/Refinement-Modell eingeführt
- `runDSP()` in klarere Schritte zerlegt
- gemeinsame Arbitration-/Budget-Sicht für refinement/record/decode zentralisiert
- initiale Betriebsprofile dokumentiert
- Tests/Build grün gehalten

---

## Phase-2-Umbau

### Phase-2 Status
- Phase 2 ist als Meilenstein abgeschlossen.

### Gelandete Wellen
- **Wave A**: operative Surveillance-Level, decimated/derived spectra, Level-Set und API-Sicht
- **Wave B**: derived candidate pass, primary/derived fusion, candidate evidence
- **Wave C**: level-aware candidate semantics und konservatives evidence-aware scoring
- **Wave D**: detection governance, Rollen für detection/support/presentation, derived-detection policy
- **Wave E**: Konsolidierung von detection/support semantics, fused candidate summaries, Debug/API/Docs

### Ergebnis
- echte mehrstufige Surveillance-Resolution-Grundlage
- derived detection governance und support-only Semantik
- fused candidate evidence summaries
- Level-Rollen und Debug-Sicht sind operativ sichtbar

---

## Phase-3-Umbau

### Phase-3 Status
- Phase 3 ist als konservativer Runtime-Intelligence-Meilenstein abgeschlossen.

### Gelandete Wellen
- **Wave 3A**: Priority-Tiers, Admission-Classes, reichere Reason-Familien
- **Wave 3B**: Budget-Preferences, Effective-Budgets, Pressure-Summaries
- **Wave 3C**: Hold-Protection, opportunistic displacement, displaced-by-hold, harmonisierte Reason-Taxonomie
- **Wave 3D**: family-aware tier floors, intent-aware hold behavior, family-priority Runtime-Semantik
- **Wave 3E**: konservative cross-resource Rebalance für refinement/record/decode mit profil-/intent-spezifischem Schutz

### Ergebnis
- refinement / record / decode sprechen eine gemeinsame Admission-/Priority-Sprache
- pressure ist sichtbar und verhaltenswirksam
- hold / protection / displacement sind erklärbar
- signal-family priorities greifen in echte Runtime-Entscheidungen ein
- conservative adaptive rebalance verschiebt Ressourcen zwischen refinement / record / decode

---

## Spätere Phasen

### Phase 4
Phase-4 Status: Monitor-Window-Betriebsmodell gelandet (Wave 4F-C Abschluss).

Gelandete Wellen (4F):
- Monitor-Windows mit Overlap, Priority/Zone-Bias und Auto-Record/Decode pro Window
- Candidate-Gating im Capture-Span + Refinement-Plan-Input
- Window-Statistiken + Outcome-Summaries in Debug/API (`/api/refinement`)
- Decision/Admission-Reason-Tags mit `window:*` + `window-zone:*`

Nächste Ausbaustufe (später):
- breitbandige Multi-Span-Profile
- 20-80 MHz konkrete Betriebsmodi
- adaptive Quality-of-Service
- weitere Scheduler-/Betriebsintelligenz über die konservative Phase-3-Rebalance hinaus

---

## Arbeitsmodus

Umbau erfolgt autonom im Fork.

Guardrails:
- Keine Pushes vor erfolgreichen Tests/Build
- Schrittweise Migration statt Big-Bang
- Bestehende Funktionalität möglichst erhalten
