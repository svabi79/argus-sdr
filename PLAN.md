# SDR Wideband Suite — Umbauplan

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

- Keine vollständige 20–80 MHz-Endlösung in einem Schritt
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

## Phase-1-Umbau (dieser Arbeitslauf)

### A. Benennung / Projektidentität
- Projektname auf `sdr-wideband-suite` umstellen
- README auf Zielbild anpassen

### B. Konfigurationsmodell vorbereiten
Neue Konfig-Teile einführen:
- `pipeline.mode`
- `surveillance.*`
- `refinement.*`
- `resources.*`
- optionale `profiles.*`

Zusatz:
- `refinement.detail_fft_size` für einen eigenständigen Detailpfad (Refinement-FFT) neben der Surveillance-FFT

Wichtig:
- Abwärtskompatibilität zur bisherigen Config möglichst erhalten
- bisherige Felder weiterhin nutzbar

### C. Analyse von UI trennen
- `fft_size` als primär **analysis/surveillance**-Parameter behandeln
- UI-seitige Bin-/FPS-Wünsche als reine Presentation-Ebene behandeln
- klare Trennung im Code etablieren

### D. Candidate-/Refinement-Modell einziehen
- neue Candidate-/Refinement-Datentypen einführen
- zunächst mit CPU-/bestehendem GPU-Extraction-Pfad implementieren
- Detector bleibt vorerst Kern der Candidate-Erzeugung
- Refiner sitzt danach explizit als eigener Schritt in der Pipeline
- Refinement-Workitems mit expliziten Ausführungsparametern (FFT/Span/Stage)

### E. Pipeline-Orchestrierung modularisieren
- `runDSP()` entflechten
- Schritte explizit machen:
  - ingest
  - spectrum
  - detect
  - refine
  - classify
  - track
  - present
  - record
- Gemeinsame Arbitration-/Budget-Sicht für refinement/record/decode zentralisieren (Admission + Queue/Hold + Debug-Surface)

### F. Dokumentierte Betriebsprofile
- initiale Profile definieren, z. B.:
  - `legacy`
  - `wideband-balanced`
  - `wideband-aggressive`
  - `archive`

### G. Tests / Build grün halten
- Go tests ausführen
- Build testen
- erst danach commit/push

---

## Spätere Phasen

### Phase 2
- echte mehrstufige Surveillance-Resolution-Engine
- GPU-seitige Reduction/Decimation
- UI-Detailfenster an Refinement koppeln

### Phase 3
- Scheduler/Priority/Budget-Engine
- Kandidatenpriorisierung
- automatische Decoder-Slot-Vergabe

### Phase 4
- breitbandige Multi-Span-Profile
- 20–80 MHz konkrete Betriebsmodi
- adaptive Quality-of-Service

---

## Erfolgskriterien für Phase 1

- Fork existiert als neues Repo
- Projekt ist logisch als Wideband-Fork positioniert
- neue Architekturbegriffe sind im Code und in der Config sichtbar
- bestehende Kernfunktionen bleiben lauffähig
- Grundlage für spätere skalierbare, autonome Arbeitsweise ist gelegt

---

## Arbeitsmodus

Umbau erfolgt autonom im Fork.

Guardrails:
- Keine Pushes vor erfolgreichen Tests/Build
- Schrittweise Migration statt Big-Bang
- Bestehende Funktionalität möglichst erhalten
