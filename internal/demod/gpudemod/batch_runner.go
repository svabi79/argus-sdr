package gpudemod

type BatchRunner struct {
	eng *Engine
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
	r.eng.Close()
	r.eng = nil
}

func (r *BatchRunner) ShiftFilterDecimateBatch(iq []complex64, jobs []ExtractJob) ([][]complex64, []int, error) {
	if r == nil || r.eng == nil {
		return nil, nil, ErrUnavailable
	}
	return r.eng.ShiftFilterDecimateBatch(iq, jobs)
}
