//go:build cufft && windows

package gpudemod

/*
#cgo windows CFLAGS: -I"C:/Program Files/NVIDIA GPU Computing Toolkit/CUDA/v13.2/include"
#cgo windows LDFLAGS: -L"C:/Program Files/NVIDIA GPU Computing Toolkit/CUDA/v13.2/bin/x64" -l:cudart64_13.dll -lkernel32
#include <windows.h>
#include <stdlib.h>
#include <cuda_runtime.h>

typedef struct { float x; float y; } gpud_float2;
typedef void* gpud_stream_handle;

typedef int (__stdcall *gpud_stream_create_fn)(gpud_stream_handle* out);
typedef int (__stdcall *gpud_stream_destroy_fn)(gpud_stream_handle stream);
typedef int (__stdcall *gpud_stream_sync_fn)(gpud_stream_handle stream);
typedef int (__stdcall *gpud_upload_fir_taps_fn)(const float* taps, int n);
typedef int (__stdcall *gpud_launch_freq_shift_stream_fn)(const gpud_float2* in, gpud_float2* out, int n, double phase_inc, double phase_start, gpud_stream_handle stream);
typedef int (__stdcall *gpud_launch_freq_shift_fn)(const gpud_float2* in, gpud_float2* out, int n, double phase_inc, double phase_start);
typedef int (__stdcall *gpud_launch_fm_discrim_fn)(const gpud_float2* in, float* out, int n);
typedef int (__stdcall *gpud_launch_fir_stream_fn)(const gpud_float2* in, gpud_float2* out, int n, int num_taps, gpud_stream_handle stream);
typedef int (__stdcall *gpud_launch_fir_v2_stream_fn)(const gpud_float2* in, gpud_float2* out, const float* taps, int n, int num_taps, gpud_stream_handle stream);
typedef int (__stdcall *gpud_launch_fir_fn)(const gpud_float2* in, gpud_float2* out, int n, int num_taps);
typedef int (__stdcall *gpud_launch_decimate_stream_fn)(const gpud_float2* in, gpud_float2* out, int n_out, int factor, gpud_stream_handle stream);
typedef int (__stdcall *gpud_launch_decimate_fn)(const gpud_float2* in, gpud_float2* out, int n_out, int factor);
typedef int (__stdcall *gpud_launch_am_envelope_fn)(const gpud_float2* in, float* out, int n);
typedef int (__stdcall *gpud_launch_ssb_product_fn)(const gpud_float2* in, float* out, int n, double phase_inc, double phase_start);
typedef int (__stdcall *gpud_launch_streaming_polyphase_prepare_fn)(const gpud_float2* in_new, int n_new, const gpud_float2* history_in, int history_len, const float* polyphase_taps, int polyphase_len, int decim, int num_taps, int phase_count_in, double phase_start, double phase_inc, gpud_float2* out, int* n_out, int* phase_count_out, double* phase_end_out, gpud_float2* history_out);
typedef int (__stdcall *gpud_launch_streaming_polyphase_stateful_fn)(const gpud_float2* in_new, int n_new, gpud_float2* shifted_new_tmp, const float* polyphase_taps, int polyphase_len, int decim, int num_taps, gpud_float2* history_state, gpud_float2* history_scratch, int history_cap, int* history_len_io, int* phase_count_state, double* phase_state, double phase_inc, gpud_float2* out, int out_cap, int* n_out);

static HMODULE gpud_mod = NULL;
static gpud_stream_create_fn gpud_p_stream_create = NULL;
static gpud_stream_destroy_fn gpud_p_stream_destroy = NULL;
static gpud_stream_sync_fn gpud_p_stream_sync = NULL;
static gpud_upload_fir_taps_fn gpud_p_upload_fir_taps = NULL;
static gpud_launch_freq_shift_stream_fn gpud_p_launch_freq_shift_stream = NULL;
static gpud_launch_freq_shift_fn gpud_p_launch_freq_shift = NULL;
static gpud_launch_fm_discrim_fn gpud_p_launch_fm_discrim = NULL;
static gpud_launch_fir_stream_fn gpud_p_launch_fir_stream = NULL;
static gpud_launch_fir_v2_stream_fn gpud_p_launch_fir_v2_stream = NULL;
static gpud_launch_fir_fn gpud_p_launch_fir = NULL;
static gpud_launch_decimate_stream_fn gpud_p_launch_decimate_stream = NULL;
static gpud_launch_decimate_fn gpud_p_launch_decimate = NULL;
static gpud_launch_am_envelope_fn gpud_p_launch_am_envelope = NULL;
static gpud_launch_ssb_product_fn gpud_p_launch_ssb_product = NULL;
static gpud_launch_streaming_polyphase_prepare_fn gpud_p_launch_streaming_polyphase_prepare = NULL;
static gpud_launch_streaming_polyphase_stateful_fn gpud_p_launch_streaming_polyphase_stateful = NULL;

static int gpud_cuda_malloc(void **ptr, size_t bytes) { return (int)cudaMalloc(ptr, bytes); }
static int gpud_cuda_free(void *ptr) { return (int)cudaFree(ptr); }
static int gpud_memcpy_h2d(void *dst, const void *src, size_t bytes) { return (int)cudaMemcpy(dst, src, bytes, cudaMemcpyHostToDevice); }
static int gpud_memcpy_d2h(void *dst, const void *src, size_t bytes) { return (int)cudaMemcpy(dst, src, bytes, cudaMemcpyDeviceToHost); }
static int gpud_device_sync() { return (int)cudaDeviceSynchronize(); }

static int gpud_load_library(const char* path) {
	if (gpud_mod != NULL) return 0;
	gpud_mod = LoadLibraryA(path);
	if (gpud_mod == NULL) return -1;
	gpud_p_stream_create = (gpud_stream_create_fn)GetProcAddress(gpud_mod, "gpud_stream_create");
	gpud_p_stream_destroy = (gpud_stream_destroy_fn)GetProcAddress(gpud_mod, "gpud_stream_destroy");
	gpud_p_stream_sync = (gpud_stream_sync_fn)GetProcAddress(gpud_mod, "gpud_stream_sync");
	gpud_p_upload_fir_taps = (gpud_upload_fir_taps_fn)GetProcAddress(gpud_mod, "gpud_upload_fir_taps_cuda");
	gpud_p_launch_freq_shift_stream = (gpud_launch_freq_shift_stream_fn)GetProcAddress(gpud_mod, "gpud_launch_freq_shift_stream_cuda");
	gpud_p_launch_freq_shift = (gpud_launch_freq_shift_fn)GetProcAddress(gpud_mod, "gpud_launch_freq_shift_cuda");
	gpud_p_launch_fm_discrim = (gpud_launch_fm_discrim_fn)GetProcAddress(gpud_mod, "gpud_launch_fm_discrim_cuda");
	gpud_p_launch_fir_stream = (gpud_launch_fir_stream_fn)GetProcAddress(gpud_mod, "gpud_launch_fir_stream_cuda");
	gpud_p_launch_fir_v2_stream = (gpud_launch_fir_v2_stream_fn)GetProcAddress(gpud_mod, "gpud_launch_fir_v2_stream_cuda");
	gpud_p_launch_fir = (gpud_launch_fir_fn)GetProcAddress(gpud_mod, "gpud_launch_fir_cuda");
	gpud_p_launch_decimate_stream = (gpud_launch_decimate_stream_fn)GetProcAddress(gpud_mod, "gpud_launch_decimate_stream_cuda");
	gpud_p_launch_decimate = (gpud_launch_decimate_fn)GetProcAddress(gpud_mod, "gpud_launch_decimate_cuda");
	gpud_p_launch_am_envelope = (gpud_launch_am_envelope_fn)GetProcAddress(gpud_mod, "gpud_launch_am_envelope_cuda");
	gpud_p_launch_ssb_product = (gpud_launch_ssb_product_fn)GetProcAddress(gpud_mod, "gpud_launch_ssb_product_cuda");
	gpud_p_launch_streaming_polyphase_prepare = (gpud_launch_streaming_polyphase_prepare_fn)GetProcAddress(gpud_mod, "gpud_launch_streaming_polyphase_prepare_cuda");
	gpud_p_launch_streaming_polyphase_stateful = (gpud_launch_streaming_polyphase_stateful_fn)GetProcAddress(gpud_mod, "gpud_launch_streaming_polyphase_stateful_cuda");
	if (!gpud_p_stream_create || !gpud_p_stream_destroy || !gpud_p_stream_sync || !gpud_p_upload_fir_taps || !gpud_p_launch_freq_shift_stream || !gpud_p_launch_freq_shift || !gpud_p_launch_fm_discrim || !gpud_p_launch_fir_stream || !gpud_p_launch_fir || !gpud_p_launch_decimate_stream || !gpud_p_launch_decimate || !gpud_p_launch_am_envelope || !gpud_p_launch_ssb_product) {
		FreeLibrary(gpud_mod);
		gpud_mod = NULL;
		return -2;
	}
	return 0;
}

static int gpud_stream_create(gpud_stream_handle* out) { if (!gpud_p_stream_create) return -1; return gpud_p_stream_create(out); }
static int gpud_stream_destroy(gpud_stream_handle stream) { if (!gpud_p_stream_destroy) return -1; return gpud_p_stream_destroy(stream); }
static int gpud_stream_sync(gpud_stream_handle stream) { if (!gpud_p_stream_sync) return -1; return gpud_p_stream_sync(stream); }
static int gpud_upload_fir_taps(const float* taps, int n) { if (!gpud_p_upload_fir_taps) return -1; return gpud_p_upload_fir_taps(taps, n); }
static int gpud_launch_freq_shift_stream(gpud_float2 *in, gpud_float2 *out, int n, double phase_inc, double phase_start, gpud_stream_handle stream) { if (!gpud_p_launch_freq_shift_stream) return -1; return gpud_p_launch_freq_shift_stream(in, out, n, phase_inc, phase_start, stream); }
static int gpud_launch_freq_shift(gpud_float2 *in, gpud_float2 *out, int n, double phase_inc, double phase_start) { if (!gpud_p_launch_freq_shift) return -1; return gpud_p_launch_freq_shift(in, out, n, phase_inc, phase_start); }
static int gpud_launch_fm_discrim(gpud_float2 *in, float *out, int n) { if (!gpud_p_launch_fm_discrim) return -1; return gpud_p_launch_fm_discrim(in, out, n); }
static int gpud_launch_fir_stream(gpud_float2 *in, gpud_float2 *out, int n, int num_taps, gpud_stream_handle stream) { if (!gpud_p_launch_fir_stream) return -1; return gpud_p_launch_fir_stream(in, out, n, num_taps, stream); }
static int gpud_launch_fir_v2_stream(gpud_float2 *in, gpud_float2 *out, const float *taps, int n, int num_taps, gpud_stream_handle stream) { if (!gpud_p_launch_fir_v2_stream) return -1; return gpud_p_launch_fir_v2_stream(in, out, taps, n, num_taps, stream); }
static int gpud_launch_fir(gpud_float2 *in, gpud_float2 *out, int n, int num_taps) { if (!gpud_p_launch_fir) return -1; return gpud_p_launch_fir(in, out, n, num_taps); }
static int gpud_launch_decimate_stream(gpud_float2 *in, gpud_float2 *out, int n_out, int factor, gpud_stream_handle stream) { if (!gpud_p_launch_decimate_stream) return -1; return gpud_p_launch_decimate_stream(in, out, n_out, factor, stream); }
static int gpud_launch_decimate(gpud_float2 *in, gpud_float2 *out, int n_out, int factor) { if (!gpud_p_launch_decimate) return -1; return gpud_p_launch_decimate(in, out, n_out, factor); }
static int gpud_launch_am_envelope(gpud_float2 *in, float *out, int n) { if (!gpud_p_launch_am_envelope) return -1; return gpud_p_launch_am_envelope(in, out, n); }
static int gpud_launch_ssb_product(gpud_float2 *in, float *out, int n, double phase_inc, double phase_start) { if (!gpud_p_launch_ssb_product) return -1; return gpud_p_launch_ssb_product(in, out, n, phase_inc, phase_start); }
static int gpud_launch_streaming_polyphase_prepare(gpud_float2 *in_new, int n_new, gpud_float2 *history_in, int history_len, float *polyphase_taps, int polyphase_len, int decim, int num_taps, int phase_count_in, double phase_start, double phase_inc, gpud_float2 *out, int *n_out, int *phase_count_out, double *phase_end_out, gpud_float2 *history_out) { if (!gpud_p_launch_streaming_polyphase_prepare) return -1; return gpud_p_launch_streaming_polyphase_prepare(in_new, n_new, history_in, history_len, polyphase_taps, polyphase_len, decim, num_taps, phase_count_in, phase_start, phase_inc, out, n_out, phase_count_out, phase_end_out, history_out); }
static int gpud_launch_streaming_polyphase_stateful(gpud_float2 *in_new, int n_new, gpud_float2 *shifted_new_tmp, float *polyphase_taps, int polyphase_len, int decim, int num_taps, gpud_float2 *history_state, gpud_float2 *history_scratch, int history_cap, int *history_len_io, int *phase_count_state, double *phase_state, double phase_inc, gpud_float2 *out, int out_cap, int *n_out) { if (!gpud_p_launch_streaming_polyphase_stateful) return -1; return gpud_p_launch_streaming_polyphase_stateful(in_new, n_new, shifted_new_tmp, polyphase_taps, polyphase_len, decim, num_taps, history_state, history_scratch, history_cap, history_len_io, phase_count_state, phase_state, phase_inc, out, out_cap, n_out); }
*/
import "C"

