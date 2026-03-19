# gpudemod

Phase 1 CUDA demod scaffolding.

## Current state

- Standard Go builds use `gpudemod_stub.go` (`!cufft`).
- `cufft` builds allocate GPU buffers and cross the CGO/CUDA launch boundary.
- If CUDA launch wrappers are not backed by compiled kernels yet, the code falls back to CPU DSP.
- The shifted IQ path is already wired so a successful GPU freq-shift result can be copied back and reused immediately.
- Build orchestration should now be considered OS-specific; see `docs/build-cuda.md`.

## First real kernel

`kernels.cu` contains the first candidate implementation:
- `gpud_freq_shift_kernel`

This is **not compiled automatically yet** in the current environment because the machine currently lacks a CUDA compiler toolchain in PATH (`nvcc` not found).

## Next machine-side step

On a CUDA-capable dev machine with toolchain installed:

1. Compile `kernels.cu` into an object file and archive it into a linkable library
   - helper script: `tools/build-gpudemod-kernel.ps1`
2. On Jan's Windows machine, the working kernel-build path currently relies on `nvcc` + MSVC `cl.exe` in PATH
3. Link `gpudemod_kernels.lib` into the `cufft` build
3. Replace `gpud_launch_freq_shift(...)` stub body with the real kernel launch
4. Validate copied-back shifted IQ against `dsp.FreqShift`
5. Only then move the next stage (FM discriminator) onto the GPU

## Why this is still useful

The runtime/buffer/recorder/fallback structure is already in place, so once kernel compilation is available, real acceleration can be inserted without another architecture rewrite.
