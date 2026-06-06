# Classifier-ML-Plan — 2026-06-06

Detaillierter Umsetzungsplan, um die heutige heuristische Signalklassifikation
durch maschinelles Lernen zu ergänzen/abzulösen. Reihenfolge bewusst:
**Daten + Benchmark zuerst → Stufe A (klassisches ML) → Stufe B (Deep Learning)**.

Status: `plan` / `proposed`
Bezug: `docs/architecture-review-2026-06-06.md` (Befund B-5), `docs/known-issues.md`
Scope: `internal/classifier`, Datenaufzeichnung im `recorder`, Eval-Harness, Trainings-/Inferenzpfad

---

## 0. Wieso überhaupt

**Problem (B-5).** `internal/classifier/rules.go` klassifiziert über hand-getunte
Schwellwerte (`bw>=80e3`, `EnvVariance<0.08`, `flat>0.55` …). Das ist spröde,
Klassen überlappen, es gibt kein Feedback/Lernen, und es ist **nicht messbar** —
wir wissen nicht, wie gut es real ist. Die gesamte Arbitration (Phase 3/4) baut
auf diesem Output auf.

**Warum ML passt.** Der Code hat bereits die ideale Naht:
- `internal/classifier` erzeugt einen **Feature-Vektor** (`Features`)
- Output ist `Classification{ ModType, Confidence, Scores map[SignalClass]float64 }`
- die **Refinement-Layer liefert pro Kandidat IQ-Snippets** in passender Rate
- der **Recorder speichert IQ + meta** → Datensatz aus dem Live-Betrieb möglich

Ein ML-Modell befüllt **dieselbe** `Scores`/`Confidence`-Struktur. Damit ändert
sich downstream (Arbitration, UI, Tracking) **nichts**. Integrationspunkt ist
ein Interface, kein Umbau.

**Warum diese Reihenfolge.** Ohne gelabelten Datensatz + Benchmark kann man weder
die Heuristik messen noch ein Modell trainieren oder vergleichen. Daten/Benchmark
sind die Grundlage für *alles* Weitere und liefern nebenbei die Baseline für B-5.

---

## 1. Leitprinzipien

1. **Messbar vor schlau.** Erst Benchmark + Baseline, dann Modelle.
2. **Hybrid, nicht ersetzen.** Harte physikalische Priors (Bandbreite,
   Frequenz-Kontext/Bandplan aus `context.go`) bleiben und werden mit der
   ML-Posterior **fusioniert**.
3. **Stabile Schnittstelle.** Jeder Classifier (Heuristik / Trees / CNN) erfüllt
   dasselbe Interface und liefert `Scores`/`Confidence`. Austausch per Config.
4. **Open-Set zuerst ernst nehmen.** „Unknown"/„Noise" sauber ablehnen ist
   wichtiger als ein paar Prozent Accuracy auf bekannten Klassen.
5. **Kein neues DLL-Drama in Stufe A.** Inferenz pure-Go halten, solange möglich
   (PATH/CGO-Probleme sind real, siehe Review §6).
6. **Reproduzierbarkeit.** Datensatz, Split, Modell und Metriken versioniert;
   Eval als `go test`-Target, das jeder reproduzieren kann.

---

## 2. Gemeinsames Interface (Voraussetzung für alle Stufen)

Bevor Modelle kommen, den Classifier hinter ein austauschbares Interface ziehen:

```go
// internal/classifier
type Backend interface {
    Classify(feat Features, iq []complex64, centerHz, snrDb float64) Classification
}
```

- `RuleBackend` = heutige `RuleClassify` (Default, Fallback)
- `TreeBackend` = Stufe A
- `IQModelBackend` = Stufe B
- `HybridBackend` = fusioniert Prior (Regeln/Kontext) + Modell-Posterior

Auswahl über Config, z. B. `classifier.backend: rules|tree|iq|hybrid`.
Damit ist jede Stufe ein additiver, rückbaubarer Schritt.

---

## 3. Phase 0 — Daten + Benchmark (Fundament)

Ziel: einen wachsenden, gelabelten Datensatz **bequem und schnell** aufbauen und
ein reproduzierbares Eval-Harness haben.