import "unsafe"

type streamHandle = C.gpud_stream_handle

type gpuFloat2 = C.gpud_float2

func bridgeLoadLibrary(path string) int {
	cp := C.CString(path)
	defer C.free(unsafe.Pointer(cp))
	return int(C.gpud_load_library(cp))
}
func bridgeCudaMalloc(ptr *unsafe.Pointer, bytes uintptr) int {
	return int(C.gpud_cuda_malloc(ptr, C.size_t(bytes)))
}
func bridgeCudaFree(ptr unsafe.Pointer) int { return int(C.gpud_cuda_free(ptr)) }
func bridgeMemcpyH2D(dst unsafe.Pointer, src unsafe.Pointer, bytes uintptr) int {
	return int(C.gpud_memcpy_h2d(dst, src, C.size_t(bytes)))
}
func bridgeMemcpyD2H(dst unsafe.Pointer, src unsafe.Pointer, bytes uintptr) int {
	return int(C.gpud_memcpy_d2h(dst, src, C.size_t(bytes)))
}
func bridgeDeviceSync() int { return int(C.gpud_device_sync()) }
func bridgeUploadFIRTaps(taps *C.float, n int) int {
	return int(C.gpud_upload_fir_taps(taps, C.int(n)))
}
func bridgeLaunchFreqShift(in *C.gpud_float2, out *C.gpud_float2, n int, phaseInc float64, phaseStart float64) int {
	return int(C.gpud_launch_freq_shift(in, out, C.int(n), C.double(phaseInc), C.double(phaseStart)))
}
func bridgeLaunchFreqShiftStream(in *C.gpud_float2, out *C.gpud_float2, n int, phaseInc float64, phaseStart float64, stream streamHandle) int {
	return int(C.gpud_launch_freq_shift_stream(in, out, C.int(n), C.double(phaseInc), C.double(phaseStart), C.gpud_stream_handle(stream)))
}
func bridgeLaunchFIR(in *C.gpud_float2, out *C.gpud_float2, n int, numTaps int) int {
	return int(C.gpud_launch_fir(in, out, C.int(n), C.int(numTaps)))
}
func bridgeLaunchFIRStream(in *C.gpud_float2, out *C.gpud_float2, n int, numTaps int, stream streamHandle) int {
	return int(C.gpud_launch_fir_stream(in, out, C.int(n), C.int(numTaps), C.gpud_stream_handle(stream)))
}
func bridgeLaunchFIRv2Stream(in *C.gpud_float2, out *C.gpud_float2, taps *C.float, n int, numTaps int, stream streamHandle) int {
	return int(C.gpud_launch_fir_v2_stream(in, out, taps, C.int(n), C.int(numTaps), C.gpud_stream_handle(stream)))
}
func bridgeLaunchDecimate(in *C.gpud_float2, out *C.gpud_float2, nOut int, factor int) int {
	return int(C.gpud_launch_decimate(in, out, C.int(nOut), C.int(factor)))
}
func bridgeLaunchDecimateStream(in *C.gpud_float2, out *C.gpud_float2, nOut int, factor int, stream streamHandle) int {
	return int(C.gpud_launch_decimate_stream(in, out, C.int(nOut), C.int(factor), C.gpud_stream_handle(stream)))
}
func bridgeLaunchFMDiscrim(in *C.gpud_float2, out *C.float, n int) int {
	return int(C.gpud_launch_fm_discrim(in, out, C.int(n)))
}
func bridgeLaunchAMEnvelope(in *C.gpud_float2, out *C.float, n int) int {
	return int(C.gpud_launch_am_envelope(in, out, C.int(n)))
}
func bridgeLaunchSSBProduct(in *C.gpud_float2, out *C.float, n int, phaseInc float64, phaseStart float64) int {
	return int(C.gpud_launch_ssb_product(in, out, C.int(n), C.double(phaseInc), C.double(phaseStart)))
}

