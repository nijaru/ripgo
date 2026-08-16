//go:build !aix && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris

package fsref

import "fmt"

// mmapHelper provides the Ref contract on platforms without syscall mmap by
// reading the file into memory. Callers still receive an explicit cleanup
// function, but there is no mapped region to release.
func mmapHelper(r Ref) ([]byte, func() error, error) {
	info := r.Info()
	if info == nil {
		return nil, nil, fmt.Errorf("cannot mmap: file info not available")
	}
	if info.Size() == 0 {
		return nil, nil, fmt.Errorf("cannot mmap empty file")
	}

	data, err := r.ReadFile()
	if err != nil {
		return nil, nil, err
	}
	return data, func() error { return nil }, nil
}