### 3.1 Was aufgezeichnet wird (Datensatz-Schema)
Pro Kandidat ein Sample:
- **Feature-Vektor** (`Features`, vollständig) — Input für Stufe A
- **IQ-Snippet** (komplex, dezimiert auf Signal-Rate, feste Länge z. B. 1024/4096) — Input für Stufe B
- **Kontext**: center_hz, bw_hz, snr_db, timestamp, sample_rate, profile
- **Heuristik-Vorhersage**: `ModType`, `Confidence`, `Scores` (= schwaches Label)
- **Label**: leer / bestätigt / korrigiert + Label-Quelle (auto/heuristik/manuell)

Format: eine Zeile JSONL pro Sample + IQ als separates `.cf32`/`.npy` referenziert,
oder kompakt als Parquet. Ablage unter `data/dataset/` (gitignored, wie andere
Runtime-Artefakte laut AGENTS.md §6).

### 3.2 Wie das Sammeln **automatisiert & komfortabel** wird
Du brauchst viele Samples — also so wenig Handarbeit pro Sample wie möglich.
Vier Hebel, kombiniert:

1. **Auto-Capture im Betrieb.** Ein Sampling-Hook im DSP-Pfad
   (`cmd/sdrd/dsp_loop.go`, nach Refinement) schreibt mit Rate-Limit jeden
   N-ten Kandidaten samt Features+IQ+Heuristik-Label weg. Läuft passiv mit,
   während die Engle ohnehin läuft → Datensatz wächst von selbst.
2. **Weak Supervision / Auto-Labeling.** Die Heuristik liefert das *vorläufige*
   Label gratis. Nur **unsichere/strittige** Fälle landen in der manuellen
   Queue:
   - hohe `Confidence` + plausibler Bandplan-Kontext → auto-akzeptiert
   - niedrige Confidence, knappe `top2`-Differenz, Kontext-Widerspruch → Review
   So labelst du nur die ~10–20 %, die wirklich Aufmerksamkeit brauchen.
3. **Bandplan-Priors als Auto-Labeler.** Bekannte Frequenzen labeln sich quasi
   selbst (z. B. UKW-Rundfunk-Band → WFM, 2 m/70 cm Repeater-Subbänder → NFM,
   FT8-Dial-Frequenzen → FT8). `context.go` hat den Kontext schon — als
   Label-Heuristik wiederverwenden. Sendet starke, billige Labels.
4. **Active Learning.** Sobald ein erstes Modell existiert: gezielt die Samples
   zur manuellen Prüfung vorschlagen, bei denen Modell **und** Heuristik
   uneinig sind oder das Modell maximal unsicher ist. Maximaler Label-Nutzen
   pro Klick.

### 3.3 Label-Tool (schnell statt schön)
- Minimal-Erweiterung der bestehenden Web-UI: Review-Queue mit
  Spektrum/Wasserfall-Ausschnitt + Audio-Vorhör (Live-Demod-Endpoint existiert
  schon: `/api/demod`), darunter die Heuristik-Top-2 als Buttons.
- Bedienung per **Tastatur** (1 = bestätigen, 2 = zweitbeste, andere Taste =
  Klasse wählen, Leertaste = nächstes). Ziel: <2 s pro Sample.
- Jede Bestätigung/Korrektur schreibt das Label zurück ins Sample.

### 3.4 Durchsatz-Abschätzung (wie viele Daten?)
- Grobe Hausnummer: **≥ 500–1.000 saubere Samples pro Klasse** für Stufe A
  (Trees), mehr für seltene Klassen; Stufe B (CNN) profitiert von 5–10×.
- Klassen-Imbalance ist real (viel WFM/NFM, wenig FT8/WSPR/DMR/D-STAR) →
  seltene Klassen gezielt sammeln (Band absuchen, Active Learning).
- Mit Auto-Capture + Weak Supervision trägst du die Masse passiv ein und
  korrigierst nur die Zweifelsfälle. Realistisch sind so einige tausend
  Samples in wenigen Betriebs-Sessions ohne stundenlanges Hand-Labeln.