// bridgeLaunchStreamingPolyphasePrepare is a transitional bridge for the
// legacy single-call prepare path. The stateful native path uses
// bridgeLaunchStreamingPolyphaseStateful.
func bridgeLaunchStreamingPolyphasePrepare(inNew *C.gpud_float2, nNew int, historyIn *C.gpud_float2, historyLen int, polyphaseTaps *C.float, polyphaseLen int, decim int, numTaps int, phaseCountIn int, phaseStart float64, phaseInc float64, out *C.gpud_float2, nOut *C.int, phaseCountOut *C.int, phaseEndOut *C.double, historyOut *C.gpud_float2) int {
	return int(C.gpud_launch_streaming_polyphase_prepare(inNew, C.int(nNew), historyIn, C.int(historyLen), polyphaseTaps, C.int(polyphaseLen), C.int(decim), C.int(numTaps), C.int(phaseCountIn), C.double(phaseStart), C.double(phaseInc), out, nOut, phaseCountOut, phaseEndOut, historyOut))
}
func bridgeLaunchStreamingPolyphaseStateful(inNew *C.gpud_float2, nNew int, shiftedNewTmp *C.gpud_float2, polyphaseTaps *C.float, polyphaseLen int, decim int, numTaps int, historyState *C.gpud_float2, historyScratch *C.gpud_float2, historyCap int, historyLenIO *C.int, phaseCountState *C.int, phaseState *C.double, phaseInc float64, out *C.gpud_float2, outCap int, nOut *C.int) int {
	return int(C.gpud_launch_streaming_polyphase_stateful(inNew, C.int(nNew), shiftedNewTmp, polyphaseTaps, C.int(polyphaseLen), C.int(decim), C.int(numTaps), historyState, historyScratch, C.int(historyCap), historyLenIO, phaseCountState, phaseState, C.double(phaseInc), out, C.int(outCap), nOut))
}
func bridgeStreamCreate() (streamHandle, int) {
	var s C.gpud_stream_handle
	res := int(C.gpud_stream_create(&s))
	return streamHandle(s), res
}
func bridgeStreamDestroy(stream streamHandle) int {
	return int(C.gpud_stream_destroy(C.gpud_stream_handle(stream)))
}
func bridgeStreamSync(stream streamHandle) int {
	return int(C.gpud_stream_sync(C.gpud_stream_handle(stream)))
}
