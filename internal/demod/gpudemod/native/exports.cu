#include <cuda_runtime.h>
#include <math.h>

#if defined(_WIN32)
#define GPUD_API extern "C" __declspec(dllexport)
#define GPUD_CALL __stdcall
#else
#define GPUD_API extern "C"
#define GPUD_CALL
#endif

typedef void* gpud_stream_handle;

GPUD_API int GPUD_CALL gpud_stream_create(gpud_stream_handle* out) {
    if (!out) return -1;
    cudaStream_t stream;
    cudaError_t err = cudaStreamCreate(&stream);
    if (err != cudaSuccess) return (int)err;
    *out = (gpud_stream_handle)stream;
    return 0;
}

GPUD_API int GPUD_CALL gpud_stream_destroy(gpud_stream_handle stream) {
    if (!stream) return 0;
    return (int)cudaStreamDestroy((cudaStream_t)stream);
}

GPUD_API int GPUD_CALL gpud_stream_sync(gpud_stream_handle stream) {
    if (!stream) return (int)cudaDeviceSynchronize();
    return (int)cudaStreamSynchronize((cudaStream_t)stream);
}

__global__ void gpud_freq_shift_kernel(
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

GPUD_API int GPUD_CALL gpud_launch_freq_shift_stream_cuda(
    const float2* in,
    float2* out,
    int n,
    double phase_inc,
    double phase_start,
    gpud_stream_handle stream
) {
    if (n <= 0) return 0;
    const int block = 256;
    const int grid = (n + block - 1) / block;
    gpud_freq_shift_kernel<<<grid, block, 0, (cudaStream_t)stream>>>(in, out, n, phase_inc, phase_start);
    return (int)cudaGetLastError();
}

GPUD_API int GPUD_CALL gpud_launch_freq_shift_cuda(
    const float2* in,
    float2* out,
    int n,
    double phase_inc,
    double phase_start
) {
    return gpud_launch_freq_shift_stream_cuda(in, out, n, phase_inc, phase_start, 0);
}

__global__ void gpud_fm_discrim_kernel(
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

GPUD_API int GPUD_CALL gpud_launch_fm_discrim_cuda(
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

__global__ void gpud_decimate_kernel(
    const float2* __restrict__ in,
    float2* __restrict__ out,
    int n_out,
    int factor
) {
    int idx = blockIdx.x * blockDim.x + threadIdx.x;
    if (idx >= n_out) return;
    out[idx] = in[idx * factor];
}

__device__ __constant__ float gpud_fir_taps[256];

__global__ void gpud_fir_kernel(
    const float2* __restrict__ in,
    float2* __restrict__ out,
    int n,
    int num_taps
) {
    extern __shared__ float2 s_data[];
    int gid = blockIdx.x * blockDim.x + threadIdx.x;
    int lid = threadIdx.x;
    int halo = num_taps - 1;

    if (gid < n) {
        s_data[lid + halo] = in[gid];
    } else {
        s_data[lid + halo] = make_float2(0.0f, 0.0f);
    }

    if (lid < halo) {
        int src = gid - halo;
        s_data[lid] = (src >= 0) ? in[src] : make_float2(0.0f, 0.0f);
    }
    __syncthreads();

    if (gid >= n) return;

    float acc_r = 0.0f;
    float acc_i = 0.0f;
    for (int k = 0; k < num_taps; ++k) {
        float2 v = s_data[lid + halo - k];
        float t = gpud_fir_taps[k];
        acc_r += v.x * t;
        acc_i += v.y * t;
    }
    out[gid] = make_float2(acc_r, acc_i);
}

GPUD_API int GPUD_CALL gpud_upload_fir_taps_cuda(const float* taps, int n) {
    if (!taps || n <= 0 || n > 256) return -1;
    cudaError_t err = cudaMemcpyToSymbol(gpud_fir_taps, taps, (size_t)n * sizeof(float));
    return (int)err;
}

GPUD_API int GPUD_CALL gpud_launch_fir_cuda(
    const float2* in,
    float2* out,
    int n,
    int num_taps
) {
    if (n <= 0 || num_taps <= 0 || num_taps > 256) return 0;
    const int block = 256;
    const int grid = (n + block - 1) / block;
    size_t sharedBytes = (size_t)(block + num_taps - 1) * sizeof(float2);
    gpud_fir_kernel<<<grid, block, sharedBytes>>>(in, out, n, num_taps);
    return (int)cudaGetLastError();
}

GPUD_API int GPUD_CALL gpud_launch_fir_stream_cuda(
    const float2* in,
    float2* out,
    int n,
    int num_taps,
    gpud_stream_handle stream
) {
    if (n <= 0 || num_taps <= 0 || num_taps > 256) return 0;
    const int block = 256;
    const int grid = (n + block - 1) / block;
    size_t sharedBytes = (size_t)(block + num_taps - 1) * sizeof(float2);
    gpud_fir_kernel<<<grid, block, sharedBytes, (cudaStream_t)stream>>>(in, out, n, num_taps);
    return (int)cudaGetLastError();
}

__global__ void gpud_fir_kernel_v2(
    const float2* __restrict__ in,
    float2* __restrict__ out,
    const float* __restrict__ taps,
    int n,
    int num_taps
) {
    extern __shared__ float2 s_data[];
    int gid = blockIdx.x * blockDim.x + threadIdx.x;
    int lid = threadIdx.x;
    int halo = num_taps - 1;

    if (gid < n) s_data[lid + halo] = in[gid];
    else s_data[lid + halo] = make_float2(0.0f, 0.0f);

    if (lid < halo) {
        int src = gid - halo;
        s_data[lid] = (src >= 0) ? in[src] : make_float2(0.0f, 0.0f);
    }
    __syncthreads();
    if (gid >= n) return;

    float acc_r = 0.0f, acc_i = 0.0f;
    for (int k = 0; k < num_taps; ++k) {
        float2 v = s_data[lid + halo - k];
        float t = taps[k];
        acc_r += v.x * t;
        acc_i += v.y * t;
    }
    out[gid] = make_float2(acc_r, acc_i);
}

GPUD_API int GPUD_CALL gpud_launch_fir_v2_stream_cuda(
    const float2* in,
    float2* out,
    const float* taps,
    int n,
    int num_taps,
    gpud_stream_handle stream
) {
    if (n <= 0 || num_taps <= 0 || num_taps > 256) return 0;
    const int block = 256;
    const int grid = (n + block - 1) / block;
    size_t sharedBytes = (size_t)(block + num_taps - 1) * sizeof(float2);
    gpud_fir_kernel_v2<<<grid, block, sharedBytes, (cudaStream_t)stream>>>(in, out, taps, n, num_taps);
    return (int)cudaGetLastError();
}

GPUD_API int GPUD_CALL gpud_launch_decimate_cuda(
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

GPUD_API int GPUD_CALL gpud_launch_decimate_stream_cuda(
    const float2* in,
    float2* out,
    int n_out,
    int factor,
    gpud_stream_handle stream
) {
    if (n_out <= 0 || factor <= 0) return 0;
    const int block = 256;
    const int grid = (n_out + block - 1) / block;
    gpud_decimate_kernel<<<grid, block, 0, (cudaStream_t)stream>>>(in, out, n_out, factor);
    return (int)cudaGetLastError();
}

__global__ void gpud_am_envelope_kernel(
    const float2* __restrict__ in,
    float* __restrict__ out,
    int n
) {
    int idx = blockIdx.x * blockDim.x + threadIdx.x;
    if (idx >= n) return;
    float2 v = in[idx];
    out[idx] = sqrtf(v.x * v.x + v.y * v.y);
}

GPUD_API int GPUD_CALL gpud_launch_am_envelope_cuda(
    const float2* in,
    float* out,
    int n
) {
    if (n <= 0) return 0;
    const int block = 256;
    const int grid = (n + block - 1) / block;
    gpud_am_envelope_kernel<<<grid, block>>>(in, out, n);
    return (int)cudaGetLastError();
}

__global__ void gpud_ssb_product_kernel(
    const float2* __restrict__ in,
    float* __restrict__ out,
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
    out[idx] = v.x * co - v.y * si;
}

GPUD_API int GPUD_CALL gpud_launch_ssb_product_cuda(
    const float2* in,
    float* out,
    int n,
    double phase_inc,
    double phase_start
) {
    if (n <= 0) return 0;
    const int block = 256;
    const int grid = (n + block - 1) / block;
    gpud_ssb_product_kernel<<<grid, block>>>(in, out, n, phase_inc, phase_start);
    return (int)cudaGetLastError();
}
