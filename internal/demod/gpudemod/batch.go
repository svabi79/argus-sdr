package gpudemod

type ExtractJob struct {
	OffsetHz float64
	BW       float64
	OutRate  int
}

func (e *Engine) ShiftFilterDecimateBatch(iq []complex64, jobs []ExtractJob) ([][]complex64, []int, error) {
	outs := make([][]complex64, len(jobs))
	rates := make([]int, len(jobs))
	for i, job := range jobs {
		out, rate, err := e.ShiftFilterDecimate(iq, job.OffsetHz, job.BW, job.OutRate)
		if err != nil {
			return nil, nil, err
		}
		outs[i] = out
		rates[i] = rate
	}
	return outs, rates, nil
}
