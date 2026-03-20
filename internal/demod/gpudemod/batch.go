package gpudemod

import "math"

type ExtractJob struct {
	OffsetHz   float64
	BW         float64
	OutRate    int
	PhaseStart float64 // FreqShift starting phase (0 for stateless, carry over for streaming)
}

// ExtractResult holds the output of a batch extraction including the ending
// phase of the FreqShift oscillator for phase-continuous streaming.
type ExtractResult struct {
	IQ       []complex64
	Rate     int
	PhaseEnd float64 // FreqShift phase at end of this block — pass as PhaseStart next frame
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

// ShiftFilterDecimateBatchWithPhase is like ShiftFilterDecimateBatch but uses
// per-job PhaseStart and returns per-job PhaseEnd for phase-continuous streaming.
func (e *Engine) ShiftFilterDecimateBatchWithPhase(iq []complex64, jobs []ExtractJob) ([]ExtractResult, error) {
	results := make([]ExtractResult, len(jobs))
	for i, job := range jobs {
		out, rate, err := e.ShiftFilterDecimate(iq, job.OffsetHz, job.BW, job.OutRate)
		if err != nil {
			return nil, err
		}
		phaseInc := -2.0 * math.Pi * job.OffsetHz / float64(e.sampleRate)
		results[i] = ExtractResult{
			IQ:       out,
			Rate:     rate,
			PhaseEnd: job.PhaseStart + phaseInc*float64(len(iq)),
		}
	}
	return results, nil
}
