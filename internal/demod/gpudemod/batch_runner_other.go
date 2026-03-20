//go:build !windows || !cufft

package gpudemod

// slotBuffers stub for non-Windows platforms
type slotBuffers struct{}

func (r *BatchRunner) freeSlotBuffers() {
	r.slotBufs = nil
}

func (r *BatchRunner) shiftFilterDecimateBatchImpl(iq []complex64) ([][]complex64, []int, error) {
	outs := make([][]complex64, len(r.slots))
	rates := make([]int, len(r.slots))
	for i := range r.slots {
		if !r.slots[i].active {
			continue
		}
		out, rate, err := r.eng.ShiftFilterDecimate(iq, r.slots[i].job.OffsetHz, r.slots[i].job.BW, r.slots[i].job.OutRate)
		if err != nil {
			return nil, nil, err
		}
		r.slots[i].out = out
		r.slots[i].rate = rate
		outs[i] = out
		rates[i] = rate
	}
	return outs, rates, nil
}
