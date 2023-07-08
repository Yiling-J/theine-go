// +build solaris,illumos,plan9,openbsd

package directio

import (
	"os"
	"syscall"
)

const (
	// Size to align the buffer to
	AlignSize = 4096

	// Minimum block size
	BlockSize = 4096
)

// OpenFile just call os.OpenFile with same params
func OpenFile(name string, flag int, perm os.FileMode) (file *os.File, err error) {
	return os.OpenFile(name, flag, perm)
}
