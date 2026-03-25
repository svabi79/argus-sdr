package gpudemod

import "unsafe"

type nativeStreamingSignalState struct {
	signalID int64

	configHash uint64
	decim      int
	numTaps    int

	dInNew          unsafe.Pointer
	dShifted        unsafe.Pointer
	dOut            unsafe.Pointer
	dTaps           unsafe.Pointer
	dHistory        unsafe.Pointer
	dHistoryScratch unsafe.Pointer

	inNewCap          int
	shiftedCap        int
	outCap            int
	tapsLen           int
	historyCap        int
	historyLen        int
	historyScratchCap int
	phaseCount        int
	phaseNCO          float64
}
