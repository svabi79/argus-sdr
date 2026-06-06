# Detection & Estimation Rework Plan — 2026-06-06

Plan, um die **Wahrnehmungsschicht** (Detektion → Bandbreiten-/Center-/SNR-Schätzung
→ Klassifikation) von „funktioniert nur halb / nur für BC-FM" auf universell und
**messbar korrekt** zu bringen. Reihenfolge: erst Ground Truth, dann von unten
nach oben — jede Stufe durch eine Zahl abgesichert.

Status: `plan` / `proposed`
Bezug: `docs/architecture-review-2026-06-06.md` (B-5), `docs/classifier-ml-plan-2026-06-06.md`,
`docs/known-issues.md` (OI-20/21/22), `ROADMAP.md` (Phase R)
Scope: `internal/detector`, `internal/cfar`, `internal/pipeline` (refiner/surveillance),
`internal/mock`, `internal/classifier`, neue Benchmark-/Eval-Targets

---

## 0. Problem (belegt, nicht vermutet)

Symptom (Betrieb): Detektion erkennt mal zu viel, mal zu wenig; Bandbreite
unzuverlässig; zuverlässig nur BC-FM, solange der Detektor darauf eingestellt ist;
nicht universell.

Ursachen im Code:
1. **Einauflösende Detektion.** `internal/detector/detector.go` arbeitet auf *einem*
   FFT-Spektrum mit Schwellwert-Crossing. Eine einzige Bin-Breite kann 100-Hz-CW
   und 180-kHz-WFM nicht gleichzeitig sauber auflösen → Über-/Untererkennung.
2. **Geometrische Bandbreite.** `BWHz = (LastBin-FirstBin+1)·binWidth` plus
   heuristisches Kanten-Auswandern (`expandSignalEdges`) und **fixer** Merge-Gap
   (`MergeGapHz`, 5 kHz). Form-/rauschabhängig; die −3-dB-Breite ist nicht die
   belegte Bandbreite.
3. **Refinement schätzt nichts nach.** `internal/pipeline/refiner.go:18` kopiert nur
   `sig.BWHz = c.BandwidthHz` und hängt Klassifikation/PLL an. Das PLAN-Versprechen
   „Refinement stabilisiert center/bw/snr" ist für Bandbreite/SNR **nicht umgesetzt**.
4. **Keine Ground Truth.** Es gibt keine Metrik, gegen die Detektion/Schätzung/
   Klassifikation gemessen werden. Ohne sie ist jede Änderung Basteln.

Folge: Der Classifier frisst grobe, einkanalige Bandbreite als sein wichtigstes
Feature → die Klassifikation kann gar nicht universell stimmen.

---

## 1. Leitprinzipien

1. **Messbar vor schlau.** Keine DSP-Änderung ohne Benchmark mit bekannter Wahrheit.
2. **Von unten nach oben.** Untergrund (Noise/PSD) → Schätzung (bw/snr) →
   Universalität (Multi-Res) → Klassifikation. Jede Stufe steht auf einer soliden.
3. **Billig & hoher Hebel vor teuer.** Bandbreiten-Reestimation zuerst, Multi-Res
   später (mit Benchmark, die den Nutzen beweist).
4. **Display von Analyse entkoppelt** (PLAN-Leitprinzip 1): Glättung/Decimation
   für die UI darf die Kern-Schätzung nicht verändern.
5. **Vorhandenes vollenden, nicht neu bauen.** mock-Quelle, IQ-Snippet-Extraktion,
   Pipeline-/Surveillance-Gerüst, Telemetrie werden genutzt.
6. **Physikalisch definierte Größen.** Belegte Bandbreite per Power-Containment
   (ITU-Stil), nicht per Schwellwert-Breite.

---

## 2. Metriken (die Sprache von „besser")

Gegen synthetische Szenen mit bekannter Wahrheit:
- **Detektion**: Precision / Recall, aufgeschlüsselt über SNR-Bins; F1.
- **Bandbreite**: mittlerer absoluter Fehler (Hz und % der wahren bw); Bias.
- **Center**: mittlerer Fehler (Hz, in Bins).
- **Über-/Untererkennung**: #erkannte vs #wahre (Split-/Merge-Rate).
- **Klassifikation**: Accuracy, Macro-F1, Confusion-Matrix, per-SNR.
Zielwerte werden **nach** der R0-Baseline kalibriert (keine Wunschzahlen vorab).

