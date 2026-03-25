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

static __forceinline__ int gpud_max_i(int a, int b) {
    return a > b ? a : b;
}

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
    // Reduce phase to [-pi, pi) BEFORE float cast to preserve precision.
    // Without this, phase accumulates to millions of radians and the
    // (float) cast loses ~0.03-0.1 rad, causing audible clicks at
    // frame boundaries in streaming audio.
    const double TWO_PI = 6.283185307179586;
    phase = phase - rint(phase / TWO_PI) * TWO_PI;
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
    const double TWO_PI = 6.283185307179586;
    phase = phase - rint(phase / TWO_PI) * TWO_PI;
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

__global__ void gpud_streaming_polyphase_accum_kernel(
    const float2* __restrict__ history_state,
    int history_len,
    const float2* __restrict__ shifted_new,
    int n_new,
    const float* __restrict__ polyphase_taps,
    int polyphase_len,
    int decim,
    int phase_len,
    int start_idx,
    int n_out,
    float2* __restrict__ out
);

__global__ void gpud_streaming_history_tail_kernel(
    const float2* __restrict__ history_state,
    int history_len,
    const float2* __restrict__ shifted_new,
    int n_new,
    int keep,
    float2* __restrict__ history_out
);

static __forceinline__ double gpud_reduce_phase(double phase);

// Transitional legacy entrypoint retained for bring-up and comparison.
// The production-native streaming path is gpud_launch_streaming_polyphase_stateful_cuda,
// which preserves per-signal carry state across NEW-samples-only chunks.
GPUD_API int GPUD_CALL gpud_launch_streaming_polyphase_prepare_cuda(
    const float2* in_new,
    int n_new,
    const float2* history_in,
    int history_len,
    const float* polyphase_taps,
    int polyphase_len,
    int decim,
    int num_taps,
    int phase_count_in,
    double phase_start,
    double phase_inc,
    float2* out,
    int* n_out,
    int* phase_count_out,
    double* phase_end_out,
    float2* history_out
) {
    if (n_new < 0 || !polyphase_taps || polyphase_len <= 0 || decim <= 0 || num_taps <= 0) return -1;
    const int phase_len = (num_taps + decim - 1) / decim;
    if (polyphase_len < decim * phase_len) return -2;

    const int keep = num_taps > 1 ? num_taps - 1 : 0;
    int clamped_history_len = history_len;
    if (clamped_history_len < 0) clamped_history_len = 0;
    if (clamped_history_len > keep) clamped_history_len = keep;
    if (clamped_history_len > 0 && !history_in) return -5;

    float2* shifted = NULL;
    cudaError_t err = cudaSuccess;
    if (n_new > 0) {
        if (!in_new) return -3;
        err = cudaMalloc((void**)&shifted, (size_t)gpud_max_i(1, n_new) * sizeof(float2));
        if (err != cudaSuccess) return (int)err;
        const int block = 256;
        const int grid_shift = (n_new + block - 1) / block;
        gpud_freq_shift_kernel<<<grid_shift, block>>>(in_new, shifted, n_new, phase_inc, phase_start);
        err = cudaGetLastError();
        if (err != cudaSuccess) {
            cudaFree(shifted);
            return (int)err;
        }
    }

    int phase_count = phase_count_in;
    if (phase_count < 0) phase_count = 0;
    if (phase_count >= decim) phase_count %= decim;
    const int total_phase = phase_count + n_new;
    const int out_count = total_phase / decim;
    if (out_count > 0) {
        if (!out) {
            cudaFree(shifted);
            return -4;
        }
        const int block = 256;
        const int grid = (out_count + block - 1) / block;
        const int start_idx = decim - phase_count - 1;
        gpud_streaming_polyphase_accum_kernel<<<grid, block>>>(
            history_in,
            clamped_history_len,
            shifted,
            n_new,
            polyphase_taps,
            polyphase_len,
            decim,
            phase_len,
            start_idx,
            out_count,
            out
        );
        err = cudaGetLastError();
        if (err != cudaSuccess) {
            cudaFree(shifted);
            return (int)err;
        }
    }

    if (history_out && keep > 0) {
        const int new_history_len = clamped_history_len + n_new < keep ? clamped_history_len + n_new : keep;
        if (new_history_len > 0) {
            const int block = 256;
            const int grid = (new_history_len + block - 1) / block;
            gpud_streaming_history_tail_kernel<<<grid, block>>>(
                history_in,
                clamped_history_len,
                shifted,
                n_new,
                new_history_len,
                history_out
            );
            err = cudaGetLastError();
            if (err != cudaSuccess) {
                cudaFree(shifted);
                return (int)err;
            }
        }
    }

    if (n_out) *n_out = out_count;
    if (phase_count_out) *phase_count_out = total_phase % decim;
    if (phase_end_out) *phase_end_out = gpud_reduce_phase(phase_start + phase_inc * (double)n_new);

    if (shifted) cudaFree(shifted);
    return 0;
}

static __device__ __forceinline__ float2 gpud_stream_sample_at(
    const float2* __restrict__ history_state,
    int history_len,
    const float2* __restrict__ shifted_new,
    int n_new,
    int idx
) {
    if (idx < 0) return make_float2(0.0f, 0.0f);
    if (idx < history_len) return history_state[idx];
    int shifted_idx = idx - history_len;
    if (shifted_idx < 0 || shifted_idx >= n_new) return make_float2(0.0f, 0.0f);
    return shifted_new[shifted_idx];
}

