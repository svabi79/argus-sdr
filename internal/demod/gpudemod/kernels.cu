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

extern "C" __global__ void gpud_decimate_kernel(
    const float2* __restrict__ in,
    float2* __restrict__ out,
    int n_out,
    int factor
) {
    int idx = blockIdx.x * blockDim.x + threadIdx.x;
    if (idx >= n_out) return;
    out[idx] = in[idx * factor];
}

extern "C" __constant__ float gpud_fir_taps[256];

extern "C" __global__ void gpud_fir_kernel(
    const float2* __restrict__ in,
    float2* __restrict__ out,
    int n,
    int num_taps
) {
    int idx = blockIdx.x * blockDim.x + threadIdx.x;
    if (idx >= n) return;

    float acc_r = 0.0f;
    float acc_i = 0.0f;
    for (int k = 0; k < num_taps; ++k) {
        int src = idx - k;
        if (src < 0) break;
        float2 v = in[src];
        float t = gpud_fir_taps[k];
        acc_r += v.x * t;
        acc_i += v.y * t;
    }
    out[idx] = make_float2(acc_r, acc_i);
}

extern "C" int gpud_upload_fir_taps_cuda(const float* taps, int n) {
    if (!taps || n <= 0 || n > 256) return -1;
    cudaError_t err = cudaMemcpyToSymbol(gpud_fir_taps, taps, (size_t)n * sizeof(float));
    return (int)err;
}

extern "C" int gpud_launch_fir_cuda(
    const float2* in,
    float2* out,
    int n,
    int num_taps
) {
    if (n <= 0 || num_taps <= 0 || num_taps > 256) return 0;
    const int block = 256;
    const int grid = (n + block - 1) / block;
    gpud_fir_kernel<<<grid, block>>>(in, out, n, num_taps);
    return (int)cudaGetLastError();
}

extern "C" int gpud_launch_decimate_cuda(
    const float2* in,
    float2* out,
    int n_out,
    int factor
) {
    if (n_out <= 0 || factor <= 0) return 0;
    const int block = 256;
    const int grid = (n_out + block - 1) / block;
    gpud_decimate_kernel<<<grid, block>>>(in, out, n_out, factor);
    return (int)cudaGetLastError();
}
