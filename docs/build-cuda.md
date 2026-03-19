# CUDA Build Strategy

## Windows: gpudemod DLL split

The recommended Windows CUDA path is now a DLL split for `gpudemod`:

1. Build `internal/demod/gpudemod/native/exports.cu` into `gpudemod_kernels.dll` using `nvcc` + MSVC
2. Build the Go app with MinGW GCC/G++ via CGO
3. Load `gpudemod_kernels.dll` at runtime from Go on Windows

This avoids direct static linking of MSVC-built CUDA objects into the MinGW-linked Go binary.

## Why

The previous failing paths mixed incompatible toolchains at final link time:
- MSVC-host CUDA object/library generation
- MinGW GCC/LD for the Go executable

The DLL split keeps that boundary at runtime instead of link time.

## Current Windows build flow

```powershell
powershell -ExecutionPolicy Bypass -File .\build-cuda-windows.ps1
powershell -ExecutionPolicy Bypass -File .\build-sdrplay.ps1
```

## Runtime expectation

`gpudemod_kernels.dll` must be available either:
- next to `sdrd.exe`, or
- in `internal/demod/gpudemod/build/` during local runs from the repo

The Windows `gpudemod` loader searches both locations.

## Linux

Linux remains the simpler direct-link path and still avoids the Windows mixed-toolchain problem entirely.