### 3.5 Benchmark / Eval-Harness
- `go test`-Target (z. B. `internal/classifier/eval_test.go` hinter Build-Tag
  `eval`), das gegen ein eingefrorenes, gelabeltes Test-Set rechnet:
  - **Accuracy**, **Macro-F1** (wegen Imbalance wichtiger als Accuracy)
  - **Confusion-Matrix** (zeigt *welche* Klassen verwechselt werden)
  - **Open-Set-Metrik**: Rejection-Rate für „Unknown"/„Noise"
  - **Per-SNR-Aufschlüsselung** (Accuracy über SNR-Bins)
- Train/Val/Test-Split **zeit-/aufnahme-getrennt** (nicht zufällig), damit
  benachbarte Frames desselben Signals nicht über Splits lecken.
- Erster Lauf = **Baseline der heutigen Heuristik** → damit ist B-5 erfüllt.

### Akzeptanzkriterien Phase 0
- [ ] Datensatz-Logging passiv im Betrieb aktiv, gitignored
- [ ] Auto-Label via Confidence + Bandplan-Kontext
- [ ] Label-Queue in der UI, tastaturbedienbar
- [ ] eingefrorenes Test-Set + `go test`-Benchmark mit Confusion-Matrix
- [ ] dokumentierte Heuristik-Baseline (Macro-F1, Confusion-Matrix)

---

## 4. Stufe A — Klassisches ML auf den vorhandenen Features

Ziel: `rules.go`-Scoring durch ein gelerntes Modell auf demselben Feature-Vektor
ersetzen — risikoarm, ohne neue native Abhängigkeit.

### Wie
- **Modell**: Gradient-Boosted Trees (LightGBM, alternativ XGBoost). Robust bei
  heterogenen Features, wenig Tuning, gut bei begrenzten Daten.
- **Training**: offline in Python (separates `ml/`-Verzeichnis oder eigenes
  Repo), Input = die geloggten Feature-Vektoren + Labels. Export als
  LightGBM-Textmodell.
- **Inferenz in Go ohne CGO/DLL**: Pure-Go-Paket `leaves` liest das
  LightGBM-Modell direkt. Keine neuen DLLs, kein PATH-Risiko (vgl. Review §6).
  → `TreeBackend` füllt `Scores` (aus den Klassen-Wahrscheinlichkeiten) und
  `Confidence`.
- **Latenz**: Mikrosekunden pro Kandidat → unkritisch bei 12 fps × N Kandidaten.

### Hybrid-Fusion
- Bandplan-/Bandbreiten-Priors aus `context.go` als multiplikativen/additiven
  Prior auf die Modell-Scores legen (z. B. WFM nur im Rundfunkband begünstigen).
- Hard-Rules als Veto für physikalisch Unmögliches behalten.

### Warum zuerst
- Nutzt die vorhandene Feature-Extraktion und denselben Datensatz, der ohnehin
  für die Baseline entsteht. Schnellster Weg zu „besser als Heuristik, messbar".

### Akzeptanzkriterien Stufe A
- [ ] `TreeBackend` hinter dem Classifier-Interface, per Config wählbar
- [ ] Pure-Go-Inferenz, keine neue native Abhängigkeit
- [ ] Macro-F1 auf dem Test-Set **messbar besser** als die Heuristik-Baseline
- [ ] Hybrid-Fusion mit Bandplan-Priors aktiv
- [ ] Fallback auf `RuleBackend`, wenn Modell fehlt/Lade-Fehler

---

## 5. Stufe B — Deep Learning direkt auf IQ

Ziel: höhere Decke durch Lernen direkt auf dem Rohsignal statt auf
hand-gebauten Features.

### Wie
- **Input**: komplexe IQ-Snippets pro Kandidat (Refinement-Layer liefert sie),
  feste Länge, normalisiert; alternativ Spektrogramm-Bild.
