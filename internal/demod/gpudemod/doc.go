// Package gpudemod contains the CUDA-tagged demodulation pipeline scaffolding.
//
// Current state:
//   - Standard builds use the !cufft stub.
//   - cufft builds allocate GPU buffers and cross the CGO/CUDA launch boundary.
//   - If/when a CUDA freq-shift launch succeeds, the shifted IQ is copied back and
//     reused by the remaining CPU-side FIR/decimate/NFM pipeline.
//
// This keeps Phase 1 incremental and verifiable while later phases replace the
// placeholder launch wrappers with real kernels.
package gpudemod
