package recorder

import (
	"bufio"
	"os"
	"unsafe"
)

func writeCF32(path string, samples []complex64) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if len(samples) == 0 {
		return nil
	}
	w := bufio.NewWriterSize(f, 1<<20)
	defer w.Flush()
	b := unsafe.Slice((*byte)(unsafe.Pointer(&samples[0])), len(samples)*8)
	_, err = w.Write(b)
	return err
}
