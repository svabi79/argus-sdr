# Architektur- & Umsetzungs-Review — 2026-06-06

Strategische Einschätzung von Konzept und Umsetzung als Grundlage für das weitere
Vorgehen. Diese Notiz ist **Richtungsempfehlung**, kein Issue-Ersatz: konkrete
Einzelprobleme bleiben in `docs/known-issues.md` (OI-*) die Quelle der Wahrheit.

Status: `review` / `advisory`
Scope: Gesamtarchitektur (Phasen 1–4), Datenpfad, Klassifikation, Tests, Build/Run

---

## 1. Kurzfassung

Das Projekt ist architektonisch **stark und tragfähig**: korrekt geschichtete,
candidate-driven, multi-resolution, policy-gesteuerte Wideband-Engine mit
First-Class-Telemetrie und ungewöhnlich guter Engineering-Disziplin (AGENTS.md,
kuratierte known-issues, Phasen-Migration statt Big-Bang).

Der zentrale Kritikpunkt ist eine **Schieflage in der Ausbaureihenfolge**: die
Ressourcen-Arbitration (Phase 3/4) ist deutlich weiter ausgebaut als
(a) die Klassifikationsqualität, von deren Output sie lebt, und
(b) die Robustheit/Testabdeckung des Datenpfads, der unter allem liegt.

**Leitlinie fürs weitere Vorgehen:** vorerst *keine* neue Policy-/Arbitration-
Intelligenz bauen. Stattdessen Datenpfad härten, GPU-Validierung schließen und
den Classifier empirisch belegen. Danach steht ein weiterer Arbitration-Ausbau
auf festem Grund.

---

## 2. Kennzahlen (Stand Review)

- ~23.500 LOC Go, 18 Packages, 56 Test-Dateien
- `cmd/sdrd` ~5.0k LOC · `pipeline` ~4.2k LOC (17 Test-Files) ·
  `recorder` ~3.8k LOC (**2 Test-Files**) · `classifier` ~1.1k LOC
- Build-Tags + Mock-Modus + GPU/CPU-Fallback + Panic-Recovery in der DSP-Goroutine

---

## 3. Was gut ist (bewahren)

- **Schichtung** Acquisition → Spectrum → Detection → Refinement → Classification
  → Tracking → Presentation ist sauber und richtig.
- **UI von Kernanalyse entkoppelt** (Leitprinzip 1) — der Fehler, den die meisten
  SDR-Projekte machen, ist hier vermieden.
- **Candidate-driven + Multi-Resolution** ist der richtige Skalierungspfad
  Richtung 20–80 MHz ohne Komplettumbau.
- **Telemetrie** ist durchgängig und an jeder DSP-Stufe vorhanden.
- **Engineering-Hygiene**: AGENTS.md, kuratierte known-issues, ehrliche
  Selbsteinschätzung offener High-Issues.
- Gute Testabdeckung im algorithmischen Kern (`pipeline`, `classifier`).

---

## 4. Befunde / Risiken

### B-1 — Ausbau-Schieflage: Arbitration > Wahrnehmung (strategisch)
Die Phase-3/4-Arbitration (Hold/Displacement/Rebalance/Pressure, ~6k LOC,
dutzende Reason-Tags) ist für einen einzelnen RSP1B bei 2,5–10 MHz sehr üppig.
`HoldPolicyFromPolicy` (`internal/pipeline/arbitration.go:53`) multipliziert
ungetestete Heuristik-Konstanten (`recMult *= 1.5`, `decMult *= 1.6`, `*= 0.85` …)
ohne empirische Grundlage und ohne End-to-End-Wirkungstest.
→ Risiko: Pflege einer ausgefeilten Policy-Engine, die noch nicht last-tragend ist.
**Empfehlung:** Arbitration-Ausbau einfrieren, bis B-2…B-4 adressiert sind.

### B-2 — Testabdeckung am dünnsten, wo das Risiko am höchsten ist
`internal/recorder/streamer.go` (Streaming-Audio, shared IQ-Buffer; Quelle des
früheren Audio-Click-Bugs) hat ~3,8k LOC und nur 2 Test-Files.
Deckt sich mit OI-14 / OI-15.
**Empfehlung:** synthetische Fixtures + Kontinuitätstests um wiederholte
`processSnippet`-Aufrufe; `allIQ`-Immutabilitätstest durch Spectrum/Detection.

