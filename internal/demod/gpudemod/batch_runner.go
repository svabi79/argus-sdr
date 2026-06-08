package gpudemod

import "math"

type batchSlot struct {
	job    ExtractJob
	out    []complex64
	rate   int
	active bool
}

type BatchRunner struct {
	eng         *Engine
	slots       []batchSlot
	slotBufs    []slotBuffers
	slotBufSize int // number of IQ samples the slot buffers were allocated for
	streamState map[int64]*ExtractStreamState
	nativeState map[int64]*nativeStreamingSignalState
	outRings    map[int64]*streamOutRing // reused per-signal output buffers (#20)
	outPool     outBufPool               // cross-signal free list, recycled on prune
}

func NewBatchRunner(maxSamples int, sampleRate int) (*BatchRunner, error) {
	eng, err := New(maxSamples, sampleRate)
	if err != nil {
		return nil, err
	}
	return &BatchRunner{
		eng:         eng,
		streamState: make(map[int64]*ExtractStreamState),
		nativeState: make(map[int64]*nativeStreamingSignalState),
	}, nil
}

func (r *BatchRunner) Close() {
	if r == nil || r.eng == nil {
		return
	}
	r.freeSlotBuffers()
	r.freeAllNativeStreamingStates()
	r.eng.Close()
	r.eng = nil
	r.slots = nil
	r.streamState = nil
	r.nativeState = nil
	r.outRings = nil
	r.outPool = outBufPool{}
}

func (r *BatchRunner) prepare(jobs []ExtractJob) {
	if cap(r.slots) < len(jobs) {
		r.slots = make([]batchSlot, len(jobs))
	} else {
		r.slots = r.slots[:len(jobs)]
	}
	for i, job := range jobs {
		r.slots[i] = batchSlot{job: job, active: true}
	}
}

func (r *BatchRunner) ShiftFilterDecimateBatch(iq []complex64, jobs []ExtractJob) ([][]complex64, []int, error) {
	if r == nil || r.eng == nil {
		return nil, nil, ErrUnavailable
	}
	r.prepare(jobs)
	return r.shiftFilterDecimateBatchImpl(iq)
}

// ShiftFilterDecimateBatchWithPhase uses per-job PhaseStart and returns
// per-job PhaseEnd for phase-continuous streaming.
func (r *BatchRunner) ShiftFilterDecimateBatchWithPhase(iq []complex64, jobs []ExtractJob) ([]ExtractResult, error) {
	if r == nil || r.eng == nil {
		return nil, ErrUnavailable
	}
	r.prepare(jobs)
	outs, rates, err := r.shiftFilterDecimateBatchImpl(iq)
	if err != nil {
		return nil, err
	}
	results := make([]ExtractResult, len(jobs))
	for i, job := range jobs {
		phaseInc := -2.0 * math.Pi * job.OffsetHz / float64(r.eng.sampleRate)
		var iq_out []complex64
		var rate int
		if i < len(outs) {
			iq_out = outs[i]
		}
		if i < len(rates) {
			rate = rates[i]
		}
		results[i] = ExtractResult{
			IQ:       iq_out,
			Rate:     rate,
			PhaseEnd: job.PhaseStart + phaseInc*float64(len(iq)),
		}
	}
	return results, nil
}
