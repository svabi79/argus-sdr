package demod

type Demodulator interface {
	Name() string
	Demod(iq []complex64, sampleRate int) []float32
	OutputSampleRate() int
}

var registry = map[string]Demodulator{}

func Register(d Demodulator) {
	registry[d.Name()] = d
}

func Get(name string) Demodulator {
	return registry[name]
}

func Names() []string {
	out := make([]string, 0, len(registry))
	for k := range registry {
		out = append(out, k)
	}
	return out
}