### B-3 — Offene High-Severity-Datenintegrität (shared-buffer mutation)
- OI-01: `DCBlocker.Apply(allIQ)` mutiert geteilten Buffer in-place
  (`cmd/sdrd/pipeline_runtime.go`) — High, aktuell „deferred".
- OI-02: FM-Diskriminator-State nicht über Segment-Splits gesnapshottet
  (`internal/recorder/streamer.go`).
Beides ist exakt die Fehlerklasse, die das Projekt schon einmal gekostet hat.
**Empfehlung:** vor weiterem Feature-Ausbau fixen (immutable/copy-Vertrag bzw.
`lastDiscrimIQ`/`lastDiscrimIQSet` in `dspStateSnapshot`).

### B-4 — GPU-Validierungslücke
OI-03 / OI-18: CPU-Oracle existiert, wird aber nicht vertraut → GPU-Regressionen
im CGO+CUDA-Pfad sind nicht automatisch fangbar.
**Empfehlung:** Oracle-Integration reparieren, GPU-vs-CPU-Vergleich als Gate
wiederherstellen.

### B-5 — Classifier = hand-getunte Magic-Thresholds
`internal/classifier/rules.go` arbeitet mit harten Grenzen (`bw>=80e3`,
`EnvVariance<0.08`, `flat>0.55` …). Pragmatisch und testbar, aber spröde,
überlappende Klassen, kein Feedback/Lernen. Die ganze Arbitration baut auf
diesem Output auf.
**Empfehlung:** gegen echte Captures tunen; Schwellwerte empirisch belegen oder
in benannte, dokumentierte Konstanten mit Begründung überführen.
**Detailplan:** `docs/classifier-ml-plan-2026-06-06.md` (ML-Pfad: Daten+Benchmark
zuerst → Stufe A Trees → Stufe B CNN, inkl. automatisierter Datensammlung).

### B-6 — `runDSP` ist eine ~320-Zeilen-Select-Schleife
`cmd/sdrd/dsp_loop.go:23` macht Capture, Surveillance, Refinement, Extraction,
Feed, Maintenance, Event-Encoding, Telemetrie, Debug-Snapshot-Bau und Broadcast
inline, mit ~30× wiederholtem `if coll != nil { coll.Observe(...) }`-Boilerplate.
**Empfehlung:** in benannte Schritte zerlegen; Telemetrie-Boilerplate in einen
Timer-/Stage-Helper kapseln. Rein struktureller Refactor, verhaltensneutral.

---

## 5. Empfohlene Ausführungsreihenfolge

1. **Datenpfad härten** — OI-01, OI-02 fixen (B-3)
2. **Regressionstests** — OI-14 (`allIQ`-Immutabilität), OI-15 (`processSnippet`) (B-2)
3. **GPU-Gate** — OI-03 reparieren, OI-18 schließen (B-4)
4. **Classifier empirisch belegen** — Schwellwerte gegen reale Captures (B-5)
5. **`runDSP` entkoppeln** — struktureller Refactor + Telemetrie-Helper (B-6)
6. **Erst danach** Arbitration/Policy weiter ausbauen (B-1 freigeben)

---

## 6. Betriebs-Nachtrag (aus der Inbetriebnahme 2026-06-06)

Beim Start nach Repo-Umzug nach `D:\Code` traten drei *Betriebs*-Probleme auf
(kein Code-Bug):
1. `SDRplayAPIService` (Windows-Dienst) war gestoppt → `sdrplay_api_Open` Fail.
2. RSP1B in hängendem Enumerations-Zustand → `sdrplay_api_GetDevices` Fail →
   per Reboot bereinigt.
3. Bare `.\sdrd.exe` in frischem Shell beendet sich still, weil die
   MinGW-Runtime-DLLs (`libstdc++-6`, `libgcc_s_seh-1`, `libwinpthread-1` aus
   `C:\msys64\mingw64\bin`) nicht im PATH sind. `start-sdr.ps1` setzt den PATH —
   daher immer darüber starten (oder PATH vorher setzen).

Mögliche kleine Verbesserung: `start-sdr.ps1` optional im Vordergrund laufen
lassen (Logs sichtbar) und ggf. den Dienst-Status vor dem Start prüfen.