---

## 3. Phasen, in kleine Schritte zerlegt

### R0 — Mess-Rückgrat (Ground Truth) — *Voraussetzung für alles* — ✅ ERLEDIGT
Umgesetzt: `internal/synth` (deterministischer Szenen-Generator), getaggte
Benchmark (`go test -tags bench`), Baseline in `docs/detection-baseline-2026-06-06.md`.
- **R0.1** Szenen-Spezifikation: Struct für eine Szene = Liste von Soll-Signalen
  `{modType, centerHz, bwHz, snrDb, duty, startHz/endHz des Capture-Spans}`.
- **R0.2** Synthese: `internal/mock` zu parametrischem Generator erweitern —
  erzeugt Breitband-IQ mit den Soll-Signalen (Modulationen: WFM, NFM, AM, SSB,
  CW, FSK/PSK, schmalband-digital), additives Rauschen für Ziel-SNR.
- **R0.3** Ground-Truth-Matching: Funktion, die Detektor-Output gegen Soll-Signale
  zuordnet (Greedy nach Frequenz-Overlap) und P/R, bw-Fehler, center-Fehler rechnet.
- **R0.4** Benchmark-Harness als `go test` hinter Build-Tag `bench`/`eval`:
  iteriert Szenen über SNR-Bins, gibt Metriken + Confusion-Matrix aus
  (Tabelle in Testlog, optional JSON-Artefakt).
- **R0.5** **Baseline-Lauf gegen die aktuelle Pipeline** → Zahlen dokumentieren.
  Damit ist B-5 erfüllt und wir wissen quantitativ, wo es hakt.
- *Akzeptanz*: reproduzierbarer Benchmark, dokumentierte Baseline-Metriken.

### R1 — Refinement schätzt belegte Bandbreite + SNR (höchster Hebel) — ✅ ERLEDIGT
Umgesetzt: `internal/estimate` (Occupied-Bandwidth per Blob+Containment + Peak/SNR),
in `refiner.go` verdrahtet (Flag `refinement.occupied_bw_fraction`). Beleg an der
Benchmark: refined Median-bw-Fehler ~24 % vs geometrisch ~49 % (WFM 27 % vs 48 %,
SSB 2 %, DIGITAL 1 %). SNR-Reestimation (R1.3): Peak-over-Noise, trackt die
konfigurierte SNR ~1:1.
- **R1.1** Lokale PSD pro Kandidat aus dem IQ-Snippet (Welch über das Snippet)
  bzw. dem High-Res-Detail-Spektrum bereitstellen.
- **R1.2** Occupied-Bandwidth-Schätzer: Band um den Power-Schwerpunkt, das einen
  konfigurierbaren Anteil β (z. B. 99 %) der In-Band-Leistung enthält; robuste
  Center-Schätzung aus derselben PSD.
- **R1.3** SNR aus In-Band-Leistung vs. lokal geschätztem Rauschen.
- **R1.4** In `refiner.go` einhängen: `sig.BWHz/CenterHz/SNRDb` aus der
  Reestimation statt aus dem groben Kopierwert; alte Werte als Fallback behalten.
- **R1.5** Config: `refinement.occupied_bw_fraction` (β), sinnvoller Default;
  Feature-Flag, um auf altes Verhalten zurückzuschalten.
- **R1.6** Tests: synthetische Signale bekannter bw → bw-Fehler an der Benchmark
  deutlich kleiner als Baseline.
- *Akzeptanz*: bw-MAE auf der Benchmark messbar besser als R0-Baseline; kein
  Regress bei Center/Detektion.

### R2 — Stabiles Surveillance-Spektrum + robuster Noise-Floor
- **R2.1** Welch-gemittelte PSD für die Surveillance statt Einzel-FFT + EMA
  (overlap-add, konfigurierbare Mittelung); Display-Glättung getrennt halten.
