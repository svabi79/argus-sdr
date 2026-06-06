# Detection Baseline — 2026-06-06

Quantified baseline of the **current** detection/estimation pipeline against the
synthetic ground-truth benchmark (Phase R, step R0.5). This is the number every
later step (R1–R3) must beat.

Reproduce:
```
go test -tags bench -run TestDetectionBaseline ./internal/synth/ -v
```
Setup: Fs = 2.5 MHz, FFT = 8192 (binWidth ≈ 305 Hz), 8 truth signals/scene,
6 frames, detector config = shipped `config.yaml` defaults (GOSCA CFAR).

## Detection / aggregate

| SNR dB | detected | precision | recall | bw MAE % | bw median % | center MAE Hz |
|-------:|---------:|----------:|-------:|---------:|------------:|--------------:|
| 5      | 11       | 0.64      | 0.88   | 40.7     | 29.7        | 612 |
| 10     | 11       | 0.64      | 0.88   | 37.3     | 22.1        | 524 |
| 20     | 12       | 0.67      | 1.00   | 258.9    | 62.8        | 298 |
| 30     | 12       | 0.67      | 1.00   | 187.1    | 52.6        | 261 |

(8 truth signals; bw MAE at 20/30 dB is inflated by the sub-bin CW signal once it
is detected — see per-kind table; the median is the more robust summary.)

## Per-kind bandwidth error @ 30 dB

| kind    | truth BW | detected BW | error % |
|---------|---------:|------------:|--------:|
| WFM     | 180000   | 34180       | 81.0 (under) |
| WFM     | 180000   | 34790       | 80.7 (under) |
| NFM     | 12000    | 17395       | 45.0 (over)  |
| NFM     | 12000    | 17090       | 42.4 (over)  |
| AM      | 8000     | 10071       | 25.9 (over)  |
| SSB     | 3000     | 4578        | 52.6 (over)  |
| CW      | 100      | 3357        | 3256.9 (sub-bin) |
| DIGITAL | 25000    | 33875       | 35.5 (over)  |

## Reading

- **Over-detection.** 8 truth → 11–12 detections; precision ~0.65. The wideband
  WFM fragments into multiple sub-peaks at a single resolution → false extras.
  (OI-21: single-resolution detection is not universal.)
- **Bandwidth is unreliable and biased per kind.** WFM is massively *under*-measured
  (−81 %: the −threshold width is far narrower than the Carson bandwidth), while
  narrowband signals are *over*-measured (+25…+53 %, from edge-walk + merge gap).
  This is the geometric-bandwidth weakness (OI-20: refinement never re-estimates).
- **Center** is reasonable (~260–610 Hz, i.e. 1–2 bins) — the power-weighted
  centroid already works; bandwidth is the broken estimate.
- **Recall** is good at ≥20 dB but drops at low SNR — R2 (Welch PSD + robust noise)
  should help here.

## Targets for the rework (to be beaten on this same benchmark)

- **R1** (occupied-bandwidth re-estimation): bw median error well below the current
  20–60 %; WFM no longer −80 %.
- **R3** (multi-resolution detection): precision toward ~1.0 (no WFM fragmentation);
  detected count ≈ truth count across the bandwidth range.
- **R2**: recall at 5 dB toward parity with the ≥20 dB case.

These are directional; exact pass thresholds get fixed once R1/R2/R3 land and we see
the achievable numbers.
