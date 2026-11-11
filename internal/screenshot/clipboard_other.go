//go:build !darwin && !windows && !linux

package screenshot

import "fmt"

// copyImageFile is unsupported on other platforms.
func copyImageFile(_ string) error {
	return fmt.Errorf("clipboard image copy is not supported on this platform")
}
