#include <cuda_runtime.h>
#include <math.h>

extern "C" __global__ void gpud_freq_shift_kernel(
    const float2* __restrict__ in,
    float2* __restrict__ out,
    int n,
    double phase_inc,
    double phase_start
) {
    int idx = blockIdx.x * blockDim.x + threadIdx.x;
    if (idx >= n) return;

    double phase = phase_start + phase_inc * (double)idx;
    float si, co;
    sincosf((float)phase, &si, &co);

    float2 v = in[idx];
    out[idx].x = v.x * co - v.y * si;
    out[idx].y = v.x * si + v.y * co;
}
