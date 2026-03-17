package filetype

import (
	"bytes"
	"os"
)

var nullBytes = []byte{0}

func IsBinary(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	buf := make([]byte, 8192)
	n, err := f.Read(buf)
	if err != nil {
		return false
	}

	return bytes.Contains(buf[:n], nullBytes)
}
