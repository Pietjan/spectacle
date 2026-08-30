//go:build !windows && !linux

package spectacle

import "fmt"

// New is not available on this platform.
func New(cfg Config) (Backend, error) {
	return nil, fmt.Errorf("spectacle: unsupported platform (build with GOOS=windows or GOOS=linux)")
}
