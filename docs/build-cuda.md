# CUDA Build Strategy

## Windows: MinGW-host NVCC path

The recommended Windows CUDA build path for this repository is:

1. Compile `internal/demod/gpudemod/kernels.cu` with `nvcc` using MinGW `g++` as the host compiler
2. Archive the result as `internal/demod/gpudemod/build/libgpudemod_kernels.a`
3. Build the Go app with MinGW GCC/G++ via CGO

This keeps the CUDA demod kernel library in a GNU-compatible format so Go's MinGW CGO linker can consume it.

### Why

The previous failing path mixed:
- `nvcc` + default MSVC host compiler (`cl.exe`) for CUDA kernels
- MinGW GCC/LD for the final Go/CGO link

That produced unresolved MSVC runtime symbols such as:
- `__GSHandlerCheck`
- `__security_cookie`
- `_Init_thread_epoch`

### Current Windows build flow

```powershell
powershell -ExecutionPolicy Bypass -File .\build-cuda-windows.ps1
powershell -ExecutionPolicy Bypass -File .\build-sdrplay.ps1
```

### Critical details

- CUDA kernel archive must be named `libgpudemod_kernels.a`
- `nvcc` must be invoked with `-ccbin C:\msys64\mingw64\bin\g++.exe`
- Windows CGO link uses:
  - SDRplay API import lib
  - MinGW CUDA import libs from `cuda-mingw/`
  - `-lgpudemod_kernels`
  - `-lcufft64_12`
  - `-lcudart64_13`
  - `-lstdc++`

### Caveat

`nvcc` + MinGW on Windows is not officially supported by NVIDIA. For the kernel launcher style used here (`extern "C"` functions, limited host C++ surface), it is the most practical path.

CUDA 13.x also drops older GPU targets such as `sm_50` and `sm_60`, so the kernel build script targets `sm_75+`.

## Linux

Linux remains the cleanest end-to-end CUDA path:

1. Build CUDA kernels with `nvcc` + GCC
2. Link via standard CGO/GCC flow
3. Avoid Windows toolchain mismatch entirely