- **R2.2** Robuste, ortsabhängige Rauschschätzung (gleitendes Perzentil/Median je
  Region) statt globalem `median(smooth)`.
- **R2.3** CFAR-Parameter auf der stabileren PSD neu bewerten (Guard/Train/Scale).
- **R2.4** Tests: Detektion-P/R bei niedrigem SNR an der Benchmark verbessert,
  Fehlalarme reduziert.
- *Akzeptanz*: bessere P/R besonders in niedrigen SNR-Bins; stabilere SNR-Schätzung.

### R3 — Multi-Resolution-Detektion (Universalität)
- **R3.1** Detektor mehrere Surveillance-Level konsumieren lassen (grob für
  Breitband, fein für Schmalband) — das Phase-2-Gerüst tatsächlich verdrahten.
- **R3.2** Kandidaten-Fusion über Level (ein Signal nicht mehrfach; Breitband
  nicht in Sub-Peaks zerfallen lassen).
- **R3.3** Skalenbewusster Merge statt fixem `MergeGapHz` (Gap relativ zur lokalen
  Signalbreite/Auflösung).
- **R3.4** Tests: Über-/Untererkennung über die gesamte Bandbreiten-Spanne (CW …
  WFM) an der Benchmark reduziert.
- *Akzeptanz*: Split-/Merge-Rate über alle Bandbreiten-Klassen niedrig; nicht mehr
  von „auf BC-FM eingestellt" abhängig.

### R4 — Klassifikation auf solider Basis
- **R4.1** Heuristik (`rules.go`) gegen die nun guten Features an der Benchmark neu
  bewerten; offensichtliche Schwellwerte gegen die Synthese kalibrieren.
- **R4.2** Realen Capture-Datensatz aufbauen (siehe `classifier-ml-plan`, Phase 0):
  Auto-Capture + Weak Supervision + Bandplan-Priors + Review-Queue.
- **R4.3** Datenbasiert entscheiden: Heuristik-Tuning vs. Stufe-A-Trees
  (`classifier-ml-plan` §4) — Backend-Interface macht den Tausch risikolos.
- *Akzeptanz*: Macro-F1 an der Benchmark + an realem Test-Set besser als heute.

### R5 — Reale Validierung (Domain-Gap)
- **R5.1** RSP1B-Captures verschiedener Bänder; prüfen, ob synthetische Gewinne
  real halten.
- **R5.2** Domain-Gap nachjustieren (Rauschmodell, Kalibrierung); Defaults/Profile
  aktualisieren.
- *Akzeptanz*: Verbesserungen reproduzieren sich auf realen Captures.

---

## 4. Reihenfolge & Abhängigkeiten

```
R0 (Ground Truth) ─┬─> R1 (bw/snr reestimation)   ← höchster Hebel, klein
                   ├─> R2 (Welch-PSD + noise)       ← stabilisiert Untergrund
                   └─> R3 (multi-res detection)      ← Universalität, groß
                                   └─> R4 (Klassifikation) └─> R5 (real)
```
R0 ist hartes Vorbild. R1/R2 sind unabhängig und können parallel; R3 profitiert von
beiden. R4 erst nach R1–R3. R5 schließt ab.

---

## 5. Aufwand / Realismus

Größenordnung Wochen, nicht Stunden. Aber: jede Stufe ist abgegrenzt, test-first
und durch die Benchmark beweisbar. Kein Big-Bang — R0+R1 liefern schon sichtbaren,
belegten Fortschritt, ohne den Rest zu blockieren.

---

## 6. Querverweise
- `docs/architecture-review-2026-06-06.md` — B-5 (Ursprung), Ausbau-Schieflage
- `docs/classifier-ml-plan-2026-06-06.md` — R4 nutzt dessen Phase 0 / Stufe A
- `docs/known-issues.md` — OI-20 (refinement copies bw), OI-21 (single-res
  detection), OI-22 (kein Ground-Truth-Benchmark)
- `ROADMAP.md` — Phase R als nächste Priorität (vor Phase 5)
- Code: `internal/detector/detector.go`, `internal/pipeline/refiner.go`,
  `internal/cfar/`, `internal/mock/`, `internal/classifier/`
