//go:build !windows

package updater

import (
	"fmt"
	"os"
)

func replaceBinary(oldPath, newPath string) error {
	if err := os.Rename(newPath, oldPath); err != nil {
		return fmt.Errorf("failed to replace binary: %w", err)
	}
	return nil
}