- **Modell**: 1D-CNN/ResNet auf IQ (Ansatz nach O'Shea/RadioML). Öffentlicher
  Datensatz **RML2018.01A** (24 Modulationen) als Vortraining möglich — aber
  **Domain-Gap** zu RSP1B + lokalen Bedingungen einplanen (Fine-Tuning auf
  eigenem Datensatz nötig).
- **Training**: PyTorch, offline. Export als **ONNX**.
- **Runtime — zwei Optionen**:
  1. **ONNX Runtime via Go** (onnxruntime-go, CGO): saubere In-Process-Inferenz,
     aber zusätzliche native Abhängigkeit (mehr DLL-Management — bewusst gegen
     Review §6 abwägen).
  2. **Python-Sidecar** über den bestehenden `tools/`-Decoder-Mechanismus /
     lokalen RPC: kein CGO im Hauptbinary, klar getrennt, etwas mehr Latenz/IO.
- **GPU**: CUDA ist bereits eingebunden → Inferenz kann auf der GPU laufen.

### Wann
- Erst wenn Phase 0 + Stufe A stehen und der Datensatz groß genug ist
  (DL braucht deutlich mehr Daten). Stufe B ist die Investition für den letzten
  Genauigkeitssprung, nicht der Einstieg.

### Akzeptanzkriterien Stufe B
- [ ] `IQModelBackend` hinter dem Classifier-Interface
- [ ] Runtime-Entscheidung (ONNX-CGO vs. Sidecar) dokumentiert und begründet
- [ ] Fine-Tuning auf eigenem Datensatz, nicht nur RML-Vortraining
- [ ] Macro-F1 und Open-Set-Rejection **besser als Stufe A** auf dem Test-Set
- [ ] Latenz/Last im Live-Betrieb verifiziert (12 fps × N Kandidaten)

---

## 6. Risiken & Fallstricke

- **Daten, nicht Modell, sind der Engpass.** Ohne repräsentative, gelabelte,
  zeit-getrennte Daten ist jedes Modell wertlos/irreführend.
- **Label-Leakage** durch zufälligen Split benachbarter Frames → zeit-/aufnahme-
  getrennt splitten.
- **Klassen-Imbalance** verzerrt Accuracy → Macro-F1 + gezieltes Sammeln seltener
  Klassen.
- **Open-Set**: das Modell darf nicht jedes Rauschen zwanghaft labeln →
  Rejection/Confidence-Schwelle als First-Class-Metrik.
- **Domain-Gap** öffentlicher Datensätze (RML) zur eigenen Hardware → Fine-Tuning.
- **Native Dep in Stufe B** widerspricht der PATH/DLL-Lehre aus Review §6 →
  Sidecar als Alternative ernsthaft prüfen.
- **Heuristik-Bias im Auto-Label**: Weak Supervision erbt die Fehler der
  Heuristik → Zweifelsfälle und Bandplan-gestützte Labels gegensteuern lassen,
  Test-Set rein manuell verifizieren.

---

## 7. Reihenfolge & Meilensteine (Zusammenfassung)

| Schritt | Inhalt | Ergebnis |
|--------|--------|----------|
| 0a | Classifier-Interface + `RuleBackend` | austauschbare Naht |
| 0b | Auto-Capture-Logging im DSP-Pfad | Datensatz wächst passiv |
| 0c | Auto-Label (Confidence + Bandplan) + Review-Queue (UI, Tastatur) | komfortables, schnelles Labeln |
| 0d | Eval-Harness (`go test`, Confusion-Matrix, Macro-F1, per-SNR) | **Heuristik-Baseline (B-5 erfüllt)** |
| A | LightGBM offline + Pure-Go `leaves`-Inferenz + Hybrid-Fusion | besser-als-Heuristik, messbar, kein DLL-Risiko |
| B | CNN auf IQ (PyTorch→ONNX), Runtime via ONNX-CGO oder Sidecar, GPU | höchste Genauigkeit |

**Wichtig:** Schritte 0a–0d sind die Voraussetzung für A und B. Kein Modell vor
Baseline. Stufe B erst nach Stufe A und ausreichendem Datensatz.

---

## 8. Querverweise
- `docs/architecture-review-2026-06-06.md` — Befund B-5 (Ursprung dieses Plans),
  Review §6 (PATH/DLL-Lehre, relevant für Inferenz-Runtime)
- `docs/known-issues.md` — kuratierte Einzelissues
- `internal/classifier/` — Features, `rules.go`, `context.go`, `Classification`
- `cmd/sdrd/dsp_loop.go` — Einhängepunkt für Auto-Capture (nach Refinement)
- `internal/recorder/` — vorhandene IQ-/meta-Persistenz
