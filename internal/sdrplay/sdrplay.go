//go:build sdrplay

package sdrplay

/*
#cgo windows LDFLAGS: -lsdrplay_api
#cgo linux LDFLAGS: -lsdrplay_api
#include "sdrplay_api.h"
#include <stdlib.h>

extern void goStreamCallback(short *xi, short *xq, unsigned int numSamples, void *cbContext);

static void StreamACallback(short *xi, short *xq, sdrplay_api_StreamCbParamsT *params, unsigned int numSamples, unsigned int reset, void *cbContext) {
	(void)params;
	(void)reset;
	goStreamCallback(xi, xq, numSamples, cbContext);
}

static void EventCallback(sdrplay_api_EventT eventId, sdrplay_api_TunerSelectT tuner, sdrplay_api_EventParamsT *params, void *cbContext) {
	(void)eventId; (void)tuner; (void)params; (void)cbContext;
}

static void sdrplay_set_fs(sdrplay_api_DeviceParamsT *p, double fsHz) {
	if (p && p->devParams) p->devParams->fsFreq.fsHz = fsHz;
}

static void sdrplay_set_rf(sdrplay_api_DeviceParamsT *p, double rfHz) {
	if (p && p->rxChannelA) p->rxChannelA->tunerParams.rfFreq.rfHz = rfHz;
}

static void sdrplay_set_gain(sdrplay_api_DeviceParamsT *p, unsigned int grDb) {
	if (p && p->rxChannelA) p->rxChannelA->tunerParams.gain.gRdB = grDb;
}

static void sdrplay_set_if_zero(sdrplay_api_DeviceParamsT *p) {
	if (p && p->rxChannelA) p->rxChannelA->tunerParams.ifType = sdrplay_api_IF_Zero;
}

static void sdrplay_disable_agc(sdrplay_api_DeviceParamsT *p) {
	if (p && p->rxChannelA) p->rxChannelA->ctrlParams.agc.enable = sdrplay_api_AGC_DISABLE;
}
*/
import "C"

import (
	"errors"
	"fmt"
	"runtime/cgo"
	"sync"
	"unsafe"

	"sdr-visual-suite/internal/sdr"
)

type Source struct {
	mu     sync.Mutex
	dev    C.sdrplay_api_DeviceT
	params *C.sdrplay_api_DeviceParamsT
	ch     chan []complex64
	handle cgo.Handle
	open   bool
}

func New(sampleRate int, centerHz float64, gainDb float64) (sdr.Source, error) {
	s := &Source{
		ch: make(chan []complex64, 16),
	}
	s.handle = cgo.NewHandle(s)
	return s, s.configure(sampleRate, centerHz, gainDb)
}

func (s *Source) configure(sampleRate int, centerHz float64, gainDb float64) error {
	if err := cErr(C.sdrplay_api_Open()); err != nil {
		return fmt.Errorf("sdrplay_api_Open: %w", err)
	}
	s.open = true

	var numDevs C.uint
	var devices [8]C.sdrplay_api_DeviceT
	if err := cErr(C.sdrplay_api_GetDevices(&devices[0], &numDevs, C.uint(len(devices)))); err != nil {
		return fmt.Errorf("sdrplay_api_GetDevices: %w", err)
	}
	if numDevs == 0 {
		return errors.New("no SDRplay devices found")
	}
	s.dev = devices[0]
	if err := cErr(C.sdrplay_api_SelectDevice(&s.dev)); err != nil {
		return fmt.Errorf("sdrplay_api_SelectDevice: %w", err)
	}

	var params *C.sdrplay_api_DeviceParamsT
	if err := cErr(C.sdrplay_api_GetDeviceParams(s.dev.dev, &params)); err != nil {
		return fmt.Errorf("sdrplay_api_GetDeviceParams: %w", err)
	}
	s.params = params
	C.sdrplay_set_fs(s.params, C.double(sampleRate))
	C.sdrplay_set_rf(s.params, C.double(centerHz))
	C.sdrplay_set_gain(s.params, C.uint(gainDb))
	C.sdrplay_set_if_zero(s.params)
	C.sdrplay_disable_agc(s.params)

	cb := C.sdrplay_api_CallbackFnsT{}
	cb.StreamACbFn = (C.sdrplay_api_StreamCallback_t)(unsafe.Pointer(C.StreamACallback))
	cb.StreamBCbFn = nil
	cb.EventCbFn = (C.sdrplay_api_EventCallback_t)(unsafe.Pointer(C.EventCallback))

	if err := cErr(C.sdrplay_api_Init(s.dev.dev, &cb, unsafe.Pointer(uintptr(s.handle)))); err != nil {
		return fmt.Errorf("sdrplay_api_Init: %w", err)
	}
	return nil
}

func (s *Source) Start() error { return nil }

func (s *Source) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.params != nil {
		_ = cErr(C.sdrplay_api_Uninit(s.dev.dev))
		s.params = nil
	}
	if s.open {
		_ = cErr(C.sdrplay_api_ReleaseDevice(&s.dev))
		_ = cErr(C.sdrplay_api_Close())
		s.open = false
	}
	if s.handle != 0 {
		s.handle.Delete()
		s.handle = 0
	}
	return nil
}

func (s *Source) ReadIQ(n int) ([]complex64, error) {
	buf := <-s.ch
	if len(buf) >= n {
		return buf[:n], nil
	}
	return buf, nil
}

//export goStreamCallback
func goStreamCallback(xi *C.short, xq *C.short, numSamples C.uint, ctx unsafe.Pointer) {
	h := cgo.Handle(uintptr(ctx))
	src, ok := h.Value().(*Source)
	if !ok || src == nil {
		return
	}
	n := int(numSamples)
	if n <= 0 {
		return
	}
	iq := make([]complex64, n)
	xiSlice := unsafe.Slice((*int16)(unsafe.Pointer(xi)), n)
	xqSlice := unsafe.Slice((*int16)(unsafe.Pointer(xq)), n)
	const scale = 1.0 / 32768.0
	for i := 0; i < n; i++ {
		re := float32(float64(xiSlice[i]) * scale)
		im := float32(float64(xqSlice[i]) * scale)
		iq[i] = complex(re, im)
	}
	select {
	case src.ch <- iq:
	default:
		// Drop if consumer is slow.
	}
}

func cErr(err C.sdrplay_api_ErrT) error {
	if err == C.sdrplay_api_Success {
		return nil
	}
	return errors.New(C.GoString(C.sdrplay_api_GetErrorString(err)))
}
