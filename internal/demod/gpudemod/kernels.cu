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

extern "C" int gpud_launch_freq_shift_cuda(
    const float2* in,
    float2* out,
    int n,
    double phase_inc,
    double phase_start
) {
    if (n <= 0) return 0;
    const int block = 256;
    const int grid = (n + block - 1) / block;
    gpud_freq_shift_kernel<<<grid, block>>>(in, out, n, phase_inc, phase_start);
    return (int)cudaGetLastError();
}

extern "C" __global__ void gpud_fm_discrim_kernel(
    const float2* __restrict__ in,
    float* __restrict__ out,
    int n
) {
    int idx = blockIdx.x * blockDim.x + threadIdx.x;
    if (idx >= n - 1) return;

    float2 prev = in[idx];
    float2 curr = in[idx + 1];
    float re = prev.x * curr.x + prev.y * curr.y;
    float im = prev.x * curr.y - prev.y * curr.x;
    out[idx] = atan2f(im, re);
}

extern "C" int gpud_launch_fm_discrim_cuda(
    const float2* in,
    float* out,
    int n
) {
    if (n <= 1) return 0;
    const int block = 256;
    const int grid = (n + block - 1) / block;
    gpud_fm_discrim_kernel<<<grid, block>>>(in, out, n);
    return (int)cudaGetLastError();
}
