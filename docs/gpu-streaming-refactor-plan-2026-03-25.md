# GPU Streaming Refactor Plan (2026-03-25)

## Goal
Replace the current overlap+trim GPU extractor model with a true stateful per-signal streaming architecture, and build a corrected CPU oracle/reference path for validation.

## Non-negotiables
- No production start-index-only patch.
- No production overlap-prepend + trim continuity model.
- Exact integer decimation only in the new streaming production path.
- Persistent per-signal state must include NCO phase, FIR history, and decimator phase/residue.
- GPU validation must compare against a corrected CPU oracle, not the legacy CPU fallback.

## Work order
1. Introduce explicit stateful streaming types in `gpudemod`.
2. Add a clean CPU oracle implementation and monolithic-vs-chunked tests.
3. Add per-signal state ownership in batch runner.
4. Implement new streaming extractor semantics in Go using NEW IQ samples only.
5. Replace legacy GPU-path assumptions (rounding decimation, overlap-prepend, trim-defined validity) in the new path.
6. Add production telemetry that proves state continuity (`phase_count`, `history_len`, `n_out`, reference error).
7. Keep legacy path isolated only for temporary comparison if needed.

## Initial files in scope
- `internal/demod/gpudemod/batch.go`
- `internal/demod/gpudemod/batch_runner.go`
- `internal/demod/gpudemod/batch_runner_windows.go`
- `internal/demod/gpudemod/kernels.cu`
- `internal/demod/gpudemod/native/exports.cu`
- `cmd/sdrd/helpers.go`

## Immediate implementation strategy
### Phase 1
- Create explicit streaming state structs in Go.
- Add CPU oracle/reference path with exact semantics and tests.
- Introduce exact integer-decimation checks.

### Phase 2
- Rework batch runner to own persistent per-signal state.
- Add config-hash-based resets.
- Stop modeling continuity via overlap tail in the new path.

### Phase 3
- Introduce a real streaming GPU entry path that consumes NEW shifted samples plus carried state.
- Move to a stateful polyphase decimator model.

## Validation expectations
- CPU oracle monolithic == CPU oracle chunked within tolerance.
- GPU streaming output == CPU oracle chunked within tolerance.
- Former periodic block-boundary clicks gone in real-world testing.
