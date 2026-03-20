package gpudemod

type batchSlot struct {
	job    ExtractJob
	out    []complex64
	rate   int
	active bool
}

type BatchRunner struct {
	eng      *Engine
	slots    []batchSlot
	slotBufs []slotBuffers
}

func NewBatchRunner(maxSamples int, sampleRate int) (*BatchRunner, error) {
	eng, err := New(maxSamples, sampleRate)
	if err != nil {
		return nil, err
	}
	return &BatchRunner{eng: eng}, nil
}

func (r *BatchRunner) Close() {
	if r == nil || r.eng == nil {
		return
	}
	r.freeSlotBuffers()
	r.eng.Close()
	r.eng = nil
	r.slots = nil
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