__global__ void gpud_streaming_polyphase_accum_kernel(
    const float2* __restrict__ history_state,
    int history_len,
    const float2* __restrict__ shifted_new,
    int n_new,
    const float* __restrict__ polyphase_taps,
    int polyphase_len,
    int decim,
    int phase_len,
    int start_idx,
    int n_out,
    float2* __restrict__ out
) {
    int out_idx = blockIdx.x * blockDim.x + threadIdx.x;
    if (out_idx >= n_out) return;

    int newest = history_len + start_idx + out_idx * decim;
    float acc_r = 0.0f;
    float acc_i = 0.0f;
    for (int p = 0; p < decim; ++p) {
        for (int k = 0; k < phase_len; ++k) {
            int tap_idx = p * phase_len + k;
            if (tap_idx >= polyphase_len) continue;
            float tap = polyphase_taps[tap_idx];
            if (tap == 0.0f) continue;
            int src_back = p + k * decim;
            int src_idx = newest - src_back;
            float2 sample = gpud_stream_sample_at(history_state, history_len, shifted_new, n_new, src_idx);
            acc_r += sample.x * tap;
            acc_i += sample.y * tap;
        }
    }
    out[out_idx] = make_float2(acc_r, acc_i);
}

__global__ void gpud_streaming_history_tail_kernel(
    const float2* __restrict__ history_state,
    int history_len,
    const float2* __restrict__ shifted_new,
    int n_new,
    int keep,
    float2* __restrict__ history_out
) {
    int idx = blockIdx.x * blockDim.x + threadIdx.x;
    if (idx >= keep) return;
    int combined_len = history_len + n_new;
    int src_idx = combined_len - keep + idx;
    history_out[idx] = gpud_stream_sample_at(history_state, history_len, shifted_new, n_new, src_idx);
}

static __forceinline__ double gpud_reduce_phase(double phase) {
    const double TWO_PI = 6.283185307179586;
    return phase - rint(phase / TWO_PI) * TWO_PI;
}

// Production-native candidate entrypoint for the stateful streaming extractor.
// Callers provide only NEW samples; overlap+trim is intentionally not part of this path.
GPUD_API int GPUD_CALL gpud_launch_streaming_polyphase_stateful_cuda(
    const float2* in_new,
    int n_new,
    float2* shifted_new_tmp,
    const float* polyphase_taps,
    int polyphase_len,
    int decim,
    int num_taps,
    float2* history_state,
    float2* history_scratch,
    int history_cap,
    int* history_len_io,
    int* phase_count_state,
    double* phase_state,
    double phase_inc,
    float2* out,
    int out_cap,
    int* n_out
) {
    if (!polyphase_taps || decim <= 0 || num_taps <= 0 || !history_len_io || !phase_count_state || !phase_state || !n_out) return -10;
    if (n_new < 0 || out_cap < 0 || history_cap < 0) return -11;
    const int phase_len = (num_taps + decim - 1) / decim;
    if (polyphase_len < decim * phase_len) return -12;

    int history_len = *history_len_io;
    if (history_len < 0) history_len = 0;
    if (history_len > history_cap) history_len = history_cap;

    int phase_count = *phase_count_state;
    if (phase_count < 0) phase_count = 0;
    if (phase_count >= decim) phase_count %= decim;

    double phase_start = *phase_state;
    if (n_new > 0) {
        if (!in_new || !shifted_new_tmp) return -13;
        const int block = 256;
        const int grid = (n_new + block - 1) / block;
        gpud_freq_shift_kernel<<<grid, block>>>(in_new, shifted_new_tmp, n_new, phase_inc, phase_start);
        cudaError_t err = cudaGetLastError();
        if (err != cudaSuccess) return (int)err;
    }

    const int total_phase = phase_count + n_new;
    const int out_count = total_phase / decim;
    if (out_count > out_cap) return -14;

    if (out_count > 0) {
        if (!out) return -15;
        const int block = 256;
        const int grid = (out_count + block - 1) / block;
        const int start_idx = decim - phase_count - 1;
        gpud_streaming_polyphase_accum_kernel<<<grid, block>>>(
            history_state,
            history_len,
            shifted_new_tmp,
            n_new,
            polyphase_taps,
            polyphase_len,
            decim,
            phase_len,
            start_idx,
            out_count,
            out
        );
        cudaError_t err = cudaGetLastError();
        if (err != cudaSuccess) return (int)err;
    }

    int new_history_len = history_len;
    if (history_cap > 0) {
        new_history_len = history_len + n_new;
        if (new_history_len > history_cap) new_history_len = history_cap;
        if (new_history_len > 0) {
            if (!history_state || !history_scratch) return -16;
            const int block = 256;
            const int grid = (new_history_len + block - 1) / block;
            gpud_streaming_history_tail_kernel<<<grid, block>>>(
                history_state,
                history_len,
                shifted_new_tmp,
                n_new,
                new_history_len,
                history_scratch
            );
            cudaError_t err = cudaGetLastError();
            if (err != cudaSuccess) return (int)err;
            err = cudaMemcpy(history_state, history_scratch, (size_t)new_history_len * sizeof(float2), cudaMemcpyDeviceToDevice);
            if (err != cudaSuccess) return (int)err;
        }
    } else {
        new_history_len = 0;
    }

    *history_len_io = new_history_len;
    *phase_count_state = total_phase % decim;
    *phase_state = gpud_reduce_phase(phase_start + phase_inc * (double)n_new);
    *n_out = out_count;
    return 0;
}
