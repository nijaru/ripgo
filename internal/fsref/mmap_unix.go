//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package fsref

import (
	"fmt"
	"syscall"
)

func mmapHelper(r Ref) ([]byte, func() error, error) {
	info := r.Info()
	if info == nil {
		return nil, nil, fmt.Errorf("cannot mmap: file info not available")
	}
	if info.Size() == 0 {
		return nil, nil, fmt.Errorf("cannot mmap empty file")
	}

	file, err := r.Open()
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()

	data, err := syscall.Mmap(int(file.Fd()), 0, int(info.Size()), syscall.PROT_READ, syscall.MAP_SHARED)
	if err != nil {
		return nil, nil, err
	}

	unmap := func() error {
		return syscall.Munmap(data)
	}

	return data, unmap, nil
}
