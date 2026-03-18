package recorder

import (
	"encoding/binary"
	"os"
)

func writeCF32(path string, samples []complex64) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	for _, v := range samples {
		if err := binary.Write(f, binary.LittleEndian, real(v)); err != nil {
			return err
		}
		if err := binary.Write(f, binary.LittleEndian, imag(v)); err != nil {
			return err
		}
	}
	return nil
}
