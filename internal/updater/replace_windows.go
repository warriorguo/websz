//go:build windows

package updater

import (
	"fmt"
	"os"
)

func replaceBinary(oldPath, newPath string) error {
	backupPath := oldPath + ".old"
	os.Remove(backupPath) // ignore error, may not exist

	if err := os.Rename(oldPath, backupPath); err != nil {
		return fmt.Errorf("failed to backup old binary: %w", err)
	}
	if err := os.Rename(newPath, oldPath); err != nil {
		// Try to restore backup
		os.Rename(backupPath, oldPath)
		return fmt.Errorf("failed to replace binary: %w", err)
	}
	os.Remove(backupPath) // best-effort cleanup
	return nil
}
