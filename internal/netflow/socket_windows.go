package netflow

import (
	"errors"
	"syscall"
)

// Winsock uses WSAECONNREFUSED (10061), not Go's synthetic POSIX errno.
func connectionRefused(err error) bool {
	return errors.Is(err, syscall.Errno(10061)) || errors.Is(err, syscall.ECONNREFUSED)
}
