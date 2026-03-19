# CUDA Build Strategy

## Problem statement

The repository currently mixes two Windows toolchain worlds:

- Go/CGO final link often goes through MinGW GCC/LD
- CUDA kernel compilation via `nvcc` on Windows prefers MSVC (`cl.exe`)

This works for isolated package tests, but full application builds can fail when an MSVC-built CUDA library is linked by MinGW, producing unresolved symbols such as:

- `__GSHandlerCheck`
- `__security_cookie`
- `_Init_thread_epoch`

## Recommended split

### Windows

Use an explicitly Windows-oriented build path:

1. Prepare CUDA kernel artifacts with `nvcc`
2. Keep the resulting CUDA linkage path clearly separated from MinGW-based fallback builds
3. Do not assume that a MinGW-linked Go binary can always consume MSVC-built CUDA archives

### Linux

Prefer a GCC/NVCC-oriented build path:

1. Build CUDA kernels with `nvcc` + GCC
2. Link through the normal Linux CGO flow
3. Avoid Windows-specific import-lib and MSVC runtime assumptions entirely

## Repository design guidance

- Keep `internal/demod/gpudemod/` platform-neutral at the Go API level
- Keep CUDA kernels in `kernels.cu`
- Use OS-specific build scripts for orchestration
- Avoid embedding Windows-only build assumptions into shared Go code when possible

## Current practical status

- `go test ./...` passes
- `go test -tags cufft ./internal/demod/gpudemod` passes with NVCC/MSVC setup
- `build-sdrplay.ps1` has progressed past the original invalid `#cgo LDFLAGS` issue
- Remaining Windows blocker is now a toolchain mismatch between MSVC-built CUDA artifacts and MinGW final linking
